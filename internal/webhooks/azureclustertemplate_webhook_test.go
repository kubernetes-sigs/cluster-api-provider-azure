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

package webhooks

import (
	"testing"

	. "github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestValidateUpdate(t *testing.T) {
	oldClusterTemplate := &infrav1.AzureClusterTemplate{
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
	}

	newClusterTemplate := &infrav1.AzureClusterTemplate{
		Spec: infrav1.AzureClusterTemplateSpec{
			Template: infrav1.AzureClusterTemplateResource{
				Spec: infrav1.AzureClusterTemplateResourceSpec{
					NetworkSpec: infrav1.NetworkTemplateSpec{
						Vnet: infrav1.VnetTemplateSpec{
							VnetClassSpec: infrav1.VnetClassSpec{
								CIDRBlocks: []string{"11.0.0.0/16"},
							},
						},
					},
				},
			},
		},
	}

	t.Run("template is immutable", func(t *testing.T) {
		g := NewWithT(t)
		_, err := (&AzureClusterTemplateWebhook{}).ValidateUpdate(t.Context(), oldClusterTemplate, newClusterTemplate)
		g.Expect(err).To(HaveOccurred())
	})
}
