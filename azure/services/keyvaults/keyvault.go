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
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "keyvault"

// KeyVaultScope defines the scope interface for a key vault service.
type KeyVaultScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	KeyVaultSpecs() []azure.ResourceSpecGetter
	ResourceGroup() string
	SubscriptionID() string
	GetKeyVaultResourceID() string
	SetVaultInfo(vaultName, vaultKeyName, vaultKeyVersion *string)
}

// Service provides operations on Azure Key Vault resources.
type Service struct {
	Scope  KeyVaultScope
	client Client
}

// New creates a new service.
func New(scope KeyVaultScope) (*Service, error) {
	client, err := newClient(scope, scope.DefaultedAzureCallTimeout())
	if err != nil {
		return nil, err
	}
	return &Service{
		Scope:  scope,
		client: client,
	}, nil
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile gets a key vault and ensures ETCD encryption key exists.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.Reconcile")
	defer done()

	// KeyVault operations need a longer timeout than the default 2s Azure call timeout
	// Use 180 seconds (3 minutes) to allow for KeyVault SDK operations to complete
	// This accounts for slow KeyVault API responses and network latency
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	// Get all KeyVault specs
	keyVaultSpecs := s.Scope.KeyVaultSpecs()
	if len(keyVaultSpecs) == 0 {
		return nil
	}

	// Process each KeyVault spec
	for _, keyVaultSpec := range keyVaultSpecs {
		log.V(2).Info("reconciling KeyVault", "keyvault", keyVaultSpec.ResourceName())

		// Check if KeyVault exists
		_, err := s.client.Get(ctx, keyVaultSpec)
		if err != nil {
			return errors.Wrapf(err, "failed to get KeyVault %s", keyVaultSpec.ResourceName())
		}
	}

	// After Key Vault is reconciled, ensure the ETCD encryption key exists
	if err := s.EnsureETCDEncryptionKey(ctx, s.Scope); err != nil {
		return errors.Wrap(err, "failed to ensure ETCD encryption key exists")
	}

	return nil
}

// Delete deletes the key vault.
func (s *Service) Delete(ctx context.Context) error {
	_, _, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.Delete")
	defer done()

	return nil
}

// IsManaged returns always returns true as CAPZ does not support BYO key vault.
func (s *Service) IsManaged(_ context.Context) (bool, error) {
	return true, nil
}

// EnsureETCDEncryptionKey ensures that the etcd encryption key exists in the Key Vault and updates the scope with vault information.
func (s *Service) EnsureETCDEncryptionKey(ctx context.Context, scope KeyVaultScope) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.EnsureETCDEncryptionKey")
	defer done()

	// Get the Key Vault resource ID from the platform spec
	keyVaultResourceID := scope.GetKeyVaultResourceID()
	if keyVaultResourceID == "" {
		// No Key Vault specified, skip key creation
		log.V(2).Info("No Key Vault specified, skipping etcd encryption key creation")
		return nil
	}

	// Extract vault name from the Key Vault resource ID
	vaultName, err := extractVaultNameFromResourceID(keyVaultResourceID)
	if err != nil {
		return errors.Wrap(err, "failed to extract vault name from resource ID")
	}

	// Construct the Key Vault URL
	keyVaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

	log.V(2).Info("ensuring etcd encryption key exists in Key Vault", "keyVaultURL", keyVaultURL, "keyName", ETCDEncryptionKeyName)

	// Ensure the etcd encryption key exists
	keyVersion, err := s.EnsureKeyExists(ctx, keyVaultURL, ETCDEncryptionKeyName)
	if err != nil {
		return errors.Wrap(err, "failed to ensure etcd encryption key exists")
	}

	// Update the scope with vault information
	scope.SetVaultInfo(ptr.To(vaultName), ptr.To(ETCDEncryptionKeyName), ptr.To(keyVersion))
	log.V(2).Info("updated scope with vault information", "vaultName", vaultName, "keyName", ETCDEncryptionKeyName, "keyVersion", keyVersion)

	return nil
}

// EnsureKeyExists checks if the specified key exists in the Key Vault and creates it if it doesn't exist.
func (s *Service) EnsureKeyExists(ctx context.Context, keyVaultURL, keyName string) (string, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.EnsureKeyExists")
	defer done()

	log.V(2).Info("ensuring key exists in Key Vault", "keyVaultURL", keyVaultURL, "keyName", keyName)

	// Parse Key Vault URL to extract vault name
	vaultName, err := extractVaultNameFromURL(keyVaultURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to extract vault name from URL %s", keyVaultURL)
	}

	// Check if key exists and get its versions
	keyVersion, err := s.getLatestKeyVersion(ctx, vaultName, keyName)
	if err != nil {
		// If key doesn't exist, create it
		if isKeyNotFoundError(err) {
			log.V(2).Info("key not found, creating new key", "keyName", keyName)
			return s.createKey(ctx, vaultName, keyName)
		}
		return "", errors.Wrapf(err, "failed to check key existence")
	}

	log.V(2).Info("key exists", "keyName", keyName, "version", keyVersion)
	return keyVersion, nil
}

