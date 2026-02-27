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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// AROControlPlaneSpec defines the desired state of AROControlPlane.
type AROControlPlaneSpec struct { //nolint: maligned
	// Resources are embedded ASO resources to be managed by this AROControlPlane.
	// This allows you to define the full infrastructure including HcpOpenShiftCluster and
	// HcpOpenShiftClustersExternalAuth resources directly using ASO types.
	//
	// All cluster configuration (version, domain prefix, channel group, etc.) should be
	// defined in the HcpOpenShiftCluster resource within this field.
	//
	// +optional
	Resources []runtime.RawExtension `json:"resources,omitempty"`

	// IdentityRef is a reference to an identity to be used when reconciling the aro control plane.
	// This field is optional. When set, CAPZ will initialize Azure SDK credentials and perform
	// Key Vault operations (check existence, create encryption keys, propagate key versions).
	//
	// When NOT set (ASO credential-based mode), ASO handles authentication via
	// serviceoperator.azure.com/credential-from annotations, and customers must manually
	// create the vault and encryption key via ASO and specify the keyVersion in HcpOpenShiftCluster spec.
	//
	// +optional
	IdentityRef *corev1.ObjectReference `json:"identityRef,omitempty"`

	// SubscriptionID is the GUID of the Azure subscription that owns this cluster.
	// Required for Azure API authentication and ARM resource ID construction.
	//
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
	Conditions []metav1.Condition `json:"conditions,omitempty"`

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

	// Resources represents the status of ASO resources managed by this AROControlPlane.
	// This is populated when using the Resources field in the spec.
	//+optional
	Resources []infrav1.ResourceStatus `json:"resources,omitempty"`

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

	// EncryptionKeyReadyCondition reports on the status of the encryption key for ETCD data encryption.
	EncryptionKeyReadyCondition clusterv1.ConditionType = "EncryptionKeyReady"
)

// +kubebuilder:object:root=true

// AROControlPlaneList contains a list of AROControlPlane.
type AROControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AROControlPlane `json:"items"`
}

// GetConditions returns the control planes conditions.
func (c *AROControlPlane) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions sets the status conditions for the AROControlPlane.
func (c *AROControlPlane) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// GetResourceStatuses returns the status of resources.
func (c *AROControlPlane) GetResourceStatuses() []infrav1.ResourceStatus {
	return c.Status.Resources
}

// SetResourceStatuses sets the status of resources.
func (c *AROControlPlane) SetResourceStatuses(r []infrav1.ResourceStatus) {
	c.Status.Resources = r
}

func init() {
	SchemeBuilder.Register(&AROControlPlane{}, &AROControlPlaneList{})
}
