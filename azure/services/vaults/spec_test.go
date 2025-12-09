/*
Copyright 2025 The Kubernetes Authors.

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

package vaults

import (
	"testing"

	"github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func TestVaultSpec_ResourceRef(t *testing.T) {
	g := NewWithT(t)

	spec := &VaultSpec{
		Name:          "test-vault",
		ResourceGroup: "test-rg",
		Location:      "eastus",
		TenantID:      "test-tenant-id",
		Tags: map[string]string{
			"environment": "test",
		},
	}

	ref := spec.ResourceRef()
	g.Expect(ref).NotTo(BeNil())
	g.Expect(ref.Name).To(Equal(azure.GetNormalizedKubernetesName("test-vault")))
}

func TestVaultSpec_Parameters(t *testing.T) {
	testCases := []struct {
		name     string
		spec     *VaultSpec
		existing *v1api20230701.Vault
		expected *v1api20230701.Vault
	}{
		{
			name: "new vault",
			spec: &VaultSpec{
				Name:          "test-vault",
				ResourceGroup: "test-rg",
				Location:      "eastus",
				TenantID:      "test-tenant-id",
				Tags: map[string]string{
					"environment": "test",
				},
			},
			existing: nil,
			expected: &v1api20230701.Vault{
				Spec: v1api20230701.Vault_Spec{
					Location: ptr.To("eastus"),
					Tags: map[string]string{
						"environment": "test",
					},
					OperatorSpec: &v1api20230701.VaultOperatorSpec{
						ConfigMapExpressions: nil,
						SecretExpressions:    nil,
					},
					Owner: &genruntime.KnownResourceReference{
						Name: "test-rg",
					},
					Properties: &v1api20230701.VaultProperties{
						AccessPolicies:               []v1api20230701.AccessPolicyEntry{},
						EnabledForDeployment:         ptr.To(false),
						EnabledForDiskEncryption:     ptr.To(false),
						EnabledForTemplateDeployment: ptr.To(false),
						EnableSoftDelete:             ptr.To(true),
						SoftDeleteRetentionInDays:    ptr.To(int(90)),
						EnableRbacAuthorization:      ptr.To(true),
						TenantId:                     ptr.To("test-tenant-id"),
						Sku: &v1api20230701.Sku{
							Family: ptr.To(v1api20230701.Sku_Family_A),
							Name:   ptr.To(v1api20230701.Sku_Name_Standard),
						},
					},
				},
			},
		},
		{
			name: "existing vault",
			spec: &VaultSpec{
				Name:          "existing-vault",
				ResourceGroup: "test-rg",
				Location:      "westus",
				TenantID:      "test-tenant-id",
				Tags: map[string]string{
					"updated": "true",
				},
			},
			existing: &v1api20230701.Vault{
				ObjectMeta: metav1.ObjectMeta{
					Name: "existing-vault",
				},
				Spec: v1api20230701.Vault_Spec{
					Location: ptr.To("eastus"),
				},
			},
			expected: &v1api20230701.Vault{
				ObjectMeta: metav1.ObjectMeta{
					Name: "existing-vault",
				},
				Spec: v1api20230701.Vault_Spec{
					Location: ptr.To("westus"),
					Tags: map[string]string{
						"updated": "true",
					},
					OperatorSpec: &v1api20230701.VaultOperatorSpec{
						ConfigMapExpressions: nil,
						SecretExpressions:    nil,
					},
					Owner: &genruntime.KnownResourceReference{
						Name: "test-rg",
					},
					Properties: &v1api20230701.VaultProperties{
						AccessPolicies:               []v1api20230701.AccessPolicyEntry{},
						EnabledForDeployment:         ptr.To(false),
						EnabledForDiskEncryption:     ptr.To(false),
						EnabledForTemplateDeployment: ptr.To(false),
						EnableSoftDelete:             ptr.To(true),
						SoftDeleteRetentionInDays:    ptr.To(int(90)),
						EnableRbacAuthorization:      ptr.To(true),
						TenantId:                     ptr.To("test-tenant-id"),
						Sku: &v1api20230701.Sku{
							Family: ptr.To(v1api20230701.Sku_Family_A),
							Name:   ptr.To(v1api20230701.Sku_Name_Standard),
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := tc.spec.Parameters(t.Context(), tc.existing)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Spec).To(Equal(tc.expected.Spec))
		})
	}
}

func TestVaultSpec_WasManaged(t *testing.T) {
	g := NewWithT(t)

	spec := &VaultSpec{
		Name:          "test-vault",
		ResourceGroup: "test-rg",
		Location:      "eastus",
		TenantID:      "test-tenant-id",
	}

	// Should always return true for vaults
	g.Expect(spec.WasManaged(nil)).To(BeTrue())
	g.Expect(spec.WasManaged(&v1api20230701.Vault{})).To(BeTrue())
}
