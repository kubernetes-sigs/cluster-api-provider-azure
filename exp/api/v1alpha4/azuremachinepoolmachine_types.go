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

package v1alpha4

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
)

const (
	// AzureMachinePoolMachineFinalizer is used to ensure deletion of dependencies (nodes, infra).
	AzureMachinePoolMachineFinalizer = "azuremachinepoolmachine.infrastructure.cluster.x-k8s.io"
)

type (

	// AzureMachinePoolMachineSpec defines the desired state of AzureMachinePoolMachine.
	AzureMachinePoolMachineSpec struct {
		// ProviderID is the identification ID of the Virtual Machine Scale Set
		ProviderID string `json:"providerID"`

		// InstanceID is the identification of the Machine Instance within the VMSS
		InstanceID string `json:"instanceID"`
	}

	// AzureMachinePoolMachineStatus defines the observed state of AzureMachinePoolMachine.
	AzureMachinePoolMachineStatus struct {
		// NodeRef will point to the corresponding Node if it exists.
		// +optional
		NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

		// Version defines the Kubernetes version for the VM Instance
		// +optional
		Version string `json:"version"`

		// ProvisioningState is the provisioning state of the Azure virtual machine instance.
		// +optional
		ProvisioningState *infrav1.ProvisioningState `json:"provisioningState"`

		// InstanceName is the name of the Machine Instance within the VMSS
		// +optional
		InstanceName string `json:"instanceName"`

		// FailureReason will be set in the event that there is a terminal problem
		// reconciling the MachinePool machine and will contain a succinct value suitable
		// for machine interpretation.
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

		// LatestModelApplied indicates the instance is running the most up-to-date VMSS model. A VMSS model describes
		// the image version the VM is running. If the instance is not running the latest model, it means the instance
		// may not be running the version of Kubernetes the Machine Pool has specified and needs to be updated.
		LatestModelApplied bool `json:"latestModelApplied"`

		// Ready is true when the provider resource is ready.
		// +optional
		Ready bool `json:"ready"`
	}

	// +kubebuilder:object:root=true
	// +kubebuilder:subresource:status
	// +kubebuilder:resource:path=azuremachinepoolmachines,scope=Namespaced,categories=cluster-api,shortName=ampm
	// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="Kubernetes version"
	// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Flag indicating infrastructure is successfully provisioned"
	// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.provisioningState",description="Azure VMSS VM provisioning state"
	// +kubebuilder:printcolumn:name="Cluster",type="string",priority=1,JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this AzureMachinePoolMachine belongs"
	// +kubebuilder:printcolumn:name="VMSS VM ID",type="string",priority=1,JSONPath=".spec.providerID",description="Azure VMSS VM ID"

	// AzureMachinePoolMachine is the Schema for the azuremachinepoolmachines API.
	AzureMachinePoolMachine struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty"`

		Spec   AzureMachinePoolMachineSpec   `json:"spec,omitempty"`
		Status AzureMachinePoolMachineStatus `json:"status,omitempty"`
	}

	// +kubebuilder:object:root=true

	// AzureMachinePoolMachineList contains a list of AzureMachinePoolMachines.
	AzureMachinePoolMachineList struct {
		metav1.TypeMeta `json:",inline"`
		metav1.ListMeta `json:"metadata,omitempty"`
		Items           []AzureMachinePoolMachine `json:"items"`
	}
)

// GetConditions returns the list of conditions for an AzureMachinePool API object.
func (ampm *AzureMachinePoolMachine) GetConditions() clusterv1.Conditions {
	return ampm.Status.Conditions
}

// SetConditions will set the given conditions on an AzureMachinePool object.
func (ampm *AzureMachinePoolMachine) SetConditions(conditions clusterv1.Conditions) {
	ampm.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&AzureMachinePoolMachine{}, &AzureMachinePoolMachineList{})
}
