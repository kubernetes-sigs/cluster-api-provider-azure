/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package securitygroups

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/securitygroups/mock_securitygroups"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

var (
	annotation = azure.SecurityRuleLastAppliedAnnotation
	fakeNSG    = NSGSpec{
		Name:        "test-nsg",
		Location:    "test-location",
		ClusterName: "my-cluster",
		SecurityRules: infrav1.SecurityRules{
			securityRule1,
		},
		ResourceGroup: "test-group",
	}
	noRulesNSG = NSGSpec{
		Name:          "test-nsg-2",
		Location:      "test-location",
		ClusterName:   "my-cluster",
		SecurityRules: infrav1.SecurityRules{},
		ResourceGroup: "test-group",
	}
	multipleRulesNSG = NSGSpec{
		Name:        "multiple-rules-nsg",
		Location:    "test-location",
		ClusterName: "my-cluster",
		SecurityRules: infrav1.SecurityRules{
			securityRule1,
			securityRule2,
		},
		ResourceGroup: "test-group",
	}
	securityRule1 = infrav1.SecurityRule{
		Name:             "allow_ssh",
		Description:      "Allow SSH",
		Priority:         2200,
		Protocol:         infrav1.SecurityGroupProtocolTCP,
		Direction:        infrav1.SecurityRuleDirectionInbound,
		Source:           ptr.To("*"),
		SourcePorts:      ptr.To("*"),
		Destination:      ptr.To("*"),
		DestinationPorts: ptr.To("22"),
		Action:           infrav1.SecurityRuleActionAllow,
	}
	securityRule2 = infrav1.SecurityRule{
		Name:             "other_rule",
		Description:      "Test Rule",
		Priority:         500,
		Protocol:         infrav1.SecurityGroupProtocolTCP,
		Direction:        infrav1.SecurityRuleDirectionInbound,
		Source:           ptr.To("*"),
		SourcePorts:      ptr.To("*"),
		Destination:      ptr.To("*"),
		DestinationPorts: ptr.To("80"),
		Action:           infrav1.SecurityRuleActionAllow,
	}
	errFake      = errors.New("this is an error")
	notDoneError = azure.NewOperationNotDoneError(&infrav1.Future{})
)

func TestReconcileSecurityGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "create single security group with single rule succeeds, should return no error",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG})
				s.UpdateAnnotationJSON(annotation, map[string]any{fakeNSG.Name: map[string]any{securityRule1.Name: securityRule1.Description}}).Times(1)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create single security group with multiple rules succeeds, should return no error",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&multipleRulesNSG})
				s.UpdateAnnotationJSON(annotation, map[string]any{multipleRulesNSG.Name: map[string]any{securityRule1.Name: securityRule1.Description, securityRule2.Name: securityRule2.Description}}).Times(1)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &multipleRulesNSG, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create multiple security groups, should return no error",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG, &noRulesNSG})
				s.UpdateAnnotationJSON(annotation, map[string]any{fakeNSG.Name: map[string]any{securityRule1.Name: securityRule1.Description}}).Times(1)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &noRulesNSG, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "first security groups create fails, should return error",
			expectedError: errFake.Error(),
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG, &noRulesNSG})
				s.UpdateAnnotationJSON(annotation, map[string]any{fakeNSG.Name: map[string]any{securityRule1.Name: securityRule1.Description}}).Times(1)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, errFake)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &noRulesNSG, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "first sg create fails, second sg create not done, should return create error",
			expectedError: errFake.Error(),
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG, &noRulesNSG})
				s.UpdateAnnotationJSON(annotation, map[string]any{fakeNSG.Name: map[string]any{securityRule1.Name: securityRule1.Description}}).Times(1)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, errFake)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &noRulesNSG, serviceName).Return(nil, notDoneError)
				s.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "security groups create not done, should return not done error",
			expectedError: notDoneError.Error(),
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG})
				s.UpdateAnnotationJSON(annotation, map[string]any{fakeNSG.Name: map[string]any{securityRule1.Name: securityRule1.Description}})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, notDoneError)
				s.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, notDoneError)
			},
		},
		{
			name:          "vnet is not managed, should skip reconcile",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(false)
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_securitygroups.NewMockNSGScope(mockCtrl)
			scopeMock.EXPECT().GetLongRunningOperationState(gomock.Any(), serviceName, infrav1.PutFuture).Return(nil).AnyTimes()
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: reconcilerMock,
			}

			err := s.Reconcile(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestReconcileSecurityGroupsPendingRuleRemoval(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	spec := multipleRulesNSG
	priorRules := map[string]any{securityRule1.Name: securityRule1.Description}
	scope := &statefulNSGScope{
		specs:      []*NSGSpec{&spec},
		annotation: map[string]any{spec.Name: priorRules},
		futures:    make(map[[3]string]*infrav1.Future),
	}
	unmanagedRule := converters.SecurityRuleToSDK(securityRule1)
	unmanagedRule.Name = ptr.To("unmanaged-rule")
	client := &fakeNSGCreator{
		t:       t,
		pending: true,
		resource: armnetwork.SecurityGroup{
			Properties: &armnetwork.SecurityGroupPropertiesFormat{
				SecurityRules: []*armnetwork.SecurityRule{converters.SecurityRuleToSDK(securityRule1), unmanagedRule},
			},
		},
	}
	s := &Service{
		Scope:      scope,
		Reconciler: async.New[armnetwork.SecurityGroupsClientCreateOrUpdateResponse, armnetwork.SecurityGroupsClientDeleteResponse](scope, client, nil),
	}

	// An accepted PUT exposes both owned rules while its future is still pending.
	g.Expect(azure.IsOperationNotDoneError(s.Reconcile(t.Context()))).To(BeTrue())
	g.Expect(scope.annotation).To(Equal(map[string]any{spec.Name: map[string]any{
		securityRule1.Name: securityRule1.Description,
		securityRule2.Name: securityRule2.Description,
	}}))
	g.Expect(priorRules).To(Equal(map[string]any{securityRule1.Name: securityRule1.Description}))
	g.Expect(scope.futures).To(HaveLen(1))
	g.Expect(client.resource.Properties.SecurityRules).To(ConsistOf(
		converters.SecurityRuleToSDK(securityRule1), converters.SecurityRuleToSDK(securityRule2), unmanagedRule,
	))

	// Retain ownership while the old PUT is still pending with a changed spec.
	spec.SecurityRules = infrav1.SecurityRules{securityRule2}
	g.Expect(azure.IsOperationNotDoneError(s.Reconcile(t.Context()))).To(BeTrue())
	g.Expect(scope.annotation).To(Equal(map[string]any{spec.Name: map[string]any{
		securityRule1.Name: securityRule1.Description,
		securityRule2.Name: securityRule2.Description,
	}}))
	g.Expect(client.puts).To(Equal(1))
	g.Expect(client.gets).To(Equal(1))
	g.Expect(scope.futures).To(HaveLen(1))

	// Completing the old PUT must evaluate the changed spec and submit the deletion.
	client.complete = true
	g.Expect(azure.IsOperationNotDoneError(s.Reconcile(t.Context()))).To(BeTrue())
	g.Expect(scope.annotation).To(Equal(map[string]any{spec.Name: map[string]any{
		securityRule1.Name: securityRule1.Description,
		securityRule2.Name: securityRule2.Description,
	}}))
	g.Expect(scope.futures).To(HaveLen(1))
	g.Expect(client.puts).To(Equal(2))
	g.Expect(client.gets).To(Equal(2))
	g.Expect(client.resumes).To(Equal(2))
	g.Expect(client.resource.Properties.SecurityRules).To(ConsistOf(converters.SecurityRuleToSDK(securityRule2), unmanagedRule))
	g.Expect(azure.IsOperationNotDoneError(scope.putError)).To(BeTrue())

	// Ownership shrinks only after the deletion completes and the current spec is checked.
	g.Expect(s.Reconcile(t.Context())).To(Succeed())
	g.Expect(scope.annotation).To(Equal(map[string]any{spec.Name: map[string]any{securityRule2.Name: securityRule2.Description}}))
	g.Expect(scope.futures).To(BeEmpty())
	g.Expect(client.puts).To(Equal(2))
	g.Expect(client.gets).To(Equal(3))
	g.Expect(client.resumes).To(Equal(3))
	g.Expect(client.resource.Properties.SecurityRules).To(ConsistOf(converters.SecurityRuleToSDK(securityRule2), unmanagedRule))
	g.Expect(scope.putError).NotTo(HaveOccurred())
	g.Expect(scope.annotationUpdates).To(Equal(4))
}

func TestReconcileSecurityGroupsOwnership(t *testing.T) {
	testcases := []struct {
		name          string
		desiredRules  infrav1.SecurityRules
		existingRules infrav1.SecurityRules
		pending       bool
		resume        bool
		createError   error
		resumeError   error
		expectedRules map[string]any
		expectedGets  int
		expectedPuts  int
	}{
		{
			name:          "pending PUT retains prior and desired ownership",
			desiredRules:  infrav1.SecurityRules{securityRule2},
			existingRules: infrav1.SecurityRules{securityRule1},
			pending:       true,
			expectedRules: map[string]any{securityRule1.Name: securityRule1.Description, securityRule2.Name: securityRule2.Description},
			expectedGets:  1,
			expectedPuts:  1,
		},
		{
			name:          "failed PUT retains prior and desired ownership",
			desiredRules:  infrav1.SecurityRules{securityRule2},
			existingRules: infrav1.SecurityRules{securityRule1},
			createError:   errFake,
			expectedRules: map[string]any{securityRule1.Name: securityRule1.Description, securityRule2.Name: securityRule2.Description},
			expectedGets:  1,
			expectedPuts:  1,
		},
		{
			name:          "failed resumed PUT retains ownership without a fresh call",
			desiredRules:  infrav1.SecurityRules{securityRule2},
			existingRules: infrav1.SecurityRules{securityRule1},
			resume:        true,
			resumeError:   errFake,
			expectedRules: map[string]any{securityRule1.Name: securityRule1.Description, securityRule2.Name: securityRule2.Description},
		},
		{
			name:          "failed current PUT after resume retains ownership",
			desiredRules:  infrav1.SecurityRules{securityRule2},
			existingRules: infrav1.SecurityRules{securityRule1},
			resume:        true,
			createError:   errFake,
			expectedRules: map[string]any{securityRule1.Name: securityRule1.Description, securityRule2.Name: securityRule2.Description},
			expectedGets:  1,
			expectedPuts:  1,
		},
		{
			name:          "successful no-op keeps only desired ownership",
			desiredRules:  infrav1.SecurityRules{securityRule2},
			existingRules: infrav1.SecurityRules{securityRule2},
			expectedRules: map[string]any{securityRule2.Name: securityRule2.Description},
			expectedGets:  1,
		},
		{
			name:          "successful PUT keeps only desired ownership",
			desiredRules:  infrav1.SecurityRules{securityRule2},
			existingRules: infrav1.SecurityRules{securityRule1},
			expectedRules: map[string]any{securityRule2.Name: securityRule2.Description},
			expectedGets:  1,
			expectedPuts:  1,
		},
		{
			name:          "deleting all rules removes NSG ownership",
			existingRules: infrav1.SecurityRules{securityRule1},
			expectedGets:  1,
			expectedPuts:  1,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			spec := fakeNSG
			spec.SecurityRules = tc.desiredRules
			priorRules := map[string]any{securityRule1.Name: securityRule1.Description}
			scope := &statefulNSGScope{
				specs:      []*NSGSpec{&spec},
				annotation: map[string]any{spec.Name: priorRules},
				futures:    make(map[[3]string]*infrav1.Future),
			}
			client := &fakeNSGCreator{
				t:           t,
				resource:    armnetwork.SecurityGroup{Properties: &armnetwork.SecurityGroupPropertiesFormat{}},
				pending:     tc.pending,
				complete:    true,
				createError: tc.createError,
				resumeError: tc.resumeError,
			}
			for _, rule := range tc.existingRules {
				client.resource.Properties.SecurityRules = append(client.resource.Properties.SecurityRules, converters.SecurityRuleToSDK(rule))
			}
			if tc.resume {
				future, err := converters.PollerToFuture(fakeNSGPoller(t), infrav1.PutFuture, serviceName, spec.Name, spec.ResourceGroup)
				g.Expect(err).NotTo(HaveOccurred())
				scope.SetLongRunningOperationState(future)
			}
			s := &Service{
				Scope:      scope,
				Reconciler: async.New[armnetwork.SecurityGroupsClientCreateOrUpdateResponse, armnetwork.SecurityGroupsClientDeleteResponse](scope, client, nil),
			}

			err := s.Reconcile(t.Context())
			switch {
			case tc.pending:
				g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())
				g.Expect(scope.futures).To(HaveLen(1))
			case tc.createError != nil || tc.resumeError != nil:
				g.Expect(err).To(MatchError(ContainSubstring(errFake.Error())))
				g.Expect(scope.futures).To(BeEmpty())
			default:
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(scope.futures).To(BeEmpty())
			}
			expectedAnnotation := make(map[string]any)
			if len(tc.expectedRules) > 0 {
				expectedAnnotation[spec.Name] = tc.expectedRules
			}
			g.Expect(scope.annotation).To(Equal(expectedAnnotation))
			g.Expect(priorRules).To(Equal(map[string]any{securityRule1.Name: securityRule1.Description}))
			g.Expect(scope.annotationUpdates).To(Equal(1))
			if err != nil {
				g.Expect(scope.putError).To(MatchError(err))
			} else {
				g.Expect(scope.putError).NotTo(HaveOccurred())
			}
			g.Expect(client.gets).To(Equal(tc.expectedGets))
			g.Expect(client.puts).To(Equal(tc.expectedPuts))
			if tc.resume {
				g.Expect(client.resumes).To(Equal(1))
			} else {
				g.Expect(client.resumes).To(BeZero())
			}
		})
	}
}

