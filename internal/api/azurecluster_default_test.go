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

package api

import (
	"encoding/json"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/featuregate"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	apifixtures "sigs.k8s.io/cluster-api-provider-azure/internal/test/apifixtures"
)

func TestResourceGroupDefault(t *testing.T) {
	cases := map[string]struct {
		cluster *infrav1.AzureCluster
		output  *infrav1.AzureCluster
	}{
		"default empty rg": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "foo",
				},
			},
		},
		"don't change if mismatched": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "bar",
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "bar",
				},
			},
		},
	}

	for name := range cases {
		c := cases[name]
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			setDefaultAzureClusterResourceGroup(c.cluster)
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
		cluster *infrav1.AzureCluster
		output  *infrav1.AzureCluster
	}{
		{
			name:    "resource group vnet specified",
			cluster: apifixtures.CreateValidCluster(),
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							ResourceGroup: "custom-vnet",
							Name:          "my-vnet",
							VnetClassSpec: infrav1.VnetClassSpec{
								CIDRBlocks: []string{DefaultVnetCIDR},
							},
						},
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},

								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "my-lb",
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "ip-config",
									PublicIP: &infrav1.PublicIPSpec{
										Name:    "public-ip",
										DNSName: "myfqdn.azure.com",
									},
								},
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU: infrav1.SKUStandard,

								Type: infrav1.Public,
							},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							FrontendIPsCount: ptr.To[int32](1),
						},
					},
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
		},
		{
			name: "vnet not specified",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					ResourceGroup:       "cluster-test",
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					ResourceGroup:       "cluster-test",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							ResourceGroup: "cluster-test",
							Name:          "cluster-test-vnet",
							VnetClassSpec: infrav1.VnetClassSpec{
								CIDRBlocks: []string{DefaultVnetCIDR},
							},
						},
					},
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
		},
		{
			name: "custom CIDR",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					ResourceGroup:       "cluster-test",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							VnetClassSpec: infrav1.VnetClassSpec{
								CIDRBlocks: []string{"10.0.0.0/16"},
							},
						},
					},
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					ResourceGroup:       "cluster-test",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							ResourceGroup: "cluster-test",
							Name:          "cluster-test-vnet",
							VnetClassSpec: infrav1.VnetClassSpec{
								CIDRBlocks: []string{"10.0.0.0/16"},
							},
						},
					},
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
		},
		{
			name: "IPv6 enabled",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					ResourceGroup:       "cluster-test",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							VnetClassSpec: infrav1.VnetClassSpec{
								CIDRBlocks: []string{DefaultVnetCIDR, "2001:1234:5678:9a00::/56"},
							},
						},
					},
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					ResourceGroup:       "cluster-test",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							ResourceGroup: "cluster-test",
							Name:          "cluster-test-vnet",
							VnetClassSpec: infrav1.VnetClassSpec{
								CIDRBlocks: []string{DefaultVnetCIDR, "2001:1234:5678:9a00::/56"},
							},
						},
					},
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
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
			setDefaultAzureClusterVnet(tc.cluster)
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
		cluster *infrav1.AzureCluster
		output  *infrav1.AzureCluster
	}{
		{
			name: "no subnets",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec:         infrav1.NetworkSpec{},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetControlPlane,
									CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
									Name:       "cluster-test-controlplane-subnet",
								},

								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{DefaultNodeSubnetCIDR},
									Name:       "cluster-test-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-node-routetable"},
								NatGateway: infrav1.NatGateway{NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
									Name: "cluster-test-node-natgw",
								}},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets with custom attributes",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetControlPlane,
									CIDRBlocks: []string{"10.0.0.16/24"},
									Name:       "my-controlplane-subnet",
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"10.1.0.16/24"},
									Name:       "my-node-subnet",
								},
								NatGateway: infrav1.NatGateway{
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "foo-natgw",
									},
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetControlPlane,
									CIDRBlocks: []string{"10.0.0.16/24"},
									Name:       "my-controlplane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"10.1.0.16/24"},
									Name:       "my-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-node-routetable"},
								NatGateway: infrav1.NatGateway{
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "foo-natgw",
									},
									NatGatewayIP: infrav1.PublicIPSpec{
										Name: "pip-foo-natgw",
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
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "cluster-test-controlplane-subnet",
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "cluster-test-node-subnet",
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetControlPlane,
									CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
									Name:       "cluster-test-controlplane-subnet",
								},

								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{DefaultNodeSubnetCIDR},
									Name:       "cluster-test-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-node-routetable"},
								NatGateway: infrav1.NatGateway{
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "cluster-test-node-natgw-1",
									},
									NatGatewayIP: infrav1.PublicIPSpec{
										Name: "pip-cluster-test-node-natgw-1",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "cluster subnet with custom attributes",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetCluster,
									CIDRBlocks: []string{"10.0.0.16/24"},
									Name:       "my-subnet",
								},
								NatGateway: infrav1.NatGateway{
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "foo-natgw",
									},
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetCluster,
									CIDRBlocks: []string{"10.0.0.16/24"},
									Name:       "my-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-routetable"},
								NatGateway: infrav1.NatGateway{
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "foo-natgw",
									},
									NatGatewayIP: infrav1.PublicIPSpec{
										Name: "pip-foo-natgw",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "cluster subnet with subnets specified",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetCluster,
									Name: "cluster-test-subnet",
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetCluster,
									CIDRBlocks: []string{DefaultClusterSubnetCIDR},
									Name:       "cluster-test-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-routetable"},
								NatGateway: infrav1.NatGateway{
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "cluster-test-natgw",
									},
									NatGatewayIP: infrav1.PublicIPSpec{
										Name: "pip-cluster-test-natgw",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets route tables specified",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "cluster-test-controlplane-subnet",
								},
								RouteTable: infrav1.RouteTable{
									Name: "control-plane-custom-route-table",
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "cluster-test-node-subnet",
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetControlPlane,
									CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
									Name:       "cluster-test-controlplane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "control-plane-custom-route-table"},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{DefaultNodeSubnetCIDR},
									Name:       "cluster-test-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-node-routetable"},
								NatGateway: infrav1.NatGateway{
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "cluster-test-node-natgw-1",
									},
									NatGatewayIP: infrav1.PublicIPSpec{
										Name: "pip-cluster-test-node-natgw-1",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "only node subnet specified",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "my-node-subnet",
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{DefaultNodeSubnetCIDR},
									Name:       "my-node-subnet",
								},

								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-node-routetable"},
								NatGateway: infrav1.NatGateway{
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "cluster-test-node-natgw-1",
									},
									NatGatewayIP: infrav1.PublicIPSpec{
										Name: "pip-cluster-test-node-natgw-1",
									},
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetControlPlane,
									CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
									Name:       "cluster-test-controlplane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    infrav1.RouteTable{},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets specified with IPv6 enabled",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							VnetClassSpec: infrav1.VnetClassSpec{
								CIDRBlocks: []string{"2001:be00::1/56"},
							},
						},
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       "control-plane",
									CIDRBlocks: []string{"2001:beef::1/64"},
									Name:       "cluster-test-controlplane-subnet",
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       "node",
									CIDRBlocks: []string{"2001:beea::1/64"},
									Name:       "cluster-test-node-subnet",
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							VnetClassSpec: infrav1.VnetClassSpec{
								CIDRBlocks: []string{"2001:be00::1/56"},
							},
						},
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetControlPlane,
									CIDRBlocks: []string{"2001:beef::1/64"},
									Name:       "cluster-test-controlplane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"2001:beea::1/64"},
									Name:       "cluster-test-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-node-routetable"},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets with custom security group",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: "control-plane",
									Name: "cluster-test-controlplane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{
									SecurityGroupClass: infrav1.SecurityGroupClass{
										SecurityRules: []infrav1.SecurityRule{
											{
												Name:             "allow_port_50000",
												Description:      "allow port 50000",
												Protocol:         "*",
												Priority:         2202,
												SourcePorts:      ptr.To("*"),
												DestinationPorts: ptr.To("*"),
												Source:           ptr.To("*"),
												Destination:      ptr.To("*"),
												Action:           infrav1.SecurityRuleActionAllow,
											},
										},
									},
									Name: "my-custom-sg",
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: "node",
									Name: "cluster-test-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{
									SecurityGroupClass: infrav1.SecurityGroupClass{
										SecurityRules: []infrav1.SecurityRule{
											{
												Name:             "allow_port_50000",
												Description:      "allow port 50000",
												Protocol:         "*",
												Priority:         2202,
												SourcePorts:      ptr.To("*"),
												DestinationPorts: ptr.To("*"),
												Source:           ptr.To("*"),
												Destination:      ptr.To("*"),
												Action:           infrav1.SecurityRuleActionAllow,
											},
										},
									},
									Name: "my-custom-node-sg",
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       "control-plane",
									CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
									Name:       "cluster-test-controlplane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{
									SecurityGroupClass: infrav1.SecurityGroupClass{
										SecurityRules: []infrav1.SecurityRule{
											{
												Name:             "allow_port_50000",
												Description:      "allow port 50000",
												Protocol:         "*",
												Priority:         2202,
												SourcePorts:      ptr.To("*"),
												DestinationPorts: ptr.To("*"),
												Source:           ptr.To("*"),
												Destination:      ptr.To("*"),
												Direction:        infrav1.SecurityRuleDirectionInbound,
												Action:           infrav1.SecurityRuleActionAllow,
											},
										},
									},
									Name: "my-custom-sg",
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{DefaultNodeSubnetCIDR},
									Name:       "cluster-test-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{
									Name: "my-custom-node-sg",
									SecurityGroupClass: infrav1.SecurityGroupClass{
										SecurityRules: []infrav1.SecurityRule{
											{
												Name:             "allow_port_50000",
												Description:      "allow port 50000",
												Protocol:         "*",
												Priority:         2202,
												SourcePorts:      ptr.To("*"),
												DestinationPorts: ptr.To("*"),
												Source:           ptr.To("*"),
												Destination:      ptr.To("*"),
												Direction:        infrav1.SecurityRuleDirectionInbound,
												Action:           infrav1.SecurityRuleActionAllow,
											},
										},
									},
								},
								RouteTable: infrav1.RouteTable{Name: "cluster-test-node-routetable"},
								NatGateway: infrav1.NatGateway{
									NatGatewayIP: infrav1.PublicIPSpec{
										Name: "pip-cluster-test-node-natgw-1",
									},
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "cluster-test-node-natgw-1",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets with custom security group to deny port 49999",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: "control-plane",
									Name: "cluster-test-controlplane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{
									SecurityGroupClass: infrav1.SecurityGroupClass{
										SecurityRules: []infrav1.SecurityRule{
											{
												Name:             "deny_port_49999",
												Description:      "deny port 49999",
												Protocol:         "*",
												Priority:         2201,
												SourcePorts:      ptr.To("*"),
												DestinationPorts: ptr.To("*"),
												Source:           ptr.To("*"),
												Destination:      ptr.To("*"),
												Action:           infrav1.SecurityRuleActionDeny,
											},
										},
									},
									Name: "my-custom-sg",
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       "control-plane",
									CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
									Name:       "cluster-test-controlplane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{
									SecurityGroupClass: infrav1.SecurityGroupClass{
										SecurityRules: []infrav1.SecurityRule{
											{
												Name:             "deny_port_49999",
												Description:      "deny port 49999",
												Protocol:         "*",
												Priority:         2201,
												SourcePorts:      ptr.To("*"),
												DestinationPorts: ptr.To("*"),
												Source:           ptr.To("*"),
												Destination:      ptr.To("*"),
												Direction:        infrav1.SecurityRuleDirectionInbound,
												Action:           infrav1.SecurityRuleActionDeny,
											},
										},
									},
									Name: "my-custom-sg",
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{DefaultNodeSubnetCIDR},
									Name:       "cluster-test-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-node-routetable"},
								NatGateway: infrav1.NatGateway{
									NatGatewayIP: infrav1.PublicIPSpec{
										Name: "",
									},
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "cluster-test-node-natgw",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "don't default NAT Gateway if subnet already exists",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "cluster-test-controlplane-subnet",
								},
								ID: "my-subnet-id",
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "cluster-test-node-subnet",
								},
								ID: "my-subnet-id-2",
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetControlPlane,
									CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
									Name:       "cluster-test-controlplane-subnet",
								},
								ID:            "my-subnet-id",
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-controlplane-nsg"},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{DefaultNodeSubnetCIDR},
									Name:       "cluster-test-node-subnet",
								},
								ID:            "my-subnet-id-2",
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-node-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-node-routetable"},
								NatGateway: infrav1.NatGateway{
									NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
										Name: "",
									},
									NatGatewayIP: infrav1.PublicIPSpec{
										Name: "",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "don't default NAT Gateway for cluster subnet if subnet already exists",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetCluster,
									Name: "cluster-test-cluster-subnet",
								},
								ID: "my-subnet-id",
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetCluster,
									CIDRBlocks: []string{DefaultClusterSubnetCIDR},
									Name:       "cluster-test-cluster-subnet",
								},
								ID:            "my-subnet-id",
								SecurityGroup: infrav1.SecurityGroup{Name: "cluster-test-nsg"},
								RouteTable:    infrav1.RouteTable{Name: "cluster-test-routetable"},
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
			setDefaultAzureClusterSubnets(tc.cluster)
			if !reflect.DeepEqual(tc.cluster, tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(tc.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestVnetPeeringDefaults(t *testing.T) {
	cases := []struct {
		name    string
		cluster *infrav1.AzureCluster
		output  *infrav1.AzureCluster
	}{
		{
			name: "no peering",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{},
				},
			},
		},
		{
			name: "peering with resource group",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "cluster-test",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							Peerings: infrav1.VnetPeerings{
								{
									VnetPeeringClassSpec: infrav1.VnetPeeringClassSpec{
										RemoteVnetName: "my-vnet",
										ResourceGroup:  "cluster-test",
									},
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "cluster-test",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							Peerings: infrav1.VnetPeerings{
								{
									VnetPeeringClassSpec: infrav1.VnetPeeringClassSpec{
										RemoteVnetName: "my-vnet",
										ResourceGroup:  "cluster-test",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "peering without resource group",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "cluster-test",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							Peerings: infrav1.VnetPeerings{
								{
									VnetPeeringClassSpec: infrav1.VnetPeeringClassSpec{RemoteVnetName: "my-vnet"},
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "cluster-test",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							Peerings: infrav1.VnetPeerings{
								{
									VnetPeeringClassSpec: infrav1.VnetPeeringClassSpec{
										RemoteVnetName: "my-vnet",
										ResourceGroup:  "cluster-test",
									},
								},
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
			setDefaultAzureClusterVnetPeering(tc.cluster)
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
		name        string
		featureGate featuregate.Feature
		cluster     *infrav1.AzureCluster
		output      *infrav1.AzureCluster
	}{
		{
			name: "no lb",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec:         infrav1.NetworkSpec{},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "cluster-test-public-lb",
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-public-lb-frontEnd",
									PublicIP: &infrav1.PublicIPSpec{
										Name:    "pip-cluster-test-apiserver",
										DNSName: "",
									},
								},
							},
							BackendPool: infrav1.BackendPool{
								Name: "cluster-test-public-lb-backendPool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
						},
					},
				},
			},
		},
		{
			name:        "no lb with APIServerILB feature gate enabled",
			featureGate: feature.APIServerILB,
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec:         infrav1.NetworkSpec{},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "cluster-test-public-lb",
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-public-lb-frontEnd",
									PublicIP: &infrav1.PublicIPSpec{
										Name:    "pip-cluster-test-apiserver",
										DNSName: "",
									},
								},
								{
									Name: "cluster-test-public-lb-frontEnd-internal-ip",
									FrontendIPClass: infrav1.FrontendIPClass{
										PrivateIPAddress: DefaultInternalLBIPAddress,
									},
								},
							},
							BackendPool: infrav1.BackendPool{
								Name: "cluster-test-public-lb-backendPool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
						},
					},
				},
			},
		},
		{
			name: "internal lb",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-internal-lb-frontEnd",
									FrontendIPClass: infrav1.FrontendIPClass{
										PrivateIPAddress: DefaultInternalLBIPAddress,
									},
								},
							},
							BackendPool: infrav1.BackendPool{
								Name: "cluster-test-internal-lb-backendPool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Internal,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
							Name: "cluster-test-internal-lb",
						},
					},
				},
			},
		},
		{
			name:        "internal lb with feature gate API Server ILB enabled",
			featureGate: feature.APIServerILB,
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-internal-lb-frontEnd",
									FrontendIPClass: infrav1.FrontendIPClass{
										PrivateIPAddress: DefaultInternalLBIPAddress,
									},
								},
							},
							BackendPool: infrav1.BackendPool{
								Name: "cluster-test-internal-lb-backendPool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Internal,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
							Name: "cluster-test-internal-lb",
						},
					},
				},
			},
		},
		{
			name: "with custom backend pool name",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
							BackendPool: infrav1.BackendPool{
								Name: "custom-backend-pool",
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-internal-lb-frontEnd",
									FrontendIPClass: infrav1.FrontendIPClass{
										PrivateIPAddress: DefaultInternalLBIPAddress,
									},
								},
							},
							BackendPool: infrav1.BackendPool{
								Name: "custom-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Internal,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
							Name: "cluster-test-internal-lb",
						},
					},
				},
			},
		},
		{
			name:        "with custom backend pool name with feature gate API Server ILB enabled",
			featureGate: feature.APIServerILB,
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
							BackendPool: infrav1.BackendPool{
								Name: "custom-backend-pool",
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-internal-lb-frontEnd",
									FrontendIPClass: infrav1.FrontendIPClass{
										PrivateIPAddress: DefaultInternalLBIPAddress,
									},
								},
							},
							BackendPool: infrav1.BackendPool{
								Name: "custom-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Internal,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
							Name: "cluster-test-internal-lb",
						},
					},
				},
			},
		},
		{
			name:        "public lb with APIServerILB feature gate enabled and custom private IP belonging to default control plane CIDR",
			featureGate: feature.APIServerILB,
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "cluster-test-public-lb",
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-public-lb-frontEnd",
									PublicIP: &infrav1.PublicIPSpec{
										Name:    "pip-cluster-test-apiserver",
										DNSName: "",
									},
								},
								{
									Name: "my-internal-ip",
									FrontendIPClass: infrav1.FrontendIPClass{
										PrivateIPAddress: "10.0.0.111",
									},
								},
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
								SKU:  infrav1.SKUStandard,
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "cluster-test-public-lb",
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-public-lb-frontEnd",
									PublicIP: &infrav1.PublicIPSpec{
										Name:    "pip-cluster-test-apiserver",
										DNSName: "",
									},
								},
								{
									Name: "my-internal-ip",
									FrontendIPClass: infrav1.FrontendIPClass{
										PrivateIPAddress: "10.0.0.111",
									},
								},
							},
							BackendPool: infrav1.BackendPool{
								Name: "cluster-test-public-lb-backendPool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
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
			if tc.featureGate != "" {
				featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, tc.featureGate, true)
			}
			setDefaultAzureClusterAPIServerLB(tc.cluster)
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
		cluster *infrav1.AzureCluster
		output  *infrav1.AzureCluster
	}{
		"default empty azure env": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AzureEnvironment: DefaultAzureCloud,
					},
				},
			},
		},
		"azure env set to AzurePublicCloud": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AzureEnvironment: DefaultAzureCloud,
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AzureEnvironment: DefaultAzureCloud,
					},
				},
			},
		},
		"azure env set to AzureGermanCloud": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AzureEnvironment: "AzureGermanCloud",
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AzureEnvironment: "AzureGermanCloud",
					},
				},
			},
		},
	}

	for name := range cases {
		c := cases[name]
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			SetDefaultAzureClusterAzureEnvironment(c.cluster)
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
		cluster *infrav1.AzureCluster
		output  *infrav1.AzureCluster
	}{
		{
			name: "default no lb for public clusters",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Public}},
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
							},
						},
					},
				},
			},
		},
		{
			name: "IPv6 enabled",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Public}},
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       "node",
									CIDRBlocks: []string{"2001:beea::1/64"},
									Name:       "cluster-test-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       "node",
									CIDRBlocks: []string{"2001:beea::1/64"},
									Name:       "cluster-test-node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
							},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "cluster-test",
							FrontendIPs: []infrav1.FrontendIP{{
								Name: "cluster-test-frontEnd",
								PublicIP: &infrav1.PublicIPSpec{
									Name: "pip-cluster-test-node-outbound",
								},
							}},
							BackendPool: infrav1.BackendPool{
								Name: "cluster-test-outboundBackendPool",
							},
							FrontendIPsCount: ptr.To[int32](1),
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
						},
					},
				},
			},
		},
		{
			name: "IPv6 enabled on 1 of 2 node subnets",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Public}},
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"2001:beea::1/64"},
									Name:       "node-subnet-1",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet-2",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"2001:beea::1/64"},
									Name:       "node-subnet-1",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet-2",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
							},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "cluster-test",
							FrontendIPs: []infrav1.FrontendIP{{
								Name: "cluster-test-frontEnd",
								PublicIP: &infrav1.PublicIPSpec{
									Name: "pip-cluster-test-node-outbound",
								},
							}},
							BackendPool: infrav1.BackendPool{
								Name: "cluster-test-outboundBackendPool",
							},
							FrontendIPsCount: ptr.To[int32](1),
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
						},
					},
				},
			},
		},
		{
			name: "multiple node subnets, IPv6 not enabled in any of them",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Public}},
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet-2",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet-3",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet-2",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet-3",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
							},
						},
					},
				},
			},
		},
		{
			name: "multiple node subnets, IPv6 enabled on all of them",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Public}},
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"2001:beea::1/64"},
									Name:       "node-subnet-1",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"2002:beee::1/64"},
									Name:       "node-subnet-2",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"2003:befa::1/64"},
									Name:       "node-subnet-3",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"2001:beea::1/64"},
									Name:       "node-subnet-1",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"2002:beee::1/64"},
									Name:       "node-subnet-2",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role:       infrav1.SubnetNode,
									CIDRBlocks: []string{"2003:befa::1/64"},
									Name:       "node-subnet-3",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
							},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "cluster-test",
							FrontendIPs: []infrav1.FrontendIP{{
								Name: "cluster-test-frontEnd",
								PublicIP: &infrav1.PublicIPSpec{
									Name: "pip-cluster-test-node-outbound",
								},
							}},
							BackendPool: infrav1.BackendPool{
								Name: "cluster-test-outboundBackendPool",
							},
							FrontendIPsCount: ptr.To[int32](1),
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
						},
					},
				},
			},
		},
		{
			name: "no lb for private clusters",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Internal}},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
		},
		{
			name: "NodeOutboundLB declared as input with non-default IdleTimeoutInMinutes, FrontendIPsCount, BackendPool values",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Public}},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							FrontendIPsCount: ptr.To[int32](2),
							BackendPool: infrav1.BackendPool{
								Name: "custom-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								IdleTimeoutInMinutes: ptr.To[int32](15),
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
							},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-frontEnd-1",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "pip-cluster-test-node-outbound-1",
									},
								},
								{
									Name: "cluster-test-frontEnd-2",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "pip-cluster-test-node-outbound-2",
									},
								},
							},
							BackendPool: infrav1.BackendPool{
								Name: "custom-backend-pool",
							},
							FrontendIPsCount: ptr.To[int32](2), // we expect the original value to be respected here
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](15), // we expect the original value to be respected here
							},
							Name: "cluster-test",
						},
					},
				},
			},
		},
		{
			name: "ensure that existing lb names are not overwritten",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "user-defined-name",
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
							},
						},
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "user-defined-name",
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "user-defined-name",
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: "control-plane-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node-subnet",
								},
								SecurityGroup: infrav1.SecurityGroup{},
								RouteTable:    infrav1.RouteTable{},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "user-defined-name",
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
							},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "user-defined-name",
							FrontendIPs: []infrav1.FrontendIP{{
								Name: "user-defined-name-frontEnd",
								PublicIP: &infrav1.PublicIPSpec{
									Name: "pip-cluster-test-node-outbound",
								},
							}},
							BackendPool: infrav1.BackendPool{
								Name: "user-defined-name-outboundBackendPool",
							},
							FrontendIPsCount: ptr.To[int32](1),
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
							},
						},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "user-defined-name",
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
			setDefaultAzureClusterNodeOutboundLB(tc.cluster)
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
		cluster *infrav1.AzureCluster
		output  *infrav1.AzureCluster
	}{
		{
			name: "no cp lb for public clusters",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Public}},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
							},
						},
					},
				},
			},
		},
		{
			name: "no cp lb for private clusters",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Internal}},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
		},
		{
			name: "frontendIPsCount > 1",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Internal}},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							FrontendIPsCount: ptr.To[int32](2),
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								IdleTimeoutInMinutes: ptr.To[int32](15),
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "cluster-test-outbound-lb",
							BackendPool: infrav1.BackendPool{
								Name: "cluster-test-outbound-lb-outboundBackendPool",
							},
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-outbound-lb-frontEnd-1",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "pip-cluster-test-controlplane-outbound-1",
									},
								},
								{
									Name: "cluster-test-outbound-lb-frontEnd-2",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "pip-cluster-test-controlplane-outbound-2",
									},
								},
							},
							FrontendIPsCount: ptr.To[int32](2),
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](15),
							},
						},
					},
				},
			},
		},
		{
			name: "custom outbound lb backend pool",
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{Type: infrav1.Internal}},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							BackendPool: infrav1.BackendPool{
								Name: "custom-outbound-lb",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								IdleTimeoutInMinutes: ptr.To[int32](15),
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-test",
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "cluster-test-outbound-lb",
							BackendPool: infrav1.BackendPool{
								Name: "custom-outbound-lb",
							},
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cluster-test-outbound-lb-frontEnd",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "pip-cluster-test-controlplane-outbound",
									},
								},
							},
							FrontendIPsCount: ptr.To[int32](1),
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								SKU:                  infrav1.SKUStandard,
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](15),
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
			setDefaultAzureClusterControlPlaneOutboundLB(tc.cluster)
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
		cluster *infrav1.AzureCluster
		output  *infrav1.AzureCluster
	}{
		"no bastion set": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{},
			},
		},
		"azure bastion enabled with no settings": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							Name: "foo-azure-bastion",
							Subnet: infrav1.SubnetSpec{

								SubnetClassSpec: infrav1.SubnetClassSpec{
									CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
									Role:       DefaultAzureBastionSubnetRole,
									Name:       "AzureBastionSubnet",
								},
							},
							PublicIP: infrav1.PublicIPSpec{
								Name: "foo-azure-bastion-pip",
							},
						},
					},
				},
			},
		},
		"azure bastion enabled with name set": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							Name: "my-fancy-name",
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							Name: "my-fancy-name",
							Subnet: infrav1.SubnetSpec{

								SubnetClassSpec: infrav1.SubnetClassSpec{
									CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
									Role:       DefaultAzureBastionSubnetRole,
									Name:       "AzureBastionSubnet",
								},
							},
							PublicIP: infrav1.PublicIPSpec{
								Name: "foo-azure-bastion-pip",
							},
						},
					},
				},
			},
		},
		"azure bastion enabled with subnet partially set": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							Subnet: infrav1.SubnetSpec{},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							Name: "foo-azure-bastion",
							Subnet: infrav1.SubnetSpec{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
									Role:       DefaultAzureBastionSubnetRole,
									Name:       "AzureBastionSubnet",
								},
							},
							PublicIP: infrav1.PublicIPSpec{
								Name: "foo-azure-bastion-pip",
							},
						},
					},
				},
			},
		},
		"azure bastion enabled with subnet fully set": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							Subnet: infrav1.SubnetSpec{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									CIDRBlocks: []string{"10.10.0.0/16"},
									Name:       "my-superfancy-name",
								},
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							Name: "foo-azure-bastion",
							Subnet: infrav1.SubnetSpec{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									CIDRBlocks: []string{"10.10.0.0/16"},
									Role:       DefaultAzureBastionSubnetRole,
									Name:       "my-superfancy-name",
								},
							},
							PublicIP: infrav1.PublicIPSpec{
								Name: "foo-azure-bastion-pip",
							},
						},
					},
				},
			},
		},
		"azure bastion enabled with public IP name set": {
			cluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							PublicIP: infrav1.PublicIPSpec{
								Name: "my-ultrafancy-pip-name",
							},
						},
					},
				},
			},
			output: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							Name: "foo-azure-bastion",
							Subnet: infrav1.SubnetSpec{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
									Role:       DefaultAzureBastionSubnetRole,
									Name:       "AzureBastionSubnet",
								},
							},
							PublicIP: infrav1.PublicIPSpec{
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
			setDefaultAzureClusterBastion(c.cluster)
			if !reflect.DeepEqual(c.cluster, c.output) {
				expected, _ := json.MarshalIndent(c.output, "", "\t")
				actual, _ := json.MarshalIndent(c.cluster, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}
