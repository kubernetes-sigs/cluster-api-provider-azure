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

package userassignedidentities

import (
	"testing"

	"github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func TestUserAssignedIdentitySpec_ResourceRef(t *testing.T) {
	g := NewWithT(t)

	spec := &UserAssignedIdentitySpec{
		Name:          "test-identity",
		ResourceGroup: "test-rg",
		Location:      "eastus",
		ConfigMapName: "test-config",
	}

	ref := spec.ResourceRef()
	g.Expect(ref).NotTo(BeNil())
	g.Expect(ref.Name).To(Equal(azure.GetNormalizedKubernetesName("test-identity")))
}

func TestUserAssignedIdentitySpec_ResourceName(t *testing.T) {
	g := NewWithT(t)

	spec := &UserAssignedIdentitySpec{
		Name: "test-identity",
	}

	g.Expect(spec.ResourceName()).To(Equal("test-identity"))
}

func TestUserAssignedIdentitySpec_ResourceGroupName(t *testing.T) {
	g := NewWithT(t)

	spec := &UserAssignedIdentitySpec{
		ResourceGroup: "test-rg",
	}

	g.Expect(spec.ResourceGroupName()).To(Equal("test-rg"))
}

func TestUserAssignedIdentitySpec_Parameters(t *testing.T) {
	testCases := []struct {
		name     string
		spec     *UserAssignedIdentitySpec
		existing *v1api20230131.UserAssignedIdentity
	}{
		{
			name: "new user assigned identity",
			spec: &UserAssignedIdentitySpec{
				Name:          "test-identity",
				ResourceGroup: "test-rg",
				Location:      "eastus",
				Tags: map[string]*string{
					"environment": ptr.To("test"),
				},
				ConfigMapName: "test-config",
			},
			existing: nil,
		},
		{
			name: "existing user assigned identity",
			spec: &UserAssignedIdentitySpec{
				Name:          "existing-identity",
				ResourceGroup: "test-rg",
				Location:      "westus",
				Tags: map[string]*string{
					"updated": ptr.To("true"),
				},
				ConfigMapName: "existing-config",
			},
			existing: &v1api20230131.UserAssignedIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "existing-identity",
				},
				Spec: v1api20230131.UserAssignedIdentity_Spec{
					Location: ptr.To("eastus"),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := tc.spec.Parameters(t.Context(), tc.existing)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).NotTo(BeNil())

			// Verify basic spec properties
			g.Expect(*result.Spec.Location).To(Equal(tc.spec.Location))
			g.Expect(result.Spec.Owner.Name).To(Equal(tc.spec.ResourceGroup))

			// Verify tags exist
			if tc.spec.Tags != nil {
				g.Expect(result.Spec.Tags).NotTo(BeNil())
				g.Expect(result.Spec.Tags).To(HaveLen(len(tc.spec.Tags)))
			}
		})
	}
}

func TestUserAssignedIdentitySpec_WasManaged(t *testing.T) {
	g := NewWithT(t)

	spec := &UserAssignedIdentitySpec{
		Name:          "test-identity",
		ResourceGroup: "test-rg",
		Location:      "eastus",
		ConfigMapName: "test-config",
	}

	// Should always return true for user assigned identities
	g.Expect(spec.WasManaged(nil)).To(BeTrue())
	g.Expect(spec.WasManaged(&v1api20230131.UserAssignedIdentity{})).To(BeTrue())
}
