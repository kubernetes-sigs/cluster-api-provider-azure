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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	fakeSubnetOneCidrSpec = SubnetSpec{
		Name:              "my-subnet-1",
		ResourceGroup:     "my-rg",
		SubscriptionID:    "123",
		CIDRs:             []string{"10.0.0.0/16"},
		IsVNetManaged:     true,
		VNetName:          "my-vnet",
		VNetResourceGroup: "my-rg",
		RouteTableName:    "my-subnet_route_table",
		SecurityGroupName: "my-sg",
		NatGatewayName:    "my-nat-gateway",
		Role:              infrav1.SubnetNode,
	}

	fakeSubnetOneCidrParams = network.Subnet{
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefix:        pointer.String("10.0.0.0/16"),
			RouteTable:           &network.RouteTable{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
			NetworkSecurityGroup: &network.SecurityGroup{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
			NatGateway:           &network.SubResource{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/natGateways/my-nat-gateway")},
			ServiceEndpoints:     &[]network.ServiceEndpointPropertiesFormat{},
		},
	}

	fakeSubnetMultipleCidrSpec = SubnetSpec{
		Name:              "my-subnet-1",
		ResourceGroup:     "my-rg",
		SubscriptionID:    "123",
		CIDRs:             []string{"10.0.0.0/16", "10.1.0.0/16", "10.2.0.0/16"},
		IsVNetManaged:     true,
		VNetName:          "my-vnet",
		VNetResourceGroup: "my-rg",
		RouteTableName:    "my-subnet_route_table",
		SecurityGroupName: "my-sg",
		NatGatewayName:    "my-nat-gateway",
		Role:              infrav1.SubnetNode,
	}

	fakeSubnetMultipleCidrParams = network.Subnet{
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefixes: &[]string{
				"10.0.0.0/16",
				"10.1.0.0/16",
				"10.2.0.0/16",
			},
			RouteTable:           &network.RouteTable{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
			NetworkSecurityGroup: &network.SecurityGroup{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
			NatGateway:           &network.SubResource{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/natGateways/my-nat-gateway")},
			ServiceEndpoints:     &[]network.ServiceEndpointPropertiesFormat{},
		},
	}

	fakeIpv6SubnetSpecNotManaged = SubnetSpec{
		Name:              "my-ipv6-subnet",
		ResourceGroup:     "my-rg",
		SubscriptionID:    "123",
		CIDRs:             []string{"10.0.0.0/16", "2001:1234:5678:9abd::/64"},
		IsVNetManaged:     false,
		VNetName:          "my-vnet",
		VNetResourceGroup: "my-vnet-rg",
		RouteTableName:    "my-subnet_route_table",
		SecurityGroupName: "my-sg",
		Role:              infrav1.SubnetNode,
	}

	fakeIpv6SubnetNotManaged = network.Subnet{
		ID:   pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-ipv6-subnet"),
		Name: pointer.String("my-ipv6-subnet"),
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefixes: &[]string{
				"10.0.0.0/16",
				"2001:1234:5678:9abd::/64",
			},
			RouteTable:           &network.RouteTable{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
			NetworkSecurityGroup: &network.SecurityGroup{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
		},
	}
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *SubnetSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "get parameters for subnet with one cidr block",
			spec:     &fakeSubnetOneCidrSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(Equal(fakeSubnetOneCidrParams))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for subnet with multiple cidr blocks",
			spec:     &fakeSubnetMultipleCidrSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(Equal(fakeSubnetMultipleCidrParams))
			},
			expectedError: "",
		},
		{
			name:     "error vnet is not managed but subnet is missing",
			spec:     &fakeSubnetSpecNotManaged,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "custom vnet was provided but subnet my-subnet-1 is missing",
		},
		{
			name:     "vnet is not managed and subnet is present",
			spec:     &fakeSubnetSpecNotManaged,
			existing: fakeSubnetNotManaged,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "error vnet is not managed but ipv6 subnet is missing",
			spec:     &fakeIpv6SubnetSpecNotManaged,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "custom vnet was provided but subnet my-ipv6-subnet is missing",
		},
		{
			name:     "vnet is not managed and ipv6 subnet is present",
			spec:     &fakeIpv6SubnetSpecNotManaged,
			existing: fakeIpv6SubnetNotManaged,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
	}
	for _, tc := range testcases {
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
