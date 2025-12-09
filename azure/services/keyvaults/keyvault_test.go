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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/keyvaults/mock_keyvault"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeKeyVaultSpec = KeyVaultSpec{
		Name:          "test-keyvault",
		ResourceGroup: "test-rg",
		Location:      "eastus",
		TenantID:      "test-tenant-id",
		SKU:           armkeyvault.SKUNameStandard,
		AccessPolicies: []*armkeyvault.AccessPolicyEntry{
			{
				TenantID: ptr.To("test-tenant-id"),
				ObjectID: ptr.To("test-object-id"),
				Permissions: &armkeyvault.Permissions{
					Keys: []*armkeyvault.KeyPermissions{
						ptr.To(armkeyvault.KeyPermissionsGet),
						ptr.To(armkeyvault.KeyPermissionsCreate),
					},
				},
			},
		},
	}

	fakeKeyVault = armkeyvault.Vault{
		ID:       ptr.To("/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-keyvault"),
		Name:     ptr.To("test-keyvault"),
		Location: ptr.To("eastus"),
		Properties: &armkeyvault.VaultProperties{
			TenantID: ptr.To("test-tenant-id"),
			SKU: &armkeyvault.SKU{
				Name: ptr.To(armkeyvault.SKUNameStandard),
			},
			VaultURI: ptr.To("https://test-keyvault.vault.azure.net/"),
		},
	}

	fakeKey = armkeyvault.Key{
		ID:   ptr.To("/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-keyvault/keys/test-key/versions/test-version"),
		Name: ptr.To("test-key"),
		Properties: &armkeyvault.KeyProperties{
			KeyURI:            ptr.To("https://test-keyvault.vault.azure.net/keys/test-key"),
			KeyURIWithVersion: ptr.To("https://test-keyvault.vault.azure.net/keys/test-key/test-version"),
		},
	}
)

// Helper functions to create fresh error instances to avoid race conditions
func newInternalError() *azcore.ResponseError {
	return &azcore.ResponseError{
		StatusCode: http.StatusInternalServerError,
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Internal Server Error: StatusCode=500")),
			StatusCode: http.StatusInternalServerError,
		},
	}
}

func newNotFoundError() *azcore.ResponseError {
	return &azcore.ResponseError{
		StatusCode: http.StatusNotFound,
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Not Found: StatusCode=404")),
			StatusCode: http.StatusNotFound,
		},
	}
}

func TestReconcileKeyVault(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "no key vault specs",
			expectedError: "",
			expect: func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder) {
				s.KeyVaultSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "key vault does not exist - should fail",
			expectedError: "failed to get KeyVault test-keyvault:",
			expect: func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder) {
				s.KeyVaultSpecs().Return([]azure.ResourceSpecGetter{&fakeKeyVaultSpec})
				c.Get(gomockinternal.AContext(), &fakeKeyVaultSpec).Return(nil, newNotFoundError())
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_keyvault.NewMockKeyVaultScope(mockCtrl)
			clientMock := mock_keyvault.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
			}

			err := s.Reconcile(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteKeyVault(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "delete does nothing",
			expectedError: "",
			expect: func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder) {
				// Delete method now does nothing, so no expectations needed
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_keyvault.NewMockKeyVaultScope(mockCtrl)
			clientMock := mock_keyvault.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
			}

			err := s.Delete(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestEnsureETCDEncryptionKey(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "key exists, update scope with version",
			expectedError: "",
			expect: func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder) {
				s.GetKeyVaultResourceID().Return("/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-keyvault")
				s.ResourceGroup().Return("test-rg")
				c.GetKey(gomockinternal.AContext(), "test-rg", "test-keyvault", ETCDEncryptionKeyName).Return(fakeKey, nil)
				s.SetVaultInfo(ptr.To("test-keyvault"), ptr.To(ETCDEncryptionKeyName), ptr.To("test-version"))
			},
		},
		{
			name:          "key does not exist, create new key",
			expectedError: "",
			expect: func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder) {
				s.GetKeyVaultResourceID().Return("/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-keyvault")
				s.ResourceGroup().Return("test-rg").Times(2) // Called by both getLatestKeyVersion and createKey
				c.GetKey(gomockinternal.AContext(), "test-rg", "test-keyvault", ETCDEncryptionKeyName).Return(nil, newNotFoundError())
				c.CreateKey(gomockinternal.AContext(), "test-rg", "test-keyvault", ETCDEncryptionKeyName, gomock.Any()).Return(fakeKey, nil)
				s.SetVaultInfo(ptr.To("test-keyvault"), ptr.To(ETCDEncryptionKeyName), ptr.To("test-version"))
			},
		},
		{
			name:          "fail to get key with non-404 error",
			expectedError: "failed to ensure etcd encryption key exists: failed to check key existence: failed to get key etcd-data-kms-encryption-key from vault test-keyvault:",
			expect: func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder) {
				s.GetKeyVaultResourceID().Return("/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-keyvault")
				s.ResourceGroup().Return("test-rg")
				c.GetKey(gomockinternal.AContext(), "test-rg", "test-keyvault", ETCDEncryptionKeyName).Return(nil, newInternalError())
			},
		},
		{
			name:          "fail to create key",
			expectedError: "failed to ensure etcd encryption key exists: failed to create key etcd-data-kms-encryption-key in vault test-keyvault:",
			expect: func(s *mock_keyvault.MockKeyVaultScopeMockRecorder, c *mock_keyvault.MockClientMockRecorder) {
				s.GetKeyVaultResourceID().Return("/subscriptions/test-sub/resourceGroups/test-rg/providers/Microsoft.KeyVault/vaults/test-keyvault")
				s.ResourceGroup().Return("test-rg").Times(2) // Called by both getLatestKeyVersion and createKey
				c.GetKey(gomockinternal.AContext(), "test-rg", "test-keyvault", ETCDEncryptionKeyName).Return(nil, newNotFoundError())
				c.CreateKey(gomockinternal.AContext(), "test-rg", "test-keyvault", ETCDEncryptionKeyName, gomock.Any()).Return(nil, newInternalError())
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_keyvault.NewMockKeyVaultScope(mockCtrl)
			clientMock := mock_keyvault.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
			}

			err := s.EnsureETCDEncryptionKey(t.Context(), scopeMock)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestExtractVaultNameFromResourceID(t *testing.T) {
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

func TestIsKeyNotFoundError(t *testing.T) {
	testcases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "404 error is key not found",
			err:      newNotFoundError(),
			expected: true,
		},
		{
			name:     "500 error is not key not found",
			err:      newInternalError(),
			expected: false,
		},
		{
			name:     "nil error is not key not found",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result := isKeyNotFoundError(tc.err)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}
