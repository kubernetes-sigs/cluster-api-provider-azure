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

package v1beta2

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	webhookutils "sigs.k8s.io/cluster-api-provider-azure/util/webhook"
)

var (
	ocpSemver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
)

// SetupAROControlPlaneWebhookWithManager sets up and registers the webhook with the manager.
func SetupAROControlPlaneWebhookWithManager(mgr ctrl.Manager) error {
	mw := &aroControlPlaneWebhook{Client: mgr.GetClient()}
	return ctrl.NewWebhookManagedBy(mgr).
		For(&AROControlPlane{}).
		WithDefaulter(mw).
		WithValidator(mw).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-controlplane-cluster-x-k8s-io-v1beta2-arocontrolplane,mutating=true,failurePolicy=fail,groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes,verbs=create;update,versions=v1beta2,name=default.arocontrolplanes.controlplane.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

// aroControlPlaneWebhook implements a validating and defaulting webhook for AROControlPlane.
type aroControlPlaneWebhook struct {
	Client client.Client
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (mw *aroControlPlaneWebhook) Default(_ context.Context, obj runtime.Object) error {
	m, ok := obj.(*AROControlPlane)
	if !ok {
		return apierrors.NewBadRequest("expected an AROControlPlane")
	}

	m.Spec.Version = setDefaultOCPVersion(m.Spec.Version)

	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-controlplane-cluster-x-k8s-io-v1beta2-arocontrolplane,mutating=false,failurePolicy=fail,groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes,versions=v1beta2,name=validation.arocontrolplanes.controlplane.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (mw *aroControlPlaneWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m, ok := obj.(*AROControlPlane)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AROControlPlane")
	}

	return nil, m.Validate(mw.Client)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (mw *aroControlPlaneWebhook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	var allErrs field.ErrorList
	old, ok := oldObj.(*AROControlPlane)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AROControlPlane")
	}
	m, ok := newObj.(*AROControlPlane)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AROControlPlane")
	}

	// Based on TypeSpec models from ARO-HCP repository and service layer immutability requirements
	// Fields without Lifecycle.Update in @visibility decorator are immutable
	immutableFields := []struct {
		path *field.Path
		old  interface{}
		new  interface{}
	}{
		// aroClusterName is cluster identity - always immutable
		{field.NewPath("spec", "aroClusterName"), old.Spec.AroClusterName, m.Spec.AroClusterName},
		// platform.networkSecurityGroupId: @visibility(Lifecycle.Read, Lifecycle.Create)
		{field.NewPath("spec", "platform", "networkSecurityGroupID"), old.Spec.Platform.NetworkSecurityGroupID, m.Spec.Platform.NetworkSecurityGroupID},
		// platform.subnetId: @visibility(Lifecycle.Read, Lifecycle.Create)
		{field.NewPath("spec", "platform", "subnet"), old.Spec.Platform.Subnet, m.Spec.Platform.Subnet},
		// platform.outboundType: @visibility(Lifecycle.Read, Lifecycle.Create)
		{field.NewPath("spec", "platform", "outboundType"), old.Spec.Platform.OutboundType, m.Spec.Platform.OutboundType},
		// platform.managedResourceGroup: @visibility(Lifecycle.Read, Lifecycle.Create)
		{field.NewPath("spec", "platform", "resourceGroup"), old.Spec.Platform.ResourceGroup, m.Spec.Platform.ResourceGroup},
		// api.visibility: @visibility(Lifecycle.Read, Lifecycle.Create)
		{field.NewPath("spec", "visibility"), old.Spec.Visibility, m.Spec.Visibility},
		// version.id: @visibility(Lifecycle.Read, Lifecycle.Create) - immutable per TypeSpec
		{field.NewPath("spec", "version"), old.Spec.Version, m.Spec.Version},
		// TODO: location seems to be immutable too
		{field.NewPath("spec", "platform", "location"), old.Spec.Platform.Location, m.Spec.Platform.Location},
	}

	for _, f := range immutableFields {
		if err := webhookutils.ValidateImmutable(f.path, f.old, f.new); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	// domainPrefix (dns.baseDomainPrefix): @visibility(Lifecycle.Read, Lifecycle.Create) - immutable per TypeSpec
	if m.Spec.DomainPrefix != "" {
		if err := webhookutils.ValidateImmutable(
			field.NewPath("spec", "domainPrefix"),
			old.Spec.DomainPrefix,
			m.Spec.DomainPrefix,
		); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	// Note: version.id is immutable per TypeSpec: @visibility(Lifecycle.Read, Lifecycle.Create)
	// Note: channelGroup is mutable per TypeSpec: @visibility(Lifecycle.Read, Lifecycle.Create, Lifecycle.Update)
	// Note: platform.operatorsAuthentication has @visibility(Lifecycle.Read, Lifecycle.Create, Lifecycle.Update)
	// so managedIdentities are mutable and should not be validated as immutable here

	// Network fields: @visibility(Lifecycle.Read, Lifecycle.Create) - immutable per TypeSpec
	if errs := m.validateNetworkUpdate(old); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) == 0 {
		return nil, m.Validate(mw.Client)
	}

	return nil, apierrors.NewInvalid(GroupVersion.WithKind(AROControlPlaneKind).GroupKind(), m.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (mw *aroControlPlaneWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Validate the Azure Managed Control Plane and return an aggregate error.
func (m *AROControlPlane) Validate(cli client.Client) error {
	var allErrs field.ErrorList
	validators := []func(client client.Client) field.ErrorList{
		m.validateIdentity,
		m.validateDNSPrefix,
		m.validateManagedIdentities,
		m.validatePlatformFields,
		m.validateExternalAuthProviders,
	}
	for _, validator := range validators {
		if err := validator(cli); err != nil {
			allErrs = append(allErrs, err...)
		}
	}

	allErrs = append(allErrs, validateOCPVersion(
		m.Spec.Version,
		field.NewPath("spec").Child("version"))...)

	allErrs = append(allErrs, validateNetwork(m.Spec.Network, field.NewPath("spec"))...)

	allErrs = append(allErrs, validateName(m.Name, field.NewPath("name"))...)

	return allErrs.ToAggregate()
}

func (m *AROControlPlane) validateDNSPrefix(_ client.Client) field.ErrorList {
	if m.Spec.DomainPrefix == "" {
		return nil
	}

	// Regex pattern for DNS prefix validation
	// 1. Between 1 and 54 characters long: {1,54}
	// 2. Alphanumerics and hyphens: [a-zA-Z0-9-]
	// 3. Start and end with alphanumeric: ^[a-zA-Z0-9].*[a-zA-Z0-9]$
	pattern := `^[a-zA-Z0-9][a-zA-Z0-9-]{0,52}[a-zA-Z0-9]$`
	regex := regexp.MustCompile(pattern)
	if regex.MatchString(m.Spec.DomainPrefix) {
		return nil
	}
	allErrs := field.ErrorList{
		field.Invalid(field.NewPath("spec", "domainPrefix"), m.Spec.DomainPrefix, "DomainPrefix is invalid, does not match regex: "+pattern),
	}
	return allErrs
}

// validateOCPVersion validates the Kubernetes version.
func validateOCPVersion(version string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if !ocpSemver.MatchString(version) {
		allErrs = append(allErrs, field.Invalid(fldPath, version, "must be in format <X.Y>"))
	}

	return allErrs
}

func validateNetwork(virtualNetwork *NetworkSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !reflect.DeepEqual(virtualNetwork, NetworkSpec{}) {
		_, _, vnetErr := net.ParseCIDR(virtualNetwork.MachineCIDR)
		if vnetErr != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("MachineCIDR"), virtualNetwork.MachineCIDR, "CIDR block is invalid"))
		}
		_, _, vnetErr = net.ParseCIDR(virtualNetwork.ServiceCIDR)
		if vnetErr != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ServiceCIDR"), virtualNetwork.ServiceCIDR, "CIDR block is invalid"))
		}
		_, _, vnetErr = net.ParseCIDR(virtualNetwork.PodCIDR)
		if vnetErr != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("PodCIDR"), virtualNetwork.PodCIDR, "CIDR block is invalid"))
		}
	}
	return allErrs
}

