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
	"testing"

	. "github.com/onsi/gomega"
)

func TestAzureClusterTemplate_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name            string
		clusterTemplate *AzureClusterTemplate
		wantErr         bool
	}{
		{
			name: "azureclustertemplate with pre-existing vnet - valid spec",
			clusterTemplate: func() *AzureClusterTemplate {
				return createAzureClusterTemplateFromCluster(createValidCluster())
			}(),
			wantErr: false,
		},
		{
			name: "azureclustertemplate without pre-existing vnet - valid spec",
			clusterTemplate: func() *AzureClusterTemplate {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""
				return createAzureClusterTemplateFromCluster(cluster)
			}(),
			wantErr: false,
		},
		{
			name: "azureclustertemplate with pre-existing vnet - lack control plane subnet",
			clusterTemplate: func() *AzureClusterTemplate {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[1:]
				return createAzureClusterTemplateFromCluster(cluster)
			}(),
			wantErr: true,
		},
		{
			name: "azureclustertemplate with pre-existing vnet - lack node subnet",
			clusterTemplate: func() *AzureClusterTemplate {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[:1]
				return createAzureClusterTemplateFromCluster(cluster)
			}(),
			wantErr: true,
		},
		{
			name: "azureclustertemplate with pre-existing vnet - invalid resourcegroup name",
			clusterTemplate: func() *AzureClusterTemplate {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = "invalid-rg-name###"
				return createAzureClusterTemplateFromCluster(cluster)
			}(),
			wantErr: true,
		},
		{
			name: "azureclustertemplate with pre-existing vnet - invalid subnet name",
			clusterTemplate: func() *AzureClusterTemplate {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = append(cluster.Spec.NetworkSpec.Subnets,
					SubnetSpec{Name: "invalid-subnet-name###", Role: "random-role"})
				return createAzureClusterTemplateFromCluster(cluster)
			}(),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.clusterTemplate.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureClusterTemplate_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name        string
		template    *AzureClusterTemplate
		oldTemplate *AzureClusterTemplate
		wantErr     bool
	}{
		{
			name: "AzureClusterTemplate with immutatble spec",
			template: &AzureClusterTemplate{
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterSpec{
							NetworkSpec: NetworkSpec{
								Vnet: VnetSpec{
									ID: "1234",
								},
							},
						},
					},
				},
			},
			oldTemplate: &AzureClusterTemplate{
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterSpec{
							NetworkSpec: NetworkSpec{
								Vnet: VnetSpec{
									ID: "1234",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureClusterTemplate with mutating spec",
			template: &AzureClusterTemplate{
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterSpec{
							NetworkSpec: NetworkSpec{
								Vnet: VnetSpec{
									ID: "1234",
								},
							},
						},
					},
				},
			},
			oldTemplate: &AzureClusterTemplate{
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterSpec{
							NetworkSpec: NetworkSpec{
								Vnet: VnetSpec{
									ID: "1234567",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := test.template.ValidateUpdate(test.oldTemplate)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createAzureClusterTemplateFromCluster(cluster *AzureCluster) *AzureClusterTemplate {
	return &AzureClusterTemplate{
		Spec: AzureClusterTemplateSpec{
			Template: AzureClusterTemplateResource{
				Spec: cluster.Spec,
			},
		},
	}
}
