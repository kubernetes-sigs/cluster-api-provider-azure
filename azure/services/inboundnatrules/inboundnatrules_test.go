/*
Copyright 2020 The Kubernetes Authors.

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

package inboundnatrules

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/inboundnatrules/mock_inboundnatrules"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeLBName    = "my-lb-1"
	fakeGroupName = "my-rg"

	noExistingRules   = []network.InboundNatRule{}
	fakeExistingRules = []network.InboundNatRule{
		{
			Name: pointer.String("other-machine-nat-rule"),
			ID:   pointer.String("some-natrules-id"),
			InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
				FrontendPort: pointer.Int32(22),
			},
		},
		{
			Name: pointer.String("other-machine-nat-rule-2"),
			ID:   pointer.String("some-natrules-id-2"),
			InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
				FrontendPort: pointer.Int32(2201),
			},
		},
	}

	fakeNatSpec = InboundNatSpec{
		Name:                      "my-machine-1",
		LoadBalancerName:          "my-lb-1",
		ResourceGroup:             fakeGroupName,
		FrontendIPConfigurationID: pointer.String("frontend-ip-config-id-1"),
	}
	fakeNatSpec2 = InboundNatSpec{
		Name:                      "my-machine-2",
		LoadBalancerName:          "my-lb-1",
		ResourceGroup:             fakeGroupName,
		FrontendIPConfigurationID: pointer.String("frontend-ip-config-id-2"),
	}

	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
)

func getFakeNatSpecWithoutPort(spec InboundNatSpec) *InboundNatSpec {
	newSpec := spec
	return &newSpec
}

func getFakeNatSpecWithPort(spec InboundNatSpec, port int32) *InboundNatSpec {
	newSpec := spec
	newSpec.SSHFrontendPort = pointer.Int32(port)
	return &newSpec
}

func TestReconcileInboundNATRule(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
			m *mock_inboundnatrules.MockclientMockRecorder,
			r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no NAT rule specs are found",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.ResourceGroup().AnyTimes().Return(fakeGroupName)
				s.APIServerLBName().AnyTimes().Return(fakeLBName)
				s.InboundNatSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "NAT rule successfully created with with no existing rules",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.ResourceGroup().AnyTimes().Return(fakeGroupName)
				s.APIServerLBName().AnyTimes().Return(fakeLBName)
				m.List(gomockinternal.AContext(), fakeGroupName, fakeLBName).Return(noExistingRules, nil)
				s.InboundNatSpecs().Return([]azure.ResourceSpecGetter{getFakeNatSpecWithoutPort(fakeNatSpec), getFakeNatSpecWithoutPort(fakeNatSpec2)})
				gomock.InOrder(
					r.CreateOrUpdateResource(gomockinternal.AContext(), getFakeNatSpecWithPort(fakeNatSpec, 22), serviceName).Return(nil, nil),
					r.CreateOrUpdateResource(gomockinternal.AContext(), getFakeNatSpecWithPort(fakeNatSpec2, 2201), serviceName).Return(nil, nil),
					s.UpdatePutStatus(infrav1.InboundNATRulesReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "NAT rule successfully created with with existing rules",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.ResourceGroup().AnyTimes().Return(fakeGroupName)
				s.APIServerLBName().AnyTimes().Return("my-lb")
				m.List(gomockinternal.AContext(), fakeGroupName, "my-lb").Return(fakeExistingRules, nil)
				s.InboundNatSpecs().Return([]azure.ResourceSpecGetter{getFakeNatSpecWithoutPort(fakeNatSpec)})
				gomock.InOrder(
					r.CreateOrUpdateResource(gomockinternal.AContext(), getFakeNatSpecWithPort(fakeNatSpec, 2202), serviceName).Return(nil, nil),
					s.UpdatePutStatus(infrav1.InboundNATRulesReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "No LB, Nat rule reconciliation is skipped",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.APIServerLBName().AnyTimes().Return("")
			},
		},
		{
			name:          "fail to get existing rules",
			expectedError: "failed to get existing NAT rules: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.ResourceGroup().AnyTimes().Return(fakeGroupName)
				s.APIServerLBName().AnyTimes().Return("my-lb")
				s.InboundNatSpecs().Return([]azure.ResourceSpecGetter{&fakeNatSpec})
				m.List(gomockinternal.AContext(), fakeGroupName, "my-lb").Return(nil, internalError)
				s.UpdatePutStatus(infrav1.InboundNATRulesReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to get existing NAT rules: #: Internal Server Error: StatusCode=500"))
			},
		},
		{
			name:          "fail to create NAT rule",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.ResourceGroup().AnyTimes().Return(fakeGroupName)
				s.APIServerLBName().AnyTimes().Return("my-lb")
				m.List(gomockinternal.AContext(), fakeGroupName, "my-lb").Return(fakeExistingRules, nil)
				s.InboundNatSpecs().Return([]azure.ResourceSpecGetter{&fakeNatSpec})
				gomock.InOrder(
					r.CreateOrUpdateResource(gomockinternal.AContext(), getFakeNatSpecWithPort(fakeNatSpec, 2202), serviceName).Return(nil, internalError),
					s.UpdatePutStatus(infrav1.InboundNATRulesReadyCondition, serviceName, internalError),
				)
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			// TODO: investigate why t.Parallel() trips the race detector here.
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_inboundnatrules.NewMockInboundNatScope(mockCtrl)
			clientMock := mock_inboundnatrules.NewMockclient(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				client:     clientMock,
				Reconciler: asyncMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteNetworkInterface(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
			m *mock_inboundnatrules.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no NAT rules are found",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.InboundNatSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "successfully delete an existing NAT rule",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.InboundNatSpecs().Return([]azure.ResourceSpecGetter{&fakeNatSpec})
				s.ResourceGroup().AnyTimes().Return(fakeGroupName)
				s.APIServerLBName().AnyTimes().Return(fakeLBName)
				gomock.InOrder(
					r.DeleteResource(gomockinternal.AContext(), &fakeNatSpec, serviceName).Return(nil),
					s.UpdateDeleteStatus(infrav1.InboundNATRulesReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "NAT rule deletion fails",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.InboundNatSpecs().Return([]azure.ResourceSpecGetter{&fakeNatSpec})
				s.ResourceGroup().AnyTimes().Return(fakeGroupName)
				s.APIServerLBName().AnyTimes().Return(fakeLBName)
				gomock.InOrder(
					r.DeleteResource(gomockinternal.AContext(), &fakeNatSpec, serviceName).Return(internalError),
					s.UpdateDeleteStatus(infrav1.InboundNATRulesReadyCondition, serviceName, internalError),
				)
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_inboundnatrules.NewMockInboundNatScope(mockCtrl)
			clientMock := mock_inboundnatrules.NewMockclient(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				client:     clientMock,
				Reconciler: asyncMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
