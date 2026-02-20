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

package scope

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/futures"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const (
	kubeconfigRefreshNeededValue = "true"
)

// Azure provisioning states for ARO HCP resources.
const (
	// ProvisioningStateSucceeded indicates the resource has been successfully provisioned.
	ProvisioningStateSucceeded = "Succeeded"
	// ProvisioningStateUpdating indicates the resource is being updated.
	ProvisioningStateUpdating = "Updating"
	// ProvisioningStateFailed indicates the resource provisioning has failed.
	ProvisioningStateFailed = "Failed"

	// ASO ARO HCP resource identifiers.
	aroHCPGroupName             = "redhatopenshift.azure.com"
	hcpOpenShiftClusterKindName = "HcpOpenShiftCluster"
)

// AROControlPlaneScopeParams defines the input parameters used to create a new Scope.
type AROControlPlaneScopeParams struct {
	AzureClients
	Client          client.Client
	Cluster         *clusterv1.Cluster
	ControlPlane    *cplane.AROControlPlane
	Cache           *AROControlPlaneCache
	Timeouts        azure.AsyncReconciler
	CredentialCache azure.CredentialCache
}

// NewAROControlPlaneScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewAROControlPlaneScope(ctx context.Context, params AROControlPlaneScopeParams) (*AROControlPlaneScope, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "azure.aroControlPlaneScope.NewAROControlPlaneScope")
	defer done()

	if params.ControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil AROControlPlane")
	}

	// Initialize Azure SDK credentials only when IdentityRef is set.
	// With identityRef: Initialize CAPZ credentials for Key Vault operations
	// Without identityRef: Skip credential initialization, ASO handles authentication
	//
	// When identityRef is not set, CAPZ cannot perform Key Vault operations:
	// - Cannot check for encryption key existence
	// - Cannot create encryption keys
	// - Cannot auto-propagate key versions to HcpOpenShiftCluster
	// Customers must manually create the vault and key via ASO and specify the key version in HcpOpenShiftCluster.
	//
	// Note: The check for empty Resources is defensive - the webhook now requires Resources mode,
	// so Resources cannot be empty in practice.
	shouldInitCredentials := len(params.ControlPlane.Spec.Resources) == 0 || params.ControlPlane.Spec.IdentityRef != nil

	if shouldInitCredentials {
		credentialsProvider, err := NewAzureCredentialsProvider(ctx, params.CredentialCache, params.Client, params.ControlPlane.Spec.IdentityRef, params.ControlPlane.Namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to init credentials provider")
		}
		err = params.AzureClients.setCredentialsWithProvider(ctx, params.ControlPlane.Spec.SubscriptionID, params.ControlPlane.Spec.AzureEnvironment, credentialsProvider)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure azure settings and credentials for Identity")
		}
	}

	if params.Cache == nil {
		params.Cache = &AROControlPlaneCache{}
	}

	helper, err := patch.NewHelper(params.ControlPlane, params.Client)
	if err != nil {
		return nil, errors.Errorf("failed to init patch helper: %v", err)
	}

	scope := &AROControlPlaneScope{
		Client:          params.Client,
		AzureClients:    params.AzureClients,
		Cluster:         params.Cluster,
		ControlPlane:    params.ControlPlane,
		patchHelper:     helper,
		cache:           params.Cache,
		AsyncReconciler: params.Timeouts,
		NetworkSpec:     &infrav1.NetworkSpec{}, // In resources mode, ASO manages network resources directly
	}

	return scope, nil
}

// AROControlPlaneScope defines the basic context for an actuator to operate upon.
type AROControlPlaneScope struct {
	Client      client.Client
	patchHelper *patch.Helper
	cache       *AROControlPlaneCache

	AzureClients
	Cluster              *clusterv1.Cluster
	ControlPlane         *cplane.AROControlPlane
	ControlPlaneEndpoint clusterv1.APIEndpoint

	NetworkSpec *infrav1.NetworkSpec

	// Key Vault related fields
	VaultName       *string
	VaultKeyName    *string
	VaultKeyVersion *string

	azure.AsyncReconciler
}

