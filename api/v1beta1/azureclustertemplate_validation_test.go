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
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func TestValdateVnetCIDRs(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		expectValid     bool
	}{
		{
			name: "valid vnet",
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
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
							},
						},
					},
				},
			},
			expectValid: true,
		},
		{
			name: "invalid vnet CIDR",
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
										CIDRBlocks: []string{"1.2.3/12"},
									},
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			res := validateVnetCIDR(
				tc.clusterTemplate.Spec.Template.Spec.NetworkSpec.Vnet.CIDRBlocks,
				field.NewPath("spec").Child("template").Child("spec").
					Child("networkSpec").Child("vnet").Child("cidrBlocks"))

			if tc.expectValid {
				g.Expect(res).To(BeNil())
			} else {
				g.Expect(res).NotTo(BeNil())
			}
		})
	}
}

func TestValidateSubnetTemplates(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		expectValid     bool
	}{
		{
			name: "valid subnets",
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
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
											Name:       "foo-controlPlane-subnet",
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{DefaultNodeSubnetCIDR},
											Name:       "foo-workerSubnet-subnet",
										},
									},
								},
							},
						},
					},
				},
			},
			expectValid: true,
		},
		{
			name: "invalid subnets - missing/empty subnet name",
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
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
											Name:       "",
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{DefaultNodeSubnetCIDR},
											Name:       "",
										},
									},
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "invalid subnets - duplicate subnet names",
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
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
											Name:       "foo-controlPlane-subnet",
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{DefaultNodeSubnetCIDR},
											Name:       "foo-workerSubnet-subnet",
										},
									},
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetNode,
											CIDRBlocks: []string{"10.2.0.0/16"},
											Name:       "foo-workerSubnet-subnet",
										},
									},
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "invalid subnets - wrong security rule priorities - lower than minimum",
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
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
										},
										SecurityGroup: SecurityGroupClass{
											SecurityRules: SecurityRules{
												SecurityRule{
													Priority: 50,
												},
											},
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
			expectValid: false,
		},
		{
			name: "invalid subnets - wrong security rule priorities - higher than maximum",
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
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{DefaultControlPlaneSubnetCIDR},
										},
										SecurityGroup: SecurityGroupClass{
											SecurityRules: SecurityRules{
												SecurityRule{
													Priority: 4097,
												},
											},
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
			expectValid: false,
		},
		{
			name: "invalid subnet CIDR",
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
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
								Subnets: SubnetTemplatesSpec{
									{
										SubnetClassSpec: SubnetClassSpec{
											Role:       SubnetControlPlane,
											CIDRBlocks: []string{"11.0.0.0/16"},
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
			expectValid: false,
		},
		{
			name: "missing required control plane role",
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
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
								Subnets: SubnetTemplatesSpec{
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
			expectValid: false,
		},
		{
			name: "missing required node role",
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
										CIDRBlocks: []string{DefaultVnetCIDR},
									},
								},
								Subnets: SubnetTemplatesSpec{
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
			expectValid: false,
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			res := validateSubnetTemplates(
				tc.clusterTemplate.Spec.Template.Spec.NetworkSpec.Subnets,
				tc.clusterTemplate.Spec.Template.Spec.NetworkSpec.Vnet,
				field.NewPath("spec").Child("template").Child("spec").Child("networkSpec").Child("subnets"),
			)

			if tc.expectValid {
				g.Expect(res).To(BeNil())
			} else {
				g.Expect(res).NotTo(BeNil())
			}
		})
	}
}

func TestValidateAPIServerLBTemplate(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		expectValid     bool
	}{
		{
			name: "valid lb",
			clusterTemplate: &AzureClusterTemplate{
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
									IdleTimeoutInMinutes: pointer.Int32(DefaultOutboundRuleIdleTimeoutInMinutes),
								},
							},
						},
					},
				},
			},
			expectValid: true,
		},
		{
			name: "invalid lb SKU",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									SKU:                  SKU("wrong"),
									Type:                 Public,
									IdleTimeoutInMinutes: pointer.Int32(DefaultOutboundRuleIdleTimeoutInMinutes),
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "invalid lb Type",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									SKU:                  SKUStandard,
									Type:                 LBType("wrong"),
									IdleTimeoutInMinutes: pointer.Int32(DefaultOutboundRuleIdleTimeoutInMinutes),
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			res := tc.clusterTemplate.validateAPIServerLB(
				field.NewPath("spec").Child("template").Child("spec").Child("networkSpec").Child("apiServerLB"),
			)

			if tc.expectValid {
				g.Expect(res).To(BeNil())
			} else {
				g.Expect(res).NotTo(BeNil())
			}
		})
	}
}