// validateNetworkUpdate validates update to VirtualNetwork.
func (m *AROControlPlane) validateNetworkUpdate(old *AROControlPlane) field.ErrorList {
	var allErrs field.ErrorList

	if old.Spec.Network.MachineCIDR != m.Spec.Network.MachineCIDR {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "network", "machineCIDR"),
				m.Spec.Network.MachineCIDR,
				"Network CIDR is immutable"))
	}

	if old.Spec.Network.PodCIDR != m.Spec.Network.PodCIDR {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "network", "podCIDR"),
				m.Spec.Network.PodCIDR,
				"Network CIDR is immutable"))
	}

	if old.Spec.Network.ServiceCIDR != m.Spec.Network.ServiceCIDR {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "network", "serviceCIDR"),
				m.Spec.Network.ServiceCIDR,
				"Network CIDR is immutable"))
	}

	if old.Spec.Network.NetworkType != m.Spec.Network.NetworkType {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "network", "networkType"),
				m.Spec.Network.NetworkType,
				"Network type is immutable"))
	}

	if old.Spec.Network.HostPrefix != m.Spec.Network.HostPrefix {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "network", "hostPrefix"),
				m.Spec.Network.HostPrefix,
				"Network host prefix is immutable"))
	}

	if old.Spec.Platform.Subnet != m.Spec.Platform.Subnet {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "platform", "subnet"),
				m.Spec.Platform.Subnet,
				"Subnet id is immutable"))
	}

	return allErrs
}

