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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
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

	// NetworkPlugin used for building Kubernetes network. Possible values include: 'Azure', 'Kubenet'. Defaults to Azure.
	// +kubebuilder:validation:Enum=Azure;Kubenet
	NetworkPlugin *string `json:"networkPlugin,omitempty"`

	// NetworkPolicy used for building Kubernetes network. Possible values include: 'Calico', 'Azure'
	// +kubebuilder:validation:Enum=Calico;Azure
	NetworkPolicy *string `json:"networkPolicy,omitempty"`

	// SSHPublicKey is a string literal containing an ssh public key.
	SSHPublicKey string `json:"sshPublicKey"`

	// DefaultPoolRef is the specification for the default pool, without which an AKS cluster cannot be created.
	// TODO(ace): consider defaulting and making optional pointer?
	DefaultPoolRef corev1.LocalObjectReference `json:"defaultPoolRef"`
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

	// VirtualNetwork is the name of the AKS-generated VNET. It's not predictable unless we pre-create it.
	// Consider pre-creating?
	VirtualNetwork string `json:"virtualNetwork,omitempty"`
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

func (c *AzureManagedControlPlane) ManagedResourceGroup() string {
	return fmt.Sprintf("MC_%s_%s_%s", c.Spec.ResourceGroup, c.Name, c.Spec.Location)
}

// SubscriptionID returns the cluster resource group.
func (c *AzureManagedControlPlane) SubscriptionID() string {
	return c.Spec.SubscriptionID
}

// ResourceGroup returns the cluster resource group.
func (c *AzureManagedControlPlane) ResourceGroup() string {
	return c.ManagedResourceGroup()
}

// ClusterName returns the cluster name.
func (c *AzureManagedControlPlane) ClusterName() string {
	for _, ref := range c.OwnerReferences {
		if ref.Kind != "Cluster" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return ""
		}
		if gv.Group == clusterv1.GroupVersion.Group {
			return ref.Name
		}
	}
	return ""
}

// Location returns the cluster location.
func (c *AzureManagedControlPlane) Location() string {
	return c.Spec.Location
}

// SetFailureDomain is a no-op on managed control planes
func (c *AzureManagedControlPlane) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {}

// AdditionalTags returns AdditionalTags from the scope's AzureManagedControlPlane.
func (c *AzureManagedControlPlane) AdditionalTags() infrav1.Tags {
	tags := make(infrav1.Tags)
	if c.Spec.AdditionalTags != nil {
		tags = c.Spec.AdditionalTags.DeepCopy()
	}
	return tags
}

// END

// NetworkDescriber implementation

// LoadBalancerName returns the node load balancer name.
func (c *AzureManagedControlPlane) LoadBalancerName() string {
	return "kubernetes" // hard-coded in aks
}

// Network returns the cluster network object.
func (c *AzureManagedControlPlane) Network() *infrav1.Network {
	return nil
}

// Vnet returns the cluster Vnet.
func (c *AzureManagedControlPlane) Vnet() *infrav1.VnetSpec {
	return &infrav1.VnetSpec{
		ResourceGroup: c.ManagedResourceGroup(),
		Name:          c.Status.VirtualNetwork,
	}
}

// IsVnetManaged returns true if the vnet is managed.
func (c *AzureManagedControlPlane) IsVnetManaged() bool {
	return true
}

// Subnets returns the cluster subnets.
func (c *AzureManagedControlPlane) Subnets() infrav1.Subnets {
	return infrav1.Subnets{
		{
			Role: infrav1.SubnetNode,
			Name: "aks-subnet",
		},
	}
}

// ControlPlaneSubnet returns the cluster control plane subnet.
func (c *AzureManagedControlPlane) ControlPlaneSubnet() *infrav1.SubnetSpec {
	return nil
}

// NodeSubnet returns the cluster node subnet.
func (c *AzureManagedControlPlane) NodeSubnet() *infrav1.SubnetSpec {
	return &infrav1.SubnetSpec{
		Role: infrav1.SubnetNode,
		Name: "aks-subnet",
	}
}

// RouteTable returns the cluster node routetable.
func (c *AzureManagedControlPlane) RouteTable() *infrav1.RouteTable {
	return nil
}

func init() {
	SchemeBuilder.Register(&AzureManagedControlPlane{}, &AzureManagedControlPlaneList{})
}
