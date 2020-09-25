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

package v1alpha3

import (
	"encoding/json"
	"reflect"
	"testing"

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
			c.cluster.setResourceGroupDefault()
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
							CIDRBlocks: []string{DefaultVnetCIDR, DefaultVnetIPv6CIDR},
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
							CIDRBlocks:    []string{DefaultVnetCIDR, DefaultVnetIPv6CIDR},
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
			tc.cluster.setVnetDefaults()
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
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
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
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
							},
							{
								Role:          SubnetNode,
								Name:          "my-node-subnet",
								CIDRBlocks:    []string{"10.1.0.16/24"},
								SecurityGroup: SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
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
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
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
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
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
								RouteTable:    RouteTable{Name: "cluster-test-node-routetable"},
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
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.cluster.setSubnetDefaults()
			if !reflect.DeepEqual(tc.cluster, tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(tc.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}