func validateName(name string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if lName := strings.ToLower(name); strings.Contains(lName, "microsoft") ||
		strings.Contains(lName, "windows") {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Name"), name,
			"cluster name is invalid because 'MICROSOFT' and 'WINDOWS' can't be used as either a whole word or a substring in the name"))
	}

	return allErrs
}

// validateIdentity validates an Identity.
func (m *AROControlPlane) validateIdentity(_ client.Client) field.ErrorList {
	var allErrs field.ErrorList

	if m.Spec.IdentityRef != nil {
		if m.Spec.IdentityRef.Name == "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "identityRef", "name"), m.Spec.IdentityRef.Name, "cannot be empty"))
		}
	}

	if len(allErrs) > 0 {
		return allErrs
	}

	return nil
}

// validateExternalAuthProviders validates external authentication providers.
func (m *AROControlPlane) validateExternalAuthProviders(_ client.Client) field.ErrorList {
	var allErrs field.ErrorList

	if len(m.Spec.ExternalAuthProviders) == 0 {
		return nil
	}

	basePath := field.NewPath("spec", "externalAuthProviders")
	const maxNameLength = 15

	for i, provider := range m.Spec.ExternalAuthProviders {
		providerPath := basePath.Index(i)

		// Validate name is not empty
		if provider.Name == "" {
			allErrs = append(allErrs, field.Required(providerPath.Child("name"), "provider name cannot be empty"))
		} else if len(provider.Name) > maxNameLength {
			// Validate name length does not exceed Azure's limit
			allErrs = append(allErrs, field.Invalid(
				providerPath.Child("name"),
				provider.Name,
				fmt.Sprintf("name length exceeds maximum allowed length of %d characters (got %d)", maxNameLength, len(provider.Name))))
		}
	}

	return allErrs
}

// validatePlatformFields validates platform-specific fields like KeyVault, Subnet, NetworkSecurityGroupID, and SubscriptionID.
func (m *AROControlPlane) validatePlatformFields(_ client.Client) field.ErrorList {
	var allErrs field.ErrorList

	// Validate KeyVault resource ID
	if m.Spec.Platform.KeyVault != "" {
		allErrs = append(allErrs, validateAzureResourceID(
			m.Spec.Platform.KeyVault,
			field.NewPath("spec", "platform", "keyvault"),
			"KeyVault")...)
	}

	// Validate Subnet resource ID
	if m.Spec.Platform.Subnet != "" {
		allErrs = append(allErrs, validateAzureResourceID(
			m.Spec.Platform.Subnet,
			field.NewPath("spec", "platform", "subnet"),
			"subnet")...)
	}

	// Validate NetworkSecurityGroupID resource ID
	if m.Spec.Platform.NetworkSecurityGroupID != "" {
		allErrs = append(allErrs, validateAzureResourceID(
			m.Spec.Platform.NetworkSecurityGroupID,
			field.NewPath("spec", "platform", "networkSecurityGroupID"),
			"networkSecurityGroup")...)
	}

	// Validate SubscriptionID (GUID format)
	if m.Spec.SubscriptionID != "" {
		allErrs = append(allErrs, validateSubscriptionID(
			m.Spec.SubscriptionID,
			field.NewPath("spec", "subscriptionID"))...)
	}

	return allErrs
}