// getLatestKeyVersion retrieves the latest version of a key from the Key Vault using ARM KeysClient.
func (s *Service) getLatestKeyVersion(ctx context.Context, vaultName, keyName string) (string, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.getLatestKeyVersion")
	defer done()

	log.V(2).Info("getting latest key version", "vaultName", vaultName, "keyName", keyName)

	resourceGroup := s.Scope.ResourceGroup()

	// Get the key using KeysClient.Get - this returns the latest version
	resp, err := s.client.GetKey(ctx, resourceGroup, vaultName, keyName)
	if err != nil {
		// Check if it's a "not found" error
		if azure.ResourceNotFound(err) {
			return "", fmt.Errorf("key not found: %s", keyName)
		}
		return "", errors.Wrapf(err, "failed to get key %s from vault %s", keyName, vaultName)
	}

	key, ok := resp.(armkeyvault.Key)
	if !ok {
		return "", errors.New("unexpected response type from GetKey")
	}

	// Extract version from the key URI
	if key.Properties != nil && key.Properties.KeyURIWithVersion != nil {
		keyVersion := extractVersionFromKeyID(*key.Properties.KeyURIWithVersion)
		if keyVersion == "" {
			return "", fmt.Errorf("could not extract version from key URI: %s", *key.Properties.KeyURIWithVersion)
		}

		log.V(2).Info("found key version", "vaultName", vaultName, "keyName", keyName, "version", keyVersion)
		return keyVersion, nil
	}

	return "", fmt.Errorf("key response missing KeyURIWithVersion for key %s", keyName)
}

// createKey creates a new key in the Key Vault using ARM KeysClient.
func (s *Service) createKey(ctx context.Context, vaultName, keyName string) (string, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.createKey")
	defer done()

	log.V(2).Info("creating new key", "vaultName", vaultName, "keyName", keyName)

	resourceGroup := s.Scope.ResourceGroup()

	// Create key parameters for RSA key (suitable for encryption)
	keyType := armkeyvault.JSONWebKeyTypeRSA
	keySize := int32(2048)

	keyParams := armkeyvault.KeyCreateParameters{
		Properties: &armkeyvault.KeyProperties{
			Kty:     &keyType,
			KeySize: &keySize,
		},
	}

	// Create the key using KeysClient.CreateIfNotExist
	resp, err := s.client.CreateKey(ctx, resourceGroup, vaultName, keyName, keyParams)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create key %s in vault %s", keyName, vaultName)
	}

	key, ok := resp.(armkeyvault.Key)
	if !ok {
		return "", errors.New("unexpected response type from CreateKey")
	}

	// Extract version from the key URI
	if key.Properties != nil && key.Properties.KeyURIWithVersion != nil {
		keyVersion := extractVersionFromKeyID(*key.Properties.KeyURIWithVersion)
		if keyVersion == "" {
			return "", fmt.Errorf("could not extract version from created key URI: %s", *key.Properties.KeyURIWithVersion)
		}

		log.V(2).Info("successfully created key", "vaultName", vaultName, "keyName", keyName, "version", keyVersion)
		return keyVersion, nil
	}

	return "", fmt.Errorf("key creation response missing KeyURIWithVersion for key %s", keyName)
}

// extractVaultNameFromURL extracts the vault name from a Key Vault resource ID or URL.
func extractVaultNameFromURL(keyVaultURL string) (string, error) {
	// Handle both resource ID format and vault URL format
	if strings.Contains(keyVaultURL, "/providers/Microsoft.KeyVault/vaults/") {
		// Resource ID format: /subscriptions/.../resourceGroups/.../providers/Microsoft.KeyVault/vaults/vaultName
		parts := strings.Split(keyVaultURL, "/")
		for i, part := range parts {
			if part == "vaults" && i+1 < len(parts) {
				return parts[i+1], nil
			}
		}
	} else if strings.Contains(keyVaultURL, ".vault.azure.net") {
		// Vault URL format: https://vaultName.vault.azure.net/
		parts := strings.Split(keyVaultURL, ".")
		if len(parts) > 0 {
			vaultName := strings.TrimPrefix(parts[0], "https://")
			return vaultName, nil
		}
	}

	return "", fmt.Errorf("invalid Key Vault URL format: %s", keyVaultURL)
}

// extractVaultNameFromResourceID extracts the vault name from a Key Vault resource ID.
func extractVaultNameFromResourceID(resourceID string) (string, error) {
	parts := strings.Split(resourceID, "/")
	for i, part := range parts {
		if part == VaultsResourceType && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("could not extract vault name from resource ID: %s", resourceID)
}

// extractVersionFromKeyID extracts the version from a Key Vault key ID.
// Key ID format: https://vaultname.vault.azure.net/keys/keyname/version
func extractVersionFromKeyID(keyID string) string {
	parts := strings.Split(keyID, "/")
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return ""
}

// isKeyNotFoundError checks if the error indicates that a key was not found.
func isKeyNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "key not found") ||
		strings.Contains(err.Error(), "KeyNotFound") ||
		strings.Contains(err.Error(), "NotFound") ||
		strings.Contains(err.Error(), "404")
}
