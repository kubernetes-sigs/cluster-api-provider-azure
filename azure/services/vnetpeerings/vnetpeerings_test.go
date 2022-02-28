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
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
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
	internalError         = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")
	notDoneError          = azure.NewOperationNotDoneError(&infrav1.Future{})
)

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
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:1])
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(&fakePeering1To2, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "noop if no peering specs are found",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "create even number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:2])
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(&fakePeering1To2, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(&fakePeering2To1, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create odd number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringExtraSpecs)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(&fakePeering1To2, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(&fakePeering2To1, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeeringExtra, serviceName).Return(&fakePeeringExtra, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create multiple peerings on one vnet",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(&fakePeering1To2, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(&fakePeering2To1, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(&fakePeering1To3, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(&fakePeering3To1, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "error in creating peering",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(&fakePeering1To2, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(&fakePeering2To1, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(nil, internalError)
				r.CreateResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(&fakePeering3To1, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "error in creating peering",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(&fakePeering1To2, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(&fakePeering2To1, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(nil, internalError)
				r.CreateResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(&fakePeering3To1, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "not done error in creating is ignored",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(&fakePeering1To2, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(nil, internalError)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(nil, notDoneError)
				r.CreateResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(&fakePeering3To1, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "not done error in creating is overwritten",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(&fakePeering1To2, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(&fakePeering2To1, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(nil, notDoneError)
				r.CreateResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(nil, internalError)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "not done error in creating remains",
			expectedError: "operation type  on Azure resource / is not done",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(&fakePeering1To2, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(&fakePeering2To1, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(nil, notDoneError)
				r.CreateResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(&fakePeering3To1, nil)
				p.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, notDoneError)
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
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
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
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:1])
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "noop if no peering specs are found",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "delete even number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs[:2])
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete odd number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringExtraSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeeringExtra, serviceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete multiple peerings on one vnet",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "error in deleting peering",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(internalError)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "not done error in deleting is ignored",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(internalError)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(notDoneError)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "not done error in deleting is overwritten",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(notDoneError)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(internalError)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "not done error in deleting remains",
			expectedError: "operation type  on Azure resource / is not done",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.VnetPeeringSpecs().Return(fakePeeringSpecs)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To2, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering2To1, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering1To3, serviceName).Return(notDoneError)
				r.DeleteResource(gomockinternal.AContext(), &fakePeering3To1, serviceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, notDoneError)
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
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
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