// MakeEmptyKubeConfigSecret creates an empty secret object that is used for storing kubeconfig secret data.
func (s *AROControlPlaneScope) MakeEmptyKubeConfigSecret() corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name(s.Cluster.Name, secret.Kubeconfig),
			Namespace: s.Cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(s.ControlPlane, infrav1.GroupVersion.WithKind(cplane.AROControlPlaneKind)),
			},
			Labels: map[string]string{clusterv1.ClusterNameLabel: s.Cluster.Name},
		},
	}
}

// SetLongRunningOperationState will set the future on the AROControlPlane status to allow the resource to continue
// in the next reconciliation.
func (s *AROControlPlaneScope) SetLongRunningOperationState(future *infrav1.Future) {
	futures.Set(s.ControlPlane, future)
}

// GetLongRunningOperationState will get the future on the AROControlPlane status.
func (s *AROControlPlaneScope) GetLongRunningOperationState(name, service, futureType string) *infrav1.Future {
	return futures.Get(s.ControlPlane, name, service, futureType)
}

// DeleteLongRunningOperationState will delete the future from the AROControlPlane status.
func (s *AROControlPlaneScope) DeleteLongRunningOperationState(name, service, futureType string) {
	futures.Delete(s.ControlPlane, name, service, futureType)
}

// UpdateDeleteStatus updates a condition on the AROControlPlaneStatus after a DELETE operation.
// This method accepts v1beta1.ConditionType for compatibility with non-ARO services.
func (s *AROControlPlaneScope) UpdateDeleteStatus(condition clusterv1beta1.ConditionType, service string, err error) {
	switch {
	case err == nil:
		conditions.Set(s.ControlPlane, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.DeletedReason,
			Message: fmt.Sprintf("%s successfully deleted", service),
		})
	case azure.IsOperationNotDoneError(err):
		conditions.Set(s.ControlPlane, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.DeletingReason,
			Message: fmt.Sprintf("%s deleting", service),
		})
	default:
		conditions.Set(s.ControlPlane, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.DeletionFailedReason,
			Message: fmt.Sprintf("%s failed to delete. err: %s", service, err.Error()),
		})
	}
}

// UpdatePutStatus updates a condition on the AROControlPlane status after a PUT operation.
// This method accepts v1beta1.ConditionType for compatibility with non-ARO services.
func (s *AROControlPlaneScope) UpdatePutStatus(condition clusterv1beta1.ConditionType, service string, err error) {
	switch {
	case err == nil:
		conditions.Set(s.ControlPlane, metav1.Condition{
			Type:   string(condition),
			Status: metav1.ConditionTrue,
			Reason: "Succeeded",
		})
	case azure.IsOperationNotDoneError(err):
		conditions.Set(s.ControlPlane, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.CreatingReason,
			Message: fmt.Sprintf("%s creating or updating", service),
		})
	default:
		conditions.Set(s.ControlPlane, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.FailedReason,
			Message: fmt.Sprintf("%s failed to create or update. err: %s", service, err.Error()),
		})
	}
}

// UpdatePatchStatus updates a condition on the AROControlPlane status after a PATCH operation.
// This method accepts v1beta1.ConditionType for compatibility with non-ARO services.
func (s *AROControlPlaneScope) UpdatePatchStatus(condition clusterv1beta1.ConditionType, service string, err error) {
	switch {
	case err == nil:
		conditions.Set(s.ControlPlane, metav1.Condition{
			Type:   string(condition),
			Status: metav1.ConditionTrue,
			Reason: "Succeeded",
		})
	case azure.IsOperationNotDoneError(err):
		conditions.Set(s.ControlPlane, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.UpdatingReason,
			Message: fmt.Sprintf("%s updating", service),
		})
	default:
		conditions.Set(s.ControlPlane, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.FailedReason,
			Message: fmt.Sprintf("%s failed to update. err: %s", service, err.Error()),
		})
	}
}

