/*
Copyright 2022 The Kubernetes Authors.

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
)

func TestVnetTemplateDefaults(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		outputTemplate  *AzureClusterTemplate
	}{
		{
			name: "vnet not specified",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Vnet: VnetTemplateSpec{
									VnetClassSpec: VnetClassSpec{
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Vnet: VnetTemplateSpec{
									VnetClassSpec: VnetClassSpec{
										CIDRBlocks: []string{"10.0.0.0/16"},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Vnet: VnetTemplateSpec{
									VnetClassSpec: VnetClassSpec{
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Vnet: VnetTemplateSpec{
									VnetClassSpec: VnetClassSpec{
										CIDRBlocks: []string{"2001:1234:5678:9a00::/56"},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Vnet: VnetTemplateSpec{
									VnetClassSpec: VnetClassSpec{
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
			tc.clusterTemplate.setVnetTemplateDefaults()
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
		clusterTemplate *AzureClusterTemplate
		outputTemplate  *AzureClusterTemplate
	}{
		{
			name: "no subnets",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{"10.0.0.16/24"},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: NatGatewayClassSpec{
											Name: "foo-natgw",
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{"10.0.0.16/24"},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: NatGatewayClassSpec{Name: "foo-natgw"},
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{"10.0.0.16/24"},
										},
										SecurityGroup: SecurityGroupClass{
											SecurityRules: []SecurityRule{
												{
													Name:             "allow_port_50000",
													Description:      "allow port 50000",
													Protocol:         "*",
													Priority:         2202,
													SourcePorts:      ptr.To("*"),
													DestinationPorts: ptr.To("*"),
													Source:           ptr.To("*"),
													Destination:      ptr.To("*"),
													Action:           SecurityRuleActionAllow,
												},
											},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: NatGatewayClassSpec{
											Name: "foo-natgw",
										},
										SecurityGroup: SecurityGroupClass{
											SecurityRules: []SecurityRule{
												{
													Name:             "allow_port_50000",
													Description:      "allow port 50000",
													Protocol:         "*",
													Priority:         2202,
													SourcePorts:      ptr.To("*"),
													DestinationPorts: ptr.To("*"),
													Source:           ptr.To("*"),
													Destination:      ptr.To("*"),
													Action:           SecurityRuleActionAllow,
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
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{"10.0.0.16/24"},
										},
										SecurityGroup: SecurityGroupClass{
											SecurityRules: SecurityRules{
												{
													Name:             "allow_port_50000",
													Description:      "allow port 50000",
													Protocol:         "*",
													Priority:         2202,
													SourcePorts:      ptr.To("*"),
													DestinationPorts: ptr.To("*"),
													Source:           ptr.To("*"),
													Destination:      ptr.To("*"),
													Direction:        SecurityRuleDirectionInbound,
													Action:           SecurityRuleActionAllow,
												},
											},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"10.1.0.16/24"},
										},
										NatGateway: NatGatewayClassSpec{Name: "foo-natgw"},
										SecurityGroup: SecurityGroupClass{
											SecurityRules: SecurityRules{
												{
													Name:             "allow_port_50000",
													Description:      "allow port 50000",
													Protocol:         "*",
													Priority:         2202,
													SourcePorts:      ptr.To("*"),
													DestinationPorts: ptr.To("*"),
													Source:           ptr.To("*"),
													Destination:      ptr.To("*"),
													Direction:        SecurityRuleDirectionInbound,
													Action:           SecurityRuleActionAllow,
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
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
			name: "only node subnet specified",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{DefaultNodeSubnetCIDR},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{"2001:beef::1/64"},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{"2001:beef::1/64"},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
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
			tc.clusterTemplate.setSubnetsTemplateDefaults()
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
		clusterTemplate *AzureClusterTemplate
		outputTemplate  *AzureClusterTemplate
	}{
		{
			name: "no lb",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									SKU:                  SKUStandard,
									Type:                 Public,
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									Type: Internal,
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									SKU:                  SKUStandard,
									Type:                 Internal,
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
			tc.clusterTemplate.Spec.Template.Spec.NetworkSpec.APIServerLB.setAPIServerLBDefaults()
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
		clusterTemplate *AzureClusterTemplate
		outputTemplate  *AzureClusterTemplate
	}{
		{
			name: "default no lb for public clusters",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
								},
								NodeOutboundLB: &LoadBalancerClassSpec{
									SKU:                  SKUStandard,
									Type:                 Public,
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
										},
									},
								},
								NodeOutboundLB: &LoadBalancerClassSpec{
									SKU:                  SKUStandard,
									Type:                 Public,
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"2002:beeb::1/64"},
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"2001:beea::1/64"},
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"2002:beeb::1/64"},
										},
									},
								},
								NodeOutboundLB: &LoadBalancerClassSpec{
									SKU:                  SKUStandard,
									Type:                 Public,
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetControlPlane,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role: SubnetNode,
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Internal},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Internal},
							},
						},
					},
				},
			},
		},
		{
			name: "NodeOutboundLB declared as input with non-default IdleTimeoutInMinutes",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Internal},
								NodeOutboundLB: &LoadBalancerClassSpec{
									IdleTimeoutInMinutes: ptr.To[int32](15),
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Internal},
								NodeOutboundLB: &LoadBalancerClassSpec{
									IdleTimeoutInMinutes: ptr.To[int32](15),
									SKU:                  SKUStandard,
									Type:                 Public,
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
			tc.clusterTemplate.setNodeOutboundLBDefaults()
			if !reflect.DeepEqual(tc.clusterTemplate, tc.outputTemplate) {
				expected, _ := json.MarshalIndent(tc.outputTemplate, "", "\t")
				actual, _ := json.MarshalIndent(tc.clusterTemplate, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestControlPlaneOutboundLBTemplateDefaults(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		outputTemplate  *AzureClusterTemplate
	}{
		{
			name: "no cp lb for public clusters",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Public},
							},
						},
					},
				},
			},
		},
		{
			name: "no cp lb for private clusters",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Internal},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{Type: Internal},
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
			setControlPlaneOutboundLBDefaults(
				tc.clusterTemplate.Spec.Template.Spec.NetworkSpec.ControlPlaneOutboundLB,
				tc.clusterTemplate.Spec.Template.Spec.NetworkSpec.APIServerLB.Type,
			)
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
		clusterTemplate *AzureClusterTemplate
		outputTemplate  *AzureClusterTemplate
	}{
		{
			name: "no bastion set",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{},
					},
				},
			},
		},
		{
			name: "azure bastion enabled with no settings",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							BastionSpec: BastionTemplateSpec{
								AzureBastion: &AzureBastionTemplateSpec{},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							BastionSpec: BastionTemplateSpec{
								AzureBastion: &AzureBastionTemplateSpec{
									Subnet: SubnetTemplateSpec{
										SubnetClassSpec: SubnetClassSpec{
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							BastionSpec: BastionTemplateSpec{
								AzureBastion: &AzureBastionTemplateSpec{
									Subnet: SubnetTemplateSpec{
										SubnetClassSpec: SubnetClassSpec{},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							BastionSpec: BastionTemplateSpec{
								AzureBastion: &AzureBastionTemplateSpec{
									Subnet: SubnetTemplateSpec{
										SubnetClassSpec: SubnetClassSpec{
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
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							BastionSpec: BastionTemplateSpec{
								AzureBastion: &AzureBastionTemplateSpec{
									Subnet: SubnetTemplateSpec{
										SubnetClassSpec: SubnetClassSpec{
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
			outputTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							BastionSpec: BastionTemplateSpec{
								AzureBastion: &AzureBastionTemplateSpec{
									Subnet: SubnetTemplateSpec{
										SubnetClassSpec: SubnetClassSpec{
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
			tc.clusterTemplate.setBastionTemplateDefaults()
			if !reflect.DeepEqual(tc.clusterTemplate, tc.outputTemplate) {
				expected, _ := json.MarshalIndent(tc.outputTemplate, "", "\t")
				actual, _ := json.MarshalIndent(tc.clusterTemplate, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}
