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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
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

	noPortsInUse      = getFakeExistingPortsInUse([]int{})
	noExistingRules   = []network.InboundNatRule{}
	fakeExistingRules = []network.InboundNatRule{
		{
			Name: pointer.StringPtr("other-machine-nat-rule"),
			ID:   pointer.StringPtr("some-natrules-id"),
			InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
				FrontendPort: to.Int32Ptr(22),
			},
		},
		{
			Name: pointer.StringPtr("other-machine-nat-rule-2"),
			ID:   pointer.StringPtr("some-natrules-id-2"),
			InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
				FrontendPort: to.Int32Ptr(2201),
			},
		},
	}
	somePortsInUse = getFakeExistingPortsInUse([]int{22, 2201})

	fakeNatSpecWithNoExisting = InboundNatSpec{
		Name:                      "my-machine-1",
		LoadBalancerName:          "my-lb-1",
		ResourceGroup:             fakeGroupName,
		FrontendIPConfigurationID: to.StringPtr("frontend-ip-config-id-2"),
		PortsInUse:                noPortsInUse,
	}
	fakeNatSpec = InboundNatSpec{
		Name:                      "my-machine-2",
		LoadBalancerName:          "my-lb-2",
		ResourceGroup:             fakeGroupName,
		FrontendIPConfigurationID: to.StringPtr("frontend-ip-config-id-2"),
		PortsInUse:                somePortsInUse,
	}
	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")
)

func getFakeExistingPortsInUse(usedPorts []int) map[int32]struct{} {
	portsInUse := make(map[int32]struct{})
	for _, port := range usedPorts {
		portsInUse[int32(port)] = struct{}{}
	}

	return portsInUse
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
				m.List(gomockinternal.AContext(), fakeGroupName, fakeLBName).Return(noExistingRules, nil)
				s.InboundNatSpecs(noPortsInUse).Return([]azure.ResourceSpecGetter{})
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
				s.InboundNatSpecs(noPortsInUse).Return([]azure.ResourceSpecGetter{&fakeNatSpecWithNoExisting})
				gomock.InOrder(
					r.CreateResource(gomockinternal.AContext(), &fakeNatSpecWithNoExisting, serviceName).Return(nil, nil),
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
				s.InboundNatSpecs(somePortsInUse).Return([]azure.ResourceSpecGetter{&fakeNatSpec})
				gomock.InOrder(
					r.CreateResource(gomockinternal.AContext(), &fakeNatSpec, serviceName).Return(nil, nil),
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
				s.InboundNatSpecs(somePortsInUse).Return([]azure.ResourceSpecGetter{&fakeNatSpec})
				gomock.InOrder(
					r.CreateResource(gomockinternal.AContext(), &fakeNatSpec, serviceName).Return(nil, internalError),
					s.UpdatePutStatus(infrav1.InboundNATRulesReadyCondition, serviceName, internalError),
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
				s.InboundNatSpecs(noPortsInUse).Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "successfully delete an existing NAT rule",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.InboundNatSpecs(noPortsInUse).Return([]azure.ResourceSpecGetter{&fakeNatSpecWithNoExisting})
				s.ResourceGroup().AnyTimes().Return(fakeGroupName)
				s.APIServerLBName().AnyTimes().Return(fakeLBName)
				gomock.InOrder(
					r.DeleteResource(gomockinternal.AContext(), &fakeNatSpecWithNoExisting, serviceName).Return(nil),
					s.UpdateDeleteStatus(infrav1.InboundNATRulesReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "NAT rule deletion fails",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.InboundNatSpecs(noPortsInUse).Return([]azure.ResourceSpecGetter{&fakeNatSpecWithNoExisting})
				s.ResourceGroup().AnyTimes().Return(fakeGroupName)
				s.APIServerLBName().AnyTimes().Return(fakeLBName)
				gomock.InOrder(
					r.DeleteResource(gomockinternal.AContext(), &fakeNatSpecWithNoExisting, serviceName).Return(internalError),
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