// ShouldReconcileKubeconfig determines if kubeconfig needs reconciliation using metadata-based validation (Pattern 3).
// This avoids direct cluster connections and prevents issues with stale/invalid secrets.
func (s *AROControlPlaneScope) ShouldReconcileKubeconfig(ctx context.Context) bool {
	kubeconfigSecret := s.MakeEmptyKubeConfigSecret()
	key := client.ObjectKeyFromObject(&kubeconfigSecret)

	if err := s.Client.Get(ctx, key, &kubeconfigSecret); err != nil {
		// Secret doesn't exist - need to create it
		return true
	}

	// Check if kubeconfig data exists
	if len(kubeconfigSecret.Data[secret.KubeconfigDataName]) == 0 {
		return true
	}

	// If secret exists but doesn't have our tracking annotation, it was created by ASO
	// and needs to be reconciled to add insecure-skip-tls-verify
	if kubeconfigSecret.Annotations == nil {
		return true
	}
	if _, exists := kubeconfigSecret.Annotations["aro.azure.com/kubeconfig-last-updated"]; !exists {
		return true
	}

	// Check for ARO-specific annotations that indicate refresh needed
	if refreshNeeded, exists := kubeconfigSecret.Annotations["aro.azure.com/kubeconfig-refresh-needed"]; exists && refreshNeeded == kubeconfigRefreshNeededValue {
		return true
	}

	// Check if secret is older than configured threshold
	if lastUpdated, exists := kubeconfigSecret.Annotations["aro.azure.com/kubeconfig-last-updated"]; exists {
		lastUpdatedTime, err := time.Parse(time.RFC3339, lastUpdated)
		if err == nil {
			kubeconfigAge := time.Since(lastUpdatedTime)
			maxAge := s.GetKubeconfigMaxAge() // Configure based on ARO token lifetime
			if kubeconfigAge > maxAge {
				return true
			}
		}
	}

	return false
}

// GetKubeconfigMaxAge returns the maximum age for kubeconfig before refresh is needed.
func (s *AROControlPlaneScope) GetKubeconfigMaxAge() time.Duration {
	// Default to 30 minutes, but could be made configurable via ControlPlane spec
	return 60 * time.Minute
}

// AROControlPlaneCache stores AROControlPlaneCache data locally so we don't have to hit the API multiple times within the same reconcile loop.
type AROControlPlaneCache struct {
}

// BaseURI returns the Azure ResourceManagerEndpoint.
func (s *AROControlPlaneScope) BaseURI() string {
	return s.ResourceManagerEndpoint
}

// GetClient returns the controller-runtime client.
func (s *AROControlPlaneScope) GetClient() client.Client {
	return s.Client
}

// GetDeletionTimestamp returns the deletion timestamp of the Cluster.
func (s *AROControlPlaneScope) GetDeletionTimestamp() *metav1.Time {
	return s.Cluster.DeletionTimestamp
}

// PatchObject persists the control plane configuration and status.
func (s *AROControlPlaneScope) PatchObject(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scope.ManagedControlPlaneScope.PatchObject")
	defer done()

	// Patch all status fields including Ready, Initialization, and all conditions
	return s.patchHelper.Patch(ctx, s.ControlPlane)
}

// Close closes the current scope persisting the control plane configuration and status.
func (s *AROControlPlaneScope) Close(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scope.AROControlPlaneScope.Close")
	defer done()

	return s.PatchObject(ctx)
}

// Location returns the location for the ARO control plane.
// It attempts to extract location from resources in the following order:
//  1. HcpOpenShiftCluster.spec.location (primary)
//  2. ResourceGroup.spec.location (first fallback)
//  3. Any other Azure resource with spec.location (second fallback)
func (s *AROControlPlaneScope) Location() (string, error) {
	if len(s.ControlPlane.Spec.Resources) == 0 {
		return "", errors.New("no resources defined in AROControlPlane.spec.resources")
	}

	// First pass: look for HcpOpenShiftCluster location (primary)
	for _, rawResource := range s.ControlPlane.Spec.Resources {
		if loc := s.extractLocationFromResource(rawResource, aroHCPGroupName, hcpOpenShiftClusterKindName); loc != "" {
			return loc, nil
		}
	}

	// Second pass: look for ResourceGroup location (fallback)
	for _, rawResource := range s.ControlPlane.Spec.Resources {
		if loc := s.extractLocationFromResource(rawResource, "resources.azure.com", "ResourceGroup"); loc != "" {
			ctrl.Log.Info("using location from ResourceGroup as HcpOpenShiftCluster location not found")
			return loc, nil
		}
	}

	// Third pass: any Azure resource with location (last resort)
	for _, rawResource := range s.ControlPlane.Spec.Resources {
		if loc := s.extractLocationFromAnyResource(rawResource); loc != "" {
			ctrl.Log.Info("using location from other Azure resource as fallback")
			return loc, nil
		}
	}

	return "", errors.New("no location found in any resource")
}

