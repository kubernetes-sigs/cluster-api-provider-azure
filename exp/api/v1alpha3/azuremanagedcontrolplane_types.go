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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

// AzureManagedControlPlaneSpec defines the desired state of AzureManagedControlPlane.
type AzureManagedControlPlaneSpec struct {
	// Version defines the desired Kubernetes version.
	// +kubebuilder:validation:MinLength:=2
	Version string `json:"version"`

	// ResourceGroupName is the name of the Azure resource group for this AKS Cluster.
	ResourceGroupName string `json:"resourceGroupName"`

	// NodeResourceGroupName is the name of the resource group
	// containining cluster IaaS resources. Will be populated to default
	// in webhook.
	NodeResourceGroupName string `json:"nodeResourceGroupName"`

	// VirtualNetwork describes the vnet for the AKS cluster. Will be created if it does not exist.
	VirtualNetwork ManagedControlPlaneVirtualNetwork `json:"virtualNetwork,omitempty"`

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
	// +optional
	SSHPublicKey *string `json:"sshPublicKey,omitempty"`

	// DNSServiceIP is an IP address assigned to the Kubernetes DNS service.
	// It must be within the Kubernetes service address range specified in serviceCidr.
	// +optional
	DNSServiceIP *string `json:"dnsServiceIP,omitempty"`

	// LoadBalancerSKU is the SKU of the loadBalancer to be provisioned.
	// +kubebuilder:validation:Enum=Basic;Standard
	// +optional
	LoadBalancerSKU *string `json:"loadBalancerSKU,omitempty"`

	// AadProfile is Azure Active Directory configuration to integrate with AKS for aad authentication.
	// +optional
	AADProfile *AADProfile `json:"aadProfile,omitempty"`

	// Sku is the SKU of the AKS to be provisioned.
	// +optional
	Sku *SKU `json:"sku,omitempty"`

	// LoadBalancerProfile is the profile of the cluster load balancer.
	// +optional
	LoadBalancerProfile *LoadBalancerProfile `json:"loadBalancerProfile,omitempty"`

	// APIServerAccessProfile is the access profile for AKS API server.
	// +optional
	APIServerAccessProfile *APIServerAccessProfile `json:"apiServerAccessProfile,omitempty"`
}

// AADProfile - AAD integration managed by AKS.
type AADProfile struct {
	// Managed - Whether to enable managed AAD.
	// +kubebuilder:validation:Required
	Managed bool `json:"managed"`

	// AdminGroupObjectIDs - AAD group object IDs that will have admin role of the cluster.
	// +kubebuilder:validation:Required
	AdminGroupObjectIDs []string `json:"adminGroupObjectIDs"`
}

// SKU - AKS SKU.
type SKU struct {
	// Tier - Tier of a managed cluster SKU.
	// +kubebuilder:validation:Enum=Free;Paid
	Tier string `json:"tier"`
}

// LoadBalancerProfile - Profile of the cluster load balancer.
type LoadBalancerProfile struct {
	// ManagedOutboundIPs - Desired managed outbound IPs for the cluster load balancer.
	// +optional
	ManagedOutboundIPs *int32 `json:"managedOutboundIPs,omitempty"`

	// OutboundIPPrefixes - Desired outbound IP Prefix resources for the cluster load balancer.
	// +optional
	OutboundIPPrefixes []string `json:"outboundIPPrefixes,omitempty"`

	// OutboundIPs - Desired outbound IP resources for the cluster load balancer.
	// +optional
	OutboundIPs []string `json:"outboundIPs,omitempty"`

	// EffectiveOutboundIPs - The effective outbound IP resources of the cluster load balancer.
	// +optional
	EffectiveOutboundIPs []string `json:"effectiveOutboundIPs,omitempty"`

	// AllocatedOutboundPorts - Desired number of allocated SNAT ports per VM. Allowed values must be in the range of 0 to 64000 (inclusive). The default value is 0 which results in Azure dynamically allocating ports.
	// +optional
	AllocatedOutboundPorts *int32 `json:"allocatedOutboundPorts,omitempty"`

	// IdleTimeoutInMinutes - Desired outbound flow idle timeout in minutes. Allowed values must be in the range of 4 to 120 (inclusive). The default value is 30 minutes.
	// +optional
	IdleTimeoutInMinutes *int32 `json:"idleTimeoutInMinutes,omitempty"`
}

// APIServerAccessProfile - access profile for AKS API server.
type APIServerAccessProfile struct {
	// AuthorizedIPRanges - Authorized IP Ranges to kubernetes API server.
	// +optional
	AuthorizedIPRanges []string `json:"authorizedIPRanges,omitempty"`
	// EnablePrivateCluster - Whether to create the cluster as a private cluster or not.
	// +optional
	EnablePrivateCluster *bool `json:"enablePrivateCluster,omitempty"`
	// PrivateDNSZone - Private dns zone mode for private cluster.
	// +kubebuilder:validation:Enum=System;None
	// +optional
	PrivateDNSZone *string `json:"privateDNSZone,omitempty"`
	// EnablePrivateClusterPublicFQDN - Whether to create additional public FQDN for private cluster or not.
	// +optional
	EnablePrivateClusterPublicFQDN *bool `json:"enablePrivateClusterPublicFQDN,omitempty"`
}

// ManagedControlPlaneVirtualNetwork describes a virtual network required to provision AKS clusters.
type ManagedControlPlaneVirtualNetwork struct {
	Name       string                      `json:"name"`
	CIDRBlocks []string                    `json:"cidrBlocks"`
	Subnets    []ManagedControlPlaneSubnet `json:"subnets,omitempty"`
}

// ManagedControlPlaneSubnet describes a subnet for an AKS cluster.
type ManagedControlPlaneSubnet struct {
	Name       string   `json:"name"`
	CIDRBlocks []string `json:"cidrBlocks"`
}

// AzureManagedControlPlaneStatus defines the observed state of AzureManagedControlPlane.
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
// +kubebuilder:subresource:status

// AzureManagedControlPlane is the Schema for the azuremanagedcontrolplanes API.
type AzureManagedControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureManagedControlPlaneSpec   `json:"spec,omitempty"`
	Status AzureManagedControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureManagedControlPlaneList contains a list of AzureManagedControlPlanes.
type AzureManagedControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureManagedControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureManagedControlPlane{}, &AzureManagedControlPlaneList{})
}
