/*
Copyright 2021 The Kubernetes Authors.

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

package vnetpeerings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vnetpeerings/mock_vnetpeerings"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakePeering1To2 = VnetPeeringSpec{
		PeeringName:         "vnet1-to-vnet2",
		SourceVnetName:      "vnet1",
		SourceResourceGroup: "group1",
		RemoteVnetName:      "vnet2",
		RemoteResourceGroup: "group2",
		SubscriptionID:      "sub1",
	}
	fakePeering2To1 = VnetPeeringSpec{
		PeeringName:         "vnet2-to-vnet1",
		SourceVnetName:      "vnet2",
		SourceResourceGroup: "group2",
		RemoteVnetName:      "vnet1",
		RemoteResourceGroup: "group1",
		SubscriptionID:      "sub1",
	}
	fakePeering1To3 = VnetPeeringSpec{
		PeeringName:         "vnet1-to-vnet3",
		SourceVnetName:      "vnet1",
		SourceResourceGroup: "group1",
		RemoteVnetName:      "vnet3",
		RemoteResourceGroup: "group3",
		SubscriptionID:      "sub1",
	}
	fakePeering3To1 = VnetPeeringSpec{
		PeeringName:         "vnet3-to-vnet1",
		SourceVnetName:      "vnet3",
		SourceResourceGroup: "group3",
		RemoteVnetName:      "vnet1",
		RemoteResourceGroup: "group1",
		SubscriptionID:      "sub1",
	}
	fakePeeringExtra = VnetPeeringSpec{
		PeeringName:         "extra-peering",
		SourceVnetName:      "vnet3",
		SourceResourceGroup: "group3",
		RemoteVnetName:      "vnet4",
		RemoteResourceGroup: "group4",
		SubscriptionID:      "sub1",
	}
	fakePeeringSpecs      = []azure.ResourceSpecGetter{&fakePeering1To2, &fakePeering2To1, &fakePeering1To3, &fakePeering3To1}
	fakePeeringExtraSpecs = []azure.ResourceSpecGetter{&fakePeering1To2, &fakePeering2To1, &fakePeeringExtra}
	fakeFuture, _         = azureautorest.NewFutureFromResponse(&http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Request: &http.Request{
			Method: http.MethodDelete,
		},
	})
	errFake = errors.New("this is an error")
	errFoo  = errors.New("foo")
)

func TestReconcileVnetPeerings(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder)
	}{
		{
			name:          "create one peering",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:1])
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil, nil)

				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create no peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:0])
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create even number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:2])
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil, nil)

				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create odd number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringExtraSpecs)
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil, nil)

				p.GetLongRunningOperationState("extra-peering", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeeringExtra).Return(nil, nil, nil)

				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create multiple peerings on one vnet",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil, nil)

				p.GetLongRunningOperationState("vnet1-to-vnet3", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering1To3).Return(nil, nil, nil)

				p.GetLongRunningOperationState("vnet3-to-vnet1", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering3To1).Return(nil, nil, nil)

				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "error in creating peering",
			expectedError: "failed to create resource group1/vnet1-to-vnet3 (service: vnetpeerings): this is an error",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil, nil)

				p.GetLongRunningOperationState("vnet1-to-vnet3", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering1To3).Return(nil, nil, errFake)

				p.GetLongRunningOperationState("vnet3-to-vnet1", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering3To1).Return(nil, nil, nil)

				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to create resource group1/vnet1-to-vnet3 (service: vnetpeerings): this is an error"))
			},
		},
		{
			name:          "error in creating peering which is not done",
			expectedError: "failed to create resource group1/vnet1-to-vnet3 (service: vnetpeerings): this is an error",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil, nil)

				p.GetLongRunningOperationState("vnet1-to-vnet3", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering1To3).Return(nil, nil, errFake)

				p.GetLongRunningOperationState("vnet3-to-vnet1", serviceName).Return(nil)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakePeering3To1).Return(nil, &fakeFuture, errFoo)
				p.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{}))

				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to create resource group1/vnet1-to-vnet3 (service: vnetpeerings): this is an error"))
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
			scopeMock := mock_vnetpeerings.NewMockVnetPeeringScope(mockCtrl)
			clientMock := mock_vnetpeerings.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
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

func TestDeleteVnetPeerings(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder)
	}{
		{
			name:          "delete one peering",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:1])
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil)

				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete no peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:0])
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete even number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:2])
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil)

				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete odd number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringExtraSpecs)
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil)

				p.GetLongRunningOperationState("extra-peering", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeeringExtra).Return(nil, nil)

				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete multiple peerings on one vnet",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil)

				p.GetLongRunningOperationState("vnet1-to-vnet3", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering1To3).Return(nil, nil)

				p.GetLongRunningOperationState("vnet3-to-vnet1", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering3To1).Return(nil, nil)

				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "error in deleting peering",
			expectedError: "failed to delete resource group1/vnet1-to-vnet3 (service: vnetpeerings): this is an error",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil)

				p.GetLongRunningOperationState("vnet1-to-vnet3", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering1To3).Return(nil, errFake)

				p.GetLongRunningOperationState("vnet3-to-vnet1", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering3To1).Return(nil, nil)

				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to delete resource group1/vnet1-to-vnet3 (service: vnetpeerings): this is an error"))
			},
		},
		{
			name:          "error in deleting peering which is not done",
			expectedError: "failed to delete resource group1/vnet1-to-vnet3 (service: vnetpeerings): this is an error",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				p.GetLongRunningOperationState("vnet1-to-vnet2", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering1To2).Return(nil, nil)

				p.GetLongRunningOperationState("vnet2-to-vnet1", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering2To1).Return(nil, nil)

				p.GetLongRunningOperationState("vnet1-to-vnet3", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering1To3).Return(nil, errFake)

				p.GetLongRunningOperationState("vnet3-to-vnet1", serviceName).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), &fakePeering3To1).Return(&fakeFuture, errFoo)
				p.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{}))

				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to delete resource group1/vnet1-to-vnet3 (service: vnetpeerings): this is an error"))
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
			scopeMock := mock_vnetpeerings.NewMockVnetPeeringScope(mockCtrl)
			clientMock := mock_vnetpeerings.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				fmt.Printf("\nExpected error:\t%s\n", tc.expectedError)
				fmt.Printf("\nActual error:\t%s\n", err.Error())
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