func TestControlPlaneOutboundLBTemplate(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		expectValid     bool
	}{
		{
			name: "valid controlplaneoutbound LB",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									Type: Public,
								},
							},
						},
					},
				},
			},
			expectValid: true,
		},
		{
			name: "invalid controlplaneoutbound LB - should not have a controlplane outbound lb for public clusters",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									Type: Public,
								},
								ControlPlaneOutboundLB: &LoadBalancerClassSpec{},
							},
						},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "invalid controlplaneoutbound LB - IdleTimeoutInMinutes less than minimum for private clusters",
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
								ControlPlaneOutboundLB: &LoadBalancerClassSpec{
									IdleTimeoutInMinutes: pointer.Int32(2),
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "invalid controlplaneoutbound LB - IdleTimeoutInMinutes more than maximum for private clusters",
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
								ControlPlaneOutboundLB: &LoadBalancerClassSpec{
									IdleTimeoutInMinutes: pointer.Int32(60),
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "valid controlplaneoutbound LB - can be empty for private clusters",
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
			expectValid: true,
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			res := tc.clusterTemplate.validateControlPlaneOutboundLB()

			if tc.expectValid {
				g.Expect(res).To(BeNil())
			} else {
				g.Expect(res).NotTo(BeNil())
			}
		})
	}
}

func TestNodeOutboundLBTemplate(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		expectValid     bool
	}{
		{
			name: "cannot be nil for public clusters",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									Type: Public,
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "can be nil for private clusters",
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
			expectValid: true,
		},
		{
			name: "timeout should not be less than minimum",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									Type: Public,
								},
								NodeOutboundLB: &LoadBalancerClassSpec{
									IdleTimeoutInMinutes: pointer.Int32(2),
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "timeout should not be more than maximum",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									Type: Public,
								},
								NodeOutboundLB: &LoadBalancerClassSpec{
									IdleTimeoutInMinutes: pointer.Int32(60),
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			res := tc.clusterTemplate.validateNodeOutboundLB()

			if tc.expectValid {
				g.Expect(res).To(BeNil())
			} else {
				g.Expect(res).NotTo(BeNil())
			}
		})
	}
}

func TestValidatePrivateDNSZoneName(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		expectValid     bool
	}{
		{
			name: "not set",
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
			expectValid: true,
		},
		{
			name: "show not be set if APIServerLB.Type is not internal",
			clusterTemplate: &AzureClusterTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								APIServerLB: LoadBalancerClassSpec{
									Type: Public,
								},
								NetworkClassSpec: NetworkClassSpec{
									PrivateDNSZoneName: "a.b.c",
								},
							},
						},
					},
				},
			},
			expectValid: false,
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			res := tc.clusterTemplate.validatePrivateDNSZoneName()

			if tc.expectValid {
				g.Expect(res).To(BeNil())
			} else {
				g.Expect(res).NotTo(BeNil())
			}
		})
	}
}
func TestValidateNetworkSpec(t *testing.T) {
	cases := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		expectValid     bool
	}{
		{
			name: "subnet with SubnetNode role and enabled IPv6 triggers needOutboundLB and calls validateNodeOutboundLB",
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
			expectValid: false,
		},
		{
			name: "subnet with non-SubnetNode role",
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
											Role:       "SomeOtherRole",
											CIDRBlocks: []string{"10.0.0.0/24"},
										},
									},
								},
							},
						},
					},
				},
			},
			expectValid: true, // No need for outbound LB when not SubnetNode
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			res := tc.clusterTemplate.validateNetworkSpec()
			if tc.expectValid {
				g.Expect(res).To(BeNil())
			} else {
				g.Expect(res).NotTo(BeNil())
			}
		})
	}
}
