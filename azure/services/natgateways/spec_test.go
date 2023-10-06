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

package natgateways

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

var (
	fakeNatGateway = armnetwork.NatGateway{
		ID: ptr.To("/subscriptions/my-sub/resourceGroups/my-rg/providers/Microsoft.Network/natGateways/my-node-natgateway-1"),
		Properties: &armnetwork.NatGatewayPropertiesFormat{
			PublicIPAddresses: []*armnetwork.SubResource{
				{ID: ptr.To("/subscriptions/my-sub/resourceGroups/my-rg/providers/Microsoft.Network/natGateways/pip-node-subnet")},
			},
		},
	}
	fakeNatGatewaySpec = NatGatewaySpec{
		Name:           "my-node-natgateway-1",
		ResourceGroup:  "my-rg",
		SubscriptionID: "my-sub",
		Location:       "westus",
		ClusterName:    "cluster",
		NatGatewayIP:   infrav1.PublicIPSpec{Name: "pip-node-subnet"},
	}
	fakeNatGatewaysTags = map[string]*string{
		"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster": ptr.To("owned"),
		"Name": ptr.To("my-node-natgateway-1"),
	}
)

func TestNatGatewaySpec_Parameters(t *testing.T) {
	testCases := []struct {
		name          string
		spec          *NatGatewaySpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "error when existing is not of NatGateway type",
			spec:     &NatGatewaySpec{},
			existing: struct{}{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "struct {} is not an armnetwork.NatGateway",
		},
		{
			name:     "get result as nil when existing NatGateway is present",
			spec:     &fakeNatGatewaySpec,
			existing: fakeNatGateway,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name: "get NatGateway when existing NatGateway is present but PublicIPAddresses is empty",
			spec: &fakeNatGatewaySpec,
			existing: armnetwork.NatGateway{
				Properties: &armnetwork.NatGatewayPropertiesFormat{PublicIPAddresses: []*armnetwork.SubResource{}},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.NatGateway{}))
				g.Expect(result.(armnetwork.NatGateway).Location).To(Equal(ptr.To[string](fakeNatGatewaySpec.Location)))
				g.Expect(result.(armnetwork.NatGateway).Name).To(Equal(ptr.To[string](fakeNatGatewaySpec.ResourceName())))
			},
			expectedError: "",
		},
		{
			name:     "get NatGateway when all values are present",
			spec:     &fakeNatGatewaySpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.NatGateway{}))
				g.Expect(result.(armnetwork.NatGateway).Location).To(Equal(ptr.To[string](fakeNatGatewaySpec.Location)))
				g.Expect(result.(armnetwork.NatGateway).Name).To(Equal(ptr.To[string](fakeNatGatewaySpec.ResourceName())))
				g.Expect(result.(armnetwork.NatGateway).Properties.PublicIPAddresses[0].ID).To(Equal(ptr.To[string](azure.PublicIPID(fakeNatGatewaySpec.SubscriptionID,
					fakeNatGatewaySpec.ResourceGroupName(), fakeNatGatewaySpec.NatGatewayIP.Name))))
				g.Expect(result.(armnetwork.NatGateway).Tags).To(Equal(fakeNatGatewaysTags))
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