// extractLocationFromResource extracts location from a specific resource type.
func (s *AROControlPlaneScope) extractLocationFromResource(rawResource runtime.RawExtension, group, kind string) string {
	var unstructuredResource unstructured.Unstructured
	if err := json.Unmarshal(rawResource.Raw, &unstructuredResource); err != nil {
		return ""
	}

	if unstructuredResource.GroupVersionKind().Group == group &&
		unstructuredResource.GroupVersionKind().Kind == kind {
		location, found, err := unstructured.NestedString(
			unstructuredResource.UnstructuredContent(),
			"spec", "location",
		)
		if err == nil && found && location != "" {
			return location
		}
	}
	return ""
}

// extractLocationFromAnyResource extracts location from any Azure resource.
func (s *AROControlPlaneScope) extractLocationFromAnyResource(rawResource runtime.RawExtension) string {
	var unstructuredResource unstructured.Unstructured
	if err := json.Unmarshal(rawResource.Raw, &unstructuredResource); err != nil {
		return ""
	}

	location, found, err := unstructured.NestedString(
		unstructuredResource.UnstructuredContent(),
		"spec", "location",
	)
	if err == nil && found && location != "" {
		return location
	}
	return ""
}

// SetVersionStatus sets the k8s version in status.
func (s *AROControlPlaneScope) SetVersionStatus(version string) {
	s.ControlPlane.Status.Version = version
}

// MakeClusterCA returns a cluster CA Secret for the managed control plane.
func (s *AROControlPlaneScope) MakeClusterCA() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name(s.Cluster.Name, secret.ClusterCA),
			Namespace: s.Cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(s.ControlPlane, cplane.GroupVersion.WithKind(cplane.AROControlPlaneKind)),
			},
		},
	}
}

// ASOOwner implements aso.Scope.
func (s *AROControlPlaneScope) ASOOwner() client.Object {
	return s.ControlPlane
}

// Vnet returns the cluster Vnet.
func (s *AROControlPlaneScope) Vnet() *infrav1.VnetSpec {
	return &s.NetworkSpec.Vnet
}

// Subnet returns the subnet with the provided name.
func (s *AROControlPlaneScope) Subnet(name string) infrav1.SubnetSpec {
	for _, sn := range s.NetworkSpec.Subnets {
		if sn.Name == name {
			return sn
		}
	}

	return infrav1.SubnetSpec{}
}

// SetSubnet sets the subnet spec for the subnet with the same name.
func (s *AROControlPlaneScope) SetSubnet(subnetSpec infrav1.SubnetSpec) {
	for i, sn := range s.NetworkSpec.Subnets {
		if sn.Name == subnetSpec.Name {
			s.NetworkSpec.Subnets[i] = subnetSpec
			return
		}
	}
}

// UpdateSubnetCIDRs updates the subnet CIDRs for the subnet with the same name.
func (s *AROControlPlaneScope) UpdateSubnetCIDRs(name string, cidrBlocks []string) {
	subnetSpecInfra := s.Subnet(name)
	subnetSpecInfra.CIDRBlocks = cidrBlocks
	s.SetSubnet(subnetSpecInfra)
}

// UpdateSubnetID updates the subnet ID for the subnet with the same name.
func (s *AROControlPlaneScope) UpdateSubnetID(name string, id string) {
	subnetSpecInfra := s.Subnet(name)
	subnetSpecInfra.ID = id
	s.SetSubnet(subnetSpecInfra)
}

// ResourceGroup returns the resource group for the ARO control plane.
// In resources mode, it extracts from HcpOpenShiftCluster resource's owner reference.
func (s *AROControlPlaneScope) ResourceGroup() string {
	// Extract resource group from HcpOpenShiftCluster owner reference
	for _, rawResource := range s.ControlPlane.Spec.Resources {
		var unstructuredResource unstructured.Unstructured
		if err := json.Unmarshal(rawResource.Raw, &unstructuredResource); err != nil {
			continue
		}

		if unstructuredResource.GroupVersionKind().Group == aroHCPGroupName &&
			unstructuredResource.GroupVersionKind().Kind == hcpOpenShiftClusterKindName {
			// Extract from spec.owner.name
			ownerName, found, err := unstructured.NestedString(
				unstructuredResource.UnstructuredContent(),
				"spec", "owner", "name",
			)
			if err == nil && found && ownerName != "" {
				return ownerName
			}
		}
	}
	// Return empty if not found
	return ""
}

