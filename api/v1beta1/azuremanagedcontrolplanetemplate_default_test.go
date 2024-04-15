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

package v1beta1

import (
	"encoding/json"
	"reflect"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultVirtualNetworkTemplate(t *testing.T) {
	cases := []struct {
		name                 string
		controlPlaneTemplate *AzureManagedControlPlaneTemplate
		outputTemplate       *AzureManagedControlPlaneTemplate
	}{
		{
			name: "virtual network not specified",
			controlPlaneTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{},
				},
			},
			outputTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{
						Spec: AzureManagedControlPlaneTemplateResourceSpec{
							AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
								VirtualNetwork: ManagedControlPlaneVirtualNetwork{
									ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
										CIDRBlock: defaultAKSVnetCIDR,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "custom cidr block",
			controlPlaneTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{
						Spec: AzureManagedControlPlaneTemplateResourceSpec{
							AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
								VirtualNetwork: ManagedControlPlaneVirtualNetwork{
									ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
										CIDRBlock: "10.0.0.16/24",
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{
						Spec: AzureManagedControlPlaneTemplateResourceSpec{
							AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
								VirtualNetwork: ManagedControlPlaneVirtualNetwork{
									ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
										CIDRBlock: "10.0.0.16/24",
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
			tc.controlPlaneTemplate.setDefaultVirtualNetwork()
			if !reflect.DeepEqual(tc.controlPlaneTemplate, tc.outputTemplate) {
				expected, _ := json.MarshalIndent(tc.outputTemplate, "", "\t")
				actual, _ := json.MarshalIndent(tc.controlPlaneTemplate, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestDefaultSubnetTemplate(t *testing.T) {
	cases := []struct {
		name                 string
		controlPlaneTemplate *AzureManagedControlPlaneTemplate
		outputTemplate       *AzureManagedControlPlaneTemplate
	}{
		{
			name: "subnet not specified",
			controlPlaneTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{},
				},
			},
			outputTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{
						Spec: AzureManagedControlPlaneTemplateResourceSpec{
							AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
								VirtualNetwork: ManagedControlPlaneVirtualNetwork{
									ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
										Subnet: ManagedControlPlaneSubnet{
											Name:      "test-cluster-template",
											CIDRBlock: defaultAKSNodeSubnetCIDR,
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
			name: "custom name",
			controlPlaneTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{
						Spec: AzureManagedControlPlaneTemplateResourceSpec{
							AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
								VirtualNetwork: ManagedControlPlaneVirtualNetwork{
									ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
										Subnet: ManagedControlPlaneSubnet{
											Name: "custom-subnet-name",
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{
						Spec: AzureManagedControlPlaneTemplateResourceSpec{
							AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
								VirtualNetwork: ManagedControlPlaneVirtualNetwork{
									ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
										Subnet: ManagedControlPlaneSubnet{
											Name:      "custom-subnet-name",
											CIDRBlock: defaultAKSNodeSubnetCIDR,
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
			name: "custom cidr block",
			controlPlaneTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{
						Spec: AzureManagedControlPlaneTemplateResourceSpec{
							AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
								VirtualNetwork: ManagedControlPlaneVirtualNetwork{
									ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
										Subnet: ManagedControlPlaneSubnet{
											CIDRBlock: "10.0.0.16/24",
										},
									},
								},
							},
						},
					},
				},
			},
			outputTemplate: &AzureManagedControlPlaneTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-template",
				},
				Spec: AzureManagedControlPlaneTemplateSpec{
					Template: AzureManagedControlPlaneTemplateResource{
						Spec: AzureManagedControlPlaneTemplateResourceSpec{
							AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
								VirtualNetwork: ManagedControlPlaneVirtualNetwork{
									ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
										Subnet: ManagedControlPlaneSubnet{
											Name:      "test-cluster-template",
											CIDRBlock: "10.0.0.16/24",
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
			tc.controlPlaneTemplate.setDefaultSubnet()
			if !reflect.DeepEqual(tc.controlPlaneTemplate, tc.outputTemplate) {
				expected, _ := json.MarshalIndent(tc.outputTemplate, "", "\t")
				actual, _ := json.MarshalIndent(tc.controlPlaneTemplate, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestSetDefault(t *testing.T) {
	g := NewGomegaWithT(t)

	type Struct struct{ name string }

	var s *Struct
	setDefault(&s, &Struct{"hello"})
	g.Expect(s.name).To(Equal("hello"))
	setDefault(&s, &Struct{"world"})
	g.Expect(s.name).To(Equal("hello"))

	r := &Struct{}
	setDefault(&r, &Struct{"a name"})
	g.Expect(r.name).To(BeEmpty())
	setDefault(&r.name, "hello")
	g.Expect(r.name).To(Equal("hello"))
	setDefault(&r.name, "world")
	g.Expect(r.name).To(Equal("hello"))

	str := ""
	setDefault(&str, "a string")
	g.Expect(str).To(Equal("a string"))
	setDefault(&str, "another string")
	g.Expect(str).To(Equal("a string"))
}
