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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AzureASOManagedMachinePoolKind is the kind for AzureASOManagedMachinePool.
	AzureASOManagedMachinePoolKind = "AzureASOManagedMachinePool"

	// ReplicasManagedByAKS is the value of the CAPI replica manager annotation that maps to the AKS built-in autoscaler.
	ReplicasManagedByAKS = "aks"
)

// AzureASOManagedMachinePoolSpec defines the desired state of AzureASOManagedMachinePool.
type AzureASOManagedMachinePoolSpec struct {
	AzureASOManagedMachinePoolTemplateResourceSpec `json:",inline"`
}

// AzureASOManagedMachinePoolStatus defines the observed state of AzureASOManagedMachinePool.
type AzureASOManagedMachinePoolStatus struct {
	// Replicas is the current number of provisioned replicas. It fulfills Cluster API's machine pool
	// infrastructure provider contract.
	//+optional
	Replicas int32 `json:"replicas"`

	// Ready represents whether or not the infrastructure is ready to be used. It fulfills Cluster API's
	// machine pool infrastructure provider contract.
	//+optional
	Ready bool `json:"ready"`

	//+optional
	Resources []ResourceStatus `json:"resources,omitempty"`

	// initialization provides observations of the AzureASOManagedMachinePool initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial MachinePool provisioning.
	// +optional
	Initialization *AzureASOManagedMachinePoolInitializationStatus `json:"initialization,omitempty"`
}

// AzureASOManagedMachinePoolInitializationStatus provides observations of the AzureASOManagedMachinePool initialization process.
type AzureASOManagedMachinePoolInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the MachinePool's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial MachinePool provisioning.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// AzureASOManagedMachinePool is the Schema for the azureasomanagedmachinepools API.
type AzureASOManagedMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureASOManagedMachinePoolSpec   `json:"spec,omitempty"`
	Status AzureASOManagedMachinePoolStatus `json:"status,omitempty"`
}

// GetResourceStatuses returns the status of resources.
func (a *AzureASOManagedMachinePool) GetResourceStatuses() []ResourceStatus {
	return a.Status.Resources
}

// SetResourceStatuses sets the status of resources.
func (a *AzureASOManagedMachinePool) SetResourceStatuses(r []ResourceStatus) {
	a.Status.Resources = r
}

//+kubebuilder:object:root=true

// AzureASOManagedMachinePoolList contains a list of AzureASOManagedMachinePool.
type AzureASOManagedMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureASOManagedMachinePool `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &AzureASOManagedMachinePool{}, &AzureASOManagedMachinePoolList{})
}
