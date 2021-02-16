/*
Copyright The Kubernetes Authors.

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
)

// AzureContainerInstanceMachineTemplateSpec defines the desired state of AzureContainerInstanceMachineTemplate
type AzureContainerInstanceMachineTemplateSpec struct {
	Template AzureContainerInstanceMachineTemplateResource `json:"template"`
}

// AzureContainerInstanceMachineTemplateResource describes the data needed to create an AzureContainerInstanceMachine from a template
type AzureContainerInstanceMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec AzureContainerInstanceMachineSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// AzureContainerInstanceMachineTemplate is the Schema for the azurecontainerinstancemachinetemplates API
type AzureContainerInstanceMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureContainerInstanceMachineTemplateSpec   `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// AzureContainerInstanceMachineTemplateList contains a list of AzureContainerInstanceMachineTemplate
type AzureContainerInstanceMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureContainerInstanceMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureContainerInstanceMachineTemplate{}, &AzureContainerInstanceMachineTemplateList{})
}
