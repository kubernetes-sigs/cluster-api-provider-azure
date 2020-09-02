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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

// AzureManagedControlPlaneSpec defines the desired state of AzureManagedControlPlane
type AzureManagedControlPlaneSpec struct {
	// Version defines the desired Kubernetes version.
	// +kubebuilder:validation:MinLength:=2
	Version string `json:"version"`

	// ResourceGroup is the name of the Azure resource group for this AKS Cluster.
	ResourceGroup string `json:"resourceGroup"`

	// SubscriotionID is the GUID of the Azure subscription to hold this cluster.
	SubscriptionID string `json:"subscriptionID,omitempty"`

	// Location is a string matching one of the canonical Azure region names. Examples: "westus2", "eastus".
	Location string `json:"location"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// AdditionalTags is an optional set of tags to add to Azure resources managed by the Azure provider, in addition to the
	// ones added by default.
	// +optional
	AdditionalTags infrav1.Tags `json:"additionalTags,omitempty"`

	// NetworkPlugin used for building Kubernetes network.
	// +kubebuilder:validation:Enum=azure;kubenet
	// +optional
	NetworkPlugin *string `json:"networkPlugin,omitempty"`

	// NetworkPolicy used for building Kubernetes network.
	// +kubebuilder:validation:Enum=azure;calico
	// +optional
	NetworkPolicy *string `json:"networkPolicy,omitempty"`

	// SSHPublicKey is a string literal containing an ssh public key base64 encoded.
	SSHPublicKey string `json:"sshPublicKey"`

	// DefaultPoolRef is the specification for the default pool, without which an AKS cluster cannot be created.
	DefaultPoolRef corev1.LocalObjectReference `json:"defaultPoolRef"`

	// DNSServiceIP is an IP address assigned to the Kubernetes DNS service.
	// It must be within the Kubernetes service address range specified in serviceCidr.
	// +optional
	DNSServiceIP *string `json:"dnsServiceIP,omitempty"`

	// LoadBalancerSKU is the SKU of the loadBalancer to be provisioned.
	// +kubebuilder:validation:Enum=Basic;Standard
	// +optional
	LoadBalancerSKU *string `json:"loadBalancerSKU,omitempty"`
}

// AzureManagedControlPlaneStatus defines the observed state of AzureManagedControlPlane
type AzureManagedControlPlaneStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Initialized is true when the the control plane is available for initial contact.
	// This may occur before the control plane is fully ready.
	// In the AzureManagedControlPlane implementation, these are identical.
	// +optional
	Initialized bool `json:"initialized,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=azuremanagedcontrolplanes,scope=Namespaced,categories=cluster-api,shortName=amcp
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// AzureManagedControlPlane is the Schema for the azuremanagedcontrolplanes API
type AzureManagedControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureManagedControlPlaneSpec   `json:"spec,omitempty"`
	Status AzureManagedControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureManagedControlPlaneList contains a list of AzureManagedControlPlane
type AzureManagedControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureManagedControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureManagedControlPlane{}, &AzureManagedControlPlaneList{})
}
