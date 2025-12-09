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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

// VersionGateAckType specifies the version gate acknowledgment.
type VersionGateAckType string

const (
	// Acknowledge if acknowledgment is required and proceed with the upgrade.
	Acknowledge VersionGateAckType = "Acknowledge"

	// WaitForAcknowledge if acknowledgment is required, wait not to proceed with the upgrade.
	WaitForAcknowledge VersionGateAckType = "WaitForAcknowledge"

	// AlwaysAcknowledge always acknowledg if required and proceed with the upgrade.
	AlwaysAcknowledge VersionGateAckType = "AlwaysAcknowledge"
)

// AROControlPlaneSpec defines the desired state of AROControlPlane.
type AROControlPlaneSpec struct { //nolint: maligned
	// Cluster name must be a valid DNS-1035 label, so it must consist of lower case alphanumeric
	// characters or '-', start with an alphabetic character, end with an alphanumeric character,
	// and have a maximum length of 54 characters.
	//
	// +immutable
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="aroClusterName is immutable"
	// +kubebuilder:validation:MaxLength:=54
	// +kubebuilder:validation:Pattern:=`^[a-z]([-a-z0-9]*[a-z0-9])?$`
	AroClusterName string `json:"aroClusterName"`

	// AROPlatformProfileControlPlane represents the Azure platform configuration.
	Platform AROPlatformProfileControlPlane `json:"platform,omitempty"`

	// Visibility represents the visibility of an API endpoint. Allowed values are public and private; default is public.
	Visibility string `json:"visibility,omitempty"`

	// Network config for the ARO HCP cluster.
	// +optional
	Network *NetworkSpec `json:"network,omitempty"`

	// DomainPrefix is an optional prefix added to the cluster's domain name. It will be used
	// when generating a sub-domain for the cluster on the openshiftapps domain. It must be a valid DNS-1035 label
	// consisting of lower case alphanumeric characters or '-', start with an alphabetic character,
	// end with an alphanumeric character, and have a maximum length of 15 characters.
	//
	// +immutable
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="domainPrefix is immutable"
	// +kubebuilder:validation:MaxLength:=15
	// +kubebuilder:validation:Pattern:=`^[a-z]([-a-z0-9]*[a-z0-9])?$`
	// +optional
	DomainPrefix string `json:"domainPrefix,omitempty"`

	// OpenShift semantic version, for example "4.14.5".
	Version string `json:"version"`

	// OpenShift version channel group; default is stable.
	//
	// +kubebuilder:validation:Enum=stable;candidate;nightly;fast;eus
	// +kubebuilder:default=stable
	ChannelGroup v1beta2.ChannelGroupType `json:"channelGroup"`

	// VersionGate requires acknowledgment when upgrading ARO-HCP y-stream versions (e.g., from 4.15 to 4.16).
	// Default is WaitForAcknowledge.
	// WaitForAcknowledge: If acknowledgment is required, the upgrade will not proceed until VersionGate is set to Acknowledge or AlwaysAcknowledge.
	// Acknowledge: If acknowledgment is required, apply it for the upgrade. After upgrade is done set the version gate to WaitForAcknowledge.
	// AlwaysAcknowledge: If acknowledgment is required, apply it and proceed with the upgrade.
	//
	// +kubebuilder:validation:Enum=Acknowledge;WaitForAcknowledge;AlwaysAcknowledge
	// +kubebuilder:default=WaitForAcknowledge
	VersionGate VersionGateAckType `json:"versionGate"`

	// IdentityRef is a reference to an identity to be used when reconciling the aro control plane.
	// If no identity is specified, the default identity for this controller will be used.
	IdentityRef *corev1.ObjectReference `json:"identityRef,omitempty"`

	// SubscriptionID is the GUID of the Azure subscription that owns this cluster.
	// +optional
	SubscriptionID string `json:"subscriptionID,omitempty"`

	// AzureEnvironment is the name of the AzureCloud to be used.
	// The default value that would be used by most users is "AzurePublicCloud", other values are:
	// - ChinaCloud: "AzureChinaCloud"
	// - PublicCloud: "AzurePublicCloud"
	// - USGovernmentCloud: "AzureUSGovernmentCloud"
	//
	// Note that values other than the default must also be accompanied by corresponding changes to the
	// aso-controller-settings Secret to configure ASO to refer to the non-Public cloud. ASO currently does
	// not support referring to multiple different clouds in a single installation. The following fields must
	// be defined in the Secret:
	// - AZURE_AUTHORITY_HOST
	// - AZURE_RESOURCE_MANAGER_ENDPOINT
	// - AZURE_RESOURCE_MANAGER_AUDIENCE
	//
	// See the [ASO docs] for more details.
	//
	// [ASO docs]: https://azure.github.io/azure-service-operator/guide/aso-controller-settings-options/
	// +optional
	AzureEnvironment string `json:"azureEnvironment,omitempty"`

	// AdditionalTags are user-defined tags to be added on the AWS resources associated with the control plane.
	// +optional
	AdditionalTags infrav1.Tags `json:"additionalTags,omitempty"`

	// EnableExternalAuthProviders enables external authentication configuration for the cluster.
	//
	// +kubebuilder:default=false
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="enableExternalAuthProviders is immutable"
	// +optional
	EnableExternalAuthProviders bool `json:"enableExternalAuthProviders,omitempty"`

	// ExternalAuthProviders are external OIDC identity providers that can issue tokens for this cluster.
	// Can only be set if "enableExternalAuthProviders" is set to "True".
	//
	// At most one provider can be configured.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=1
	ExternalAuthProviders []ExternalAuthProvider `json:"externalAuthProviders,omitempty"`
}

