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

package v1alpha4

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceGroupDefault(t *testing.T) {
	cases := map[string]struct {
		cluster *AzureCluster
		output  *AzureCluster
	}{
		"default empty rg": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					ResourceGroup: "foo",
				},
			},
		},
		"don't change if mismatched": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					ResourceGroup: "bar",
				},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					ResourceGroup: "bar",
				},
			},
		},
	}

	for name := range cases {
		c := cases[name]
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c.cluster.Spec.setResourceGroupDefault(c.cluster.Name)
			if !reflect.DeepEqual(c.cluster, c.output) {
				expected, _ := json.MarshalIndent(c.output, "", "\t")
				actual, _ := json.MarshalIndent(c.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestVnetDefaults(t *testing.T) {
	cases := []struct {
		name    string
		cluster *AzureCluster
		output  *AzureCluster
	}{
		{
			name:    "resource group vnet specified",
			cluster: createValidCluster(),
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Vnet: VnetSpec{
							ResourceGroup: "custom-vnet",
							Name:          "my-vnet",
							CIDRBlocks:    []string{DefaultVnetCIDR},
						},
						Subnets: Subnets{
							{
								Role:          SubnetControlPlane,
								Name:          "control-plane-subnet",
								SecurityGroup: SecurityGroup{},
								RouteTable:    RouteTable{},
							},
							{
								Role:          SubnetNode,
								Name:          "node-subnet",
								SecurityGroup: SecurityGroup{},
								RouteTable:    RouteTable{},
							},
						},
						APIServerLB: LoadBalancerSpec{
							Name: "my-lb",
							SKU:  SKUStandard,
							FrontendIPs: []FrontendIP{
								{
									Name: "ip-config",
									PublicIP: &PublicIPSpec{
										Name:    "public-ip",
										DNSName: "myfqdn.azure.com",
									},
								},
							},
							Type: Public,
						},
						NodeOutboundLB: &LoadBalancerSpec{
							FrontendIPsCount: to.Int32Ptr(1),
						},
					},
				},
			},
		},
		{
			name: "vnet not specified",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					ResourceGroup: "cluster-test",
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					ResourceGroup: "cluster-test",
					NetworkSpec: NetworkSpec{
						Vnet: VnetSpec{
							ResourceGroup: "cluster-test",
							Name:          "cluster-test-vnet",
							CIDRBlocks:    []string{DefaultVnetCIDR},
						},
					},
				},
			},
		},
		{
			name: "custom CIDR",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					ResourceGroup: "cluster-test",
					NetworkSpec: NetworkSpec{
						Vnet: VnetSpec{
							CIDRBlocks: []string{"10.0.0.0/16"},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					ResourceGroup: "cluster-test",
					NetworkSpec: NetworkSpec{
						Vnet: VnetSpec{
							ResourceGroup: "cluster-test",
							Name:          "cluster-test-vnet",
							CIDRBlocks:    []string{"10.0.0.0/16"},
						},
					},
				},
			},
		},
		{
			name: "IPv6 enabled",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					ResourceGroup: "cluster-test",
					NetworkSpec: NetworkSpec{
						Vnet: VnetSpec{
							CIDRBlocks: []string{DefaultVnetCIDR, "2001:1234:5678:9a00::/56"},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					ResourceGroup: "cluster-test",
					NetworkSpec: NetworkSpec{
						Vnet: VnetSpec{
							ResourceGroup: "cluster-test",
							Name:          "cluster-test-vnet",
							CIDRBlocks:    []string{DefaultVnetCIDR, "2001:1234:5678:9a00::/56"},
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.cluster.Spec.setVnetDefaults(tc.cluster.Name)
			if !reflect.DeepEqual(tc.cluster, tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(tc.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestSubnetDefaults(t *testing.T) {
	cases := []struct {
		name    string
		cluster *AzureCluster
		output  *AzureCluster
	}{
		{
			name: "no subnets",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Role:          SubnetControlPlane,
								Name:          "cluster-test-controlplane-subnet",
								CIDRBlocks:    []string{DefaultControlPlaneSubnetCIDR},
								SecurityGroup: SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    RouteTable{},
							},
							{
								Role:          SubnetNode,
								Name:          "cluster-test-node-subnet",
								CIDRBlocks:    []string{DefaultNodeSubnetCIDR},
								SecurityGroup: SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets with custom attributes",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Role:       SubnetControlPlane,
								Name:       "my-controlplane-subnet",
								CIDRBlocks: []string{"10.0.0.16/24"},
							},
							{
								Role:       SubnetNode,
								Name:       "my-node-subnet",
								CIDRBlocks: []string{"10.1.0.16/24"},
								NatGateway: NatGateway{Name: "foo-natgw"},
							},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Role:          SubnetControlPlane,
								Name:          "my-controlplane-subnet",
								CIDRBlocks:    []string{"10.0.0.16/24"},
								SecurityGroup: SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    RouteTable{},
							},
							{
								Role:          SubnetNode,
								Name:          "my-node-subnet",
								CIDRBlocks:    []string{"10.1.0.16/24"},
								SecurityGroup: SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
								NatGateway: NatGateway{
									Name: "foo-natgw",
									NatGatewayIP: PublicIPSpec{
										Name: "pip-cluster-test-my-node-subnet-natgw",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets specified",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Role: SubnetControlPlane,
								Name: "cluster-test-controlplane-subnet",
							},
							{
								Role: SubnetNode,
								Name: "cluster-test-node-subnet",
							},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Role:          SubnetControlPlane,
								Name:          "cluster-test-controlplane-subnet",
								CIDRBlocks:    []string{DefaultControlPlaneSubnetCIDR},
								SecurityGroup: SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    RouteTable{},
							},
							{
								Role:          SubnetNode,
								Name:          "cluster-test-node-subnet",
								CIDRBlocks:    []string{DefaultNodeSubnetCIDR},
								SecurityGroup: SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets route tables specified",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Role: SubnetControlPlane,
								Name: "cluster-test-controlplane-subnet",
								RouteTable: RouteTable{
									Name: "control-plane-custom-route-table",
								},
							},
							{
								Role: SubnetNode,
								Name: "cluster-test-node-subnet",
							},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Role:          SubnetControlPlane,
								Name:          "cluster-test-controlplane-subnet",
								CIDRBlocks:    []string{DefaultControlPlaneSubnetCIDR},
								SecurityGroup: SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    RouteTable{Name: "control-plane-custom-route-table"},
							},
							{
								Role:          SubnetNode,
								Name:          "cluster-test-node-subnet",
								CIDRBlocks:    []string{DefaultNodeSubnetCIDR},
								SecurityGroup: SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
							},
						},
					},
				},
			},
		},
		{
			name: "only node subnet specified",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Role: SubnetNode,
								Name: "my-node-subnet",
							},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Role:          SubnetNode,
								Name:          "my-node-subnet",
								CIDRBlocks:    []string{DefaultNodeSubnetCIDR},
								SecurityGroup: SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
							},
							{
								Role:          SubnetControlPlane,
								Name:          "cluster-test-controlplane-subnet",
								CIDRBlocks:    []string{DefaultControlPlaneSubnetCIDR},
								SecurityGroup: SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    RouteTable{},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets specified with IPv6 enabled",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Vnet: VnetSpec{
							CIDRBlocks: []string{"2001:be00::1/56"},
						},
						Subnets: Subnets{
							{
								Name:       "cluster-test-controlplane-subnet",
								Role:       "control-plane",
								CIDRBlocks: []string{"2001:beef::1/64"},
							},
							{
								Name:       "cluster-test-node-subnet",
								Role:       "node",
								CIDRBlocks: []string{"2001:beea::1/64"},
							},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Vnet: VnetSpec{
							CIDRBlocks: []string{"2001:be00::1/56"},
						},
						Subnets: Subnets{
							{
								Role:          SubnetControlPlane,
								Name:          "cluster-test-controlplane-subnet",
								CIDRBlocks:    []string{"2001:beef::1/64"},
								SecurityGroup: SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    RouteTable{},
							},
							{
								Role:          SubnetNode,
								Name:          "cluster-test-node-subnet",
								CIDRBlocks:    []string{"2001:beea::1/64"},
								SecurityGroup: SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets with custom security group",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Name: "cluster-test-controlplane-subnet",
								Role: "control-plane",
								SecurityGroup: SecurityGroup{
									Name: "my-custom-sg",
									SecurityRules: []SecurityRule{
										{
											Name:             "allow_port_50000",
											Description:      "allow port 50000",
											Protocol:         "*",
											Priority:         2202,
											SourcePorts:      to.StringPtr("*"),
											DestinationPorts: to.StringPtr("*"),
											Source:           to.StringPtr("*"),
											Destination:      to.StringPtr("*"),
										},
									},
								},
							},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						Subnets: Subnets{
							{
								Name:       "cluster-test-controlplane-subnet",
								Role:       "control-plane",
								CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
								SecurityGroup: SecurityGroup{
									Name: "my-custom-sg",
									SecurityRules: []SecurityRule{
										{
											Name:             "allow_port_50000",
											Description:      "allow port 50000",
											Protocol:         "*",
											Priority:         2202,
											SourcePorts:      to.StringPtr("*"),
											DestinationPorts: to.StringPtr("*"),
											Source:           to.StringPtr("*"),
											Destination:      to.StringPtr("*"),
											Direction:        SecurityRuleDirectionInbound,
										},
									},
								},
							},
							{
								Role:          SubnetNode,
								Name:          "cluster-test-node-subnet",
								CIDRBlocks:    []string{DefaultNodeSubnetCIDR},
								SecurityGroup: SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
							},
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.cluster.Spec.setSubnetDefaults(tc.cluster.Name)
			if !reflect.DeepEqual(tc.cluster, tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(tc.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestAPIServerLBDefaults(t *testing.T) {
	cases := []struct {
		name    string
		cluster *AzureCluster
		output  *AzureCluster
	}{
		{
			name: "no lb",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{
							Name: "cluster-test-public-lb",
							SKU:  SKUStandard,
							FrontendIPs: []FrontendIP{
								{
									Name: "cluster-test-public-lb-frontEnd",
									PublicIP: &PublicIPSpec{
										Name:    "pip-cluster-test-apiserver",
										DNSName: "",
									},
								},
							},
							Type:                 Public,
							IdleTimeoutInMinutes: to.Int32Ptr(DefaultOutboundRuleIdleTimeoutInMinutes),
						},
					},
				},
			},
		},
		{
			name: "internal lb",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{
							Type: Internal,
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{
							Name: "cluster-test-internal-lb",
							SKU:  SKUStandard,
							FrontendIPs: []FrontendIP{
								{
									Name:             "cluster-test-internal-lb-frontEnd",
									PrivateIPAddress: DefaultInternalLBIPAddress,
								},
							},
							Type:                 Internal,
							IdleTimeoutInMinutes: to.Int32Ptr(DefaultOutboundRuleIdleTimeoutInMinutes),
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.cluster.Spec.setAPIServerLBDefaults(tc.cluster.Name)
			if !reflect.DeepEqual(tc.cluster, tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(tc.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestAzureEnviromentDefault(t *testing.T) {
	cases := map[string]struct {
		cluster *AzureCluster
		output  *AzureCluster
	}{
		"default empty azure env": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					AzureEnvironment: DefaultAzureCloud,
				},
			},
		},
		"azure env set to AzurePublicCloud": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					AzureEnvironment: DefaultAzureCloud,
				},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					AzureEnvironment: DefaultAzureCloud,
				},
			},
		},
		"azure env set to AzureGermanCloud": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					AzureEnvironment: "AzureGermanCloud",
				},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					AzureEnvironment: "AzureGermanCloud",
				},
			},
		},
	}

	for name := range cases {
		c := cases[name]
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c.cluster.Spec.setAzureEnvironmentDefault()
			if !reflect.DeepEqual(c.cluster, c.output) {
				expected, _ := json.MarshalIndent(c.output, "", "\t")
				actual, _ := json.MarshalIndent(c.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestNodeOutboundLBDefaults(t *testing.T) {
	cases := []struct {
		name    string
		cluster *AzureCluster
		output  *AzureCluster
	}{
		{
			name: "default lb for public clusters",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{Type: Public},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{
							Type: Public,
						},
						NodeOutboundLB: &LoadBalancerSpec{
							Name: "cluster-test",
							SKU:  SKUStandard,
							FrontendIPs: []FrontendIP{{
								Name: "cluster-test-frontEnd",
								PublicIP: &PublicIPSpec{
									Name: "pip-cluster-test-node-outbound",
								},
							}},
							Type:                 Public,
							FrontendIPsCount:     to.Int32Ptr(1),
							IdleTimeoutInMinutes: to.Int32Ptr(DefaultOutboundRuleIdleTimeoutInMinutes),
						},
					},
				},
			},
		},
		{
			name: "no lb for private clusters",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{Type: Internal},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{
							Type: Internal,
						},
					},
				},
			},
		},
		{
			name: "frontendIPsCount > 1",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{Type: Public},
						NodeOutboundLB: &LoadBalancerSpec{
							FrontendIPsCount:     to.Int32Ptr(2),
							IdleTimeoutInMinutes: to.Int32Ptr(15),
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{
							Type: Public,
						},
						NodeOutboundLB: &LoadBalancerSpec{
							Name: "cluster-test",
							SKU:  SKUStandard,
							FrontendIPs: []FrontendIP{
								{
									Name: "cluster-test-frontEnd-1",
									PublicIP: &PublicIPSpec{
										Name: "pip-cluster-test-node-outbound-1",
									},
								},
								{
									Name: "cluster-test-frontEnd-2",
									PublicIP: &PublicIPSpec{
										Name: "pip-cluster-test-node-outbound-2",
									},
								},
							},
							Type:                 Public,
							FrontendIPsCount:     to.Int32Ptr(2),
							IdleTimeoutInMinutes: to.Int32Ptr(15),
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.cluster.Spec.setNodeOutboundLBDefaults(tc.cluster.Name)
			if !reflect.DeepEqual(tc.cluster, tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(tc.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestControlPlaneOutboundLBDefaults(t *testing.T) {
	cases := []struct {
		name    string
		cluster *AzureCluster
		output  *AzureCluster
	}{
		{
			name: "no cp lb for public clusters",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{Type: Public},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{
							Type: Public,
						},
					},
				},
			},
		},
		{
			name: "no cp lb for private clusters",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{Type: Internal},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{
							Type: Internal,
						},
					},
				},
			},
		},
		{
			name: "frontendIPsCount > 1",
			cluster: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{Type: Internal},
						ControlPlaneOutboundLB: &LoadBalancerSpec{
							FrontendIPsCount:     to.Int32Ptr(2),
							IdleTimeoutInMinutes: to.Int32Ptr(15),
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						APIServerLB: LoadBalancerSpec{
							Type: Internal,
						},
						ControlPlaneOutboundLB: &LoadBalancerSpec{
							Name: "cluster-test-outbound-lb",
							SKU:  SKUStandard,
							FrontendIPs: []FrontendIP{
								{
									Name: "cluster-test-outbound-lb-frontEnd-1",
									PublicIP: &PublicIPSpec{
										Name: "pip-cluster-test-controlplane-outbound-1",
									},
								},
								{
									Name: "cluster-test-outbound-lb-frontEnd-2",
									PublicIP: &PublicIPSpec{
										Name: "pip-cluster-test-controlplane-outbound-2",
									},
								},
							},
							Type:                 Public,
							FrontendIPsCount:     to.Int32Ptr(2),
							IdleTimeoutInMinutes: to.Int32Ptr(15),
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.cluster.Spec.setControlPlaneOutboundLBDefaults(tc.cluster.Name)
			if !reflect.DeepEqual(tc.cluster, tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(tc.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestBastionDefault(t *testing.T) {
	cases := map[string]struct {
		cluster *AzureCluster
		output  *AzureCluster
	}{
		"no bastion set": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{},
			},
		},
		"azure bastion enabled with no settings": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{
							Name: "foo-azure-bastion",
							Subnet: SubnetSpec{
								Name:       "AzureBastionSubnet",
								CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
							},
							PublicIP: PublicIPSpec{
								Name: "foo-azure-bastion-pip",
							},
						},
					},
				},
			},
		},
		"azure bastion enabled with name set": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{
							Name: "my-fancy-name",
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{
							Name: "my-fancy-name",
							Subnet: SubnetSpec{
								Name:       "AzureBastionSubnet",
								CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
							},
							PublicIP: PublicIPSpec{
								Name: "foo-azure-bastion-pip",
							},
						},
					},
				},
			},
		},
		"azure bastion enabled with subnet partially set": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{
							Subnet: SubnetSpec{},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{
							Name: "foo-azure-bastion",
							Subnet: SubnetSpec{
								Name:       "AzureBastionSubnet",
								CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
							},
							PublicIP: PublicIPSpec{
								Name: "foo-azure-bastion-pip",
							},
						},
					},
				},
			},
		},
		"azure bastion enabled with subnet fully set": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{
							Subnet: SubnetSpec{
								Name:       "my-superfancy-name",
								CIDRBlocks: []string{"10.10.0.0/16"},
							},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{
							Name: "foo-azure-bastion",
							Subnet: SubnetSpec{
								Name:       "my-superfancy-name",
								CIDRBlocks: []string{"10.10.0.0/16"},
							},
							PublicIP: PublicIPSpec{
								Name: "foo-azure-bastion-pip",
							},
						},
					},
				},
			},
		},
		"azure bastion enabled with public IP name set": {
			cluster: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{
							PublicIP: PublicIPSpec{
								Name: "my-ultrafancy-pip-name",
							},
						},
					},
				},
			},
			output: &AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: AzureClusterSpec{
					BastionSpec: BastionSpec{
						AzureBastion: &AzureBastion{
							Name: "foo-azure-bastion",
							Subnet: SubnetSpec{
								Name:       "AzureBastionSubnet",
								CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
							},
							PublicIP: PublicIPSpec{
								Name: "my-ultrafancy-pip-name",
							},
						},
					},
				},
			},
		},
	}

	for name := range cases {
		c := cases[name]
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c.cluster.Spec.setBastionDefaults(c.cluster.Name)
			if !reflect.DeepEqual(c.cluster, c.output) {
				expected, _ := json.MarshalIndent(c.output, "", "\t")
				actual, _ := json.MarshalIndent(c.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}
