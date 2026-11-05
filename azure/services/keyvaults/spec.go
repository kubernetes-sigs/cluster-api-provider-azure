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
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const (
	// ETCDEncryptionKeyName is the name of the key used for etcd encryption.
	ETCDEncryptionKeyName = "etcd-data-kms-encryption-key"
	// VaultsResourceType is the resource type name for Key Vaults.
	VaultsResourceType = "vaults"
)

// KeyVaultSpec defines the specification for a Key Vault.
type KeyVaultSpec struct {
	Name           string
	ResourceGroup  string
	Location       string
	TenantID       string
	SKU            armkeyvault.SKUName
	AccessPolicies []*armkeyvault.AccessPolicyEntry
	Tags           map[string]*string
}

// ResourceName returns the name of the Key Vault.
func (s *KeyVaultSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *KeyVaultSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName returns the name of the owner resource.
func (s *KeyVaultSpec) OwnerResourceName() string {
	return s.Name
}

// ResourceType returns the resource type.
func (s *KeyVaultSpec) ResourceType() string {
	return VaultsResourceType
}

// Parameters returns the parameters for the Key Vault.
func (s *KeyVaultSpec) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "keyvault.KeyVaultSpec.Parameters")
	defer done()

	var existingKeyVault *armkeyvault.Vault
	if existing != nil {
		vault, ok := existing.(armkeyvault.Vault)
		if !ok {
			return nil, errors.Errorf("%T is not an armkeyvault.Vault", existing)
		}
		existingKeyVault = &vault
	}

	if existingKeyVault == nil {
		return nil, fmt.Errorf("vault %q not exists", s.Name)
	}
	return nil, nil
}

// KeySpec defines the specification for a Key Vault key.
type KeySpec struct {
	VaultName     string
	KeyName       string
	ResourceGroup string
	KeyType       string
	KeySize       int32
}

// GetKeyVaultURL constructs the Key Vault URL from the vault name.
func (s *KeySpec) GetKeyVaultURL() string {
	return fmt.Sprintf("https://%s.vault.azure.net/", s.VaultName)
}

// GetKeyURI constructs the key URI.
func (s *KeySpec) GetKeyURI() string {
	return fmt.Sprintf("https://%s.vault.azure.net/keys/%s", s.VaultName, s.KeyName)
}

// GetKeyURIWithVersion constructs the key URI with a specific version.
func (s *KeySpec) GetKeyURIWithVersion(version string) string {
	return fmt.Sprintf("https://%s.vault.azure.net/keys/%s/%s", s.VaultName, s.KeyName, version)
}

// ExtractKeyVersionFromURI extracts the key version from a versioned key URI.
func ExtractKeyVersionFromURI(keyURI string) string {
	// Split by '/' and get the last part which should be the version
	parts := strings.Split(keyURI, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// ValidateKeyVaultResourceID validates that the provided string is a valid Key Vault resource ID.
func ValidateKeyVaultResourceID(resourceID string) error {
	if resourceID == "" {
		return nil // Empty is valid (optional field)
	}

	// Check if it's a valid Key Vault resource ID format
	if !strings.Contains(resourceID, "/providers/Microsoft.KeyVault/vaults/") {
		return fmt.Errorf("invalid Key Vault resource ID format: %s", resourceID)
	}

	// Extract vault name to ensure it exists
	parts := strings.Split(resourceID, "/")
	found := false
	for i, part := range parts {
		if part == VaultsResourceType && i+1 < len(parts) {
			if parts[i+1] == "" {
				return fmt.Errorf("key vault name cannot be empty in resource ID: %s", resourceID)
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("could not extract Key Vault name from resource ID: %s", resourceID)
	}

	return nil
}