func TestReconcileSecurityGroupsMixedOwnership(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	mockCtrl := gomock.NewController(t)
	r := mock_async.NewMockReconciler(mockCtrl)
	scope := &statefulNSGScope{
		annotation: make(map[string]any),
		futures:    make(map[[3]string]*infrav1.Future),
	}
	expectedAnnotation := make(map[string]any)
	for _, outcome := range []struct {
		name string
		err  error
	}{
		{name: "failed-nsg", err: errFake},
		{name: "pending-nsg", err: notDoneError},
		{name: "successful-nsg"},
	} {
		spec := fakeNSG
		spec.Name = outcome.name
		spec.SecurityRules = infrav1.SecurityRules{securityRule2}
		scope.specs = append(scope.specs, &spec)
		scope.annotation[spec.Name] = map[string]any{securityRule1.Name: securityRule1.Description}
		expectedRules := map[string]any{securityRule2.Name: securityRule2.Description}
		if outcome.err != nil {
			expectedRules[securityRule1.Name] = securityRule1.Description
		}
		expectedAnnotation[spec.Name] = expectedRules
		spec.LastAppliedSecurityRules = scope.annotation[spec.Name].(map[string]any)
		r.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), &spec, serviceName).Return(nil, outcome.err)
	}
	s := &Service{Scope: scope, Reconciler: r}

	g.Expect(s.Reconcile(t.Context())).To(MatchError(errFake))
	g.Expect(scope.putError).To(MatchError(errFake))
	g.Expect(scope.annotation).To(Equal(expectedAnnotation))
	g.Expect(scope.annotationUpdates).To(Equal(1))
}

