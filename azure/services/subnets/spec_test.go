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

package subnets

import (
	"context"
	"testing"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestParameters(t *testing.T) {
	tests := []struct {
		name     string
		spec     *SubnetSpec
		existing *asonetworkv1.VirtualNetworksSubnet
		expected *asonetworkv1.VirtualNetworksSubnet
	}{
		{
			name: "no existing subnet",
			spec: &SubnetSpec{
				IsVNetManaged:     true,
				Name:              "subnet",
				SubscriptionID:    "sub",
				ResourceGroup:     "rg",
				VNetName:          "vnet",
				VNetResourceGroup: "vnet-rg",
				CIDRs:             []string{"cidr"},
				RouteTableName:    "routetable",
				NatGatewayName:    "natgateway",
				SecurityGroupName: "securitygroup",
				ServiceEndpoints: infrav1.ServiceEndpoints{
					{
						Service:   "service",
						Locations: []string{"location"},
					},
				},
			},
			existing: nil,
			expected: &asonetworkv1.VirtualNetworksSubnet{
				Spec: asonetworkv1.VirtualNetworks_Subnet_Spec{
					AzureName: "subnet",
					Owner: &genruntime.KnownResourceReference{
						Name: "vnet",
					},
					AddressPrefixes: []string{"cidr"},
					AddressPrefix:   ptr.To("cidr"),
					RouteTable: &asonetworkv1.RouteTableSpec_VirtualNetworks_Subnet_SubResourceEmbedded{
						Reference: &genruntime.ResourceReference{
							ARMID: "/subscriptions/sub/resourceGroups/vnet-rg/providers/Microsoft.Network/routeTables/routetable",
						},
					},
					NatGateway: &asonetworkv1.SubResource{
						Reference: &genruntime.ResourceReference{
							ARMID: "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/natGateways/natgateway",
						},
					},
					NetworkSecurityGroup: &asonetworkv1.NetworkSecurityGroupSpec_VirtualNetworks_Subnet_SubResourceEmbedded{
						Reference: &genruntime.ResourceReference{
							ARMID: "/subscriptions/sub/resourceGroups/vnet-rg/providers/Microsoft.Network/networkSecurityGroups/securitygroup",
						},
					},
					ServiceEndpoints: []asonetworkv1.ServiceEndpointPropertiesFormat{
						{
							Service:   ptr.To("service"),
							Locations: []string{"location"},
						},
					},
				},
			},
		},
		{
			name: "with existing subnet",
			spec: &SubnetSpec{
				IsVNetManaged:     true,
				Name:              "subnet",
				SubscriptionID:    "sub",
				ResourceGroup:     "rg",
				VNetName:          "vnet",
				VNetResourceGroup: "vnet-rg",
				CIDRs:             []string{"cidr"},
				RouteTableName:    "routetable",
				NatGatewayName:    "natgateway",
				SecurityGroupName: "securitygroup",
				ServiceEndpoints: infrav1.ServiceEndpoints{
					{
						Service:   "service",
						Locations: []string{"location"},
					},
				},
			},
			existing: &asonetworkv1.VirtualNetworksSubnet{
				Status: asonetworkv1.VirtualNetworks_Subnet_STATUS{
					Id: ptr.To("status is preserved"),
				},
			},
			expected: &asonetworkv1.VirtualNetworksSubnet{
				Spec: asonetworkv1.VirtualNetworks_Subnet_Spec{
					AzureName: "subnet",
					Owner: &genruntime.KnownResourceReference{
						Name: "vnet",
					},
					AddressPrefixes: []string{"cidr"},
					AddressPrefix:   ptr.To("cidr"),
					RouteTable: &asonetworkv1.RouteTableSpec_VirtualNetworks_Subnet_SubResourceEmbedded{
						Reference: &genruntime.ResourceReference{
							ARMID: "/subscriptions/sub/resourceGroups/vnet-rg/providers/Microsoft.Network/routeTables/routetable",
						},
					},
					NatGateway: &asonetworkv1.SubResource{
						Reference: &genruntime.ResourceReference{
							ARMID: "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/natGateways/natgateway",
						},
					},
					NetworkSecurityGroup: &asonetworkv1.NetworkSecurityGroupSpec_VirtualNetworks_Subnet_SubResourceEmbedded{
						Reference: &genruntime.ResourceReference{
							ARMID: "/subscriptions/sub/resourceGroups/vnet-rg/providers/Microsoft.Network/networkSecurityGroups/securitygroup",
						},
					},
					ServiceEndpoints: []asonetworkv1.ServiceEndpointPropertiesFormat{
						{
							Service:   ptr.To("service"),
							Locations: []string{"location"},
						},
					},
				},
				Status: asonetworkv1.VirtualNetworks_Subnet_STATUS{
					Id: ptr.To("status is preserved"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			result, err := test.spec.Parameters(context.Background(), test.existing)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cmp.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}
