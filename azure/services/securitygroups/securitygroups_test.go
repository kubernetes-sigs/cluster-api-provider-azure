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
	"net/http"
	"net/url"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"

	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/securitygroups/mock_securitygroups"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeNSG = NSGSpec{
		Name:     "test-nsg",
		Location: "test-location",
		SecurityRules: infrav1.SecurityRules{
			{
				Name:             "allow_ssh",
				Description:      "Allow SSH",
				Priority:         2200,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Direction:        infrav1.SecurityRuleDirectionInbound,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr("22"),
			},
			{
				Name:             "other_rule",
				Description:      "Test Rule",
				Priority:         500,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Direction:        infrav1.SecurityRuleDirectionInbound,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr("80"),
			},
		},
		ResourceGroup: "test-group",
	}
	fakeNSG2 = NSGSpec{
		Name:          "test-nsg-2",
		Location:      "test-location",
		SecurityRules: infrav1.SecurityRules{},
		ResourceGroup: "test-group",
	}
	errFake = errors.New("this is an error")
	errFoo  = errors.New("foo")
	notDoneError          = azure.NewOperationNotDoneError(&infrav1.Future{})
)

func TestReconcileSecurityGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "create multiple security groups succeeds, should return no error",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG, &fakeNSG2})
				r.CreateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), &fakeNSG2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "first security groups create fails, should return error",
			expectedError: "failed to create resource test-group/test-nsg (service: securitygroups): this is an error",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG, &fakeNSG2})
				r.CreateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, errFake)
				r.CreateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to create resource test-group/test-nsg (service: securitygroups): this is an error"))
			},
		},
		{
			name:          "first sg create fails, second sg create not done, should return create error",
			expectedError: "failed to create resource test-group/test-nsg (service: securitygroups): this is an error",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.NSGSpecs().Return([]azure.ResourceSpecGetter{&fakeNSG, &fakeNSG2})
				r.CreateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, errFake)
				r.CreateResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil, notDoneError)			
				s.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to create resource test-group/test-nsg (service: securitygroups): this is an error"))
			},
		},
		{
			name:          "vnet is not managed, should skip reconcile",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(false, nil)
			},
		},
		{
			name:          "fail to check if vnet is managed, should return an error",
			expectedError: "failed to determine if network security groups are managed: this is an error",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(false, errFake)
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

			scopeMock := mock_securitygroups.NewMockNSGScope(mockCtrl)
			clientMock := mock_securitygroups.NewMockclient(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
				Reconciler: reconcilerMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteSecurityGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "delete multiple security groups succeeds, should return no error",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.NSGSpecs().Return(fakeNSGSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG2, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.SecurityGroupsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "first security groups delete fails, should return an error",
			expectedError: "failed to delete resource test-group/test-nsg (service: securitygroups): this is an error",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.NSGSpecs().Return(fakeNSGSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(errFake)
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG2, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.SecurityGroupsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to delete resource test-group/test-nsg (service: securitygroups): this is an error"))
			},
		},
		{
			name:          "first security groups delete fails and second security groups create not done, should return an error",
			expectedError: "failed to delete resource test-group/test-nsg (service: securitygroups): this is an error",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.NSGSpecs().Return(fakeNSGSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG, serviceName).Return(errFake)
				r.DeleteResource(gomockinternal.AContext(), &fakeNSG2, serviceName).Return(notDoneError)
				s.UpdateDeleteStatus(infrav1.SecurityGroupsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to delete resource test-group/test-nsg (service: securitygroups): this is an error"))
			},
		},
		{
			name:          "vnet is not managed, should skip delete",
			expectedError: "",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(false, nil)
			},
		},
		{
			name:          "fail to check if vnet is managed, should return an error",
			expectedError: "failed to determine if network security groups are managed: this is an error",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(false, errFake)
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

			scopeMock := mock_securitygroups.NewMockNSGScope(mockCtrl)
			clientMock := mock_securitygroups.NewMockclient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
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

var (
	ruleA = network.SecurityRule{
		Name: to.StringPtr("A"),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Description:              to.StringPtr("this is rule A"),
			Protocol:                 network.SecurityRuleProtocolTCP,
			DestinationPortRange:     to.StringPtr("*"),
			SourcePortRange:          to.StringPtr("*"),
			DestinationAddressPrefix: to.StringPtr("*"),
			SourceAddressPrefix:      to.StringPtr("*"),
			Priority:                 to.Int32Ptr(100),
			Direction:                network.SecurityRuleDirectionInbound,
		},
	}
	ruleB = network.SecurityRule{
		Name: to.StringPtr("B"),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Description:              to.StringPtr("this is rule B"),
			Protocol:                 network.SecurityRuleProtocolTCP,
			DestinationPortRange:     to.StringPtr("*"),
			SourcePortRange:          to.StringPtr("*"),
			DestinationAddressPrefix: to.StringPtr("*"),
			SourceAddressPrefix:      to.StringPtr("*"),
			Priority:                 to.Int32Ptr(100),
			Direction:                network.SecurityRuleDirectionOutbound,
		},
	}
	ruleBModified = network.SecurityRule{
		Name: to.StringPtr("B"),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Description:              to.StringPtr("this is rule B"),
			Protocol:                 network.SecurityRuleProtocolTCP,
			DestinationPortRange:     to.StringPtr("80"),
			SourcePortRange:          to.StringPtr("*"),
			DestinationAddressPrefix: to.StringPtr("*"),
			SourceAddressPrefix:      to.StringPtr("*"),
			Priority:                 to.Int32Ptr(100),
			Direction:                network.SecurityRuleDirectionOutbound,
		},
	}
)

func TestRuleExists(t *testing.T) {
	testcases := []struct {
		name     string
		rules    []network.SecurityRule
		rule     network.SecurityRule
		expected bool
	}{
		{
			name:     "rule doesn't exitst",
			rules:    []network.SecurityRule{ruleA},
			rule:     ruleB,
			expected: false,
		},
		{
			name:     "rule exists",
			rules:    []network.SecurityRule{ruleA, ruleB},
			rule:     ruleB,
			expected: true,
		},
		{
			name:     "rule exists but has been modified",
			rules:    []network.SecurityRule{ruleA, ruleB},
			rule:     ruleBModified,
			expected: false,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			result := ruleExists(tc.rules, tc.rule)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}
