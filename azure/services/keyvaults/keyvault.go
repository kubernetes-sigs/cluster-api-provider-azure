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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "keyvault"

// KeyVaultScope defines the scope interface for a key vault service.
type KeyVaultScope interface {
	azure.Authorizer
	azure.AsyncReconciler
	KeyVaultSpecs() []azure.ResourceSpecGetter
	GetKeyVaultResourceID() string
	SetVaultInfo(vaultName, vaultKeyName, vaultKeyVersion *string)
	GetClient() client.Client
	ClusterName() string
	Namespace() string
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

	// For ARO HCP (resources mode), KeyVault vault resources are managed by ASO,
	// so KeyVaultSpecs may be empty. However, we still need to ensure encryption keys exist.
	// For legacy non-ARO clusters, KeyVaultSpecs contains vault definitions to reconcile.
	if len(keyVaultSpecs) > 0 {
		// Process each KeyVault spec (legacy path - not used by ARO HCP)
		for _, keyVaultSpec := range keyVaultSpecs {
			log.V(2).Info("reconciling KeyVault", "keyvault", keyVaultSpec.ResourceName())

			// Check if KeyVault exists
			_, err := s.client.Get(ctx, keyVaultSpec)
			if err != nil {
				return errors.Wrapf(err, "failed to get KeyVault %s", keyVaultSpec.ResourceName())
			}
		}
	}

	// Ensure the ETCD encryption key exists
	// For ARO HCP (resources mode): vault is managed by ASO, keys managed via Azure SDK
	// For legacy clusters: both vault and keys managed via Azure SDK
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

	// Get the Azure vault name from HcpOpenShiftCluster KMS configuration
	azureVaultName := scope.GetKeyVaultResourceID()
	if azureVaultName == "" {
		// No Key Vault specified, skip key creation
		log.V(2).Info("No Key Vault specified, skipping etcd encryption key creation")
		return nil
	}

	// Get vault K8s metadata (name, namespace, API version) from AROCluster.spec.resources
	vaultK8sName, vaultNamespace, vaultAPIVersion, err := s.getVaultK8sInfo(ctx, scope, azureVaultName)
	if err != nil {
		return errors.Wrap(err, "failed to get vault k8s info")
	}

	// Check if the vault is ready (if managed by AROCluster)
	vaultReady, err := s.isVaultReadyInAROCluster(ctx, scope, vaultK8sName)
	if err != nil {
		return errors.Wrap(err, "failed to check vault readiness")
	}
	if !vaultReady {
		log.V(2).Info("Key Vault not ready yet, waiting for AROCluster to provision it", "k8sName", vaultK8sName, "azureName", azureVaultName)
		return nil
	}

	// Construct the Key Vault URL
	keyVaultURL := fmt.Sprintf("https://%s.vault.azure.net/", azureVaultName)

	log.V(2).Info("ensuring etcd encryption key exists in Key Vault", "keyVaultURL", keyVaultURL, "keyName", ETCDEncryptionKeyName)

	// Ensure the etcd encryption key exists
	keyVersion, err := s.EnsureKeyExists(ctx, keyVaultURL, ETCDEncryptionKeyName, vaultK8sName, vaultNamespace, vaultAPIVersion)
	if err != nil {
		return errors.Wrap(err, "failed to ensure etcd encryption key exists")
	}

	// Update the scope with vault information
	scope.SetVaultInfo(ptr.To(azureVaultName), ptr.To(ETCDEncryptionKeyName), ptr.To(keyVersion))
	log.V(2).Info("updated scope with vault information", "azureName", azureVaultName, "keyName", ETCDEncryptionKeyName, "keyVersion", keyVersion)

	return nil
}

// EnsureKeyExists checks if the specified key exists in the Key Vault and creates it if it doesn't exist.
func (s *Service) EnsureKeyExists(ctx context.Context, keyVaultURL, keyName, vaultK8sName, vaultNamespace, vaultAPIVersion string) (string, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.EnsureKeyExists")
	defer done()

	log.V(2).Info("ensuring key exists in Key Vault", "keyVaultURL", keyVaultURL, "keyName", keyName)

	// Parse Key Vault URL to extract vault name
	vaultName, err := extractVaultNameFromURL(keyVaultURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to extract vault name from URL %s", keyVaultURL)
	}

	// Check if key exists and get its versions
	keyVersion, err := s.getLatestKeyVersion(ctx, vaultName, keyName, vaultK8sName, vaultNamespace, vaultAPIVersion)
	if err != nil {
		// If key doesn't exist, create it
		if isKeyNotFoundError(err) {
			log.V(2).Info("key not found, creating new key", "keyName", keyName)
			return s.createKey(ctx, vaultName, keyName, vaultK8sName, vaultNamespace, vaultAPIVersion)
		}
		return "", errors.Wrapf(err, "failed to check key existence")
	}

	log.V(2).Info("key exists", "keyName", keyName, "version", keyVersion)
	return keyVersion, nil
}