// validateManagedIdentities validates all managed identities in the ManagedIdentities structure.
func (m *AROControlPlane) validateManagedIdentities(_ client.Client) field.ErrorList {
	var allErrs field.ErrorList

	// Check if ManagedIdentities is zero value (empty struct)
	if reflect.DeepEqual(m.Spec.Platform.ManagedIdentities, ManagedIdentities{}) {
		return allErrs
	}

	managedIdentities := m.Spec.Platform.ManagedIdentities
	basePath := field.NewPath("spec", "platform", "managedIdentities")

	// Validate ServiceManagedIdentity
	if managedIdentities.ServiceManagedIdentity != "" {
		if errs := validateUserAssignedIdentity(managedIdentities.ServiceManagedIdentity, basePath.Child("serviceManagedIdentity")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	// Validate ControlPlaneOperators identities
	if managedIdentities.ControlPlaneOperators != nil {
		controlPlanePath := basePath.Child("controlPlaneOperators")
		controlPlaneOperators := managedIdentities.ControlPlaneOperators

		if controlPlaneOperators.ControlPlaneManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(controlPlaneOperators.ControlPlaneManagedIdentities, controlPlanePath.Child("controlPlaneOperatorsManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if controlPlaneOperators.ClusterAPIAzureManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(controlPlaneOperators.ClusterAPIAzureManagedIdentities, controlPlanePath.Child("clusterApiAzureManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if controlPlaneOperators.CloudControllerManagerManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(controlPlaneOperators.CloudControllerManagerManagedIdentities, controlPlanePath.Child("cloudControllerManager")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if controlPlaneOperators.IngressManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(controlPlaneOperators.IngressManagedIdentities, controlPlanePath.Child("ingressManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if controlPlaneOperators.DiskCsiDriverManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(controlPlaneOperators.DiskCsiDriverManagedIdentities, controlPlanePath.Child("diskCsiDriverManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if controlPlaneOperators.FileCsiDriverManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(controlPlaneOperators.FileCsiDriverManagedIdentities, controlPlanePath.Child("fileCsiDriverManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if controlPlaneOperators.ImageRegistryManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(controlPlaneOperators.ImageRegistryManagedIdentities, controlPlanePath.Child("imageRegistryManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if controlPlaneOperators.CloudNetworkConfigManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(controlPlaneOperators.CloudNetworkConfigManagedIdentities, controlPlanePath.Child("cloudNetworkConfigManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if controlPlaneOperators.KmsManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(controlPlaneOperators.KmsManagedIdentities, controlPlanePath.Child("kmsManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}
	}

	// Validate DataPlaneOperators identities
	if managedIdentities.DataPlaneOperators != nil {
		dataPlaneOperators := managedIdentities.DataPlaneOperators
		dataPlanePath := basePath.Child("dataPlaneOperators")

		if dataPlaneOperators.DiskCsiDriverManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(dataPlaneOperators.DiskCsiDriverManagedIdentities, dataPlanePath.Child("diskCsiDriverManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if dataPlaneOperators.FileCsiDriverManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(dataPlaneOperators.FileCsiDriverManagedIdentities, dataPlanePath.Child("fileCsiDriverManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}

		if dataPlaneOperators.ImageRegistryManagedIdentities != "" {
			if errs := validateUserAssignedIdentity(dataPlaneOperators.ImageRegistryManagedIdentities, dataPlanePath.Child("imageRegistryManagedIdentities")); len(errs) > 0 {
				allErrs = append(allErrs, errs...)
			}
		}
	}

	return allErrs
}

func setDefaultOCPVersion(version string) string {
	if strings.HasPrefix(version, "openshift-v") {
		normalizedVersion := version[11:]
		version = normalizedVersion
	}
	if strings.HasPrefix(version, "v") {
		normalizedVersion := version[1:]
		version = normalizedVersion
	}
	return version
}

// validateAzureResourceID validates an Azure resource ID format with provider-specific checks.
func validateAzureResourceID(resourceID string, fldPath *field.Path, resourceType string) field.ErrorList {
	var allErrs field.ErrorList

	if resourceID == "" {
		return allErrs // Empty is valid (optional field)
	}

	// Parse the resource ID using Azure SDK
	parsedID, err := azureutil.ParseResourceID(resourceID)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, resourceID, "must be a valid Azure "+resourceType+" resource ID"))
		return allErrs
	}

	// Validate the resource ID format and provider-specific requirements
	if validationErrs := validateAzureResourceIDFormat(resourceID, parsedID, fldPath, resourceType); len(validationErrs) > 0 {
		allErrs = append(allErrs, validationErrs...)
	}

	return allErrs
}

// validateSubscriptionID validates an Azure subscription ID (GUID format).
func validateSubscriptionID(subscriptionID string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if subscriptionID == "" {
		return allErrs // Empty is valid (optional field)
	}

	if _, err := uuid.Parse(subscriptionID); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, subscriptionID, "must be a valid GUID"))
	}

	return allErrs
}

// validateUserAssignedIdentity validates a user-assigned identity resource ID.
func validateUserAssignedIdentity(identityResourceID string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if identityResourceID == "" {
		return allErrs // Empty is valid (optional field)
	}

	// Parse the resource ID using Azure SDK
	parsedID, err := azureutil.ParseResourceID(identityResourceID)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, identityResourceID, "must be a valid Azure resource ID"))
		return allErrs
	}

	// Validate the resource ID format and provider-specific requirements
	if validationErrs := validateAzureResourceIDFormat(identityResourceID, parsedID, fldPath, "userAssignedIdentity"); len(validationErrs) > 0 {
		allErrs = append(allErrs, validationErrs...)
	}

	return allErrs
}

// validateAzureResourceIDFormat validates the format and provider-specific requirements of Azure resource IDs.
func validateAzureResourceIDFormat(resourceID string, parsedID *arm.ResourceID, fldPath *field.Path, resourceType string) field.ErrorList {
	var allErrs field.ErrorList

	// Validate basic format: must start with /subscriptions/{subscriptionID}/resourceGroups/{resourceGroupName}
	if parsedID.SubscriptionID == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, resourceID, "must contain a valid subscription ID"))
	} else {
		// Validate subscription ID is a GUID
		if _, err := uuid.Parse(parsedID.SubscriptionID); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, resourceID, "subscription ID must be a valid GUID"))
		}
	}

	if parsedID.ResourceGroupName == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, resourceID, "must contain a valid resource group name"))
	}

	// Validate provider and resource type based on the expected resource type using string parsing
	expectedProviderTypes := getExpectedProviderTypes(resourceType)
	if len(expectedProviderTypes) > 0 {
		providerType := extractProviderTypeFromResourceID(resourceID)
		if providerType != "" && !contains(expectedProviderTypes, providerType) {
			allErrs = append(allErrs, field.Invalid(fldPath, resourceID,
				"provider/type must be one of: "+strings.Join(expectedProviderTypes, ", ")+", got: "+providerType))
		}
	}

	// Validate resource name is not empty
	if parsedID.Name == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, resourceID, "must contain a valid resource name"))
	}

	return allErrs
}

// getExpectedProviderTypes returns the expected provider/type combinations for a given resource type.
func getExpectedProviderTypes(resourceType string) []string {
	switch resourceType {
	case "KeyVault":
		return []string{"Microsoft.KeyVault/vaults"}
	case "subnet":
		return []string{"Microsoft.Network/virtualNetworks"}
	case "networkSecurityGroup":
		return []string{"Microsoft.Network/networkSecurityGroups"}
	case "userAssignedIdentity":
		return []string{"Microsoft.ManagedIdentity/userAssignedIdentities"}
	default:
		// For unknown resource types, allow any provider/type but still validate basic format
		return []string{}
	}
}

// extractProviderTypeFromResourceID extracts the provider/type from an Azure resource ID string.
// Expected format: /subscriptions/{sub}/resourceGroups/{rg}/providers/{provider}/{type}/...
func extractProviderTypeFromResourceID(resourceID string) string {
	parts := strings.Split(resourceID, "/")

	// Find the "providers" segment
	for i, part := range parts {
		if part == "providers" && i+2 < len(parts) {
			// Return provider/type (e.g., "Microsoft.Network/virtualNetworks")
			return parts[i+1] + "/" + parts[i+2]
		}
	}

	return ""
}

// contains checks if a slice contains a specific string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
