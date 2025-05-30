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
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vnetpeerings/mock_vnetpeerings"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
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
	fakePeeringHubToSpoke = VnetPeeringSpec{
		PeeringName:               "hub-to-spoke",
		SourceVnetName:            "hub-vnet",
		SourceResourceGroup:       "hub-group",
		RemoteVnetName:            "spoke-vnet",
		RemoteResourceGroup:       "spoke-group",
		SubscriptionID:            "sub1",
		AllowForwardedTraffic:     ptr.To(true),
		AllowGatewayTransit:       ptr.To(true),
		AllowVirtualNetworkAccess: ptr.To(true),
		UseRemoteGateways:         ptr.To(false),
	}
	fakePeeringSpokeToHub = VnetPeeringSpec{
		PeeringName:               "spoke-to-hub",
		SourceVnetName:            "spoke-vnet",
		SourceResourceGroup:       "spoke-group",
		RemoteVnetName:            "hub-vnet",
		RemoteResourceGroup:       "hub-group",
		SubscriptionID:            "sub1",
		AllowForwardedTraffic:     ptr.To(true),
		AllowGatewayTransit:       ptr.To(false),
		AllowVirtualNetworkAccess: ptr.To(true),
		UseRemoteGateways:         ptr.To(true),
	}
	fakePeeringExtra = VnetPeeringSpec{
		PeeringName:         "extra-peering",
		SourceVnetName:      "vnet3",
		SourceResourceGroup: "group3",
		RemoteVnetName:      "vnet4",
		RemoteResourceGroup: "group4",
		SubscriptionID:      "sub1",
	}
	fakePeeringSpecs      = []azure.ResourceSpecGetter{&fakePeering1To2, &fakePeering2To1, &fakePeering1To3, &fakePeering3To1, &fakePeeringHubToSpoke, &fakePeeringSpokeToHub}
	fakePeeringExtraSpecs = []azure.ResourceSpecGetter{&fakePeering1To2, &fakePeering2To1, &fakePeeringExtra}
	notDoneError          = azure.NewOperationNotDoneError(&infrav1.Future{})
)

func internalError() *azcore.ResponseError {
	return &azcore.ResponseError{
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Internal Server Error: StatusCode=500")),
			StatusCode: http.StatusInternalServerError,
		},
	}
}

func TestReconcileVnetPeerings(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "create one peering",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:1])
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(&fakePeering1To2, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "noop if no peering specs are found",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "create even number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:2])
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(&fakePeering1To2, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(&fakePeering2To1, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "create odd number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringExtraSpecs)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(&fakePeering1To2, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(&fakePeering2To1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringExtra, ServiceName).Return(&fakePeeringExtra, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "create multiple peerings on one vnet",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(&fakePeering1To2, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(&fakePeering2To1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(&fakePeering1To3, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(&fakePeering3To1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(&fakePeeringHubToSpoke, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(&fakePeeringSpokeToHub, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "error in creating peering",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(&fakePeering1To2, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(&fakePeering2To1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(nil, internalError())
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(&fakePeering3To1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(&fakePeeringHubToSpoke, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(&fakePeeringSpokeToHub, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, ServiceName, internalError())
			},
		},
		{
			name:          "not done error in creating is ignored",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(&fakePeering1To2, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(nil, internalError())
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(nil, notDoneError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(&fakePeering3To1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(&fakePeeringHubToSpoke, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(&fakePeeringSpokeToHub, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, ServiceName, internalError())
			},
		},
		{
			name:          "not done error in creating is overwritten",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(&fakePeering1To2, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(&fakePeering2To1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(nil, notDoneError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(nil, internalError())
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(&fakePeeringHubToSpoke, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(&fakePeeringSpokeToHub, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, ServiceName, internalError())
			},
		},
		{
			name:          "not done error in creating remains",
			expectedError: "operation type  on Azure resource / is not done",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(&fakePeering1To2, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(&fakePeering2To1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(nil, notDoneError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(&fakePeering3To1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(&fakePeeringHubToSpoke, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(&fakePeeringSpokeToHub, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, ServiceName, notDoneError)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_vnetpeerings.NewMockVnetPeeringScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
			}

			err := s.Reconcile(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
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
		expect        func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "delete one peering",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:1])
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "noop if no peering specs are found",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "delete even number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:2])
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "delete odd number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringExtraSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringExtra, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "delete multiple peerings on one vnet",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "error in deleting peering",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(internalError())
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, ServiceName, internalError())
			},
		},
		{
			name:          "not done error in deleting is ignored",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(internalError())
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(notDoneError)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, ServiceName, internalError())
			},
		},
		{
			name:          "not done error in deleting is overwritten",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(notDoneError)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(internalError())
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, ServiceName, internalError())
			},
		},
		{
			name:          "not done error in deleting remains",
			expectedError: "operation type  on Azure resource / is not done",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, ServiceName).Return(notDoneError)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringHubToSpoke, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringSpokeToHub, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, ServiceName, notDoneError)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_vnetpeerings.NewMockVnetPeeringScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
			}

			err := s.Delete(t.Context())
			if tc.expectedError != "" {
				fmt.Printf("\nExpected error:\t%s\n", tc.expectedError)
				fmt.Printf("\nActual error:\t%s\n", err.Error())
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