// getLatestKeyVersion retrieves the latest version of a key from the Key Vault using ARM KeysClient.
func (s *Service) getLatestKeyVersion(ctx context.Context, azureVaultName, keyName, vaultK8sName, vaultNamespace, vaultAPIVersion string) (string, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.getLatestKeyVersion")
	defer done()

	log.V(2).Info("getting latest key version", "azureVaultName", azureVaultName, "keyName", keyName, "k8sName", vaultK8sName)

	// Get resource group from deployed Vault's status.id
	// At this point the vault is ready (verified by isVaultReadyInAROCluster)
	resourceGroup, err := s.getVaultResourceGroupFromStatus(ctx, vaultK8sName, vaultNamespace, vaultAPIVersion)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get resource group for vault %s", vaultK8sName)
	}

	// Get the key using KeysClient.Get - this returns the latest version
	resp, err := s.client.GetKey(ctx, resourceGroup, azureVaultName, keyName)
	if err != nil {
		// Check if it's a "not found" error
		if azure.ResourceNotFound(err) {
			return "", fmt.Errorf("key not found: %s", keyName)
		}
		return "", errors.Wrapf(err, "failed to get key %s from vault %s", keyName, azureVaultName)
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

		log.V(2).Info("found key version", "azureVaultName", azureVaultName, "keyName", keyName, "version", keyVersion)
		return keyVersion, nil
	}

	return "", fmt.Errorf("key response missing KeyURIWithVersion for key %s", keyName)
}

// createKey creates a new key in the Key Vault using ARM KeysClient.
func (s *Service) createKey(ctx context.Context, azureVaultName, keyName, vaultK8sName, vaultNamespace, vaultAPIVersion string) (string, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.createKey")
	defer done()

	log.V(2).Info("creating new key", "azureVaultName", azureVaultName, "keyName", keyName, "k8sName", vaultK8sName)

	// Get resource group from deployed Vault's status.id
	// At this point the vault is ready (verified by isVaultReadyInAROCluster)
	resourceGroup, err := s.getVaultResourceGroupFromStatus(ctx, vaultK8sName, vaultNamespace, vaultAPIVersion)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get resource group for vault %s", vaultK8sName)
	}

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
	resp, err := s.client.CreateKey(ctx, resourceGroup, azureVaultName, keyName, keyParams)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create key %s in vault %s", keyName, azureVaultName)
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

		log.V(2).Info("successfully created key", "azureVaultName", azureVaultName, "keyName", keyName, "version", keyVersion)
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
// This is an alias for extractVaultNameFromURL which handles both resource IDs and URLs.
func extractVaultNameFromResourceID(resourceID string) (string, error) {
	return extractVaultNameFromURL(resourceID)
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

// getVaultResourceGroupFromStatus queries the deployed Vault and extracts the resource group
// from its status.id (Azure ARM resource ID). This is called after isVaultReadyInAROCluster
// confirms the vault is ready, so status.id is guaranteed to be populated.
func (s *Service) getVaultResourceGroupFromStatus(ctx context.Context, vaultK8sName, vaultNamespace, vaultAPIVersion string) (string, error) {
	kubeclient := s.Scope.GetClient()
	if kubeclient == nil {
		return "", errors.New("kubernetes client not available")
	}

	// Query the specific Vault using its K8s name, namespace, and API version
	vault := &unstructured.Unstructured{}
	vault.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "keyvault.azure.com",
		Version: vaultAPIVersion,
		Kind:    "Vault",
	})

	err := kubeclient.Get(ctx, client.ObjectKey{
		Namespace: vaultNamespace,
		Name:      vaultK8sName,
	}, vault)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get vault %s/%s", vaultNamespace, vaultK8sName)
	}

	// Extract status.id which contains the full Azure ARM resource ID
	statusID, found, err := unstructured.NestedString(
		vault.UnstructuredContent(),
		"status", "id",
	)
	if err != nil || !found || statusID == "" {
		return "", errors.Errorf("vault %s is ready but status.id is not populated", vaultK8sName)
	}

	// Parse resource group from ARM ID using robust attribute/value parser
	// ARM ID format: /attribute1/value1/attribute2/value2/...
	// Example: /subscriptions/{sub}/resourceGroups/{rg}/providers/{provider}/...
	resourceGroup, err := parseARMResourceAttribute(statusID, "resourceGroups")
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse resource group from vault status.id: %s", statusID)
	}

	return resourceGroup, nil
}

// parseARMResourceAttribute parses an Azure ARM resource ID to extract a specific attribute's value.
// ARM IDs follow the pattern: /attribute1/value1/attribute2/value2/...
// For example: /subscriptions/xxx/resourceGroups/my-rg/providers/Microsoft.KeyVault/vaults/my-vault.
func parseARMResourceAttribute(armID, attribute string) (string, error) {
	if armID == "" {
		return "", errors.New("ARM resource ID is empty")
	}

	parts := strings.Split(armID, "/")
	// ARM IDs start with /, so parts[0] is empty
	// parts are: ["", "subscriptions", "xxx", "resourceGroups", "yyy", ...]

	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], attribute) && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}

	return "", errors.Errorf("attribute %q not found in ARM resource ID: %s", attribute, armID)
}