// ClusterName returns the cluster name.
func (s *AROControlPlaneScope) ClusterName() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *AROControlPlaneScope) Namespace() string {
	return s.Cluster.Namespace
}

// ExtendedLocation returns the extended location specification.
func (s *AROControlPlaneScope) ExtendedLocation() *infrav1.ExtendedLocationSpec {
	return nil
}

// AnnotationJSON returns a map[string]interface from a JSON annotation.
func (s *AROControlPlaneScope) AnnotationJSON(annotation string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	jsonAnnotation := s.ControlPlane.GetAnnotations()[annotation]
	if jsonAnnotation == "" {
		return out, nil
	}
	err := json.Unmarshal([]byte(jsonAnnotation), &out)
	if err != nil {
		return out, err
	}
	return out, nil
}

// UpdateAnnotationJSON updates the `annotation` with
// `content`. `content` in this case should be a `map[string]interface{}`
// suitable for turning into JSON. This `content` map will be marshalled into a
// JSON string before being set as the given `annotation`.
func (s *AROControlPlaneScope) UpdateAnnotationJSON(annotation string, content map[string]interface{}) error {
	b, err := json.Marshal(content)
	if err != nil {
		return err
	}
	s.SetAnnotation(annotation, string(b))
	return nil
}

// SetAnnotation sets a key value annotation on the ControlPlane.
func (s *AROControlPlaneScope) SetAnnotation(key, value string) {
	if s.ControlPlane.Annotations == nil {
		s.ControlPlane.Annotations = map[string]string{}
	}
	s.ControlPlane.Annotations[key] = value
}

// Name returns the cluster name for role assignment scope.
func (s *AROControlPlaneScope) Name() string {
	return s.ClusterName()
}

// KeyVaultSpecs returns empty specs for resources mode.
// KeyVault resources are managed via ASO in resources mode.
func (s *AROControlPlaneScope) KeyVaultSpecs() []azure.ResourceSpecGetter {
	return []azure.ResourceSpecGetter{}
}

// GetKeyVaultResourceID returns the Key Vault resource ID.
// In resources mode, it extracts from HcpOpenShiftCluster KMS configuration.
func (s *AROControlPlaneScope) GetKeyVaultResourceID() string {
	// Extract from the HcpOpenShiftCluster's etcd.dataEncryption.customerManaged.kms
	for _, rawResource := range s.ControlPlane.Spec.Resources {
		var unstructuredResource unstructured.Unstructured
		if err := json.Unmarshal(rawResource.Raw, &unstructuredResource); err != nil {
			continue
		}

		if unstructuredResource.GroupVersionKind().Group == aroHCPGroupName &&
			unstructuredResource.GroupVersionKind().Kind == hcpOpenShiftClusterKindName {
			// Extract vaultName from spec.properties.etcd.dataEncryption.customerManaged.kms.activeKey.vaultName
			vaultName, found, err := unstructured.NestedString(
				unstructuredResource.UnstructuredContent(),
				"spec", "properties", "etcd", "dataEncryption", "customerManaged", "kms", "activeKey", "vaultName",
			)
			if err == nil && found && vaultName != "" {
				// Construct resource ID from vault name
				// Format: /subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.KeyVault/vaults/{vaultName}
				return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.KeyVault/vaults/%s",
					s.SubscriptionID(), s.ResourceGroup(), vaultName)
			}
		}
	}

	// Return empty if not found
	return ""
}

// SetVaultInfo sets the vault information in the scope.
func (s *AROControlPlaneScope) SetVaultInfo(vaultName, keyName, keyVersion *string) {
	s.VaultName = vaultName
	s.VaultKeyName = keyName
	s.VaultKeyVersion = keyVersion
}

// GetVaultInfo returns the vault information from the scope.
func (s *AROControlPlaneScope) GetVaultInfo() (vaultName, keyName, keyVersion *string) {
	return s.VaultName, s.VaultKeyName, s.VaultKeyVersion
}

// SubscriptionID returns the subscription ID.
func (s *AROControlPlaneScope) SubscriptionID() string {
	return s.ControlPlane.Spec.SubscriptionID
}