type statefulNSGScope struct {
	NSGScope
	specs             []*NSGSpec
	annotation        map[string]any
	futures           map[[3]string]*infrav1.Future
	putError          error
	annotationUpdates int
}

func (s *statefulNSGScope) NSGSpecs() []azure.ResourceSpecGetter {
	specs := make([]azure.ResourceSpecGetter, len(s.specs))
	for i, spec := range s.specs {
		specCopy := *spec
		specCopy.LastAppliedSecurityRules, _ = s.annotation[spec.Name].(map[string]any)
		specs[i] = &specCopy
	}
	return specs
}

func (s *statefulNSGScope) IsVnetManaged() bool {
	return true
}

func (s *statefulNSGScope) UpdateAnnotationJSON(_ string, value map[string]any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.annotation = nil
	s.annotationUpdates++
	return json.Unmarshal(data, &s.annotation)
}

func (s *statefulNSGScope) SetLongRunningOperationState(future *infrav1.Future) {
	s.futures[[3]string{future.Name, future.ServiceName, future.Type}] = future
}

func (s *statefulNSGScope) GetLongRunningOperationState(name, service, futureType string) *infrav1.Future {
	return s.futures[[3]string{name, service, futureType}]
}

func (s *statefulNSGScope) DeleteLongRunningOperationState(name, service, futureType string) {
	delete(s.futures, [3]string{name, service, futureType})
}

func (s *statefulNSGScope) DefaultedAzureServiceReconcileTimeout() time.Duration {
	return reconciler.DefaultAzureServiceReconcileTimeout
}

func (s *statefulNSGScope) DefaultedReconcilerRequeue() time.Duration {
	return reconciler.DefaultReconcilerRequeue
}

func (s *statefulNSGScope) UpdatePutStatus(_ clusterv1beta1.ConditionType, _ string, err error) {
	s.putError = err
}

type fakeNSGCreator struct {
	t           *testing.T
	resource    armnetwork.SecurityGroup
	pending     bool
	complete    bool
	createError error
	resumeError error
	gets        int
	puts        int
	resumes     int
}

func (c *fakeNSGCreator) Get(context.Context, azure.ResourceSpecGetter) (any, error) {
	c.gets++
	return c.resource, nil
}

