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
	// initialization provides observations of the AzureASOManagedMachinePool initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization AzureASOManagedMachinePoolInitializationStatus `json:"initialization,omitempty,omitzero"`

	// Replicas is the current number of provisioned replicas. It fulfills Cluster API's machine pool
	// infrastructure provider contract.
	//+optional
	Replicas int32 `json:"replicas"`

	//+optional
	Resources []ResourceStatus `json:"resources,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed in a future version.
	// +optional
	Deprecated *AzureASOManagedMachinePoolDeprecatedStatus `json:"deprecated,omitempty"`
}

// AzureASOManagedMachinePoolInitializationStatus provides observations of the AzureASOManagedMachinePool initialization process.
// +kubebuilder:validation:MinProperties=1
type AzureASOManagedMachinePoolInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the AzureASOManagedMachinePool's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// AzureASOManagedMachinePoolDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
type AzureASOManagedMachinePoolDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *AzureASOManagedMachinePoolV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// AzureASOManagedMachinePoolV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
type AzureASOManagedMachinePoolV1Beta1DeprecatedStatus struct {
	// ready is true when the provider resource is ready.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	Ready bool `json:"ready,omitempty"`
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
