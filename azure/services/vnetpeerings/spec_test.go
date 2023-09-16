/*
Copyright 2023 The Kubernetes Authors.

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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

var (
	fakeVnetPeering = armnetwork.VirtualNetworkPeering{
		ID:   ptr.To("fake-id"),
		Name: ptr.To("fake-name"),
		Type: ptr.To("fake-type"),
	}
	fakeVnetPeeringSpec = VnetPeeringSpec{
		PeeringName:               "hub-to-spoke",
		RemoteVnetName:            "spoke-vnet",
		RemoteResourceGroup:       "spoke-group",
		SubscriptionID:            "sub1",
		AllowForwardedTraffic:     ptr.To(true),
		AllowGatewayTransit:       ptr.To(true),
		AllowVirtualNetworkAccess: ptr.To(true),
		UseRemoteGateways:         ptr.To(false),
	}
)

func TestVnetPeeringSpec_Parameters(t *testing.T) {
	testCases := []struct {
		name          string
		spec          *VnetPeeringSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "error when existing is not of VnetPeering type",
			spec:     &VnetPeeringSpec{},
			existing: struct{}{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "struct {} is not an armnetwork.VnetPeering",
		},
		{
			name:     "get result as nil when existing VnetPeering is present",
			spec:     &fakeVnetPeeringSpec,
			existing: fakeVnetPeering,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get result as nil when existing VnetPeering is present with empty data",
			spec:     &fakeVnetPeeringSpec,
			existing: armnetwork.VirtualNetworkPeering{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get VirtualNetworkPeering when all values are present",
			spec:     &fakeVnetPeeringSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.VirtualNetworkPeering{}))
				g.Expect(result.(armnetwork.VirtualNetworkPeering).Name).To(Equal(ptr.To[string](fakeVnetPeeringSpec.ResourceName())))
				g.Expect(result.(armnetwork.VirtualNetworkPeering).Properties.RemoteVirtualNetwork.ID).To(Equal(ptr.To[string](azure.VNetID(fakeVnetPeeringSpec.SubscriptionID,
					fakeVnetPeeringSpec.RemoteResourceGroup, fakeVnetPeeringSpec.RemoteVnetName))))
				g.Expect(result.(armnetwork.VirtualNetworkPeering).Properties.AllowForwardedTraffic).To(Equal(fakeVnetPeeringSpec.AllowForwardedTraffic))
				g.Expect(result.(armnetwork.VirtualNetworkPeering).Properties.AllowGatewayTransit).To(Equal(fakeVnetPeeringSpec.AllowGatewayTransit))
				g.Expect(result.(armnetwork.VirtualNetworkPeering).Properties.AllowVirtualNetworkAccess).To(Equal(fakeVnetPeeringSpec.AllowVirtualNetworkAccess))
				g.Expect(result.(armnetwork.VirtualNetworkPeering).Properties.UseRemoteGateways).To(Equal(fakeVnetPeeringSpec.UseRemoteGateways))
			},
			expectedError: "",
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, result)
		})
	}
}
