/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha3

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/errors"
)

const (
	// MachineFinalizer allows ReconcileAzureMachine to clean up Azure resources associated with AzureMachine before
	// removing it from the apiserver.
	MachineFinalizer = "azuremachine.infrastructure.cluster.x-k8s.io"
)

// AzureMachineSpec defines the desired state of AzureMachine
type AzureMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	VMSize           string           `json:"vmSize"`
	AvailabilityZone AvailabilityZone `json:"availabilityZone,omitempty"`

	// Image is used to provide details of an image to use during VM creation.
	// If image details are omitted the image will default the Azure Marketplace "capi" offer,
	// which is based on Ubuntu.
	// +kubebuilder:validation:nullable
	// +optional
	Image *Image `json:"image,omitempty"`

	// Identity is the type of identity used for the virtual machine.
	// The type 'SystemAssigned' is an implicitly created identity
	// The generated identity will be assigned a Subscription contributor role
	// +kubebuilder:default=None
	// +optional
	Identity VMIdentity `json:"identity,omitempty"`

	OSDisk OSDisk `json:"osDisk"`

	Location string `json:"location"`

	SSHPublicKey string `json:"sshPublicKey"`

	// AdditionalTags is an optional set of tags to add to an instance, in addition to the ones added by default by the
	// Azure provider. If both the AzureCluster and the AzureMachine specify the same tag name with different values, the
	// AzureMachine's value takes precedence.
	// +optional
	AdditionalTags Tags `json:"additionalTags,omitempty"`

	// AllocatePublicIP allows the ability to create dynamic public ips for machines where this value is true.
	// +optional
	AllocatePublicIP bool `json:"allocatePublicIP,omitempty"`
}

// AzureMachineStatus defines the observed state of AzureMachine
type AzureMachineStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Addresses contains the Azure instance associated addresses.
	Addresses []v1.NodeAddress `json:"addresses,omitempty"`

	// VMState is the provisioning state of the Azure virtual machine.
	// +optional
	VMState *VMState `json:"vmState,omitempty"`

	// ErrorReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// ErrorMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="AzureMachine ready status"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.vmState",description="Azure VM provisioning state"
// +kubebuilder:printcolumn:name="Cluster",type="string",priority=1,JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this AzureMachine belongs"
// +kubebuilder:printcolumn:name="Machine",type="string",priority=1,JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object to which this AzureMachine belongs"
// +kubebuilder:printcolumn:name="VM ID",type="string",priority=1,JSONPath=".spec.providerID",description="Azure VM ID"
// +kubebuilder:printcolumn:name="VM Size",type="string",priority=1,JSONPath=".spec.vmSize",description="Azure VM Size"
// +kubebuilder:resource:path=azuremachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// AzureMachine is the Schema for the azuremachines API
type AzureMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureMachineSpec   `json:"spec,omitempty"`
	Status AzureMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureMachineList contains a list of AzureMachine
type AzureMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureMachine{}, &AzureMachineList{})
}
