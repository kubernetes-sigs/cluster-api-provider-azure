/*
Copyright The Kubernetes Authors.

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

package v1beta1

import (
	"encoding/json"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestVnetTemplateDefaults(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *infrav1.AzureClusterTemplate
		outputTemplate  *infrav1.AzureClusterTemplate
	}{
		{
			name: "vnet not specified",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Vnet: infrav1.VnetTemplateSpec{
									VnetClassSpec: infrav1.VnetClassSpec{
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "custom CIDR",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Vnet: infrav1.VnetTemplateSpec{
									VnetClassSpec: infrav1.VnetClassSpec{
										CIDRBlocks: []string{"10.0.0.0/16"},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Vnet: infrav1.VnetTemplateSpec{
									VnetClassSpec: infrav1.VnetClassSpec{
										CIDRBlocks: []string{"10.0.0.0/16"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "IPv6 enabled",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Vnet: infrav1.VnetTemplateSpec{
									VnetClassSpec: infrav1.VnetClassSpec{
										CIDRBlocks: []string{"2001:1234:5678:9a00::/56"},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Vnet: infrav1.VnetTemplateSpec{
									VnetClassSpec: infrav1.VnetClassSpec{
										CIDRBlocks: []string{"2001:1234:5678:9a00::/56"},
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
			setDefaultAzureClusterTemplateVnetTemplate(tc.clusterTemplate)
			if !reflect.DeepEqual(tc.clusterTemplate, tc.outputTemplate) {
				expected, _ := json.MarshalIndent(tc.outputTemplate, "", "\t")
				actual, _ := json.MarshalIndent(tc.clusterTemplate, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestSubnetsTemplateDefaults(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *infrav1.AzureClusterTemplate
		outputTemplate  *infrav1.AzureClusterTemplate
	}{
		{
			name: "no subnets",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{DefaultNodeSubnetCIDR},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "subnet with custom attributes",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetControlPlane,
											CIDRBlocks: []string{"10.0.0.16/24"},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: infrav1.NatGatewayClassSpec{
											Name: "foo-natgw",
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetControlPlane,
											CIDRBlocks: []string{"10.0.0.16/24"},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: infrav1.NatGatewayClassSpec{Name: "foo-natgw"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "subnet with custom attributes and security groups",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetControlPlane,
											CIDRBlocks: []string{"10.0.0.16/24"},
										},
										SecurityGroup: infrav1.SecurityGroupClass{
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
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: infrav1.NatGatewayClassSpec{
											Name: "foo-natgw",
										},
										SecurityGroup: infrav1.SecurityGroupClass{
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
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetControlPlane,
											CIDRBlocks: []string{"10.0.0.16/24"},
										},
										SecurityGroup: infrav1.SecurityGroupClass{
											SecurityRules: infrav1.SecurityRules{
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
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: infrav1.NatGatewayClassSpec{Name: "foo-natgw"},
										SecurityGroup: infrav1.SecurityGroupClass{
											SecurityRules: infrav1.SecurityRules{
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
								},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets specified",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{DefaultNodeSubnetCIDR},
										},
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
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetCluster,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: infrav1.NatGatewayClassSpec{
											Name: "foo-natgw",
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetCluster,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: infrav1.NatGatewayClassSpec{Name: "foo-natgw"},
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
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetCluster,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetCluster,
											CIDRBlocks: []string{DefaultClusterSubnetCIDR},
										},
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
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{DefaultNodeSubnetCIDR},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "subnets specified with IPv6 enabled",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetControlPlane,
											CIDRBlocks: []string{"2001:beef::1/64"},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetControlPlane,
											CIDRBlocks: []string{"2001:beef::1/64"},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
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
			setDefaultAzureClusterTemplateSubnetsTemplate(tc.clusterTemplate)
			if !reflect.DeepEqual(tc.clusterTemplate, tc.outputTemplate) {
				expected, _ := json.MarshalIndent(tc.outputTemplate, "", "\t")
				actual, _ := json.MarshalIndent(tc.clusterTemplate, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestAPIServerLBClassDefaults(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *infrav1.AzureClusterTemplate
		outputTemplate  *infrav1.AzureClusterTemplate
	}{
		{
			name: "no lb",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{
									SKU:                  infrav1.SKUStandard,
									Type:                 infrav1.Public,
									IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "internal lb",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{
									Type: infrav1.Internal,
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{
									SKU:                  infrav1.SKUStandard,
									Type:                 infrav1.Internal,
									IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
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
			setDefaultLoadBalancerClassSpecAPIServerLB(&tc.clusterTemplate.Spec.Template.Spec.NetworkSpec.APIServerLB)
			if !reflect.DeepEqual(tc.clusterTemplate, tc.outputTemplate) {
				expected, _ := json.MarshalIndent(tc.outputTemplate, "", "\t")
				actual, _ := json.MarshalIndent(tc.clusterTemplate, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestNodeOutboundLBClassDefaults(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *infrav1.AzureClusterTemplate
		outputTemplate  *infrav1.AzureClusterTemplate
	}{
		{
			name: "default no lb for public clusters",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "default lb when IPv6 enabled",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
								},
								NodeOutboundLB: &infrav1.LoadBalancerClassSpec{
									SKU:                  infrav1.SKUStandard,
									Type:                 infrav1.Public,
									IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "IPv6 enabled on 1 of 2 node subnets",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
								},
								NodeOutboundLB: &infrav1.LoadBalancerClassSpec{
									SKU:                  infrav1.SKUStandard,
									Type:                 infrav1.Public,
									IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple subnets specified with IPv6 enabled",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2002:beeb::1/64"},
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       infrav1.SubnetNode,
											CIDRBlocks: []string{"2002:beeb::1/64"},
										},
									},
								},
								NodeOutboundLB: &infrav1.LoadBalancerClassSpec{
									SKU:                  infrav1.SKUStandard,
									Type:                 infrav1.Public,
									IdleTimeoutInMinutes: ptr.To[int32](DefaultOutboundRuleIdleTimeoutInMinutes),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple node subnets, Ipv6 not enabled in any of them",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Public},
								Subnets: infrav1.SubnetTemplatesSpec{
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
									{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role: infrav1.SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "no LB for private clusters",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Internal},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Internal},
							},
						},
					},
				},
			},
		},
		{
			name: "NodeOutboundLB declared as input with non-default IdleTimeoutInMinutes",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Internal},
								NodeOutboundLB: &infrav1.LoadBalancerClassSpec{
									IdleTimeoutInMinutes: ptr.To[int32](15),
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							NetworkSpec: infrav1.NetworkTemplateSpec{
								APIServerLB: infrav1.LoadBalancerClassSpec{Type: infrav1.Internal},
								NodeOutboundLB: &infrav1.LoadBalancerClassSpec{
									IdleTimeoutInMinutes: ptr.To[int32](15),
									SKU:                  infrav1.SKUStandard,
									Type:                 infrav1.Public,
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
			setDefaultAzureClusterTemplateNodeOutboundLB(tc.clusterTemplate)
			if !reflect.DeepEqual(tc.clusterTemplate, tc.outputTemplate) {
				expected, _ := json.MarshalIndent(tc.outputTemplate, "", "\t")
				actual, _ := json.MarshalIndent(tc.clusterTemplate, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestBastionTemplateDefaults(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *infrav1.AzureClusterTemplate
		outputTemplate  *infrav1.AzureClusterTemplate
	}{
		{
			name: "no bastion set",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{},
					},
				},
			},
		},
		{
			name: "azure bastion enabled with no settings",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							BastionSpec: infrav1.BastionTemplateSpec{
								AzureBastion: &infrav1.AzureBastionTemplateSpec{},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							BastionSpec: infrav1.BastionTemplateSpec{
								AzureBastion: &infrav1.AzureBastionTemplateSpec{
									Subnet: infrav1.SubnetTemplateSpec{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       DefaultAzureBastionSubnetRole,
											CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "azure bastion enabled with subnet partially set",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							BastionSpec: infrav1.BastionTemplateSpec{
								AzureBastion: &infrav1.AzureBastionTemplateSpec{
									Subnet: infrav1.SubnetTemplateSpec{
										SubnetClassSpec: infrav1.SubnetClassSpec{},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							BastionSpec: infrav1.BastionTemplateSpec{
								AzureBastion: &infrav1.AzureBastionTemplateSpec{
									Subnet: infrav1.SubnetTemplateSpec{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       DefaultAzureBastionSubnetRole,
											CIDRBlocks: []string{DefaultAzureBastionSubnetCIDR},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "azure bastion enabled with subnet fully set",
			clusterTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							BastionSpec: infrav1.BastionTemplateSpec{
								AzureBastion: &infrav1.AzureBastionTemplateSpec{
									Subnet: infrav1.SubnetTemplateSpec{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       DefaultAzureBastionSubnetRole,
											CIDRBlocks: []string{"10.10.0.0/16"},
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &infrav1.AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: infrav1.AzureClusterTemplateSpec{
					Template: infrav1.AzureClusterTemplateResource{
						Spec: infrav1.AzureClusterTemplateResourceSpec{
							BastionSpec: infrav1.BastionTemplateSpec{
								AzureBastion: &infrav1.AzureBastionTemplateSpec{
									Subnet: infrav1.SubnetTemplateSpec{
										SubnetClassSpec: infrav1.SubnetClassSpec{
											Role:       DefaultAzureBastionSubnetRole,
											CIDRBlocks: []string{"10.10.0.0/16"},
										},
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
			setDefaultAzureClusterTemplateBastionTemplate(tc.clusterTemplate)
			if !reflect.DeepEqual(tc.clusterTemplate, tc.outputTemplate) {
				expected, _ := json.MarshalIndent(tc.outputTemplate, "", "\t")
				actual, _ := json.MarshalIndent(tc.clusterTemplate, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}