// getVaultK8sInfo extracts vault Kubernetes metadata from AROCluster.spec.resources.
// Returns: k8sName, namespace, apiVersion, error.
func (s *Service) getVaultK8sInfo(ctx context.Context, scope KeyVaultScope, azureVaultName string) (string, string, string, error) {
	kubeclient := scope.GetClient()
	if kubeclient == nil {
		return "", "", "", errors.New("kubernetes client not available")
	}

	// Get AROCluster to access spec.resources
	aroCluster := &infrav1exp.AROCluster{}
	aroClusterKey := client.ObjectKey{
		Namespace: scope.Namespace(),
		Name:      scope.ClusterName(),
	}

	if err := kubeclient.Get(ctx, aroClusterKey, aroCluster); err != nil {
		return "", "", "", errors.Wrapf(err, "failed to get AROCluster %s/%s", aroClusterKey.Namespace, aroClusterKey.Name)
	}

	// Find Vault in spec.resources
	for _, rawResource := range aroCluster.Spec.Resources {
		var resource unstructured.Unstructured
		if err := resource.UnmarshalJSON(rawResource.Raw); err != nil {
			continue
		}

		if resource.GroupVersionKind().Group == "keyvault.azure.com" &&
			resource.GroupVersionKind().Kind == "Vault" {
			// Get spec.azureName, fallback to K8s object name if not set
			vaultAzureName, found, _ := unstructured.NestedString(
				resource.UnstructuredContent(),
				"spec", "azureName",
			)
			if !found || vaultAzureName == "" {
				vaultAzureName = resource.GetName()
			}

			if vaultAzureName == azureVaultName {
				// Found the vault! Return K8s metadata
				return resource.GetName(), resource.GetNamespace(), resource.GroupVersionKind().Version, nil
			}
		}
	}

	return "", "", "", errors.Errorf("vault with azureName %q not found in AROCluster spec.resources - "+
		"when using encryption with identityRef, the Vault resource must be declared in AROCluster.spec.resources[] "+
		"(CAPZ will create the encryption key inside the vault, but ASO must create the vault itself). "+
		"Add a Vault resource with spec.azureName: %q to AROCluster.spec.resources[]", azureVaultName, azureVaultName)
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

// isVaultReadyInAROCluster checks if the Key Vault is ready in the AROCluster's status.
// This prevents race conditions where we try to create a key before the vault is provisioned by ASO.
// Returns true if:
// - The vault is found in AROCluster.Status.Resources and is ready.
// - AROCluster doesn't exist (vault managed elsewhere).
// - Vault not found in AROCluster resources (vault managed elsewhere).
func (s *Service) isVaultReadyInAROCluster(ctx context.Context, scope KeyVaultScope, vaultK8sName string) (bool, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "keyvault.Service.isVaultReadyInAROCluster")
	defer done()

	kubeclient := scope.GetClient()
	if kubeclient == nil {
		// No k8s client available, assume vault is ready (not managed by AROCluster)
		log.V(2).Info("No kubernetes client available, assuming vault is ready")
		return true, nil
	}

	// Try to get AROCluster from the cluster namespace
	aroCluster := &infrav1exp.AROCluster{}
	aroClusterKey := client.ObjectKey{
		Namespace: scope.Namespace(),
		Name:      scope.ClusterName(),
	}

	if err := kubeclient.Get(ctx, aroClusterKey, aroCluster); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return false, errors.Wrapf(err, "failed to get AROCluster %s/%s", aroClusterKey.Namespace, aroClusterKey.Name)
		}
		// AROCluster not found - vault is not managed by AROCluster, assume it's ready
		log.V(2).Info("AROCluster not found, assuming vault is managed elsewhere and is ready",
			"aroCluster", aroClusterKey.Name, "namespace", aroClusterKey.Namespace)
		return true, nil
	}

	// Check if the vault is in the AROCluster's status resources using K8s object name
	for _, resource := range aroCluster.Status.Resources {
		if resource.Resource.Group == "keyvault.azure.com" &&
			resource.Resource.Kind == "Vault" &&
			resource.Resource.Name == vaultK8sName {
			// Found the vault in status, check if it's ready
			if resource.Ready {
				log.V(2).Info("Vault is ready in AROCluster status", "vaultK8sName", vaultK8sName)
				return true, nil
			}
			// Vault exists but not ready yet
			log.V(2).Info("Vault not ready yet in AROCluster status", "vaultK8sName", vaultK8sName)
			return false, nil
		}
	}

	// Vault not found in status.resources - either not managed by AROCluster or still being created
	log.V(2).Info("Vault not found in AROCluster status, assuming it's managed elsewhere or still being created", "vaultK8sName", vaultK8sName)
	return false, nil
}