// AROPlatformProfileControlPlane represents the Azure platform configuration.
type AROPlatformProfileControlPlane struct {
	// Location should be valid Azure location ex; centralus
	Location string `json:"location,omitempty"`

	// Resource group name where the ARO-hcp will be attached to it.
	ResourceGroup string `json:"resourceGroup,omitempty"`

	// ResourceGroupRef is the name that is used to create the ResourceGroup CR. The ResourceGroupRef must be in the same namespace as the AROControlPlane and cannot be set with ResourceGroup.
	ResourceGroupRef string `json:"resourceGroupRef,omitempty"`

	// Azure subnet id
	Subnet string `json:"subnet,omitempty"`

	// SubnetRef is the name that is used to create the VirtualNetworksSubnet CR. The SubnetRef must be in the same namespace as the AROControlPlane and cannot be set with Subnet.
	SubnetRef string `json:"subnetRef,omitempty"`

	// OutboundType represents a routing strategy to provide egress to the Internet. Allowed value is loadBalancer
	OutboundType string `json:"outboundType,omitempty"`

	// Azure Network Security Group ID
	NetworkSecurityGroupID string `json:"networkSecurityGroupId,omitempty"`

	// Azure KeyVault id
	KeyVault string `json:"keyVault,omitempty"`

	// ManagedIdentities Azure managed identities for ARO HCP.
	ManagedIdentities ManagedIdentities `json:"managedIdentities,omitempty"`
}

// ManagedIdentities represents managed identities for the Azure platform configuration.
type ManagedIdentities struct {
	// CreateAROHCPManagedIdentities is used to create the required ARO-HCP managed identities if not provided.
	// It will create UserAssignedIdentity CR for each required managed identity. Default is false.
	CreateAROHCPManagedIdentities bool `json:"createAROHCPManagedIdentities,omitempty"`

	// ControlPlaneOperators Ref to Microsoft.ManagedIdentity/userAssignedIdentities
	ControlPlaneOperators *ControlPlaneOperators `json:"controlPlaneOperators,omitempty"`

	// DataPlaneOperators ref to Microsoft.ManagedIdentity/userAssignedIdentities
	DataPlaneOperators *DataPlaneOperators `json:"dataPlaneOperators,omitempty"`

	// ServiceManagedIdentity ref to Microsoft.ManagedIdentity/userAssignedIdentities
	ServiceManagedIdentity string `json:"serviceManagedIdentity,omitempty"`
}

