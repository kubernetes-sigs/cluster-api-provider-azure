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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
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

	fakeSubnetOneCidrParams = armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix:        ptr.To("10.0.0.0/16"),
			RouteTable:           &armnetwork.RouteTable{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
			NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
			NatGateway:           &armnetwork.SubResource{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/natGateways/my-nat-gateway")},
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

	fakeSubnetMultipleCidrParams = armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefixes: []*string{
				ptr.To("10.0.0.0/16"),
				ptr.To("10.1.0.0/16"),
				ptr.To("10.2.0.0/16"),
			},
			RouteTable:           &armnetwork.RouteTable{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
			NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
			NatGateway:           &armnetwork.SubResource{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/natGateways/my-nat-gateway")},
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

	fakeIpv6SubnetNotManaged = armnetwork.Subnet{
		ID:   ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-ipv6-subnet"),
		Name: ptr.To("my-ipv6-subnet"),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefixes: []*string{
				ptr.To("10.0.0.0/16"),
				ptr.To("2001:1234:5678:9abd::/64"),
			},
			RouteTable:           &armnetwork.RouteTable{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
			NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
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

func TestSubnetSpec_shouldUpdate(t *testing.T) {
	type fields struct {
		Name              string
		ResourceGroup     string
		SubscriptionID    string
		CIDRs             []string
		VNetName          string
		VNetResourceGroup string
		IsVNetManaged     bool
		RouteTableName    string
		SecurityGroupName string
		Role              infrav1.SubnetRole
		NatGatewayName    string
		ServiceEndpoints  infrav1.ServiceEndpoints
	}
	type args struct {
		existingSubnet armnetwork.Subnet
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "subnet should not be updated when the VNet is not managed",
			fields: fields{
				Name:           "my-subnet",
				ResourceGroup:  "my-rg",
				SubscriptionID: "123",
				IsVNetManaged:  false,
			},
			args: args{
				existingSubnet: armnetwork.Subnet{
					Name: ptr.To("my-subnet"),
				},
			},
			want: false,
		},
		{
			name: "subnet should be updated when NAT Gateway gets added",
			fields: fields{
				Name:           "my-subnet",
				ResourceGroup:  "my-rg",
				SubscriptionID: "123",
				IsVNetManaged:  true,
				NatGatewayName: "my-nat-gateway",
			},
			args: args{
				existingSubnet: armnetwork.Subnet{
					Name: ptr.To("my-subnet"),
					Properties: &armnetwork.SubnetPropertiesFormat{
						NatGateway: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "subnet should be updated if service endpoints changed",
			fields: fields{
				Name:           "my-subnet",
				ResourceGroup:  "my-rg",
				SubscriptionID: "123",
				IsVNetManaged:  true,
				ServiceEndpoints: infrav1.ServiceEndpoints{
					{
						Service: "Microsoft.Storage",
					},
				},
			},
			args: args{
				existingSubnet: armnetwork.Subnet{
					Name: ptr.To("my-subnet"),
					Properties: &armnetwork.SubnetPropertiesFormat{
						ServiceEndpoints: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "subnet should not be updated if other properties change",
			fields: fields{
				Name:           "my-subnet",
				ResourceGroup:  "my-rg",
				SubscriptionID: "123",
				IsVNetManaged:  true,
				CIDRs:          []string{"10.1.0.0/16"},
			},
			args: args{
				existingSubnet: armnetwork.Subnet{
					Name: ptr.To("my-subnet"),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefixes: []*string{ptr.To("10.1.0.0/8")},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SubnetSpec{
				Name:              tt.fields.Name,
				ResourceGroup:     tt.fields.ResourceGroup,
				SubscriptionID:    tt.fields.SubscriptionID,
				CIDRs:             tt.fields.CIDRs,
				VNetName:          tt.fields.VNetName,
				VNetResourceGroup: tt.fields.VNetResourceGroup,
				IsVNetManaged:     tt.fields.IsVNetManaged,
				RouteTableName:    tt.fields.RouteTableName,
				SecurityGroupName: tt.fields.SecurityGroupName,
				Role:              tt.fields.Role,
				NatGatewayName:    tt.fields.NatGatewayName,
				ServiceEndpoints:  tt.fields.ServiceEndpoints,
			}
			if got := s.shouldUpdate(tt.args.existingSubnet); got != tt.want {
				t.Errorf("SubnetSpec.shouldUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}
