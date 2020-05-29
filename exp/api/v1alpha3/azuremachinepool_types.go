/*
Copyright 2020 The Kubernetes Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

type (
	AzureMachineTemplate struct {
		// VMSize is the size of the Virtual Machine to build.
		// See https://docs.microsoft.com/en-us/rest/api/compute/virtualmachines/createorupdate#virtualmachinesizetypes
		VMSize string `json:"vmSize"`

		// Image is used to provide details of an image to use during Virtual Machine creation.
		// If image details are omitted the image will default the Azure Marketplace "capi" offer,
		// which is based on Ubuntu.
		// +kubebuilder:validation:nullable
		// +optional
		Image *infrav1.Image `json:"image,omitempty"`

		// OSDisk contains the operating system disk information for a Virtual Machine
		OSDisk infrav1.OSDisk `json:"osDisk"`

		// DataDisks specifies the list of data disks to be created for a Virtual Machine
		// +optional
		DataDisks []infrav1.DataDisk `json:"dataDisks,omitempty"`

		// SSHPublicKey is the SSH public key string base64 encoded to add to a Virtual Machine
		SSHPublicKey string `json:"sshPublicKey"`

		// AcceleratedNetworking enables or disables Azure accelerated networking. If omitted, it will be set based on
		// whether the requested VMSize supports accelerated networking.
		// If AcceleratedNetworking is set to true with a VMSize that does not support it, Azure will return an error.
		// +optional
		AcceleratedNetworking *bool `json:"acceleratedNetworking,omitempty"`
	}

	// AzureMachinePoolSpec defines the desired state of AzureMachinePool
	AzureMachinePoolSpec struct {
		// Location is the Azure region location e.g. westus2
		Location string `json:"location"`

		// Template contains the details used to build a replica virtual machine within the Machine Pool
		Template AzureMachineTemplate `json:"template"`

		// AdditionalTags is an optional set of tags to add to an instance, in addition to the ones added by default by the
		// Azure provider. If both the AzureCluster and the AzureMachine specify the same tag name with different values, the
		// AzureMachine's value takes precedence.
		// +optional
		AdditionalTags infrav1.Tags `json:"additionalTags,omitempty"`

		// ProviderID is the identification ID of the Virtual Machine Scale Set
		ProviderID string `json:"providerID,omitempty"`

		// ProviderIDList are the identification IDs of machine instances provided by the provider.
		// This field must match the provider IDs as seen on the node objects corresponding to a machine pool's machine instances.
		// +optional
		ProviderIDList []string `json:"providerIDList,omitempty"`
	}

	// AzureMachinePoolStatus defines the observed state of AzureMachinePool
	AzureMachinePoolStatus struct {
		// Ready is true when the provider resource is ready.
		// +optional
		Ready bool `json:"ready"`

		// Replicas is the most recently observed number of replicas.
		// +optional
		Replicas int32 `json:"replicas"`

		// VMState is the provisioning state of the Azure virtual machine.
		// +optional
		ProvisioningState *infrav1.VMState `json:"provisioningState,omitempty"`

		// ErrorReason will be set in the event that there is a terminal problem
		// reconciling the MachinePool and will contain a succinct value suitable
		// for machine interpretation.
		//
		// This field should not be set for transitive errors that a controller
		// faces that are expected to be fixed automatically over
		// time (like service outages), but instead indicate that something is
		// fundamentally wrong with the MachinePool's spec or the configuration of
		// the controller, and that manual intervention is required. Examples
		// of terminal errors would be invalid combinations of settings in the
		// spec, values that are unsupported by the controller, or the
		// responsible controller itself being critically misconfigured.
		//
		// Any transient errors that occur during the reconciliation of MachinePools
		// can be added as events to the MachinePool object and/or logged in the
		// controller's output.
		// +optional
		FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

		// ErrorMessage will be set in the event that there is a terminal problem
		// reconciling the MachinePool and will contain a more verbose string suitable
		// for logging and human consumption.
		//
		// This field should not be set for transitive errors that a controller
		// faces that are expected to be fixed automatically over
		// time (like service outages), but instead indicate that something is
		// fundamentally wrong with the MachinePool's spec or the configuration of
		// the controller, and that manual intervention is required. Examples
		// of terminal errors would be invalid combinations of settings in the
		// spec, values that are unsupported by the controller, or the
		// responsible controller itself being critically misconfigured.
		//
		// Any transient errors that occur during the reconciliation of MachinePools
		// can be added as events to the MachinePool object and/or logged in the
		// controller's output.
		// +optional
		FailureMessage *string `json:"failureMessage,omitempty"`
	}

	// +kubebuilder:object:root=true
	// +kubebuilder:subresource:status
	// +kubebuilder:resource:path=azuremachinepools,scope=Namespaced,categories=cluster-api,shortName=amp
	// +kubebuilder:printcolumn:name="Replicas",type="string",JSONPath=".status.replicas",description="AzureMachinePool replicas count"
	// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="AzureMachinePool replicas count"
	// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.provisioningState",description="Azure VMSS provisioning state"
	// +kubebuilder:printcolumn:name="Cluster",type="string",priority=1,JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this AzureMachinePool belongs"
	// +kubebuilder:printcolumn:name="MachinePool",type="string",priority=1,JSONPath=".metadata.ownerReferences[?(@.kind==\"MachinePool\")].name",description="MachinePool object to which this AzureMachinePool belongs"
	// +kubebuilder:printcolumn:name="VMSS ID",type="string",priority=1,JSONPath=".spec.providerID",description="Azure VMSS ID"
	// +kubebuilder:printcolumn:name="VM Size",type="string",priority=1,JSONPath=".spec.template.vmSize",description="Azure VM Size"

	// AzureMachinePool is the Schema for the azuremachinepools API
	AzureMachinePool struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty"`

		Spec   AzureMachinePoolSpec   `json:"spec,omitempty"`
		Status AzureMachinePoolStatus `json:"status,omitempty"`
	}

	// +kubebuilder:object:root=true

	// AzureMachinePoolList contains a list of AzureMachinePool
	AzureMachinePoolList struct {
		metav1.TypeMeta `json:",inline"`
		metav1.ListMeta `json:"metadata,omitempty"`
		Items           []AzureMachinePool `json:"items"`
	}
)

func init() {
	SchemeBuilder.Register(&AzureMachinePool{}, &AzureMachinePoolList{})
}