func (c *fakeNSGCreator) CreateOrUpdateAsync(_ context.Context, _ azure.ResourceSpecGetter, resumeToken string, parameters any) (any, *runtime.Poller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse], error) {
	g := NewWithT(c.t)
	if resumeToken != "" {
		g.Expect(parameters).To(BeNil())
		c.resumes++
		if c.resumeError != nil {
			return nil, nil, c.resumeError
		}
		if c.complete {
			return c.resource, nil, nil
		}
	} else {
		c.puts++
		if c.createError != nil {
			return nil, nil, c.createError
		}
		c.resource = parameters.(armnetwork.SecurityGroup)
		if !c.pending {
			return c.resource, nil, nil
		}
	}
	return nil, fakeNSGPoller(c.t), context.DeadlineExceeded
}

func fakeNSGPoller(t *testing.T) *runtime.Poller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse] {
	t.Helper()
	g := NewWithT(t)
	response := &http.Response{
		Body: io.NopCloser(strings.NewReader("")),
		Request: &http.Request{
			Method: http.MethodPut,
			URL:    &url.URL{Path: "/"},
		},
		StatusCode: http.StatusAccepted,
	}
	pipeline := runtime.NewPipeline("testmodule", "v0.1.0", runtime.PipelineOptions{}, nil)
	poller, err := runtime.NewPoller[armnetwork.SecurityGroupsClientCreateOrUpdateResponse](response, pipeline, nil)
	g.Expect(err).NotTo(HaveOccurred())
	return poller
}

func TestDeleteSecurityGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "delete multiple security groups succeeds, should return no error",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG, &noRulesNSG})
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &noRulesNSG, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.SecurityGroupsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "first security groups delete fails, should return an error",
			expectedError: errFake.Error(),
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG, &noRulesNSG})
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(errFake)
				r.DeleteResource(gomockinternal.AContext(), &noRulesNSG, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.SecurityGroupsReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "first security groups delete fails and second security groups create not done, should return an error",
			expectedError: errFake.Error(),
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG, &noRulesNSG})
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(errFake)
				r.DeleteResource(gomockinternal.AContext(), &noRulesNSG, serviceName).Return(notDoneError)
				s.UpdateDeleteStatus(infrav1.SecurityGroupsReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "security groups delete not done, should return not done error",
			expectedError: notDoneError.Error(),
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG})
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(notDoneError)
				s.UpdateDeleteStatus(infrav1.SecurityGroupsReadyCondition, serviceName, notDoneError)
			},
		},
		{
			name:          "vnet is not managed, should skip delete",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(false)
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_securitygroups.NewMockNSGScope(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: reconcilerMock,
			}

			err := s.Delete(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

var (
	ruleA = &armnetwork.SecurityRule{
		Name: ptr.To("A"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Description:              ptr.To("this is rule A"),
			Protocol:                 ptr.To(armnetwork.SecurityRuleProtocolTCP),
			DestinationPortRange:     ptr.To("*"),
			SourcePortRange:          ptr.To("*"),
			DestinationAddressPrefix: ptr.To("*"),
			SourceAddressPrefix:      ptr.To("*"),
			Priority:                 ptr.To[int32](100),
			Direction:                ptr.To(armnetwork.SecurityRuleDirectionInbound),
			Access:                   ptr.To(armnetwork.SecurityRuleAccessAllow),
		},
	}
	ruleB = &armnetwork.SecurityRule{
		Name: ptr.To("B"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Description:              ptr.To("this is rule B"),
			Protocol:                 ptr.To(armnetwork.SecurityRuleProtocolTCP),
			DestinationPortRange:     ptr.To("*"),
			SourcePortRange:          ptr.To("*"),
			DestinationAddressPrefix: ptr.To("*"),
			SourceAddressPrefix:      ptr.To("*"),
			Priority:                 ptr.To[int32](100),
			Direction:                ptr.To(armnetwork.SecurityRuleDirectionOutbound),
			Access:                   ptr.To(armnetwork.SecurityRuleAccessAllow),
		},
	}
	ruleBModified = &armnetwork.SecurityRule{
		Name: ptr.To("B"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Description:              ptr.To("this is rule B"),
			Protocol:                 ptr.To(armnetwork.SecurityRuleProtocolTCP),
			DestinationPortRange:     ptr.To("80"),
			SourcePortRange:          ptr.To("*"),
			DestinationAddressPrefix: ptr.To("*"),
			SourceAddressPrefix:      ptr.To("*"),
			Priority:                 ptr.To[int32](100),
			Direction:                ptr.To(armnetwork.SecurityRuleDirectionOutbound),
			Access:                   ptr.To(armnetwork.SecurityRuleAccessAllow),
		},
	}
)
