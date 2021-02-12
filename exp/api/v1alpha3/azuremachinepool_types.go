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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

const (
	// MachinePoolNameLabel indicates the AzureMachinePool name the AzureMachinePoolMachine belongs
	MachinePoolNameLabel = "azuremachinepool.infrastructure.cluster.x-k8s.io/machine-pool"
)

type (
	// AzureMachineTemplate defines the template for an AzureMachine.
	AzureMachineTemplate struct {
		// VMSize is the size of the Virtual Machine to build.
		// See https://docs.microsoft.com/en-us/rest/api/compute/virtualmachines/createorupdate#virtualmachinesizetypes
		VMSize string `json:"vmSize"`

		// Image is used to provide details of an image to use during Virtual Machine creation.
		// If image details are omitted the image will default the Azure Marketplace "capi" offer,
		// which is based on Ubuntu.
		// +kubebuilder:validation:nullable
		// +optional
		Image *AzureDefaultingImage `json:"image,omitempty"`

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

		// TerminateNotificationTimeout enables or disables VMSS scheduled events termination notification with specified timeout
		// allowed values are between 5 and 15 (mins)
		// +optional
		TerminateNotificationTimeout *int `json:"terminateNotificationTimeout,omitempty"`

		// SecurityProfile specifies the Security profile settings for a virtual machine.
		// +optional
		SecurityProfile *infrav1.SecurityProfile `json:"securityProfile,omitempty"`

		// SpotVMOptions allows the ability to specify the Machine should use a Spot VM
		// +optional
		SpotVMOptions *infrav1.SpotVMOptions `json:"spotVMOptions,omitempty"`
	}

	// AzureDefaultingImage defines information about the image to use for VMSS VM creation.
	// There are three ways to specify an image: by ID, Marketplace Image or SharedImageGallery
	// One of ID, SharedImage or Marketplace should be set.
	//
	// If no image is specified, the image will be defaulted by the controller and Defaulted set to true which will
	// inform the controller that if the parent MachinePool.Spec.Template.Spec.Version field is updated, this image
	// will be updated with the corresponding default image. If Defaulted is set to false, the controller will not
	// update the image reference when the MachinePool.Spec.Template.Spec.Version changes.
	AzureDefaultingImage struct {
		*infrav1.Image `json:",omitempty"`

		// Defaulted informs the controller that the image reference was defaulted so that it can be updated by changes
		// to the MachinePool.Spec.Template.Spec.Version field by default.
		// +optional
		Defaulted bool `json:"defaulted,omitempty"`
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
		// +optional
		ProviderID string `json:"providerID,omitempty"`

		// ProviderIDList are the identification IDs of machine instances provided by the provider.
		// This field must match the provider IDs as seen on the node objects corresponding to a machine pool's machine instances.
		// +optional
		ProviderIDList []string `json:"providerIDList,omitempty"`

		// Identity is the type of identity used for the Virtual Machine Scale Set.
		// The type 'SystemAssigned' is an implicitly created identity.
		// The generated identity will be assigned a Subscription contributor role.
		// The type 'UserAssigned' is a standalone Azure resource provided by the user
		// and assigned to the VM
		// +kubebuilder:default=None
		// +optional
		Identity infrav1.VMIdentity `json:"identity,omitempty"`

		// UserAssignedIdentities is a list of standalone Azure identities provided by the user
		// The lifecycle of a user-assigned identity is managed separately from the lifecycle of
		// the AzureMachinePool.
		// See https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/how-to-manage-ua-identity-cli
		// +optional
		UserAssignedIdentities []infrav1.UserAssignedIdentity `json:"userAssignedIdentities,omitempty"`

		// RoleAssignmentName is the name of the role assignment to create for a system assigned identity. It can be any valid GUID.
		// If not specified, a random GUID will be generated.
		// +optional
		RoleAssignmentName string `json:"roleAssignmentName,omitempty"`

		// MaxUnavailable is the max number of replicas which can be unavailable at any time. For example, if the
		// machine pool has a replica count of 5 and a MaxUnavailable value of 2, a rolling upgrade will never exceed
		// more than 2 machines unavailable at a time.
		//
		// Note: If the MaxUnavailable value is equal to the machine pool replica value, there is
		// the possibility of application down time.
		// +kubebuilder:default=1
		MaxUnavailable int32 `json:"maxUnavailable,omitempty"`

		// MaxSurge is the number of replicas to surge during an upgrade. For example, if the machine pool has a replica
		// count of 5 and a MaxSurge value of 4, during an upgrade, the number of Virtual Machine Scale Set instances
		// will increase to 9 consisting of the 5 existing replicas and 4 new replicas running the updated model. The
		// surge will temporaly increase the number of machines in the VMSS to expedite upgrades.
		//
		// Note: Azure Subscriptions have a limited quota of CPUs. This quota can be expanded upon request. Due to the
		// nature of this feature, spikes due to surge may cause you to exceed quota.
		//
		// See Also: https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/azure-subscription-service-limits
		MaxSurge *int32 `json:"maxSurge,omitempty"`
	}

	// AzureMachinePoolStatus defines the observed state of AzureMachinePool
	AzureMachinePoolStatus struct {
		// Ready is true when the provider resource is ready.
		// +optional
		Ready bool `json:"ready"`

		// Replicas is the most recently observed number of replicas.
		// +optional
		Replicas int32 `json:"replicas"`

		// Version is the Kubernetes version for the current VMSS model
		// +optional
		Version string `json:"version"`

		// ProvisioningState is the provisioning state of the Azure virtual machine.
		// +optional
		ProvisioningState *infrav1.VMState `json:"provisioningState,omitempty"`

		// FailureReason will be set in the event that there is a terminal problem
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

		// FailureMessage will be set in the event that there is a terminal problem
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

		// Conditions defines current service state of the AzureMachinePool.
		// +optional
		Conditions clusterv1.Conditions `json:"conditions,omitempty"`

		// LongRunningOperationState saves the state for an Azure long running operations so it can be continued on the
		// next reconciliation loop.
		// +optional
		LongRunningOperationState *infrav1.Future `json:"longRunningOperationState,omitempty"`
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

// GetConditions returns the list of conditions for an AzureMachinePool API object.
func (amp *AzureMachinePool) GetConditions() clusterv1.Conditions {
	return amp.Status.Conditions
}

// SetConditions will set the given conditions on an AzureMachinePool object
func (amp *AzureMachinePool) SetConditions(conditions clusterv1.Conditions) {
	amp.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&AzureMachinePool{}, &AzureMachinePoolList{})
}
