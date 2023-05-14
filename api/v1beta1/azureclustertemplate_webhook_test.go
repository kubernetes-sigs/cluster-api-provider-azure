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
	"k8s.io/utils/pointer"
)

func TestValidateUpdate(t *testing.T) {
	tests := []struct {
		name               string
		oldClusterTemplate *AzureClusterTemplate
		newClusterTemplate *AzureClusterTemplate
		wantErr            bool
	}{
		{
			name: "AzureClusterTemplate is immutable",
			oldClusterTemplate: &AzureClusterTemplate{
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
			newClusterTemplate: &AzureClusterTemplate{
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Vnet: VnetTemplateSpec{
									VnetClassSpec: VnetClassSpec{
										CIDRBlocks: []string{"11.0.0.0/16"},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "NodeOutboundLB will be updated per the new one",
			oldClusterTemplate: &AzureClusterTemplate{
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								NodeOutboundLB: &LoadBalancerClassSpec{
									IdleTimeoutInMinutes: pointer.Int32(DefaultOutboundRuleIdleTimeoutInMinutes),
									SKU:                  SKUStandard,
									Type:                 Public,
								},
							},
						},
					},
				},
			},
			newClusterTemplate: &AzureClusterTemplate{
				Spec: AzureClusterTemplateSpec{
					Template: AzureClusterTemplateResource{
						Spec: AzureClusterTemplateResourceSpec{
							NetworkSpec: NetworkTemplateSpec{
								Vnet: VnetTemplateSpec{},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	t.Run("template is immutable", func(t *testing.T) {
		g := NewWithT(t)
		for _, act := range tests {
			act := act
			t.Run(act.name, func(t *testing.T) {
				err := act.newClusterTemplate.ValidateUpdate(act.oldClusterTemplate)
				if act.wantErr {
					g.Expect(err).To(HaveOccurred())
				} else {
					g.Expect(err).NotTo(HaveOccurred())
				}
			})
		}

	})
}
