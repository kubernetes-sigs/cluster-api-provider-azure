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

package keyvaults

import (
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name     string
		spec     KeyVaultSpec
		existing interface{}
		expected interface{}
		errorMsg string
	}{
		{
			name:     "no existing key vault",
			spec:     fakeKeyVaultSpec,
			existing: nil,
			expected: nil,
			errorMsg: "vault \"test-keyvault\" not exists",
		},
		{
			name:     "existing is not a key vault",
			spec:     fakeKeyVaultSpec,
			existing: "wrong type",
			errorMsg: "string is not an armkeyvault.Vault",
		},
		{
			name:     "existing key vault - no changes",
			spec:     fakeKeyVaultSpec,
			existing: fakeKeyVault,
			expected: nil,
		},
		{
			name:     "existing key vault - changes detected",
			spec:     fakeKeyVaultSpecWithDifferentSKU(),
			existing: fakeKeyVault,
			expected: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(t.Context(), tc.existing)
			if tc.errorMsg != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.errorMsg))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Got difference between expected result and computed result:\n%s", cmp.Diff(tc.expected, result))
			}
		})
	}
}

func TestResourceName(t *testing.T) {
	g := NewWithT(t)
	t.Parallel()

	spec := fakeKeyVaultSpec
	g.Expect(spec.ResourceName()).To(Equal("test-keyvault"))
}

func TestResourceGroupName(t *testing.T) {
	g := NewWithT(t)
	t.Parallel()

	spec := fakeKeyVaultSpec
	g.Expect(spec.ResourceGroupName()).To(Equal("test-rg"))
}

func TestOwnerResourceName(t *testing.T) {
	g := NewWithT(t)
	t.Parallel()

	spec := fakeKeyVaultSpec
	g.Expect(spec.OwnerResourceName()).To(Equal("test-keyvault"))
}

func TestResourceType(t *testing.T) {
	g := NewWithT(t)
	t.Parallel()

	spec := fakeKeyVaultSpec
	g.Expect(spec.ResourceType()).To(Equal(VaultsResourceType))
}

func TestExtractVaultNameFromResourceIDSpec(t *testing.T) {
	testcases := []struct {
		name         string
		resourceID   string
		expectedName string
		expectError  bool
	}{
		{
			name:         "valid resource ID",
			resourceID:   "/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-vault",
			expectedName: "test-vault",
			expectError:  false,
		},
		{
			name:         "invalid resource ID - too short",
			resourceID:   "/subscriptions/test-sub",
			expectedName: "",
			expectError:  true,
		},
		{
			name:         "empty resource ID",
			resourceID:   "",
			expectedName: "",
			expectError:  true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			name, err := extractVaultNameFromResourceID(tc.resourceID)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(name).To(Equal(tc.expectedName))
			}
		})
	}
}

// Helper functions for test data

func fakeKeyVaultSpecWithDifferentSKU() KeyVaultSpec {
	spec := fakeKeyVaultSpec
	spec.SKU = armkeyvault.SKUNamePremium
	return spec
}
