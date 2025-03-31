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
	"context"
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateUpdate(t *testing.T) {
	oldClusterTemplate := &AzureClusterTemplate{
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
	}

	newClusterTemplate := &AzureClusterTemplate{
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
	}

	t.Run("template is immutable", func(t *testing.T) {
		g := NewWithT(t)
		_, err := newClusterTemplate.ValidateUpdate(context.TODO(), nil, oldClusterTemplate)
		g.Expect(err).To(HaveOccurred())
	})
}