// ControlPlaneOperators represents managed identities for the ControlPlane.
type ControlPlaneOperators struct {
	// ControlPlaneManagedIdentities "control-plane" Microsoft.ManagedIdentity/userAssignedIdentities
	ControlPlaneManagedIdentities string `json:"controlPlaneOperatorsManagedIdentities,omitempty"`

	// ClusterAPIAzureManagedIdentities "cluster-api-azure" Microsoft.ManagedIdentity/userAssignedIdentities
	ClusterAPIAzureManagedIdentities string `json:"clusterApiAzureManagedIdentities,omitempty"`

	// CloudControllerManagerManagedIdentities "cloud-controller-manager" Microsoft.ManagedIdentity/userAssignedIdentities
	CloudControllerManagerManagedIdentities string `json:"cloudControllerManager,omitempty"`

	// IngressManagedIdentities "ingress" Microsoft.ManagedIdentity/userAssignedIdentities
	IngressManagedIdentities string `json:"ingressManagedIdentities,omitempty"`

	// DiskCsiDriverManagedIdentities "disk-csi-driver" Microsoft.ManagedIdentity/userAssignedIdentities
	DiskCsiDriverManagedIdentities string `json:"diskCsiDriverManagedIdentities,omitempty"`

	// FileCsiDriverManagedIdentities "file-csi-driver" Microsoft.ManagedIdentity/userAssignedIdentities
	FileCsiDriverManagedIdentities string `json:"fileCsiDriverManagedIdentities,omitempty"`

	// ImageRegistryManagedIdentities "image-registry" Microsoft.ManagedIdentity/userAssignedIdentities
	ImageRegistryManagedIdentities string `json:"imageRegistryManagedIdentities,omitempty"`

	// CloudNetworkConfigManagedIdentities "cloud-network-config" Microsoft.ManagedIdentity/userAssignedIdentities
	CloudNetworkConfigManagedIdentities string `json:"cloudNetworkConfigManagedIdentities,omitempty"`

	// KmsManagedIdentities "kms" Microsoft.ManagedIdentity/userAssignedIdentities
	KmsManagedIdentities string `json:"kmsManagedIdentities,omitempty"`
}

// DataPlaneOperators represents managed identities for the DataPlane.
type DataPlaneOperators struct {
	// DiskCsiDriverManagedIdentities "disk-csi-driver" Microsoft.ManagedIdentity/userAssignedIdentities
	DiskCsiDriverManagedIdentities string `json:"diskCsiDriverManagedIdentities,omitempty"`

	// FileCsiDriverManagedIdentities "file-csi-driver" Microsoft.ManagedIdentity/userAssignedIdentities
	FileCsiDriverManagedIdentities string `json:"fileCsiDriverManagedIdentities,omitempty"`

	// ImageRegistryManagedIdentities "image-registry" Microsoft.ManagedIdentity/userAssignedIdentities
	ImageRegistryManagedIdentities string `json:"imageRegistryManagedIdentities,omitempty"`
}

// NetworkSpec for ARO-HCP.
type NetworkSpec struct {
	// IP addresses block used by OpenShift while installing the cluster, for example "10.0.0.0/16".
	// +kubebuilder:validation:Format=cidr
	// +optional
	MachineCIDR string `json:"machineCIDR,omitempty"`

	// IP address block from which to assign pod IP addresses, for example `10.128.0.0/14`.
	// +kubebuilder:validation:Format=cidr
	// +optional
	PodCIDR string `json:"podCIDR,omitempty"`

	// IP address block from which to assign service IP addresses, for example `172.30.0.0/16`.
	// +kubebuilder:validation:Format=cidr
	// +optional
	ServiceCIDR string `json:"serviceCIDR,omitempty"`

	// Network host prefix which is defaulted to `23` if not specified.
	// +kubebuilder:default=23
	// +optional
	HostPrefix int `json:"hostPrefix,omitempty"`

	// The CNI network type; default is OVNKubernetes.
	// +kubebuilder:validation:Enum=OVNKubernetes;Other
	// +kubebuilder:default=OVNKubernetes
	// +optional
	NetworkType string `json:"networkType,omitempty"`
}

// AROControlPlaneStatus defines the observed state of AROControlPlane.
type AROControlPlaneStatus struct {
	// initialization provides observations of the AROControlPlane initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization *AROControlPlaneInitializationStatus `json:"initialization,omitempty"`
	// Ready denotes that the AROControlPlane API Server is ready to receive requests.
	// +kubebuilder:default=false
	Ready bool `json:"ready"`
	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the state and will be set to a descriptive error message.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the spec or the configuration of
	// the controller, and that manual intervention is required.
	//
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
	// Conditions specifies the conditions for the managed control plane
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// ID is the cluster ID given by ARO.
	ID string `json:"id,omitempty"`
	// ConsoleURL is the url for the openshift console.
	ConsoleURL string `json:"consoleURL,omitempty"`

	// APIURL is the url for the ARO-HCP openshift cluster api endPoint.
	APIURL string `json:"apiURL,omitempty"`

	// ARO-HCP OpenShift semantic version, for example "4.20.0".
	Version string `json:"version,omitempty"`

	// Available upgrades for the ARO hosted control plane.
	AvailableUpgrades []string `json:"availableUpgrades,omitempty"`

	// LongRunningOperationStates saves the state for ARO long-running operations so they can be continued on the
	// next reconciliation loop.
	// +optional
	LongRunningOperationStates infrav1.Futures `json:"longRunningOperationStates,omitempty"`
}

// AROControlPlaneInitializationStatus provides observations of the AROControlPlane initialization process.
type AROControlPlaneInitializationStatus struct {
	// controlPlaneInitialized is true when the AROControlPlane provider reports that the Kubernetes control plane is initialized;
	// A control plane is considered initialized when it can accept requests, no matter if this happens before
	// the control plane is fully provisioned or not.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Machine provisioning.
	// +optional
	ControlPlaneInitialized bool `json:"controlPlaneInitialized,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=arocontrolplanes,shortName=arocp,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this AROControl belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Control plane infrastructure is ready for worker nodes"
// +kubebuilder:printcolumn:name="Console URL",type="string",JSONPath=".status.consoleURL",description="OpenShift Console URL"
// +k8s:defaulter-gen=true
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes/finalizers,verbs=update

// AROControlPlane is the Schema for the AROControlPlanes API.
type AROControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AROControlPlaneSpec   `json:"spec,omitempty"`
	Status AROControlPlaneStatus `json:"status,omitempty"`
}

// GetFutures returns the list of long running operation states for an AzureCluster API object.
func (c *AROControlPlane) GetFutures() infrav1.Futures {
	return c.Status.LongRunningOperationStates
}

// SetFutures will set the given long running operation states on an AzureCluster object.
func (c *AROControlPlane) SetFutures(futures infrav1.Futures) {
	c.Status.LongRunningOperationStates = futures
}

const (
	// AROControlPlaneKind is the kind for AROControlPlane.
	AROControlPlaneKind = "AROControlPlane"

	// AROControlPlaneFinalizer is the finalizer added to AROControlPlanes.
	AROControlPlaneFinalizer = "arocontrolplanes/finalizer"

	// AROControlPlaneReadyCondition condition reports on the successful reconciliation of AROControlPlane.
	AROControlPlaneReadyCondition clusterv1.ConditionType = "AROControlPlaneReady"

	// AROControlPlaneValidCondition condition reports whether AROControlPlane configuration is valid.
	AROControlPlaneValidCondition clusterv1.ConditionType = "AROControlPlaneValid"

	// AROControlPlaneUpgradingCondition condition reports whether AROControlPlane is upgrading or not.
	AROControlPlaneUpgradingCondition clusterv1.ConditionType = "AROControlPlaneUpgrading"

	// HcpClusterReadyCondition mirrors the Ready condition from the HcpOpenShiftCluster ASO resource.
	HcpClusterReadyCondition clusterv1.ConditionType = "HcpClusterReady"

	// ExternalAuthReadyCondition reports on the successful configuration of external authentication providers.
	ExternalAuthReadyCondition clusterv1.ConditionType = "ExternalAuthReady"
)

// +kubebuilder:object:root=true

// AROControlPlaneList contains a list of AROControlPlane.
type AROControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AROControlPlane `json:"items"`
}

// GetConditions returns the control planes conditions.
func (c *AROControlPlane) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the status conditions for the AROControlPlane.
func (c *AROControlPlane) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// NodeResourceGroup returns the node resource group name for the ARO control plane.
func (c *AROControlPlane) NodeResourceGroup() string {
	nodeResourceGroup := fmt.Sprintf("capz_node_%s_%s_rg", c.Spec.AroClusterName, c.Spec.Platform.ResourceGroup)
	return nodeResourceGroup
}

func init() {
	SchemeBuilder.Register(&AROControlPlane{}, &AROControlPlaneList{})
}
