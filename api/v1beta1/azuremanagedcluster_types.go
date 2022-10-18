/*
Copyright 2022 The Kubernetes Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ManagedClusterFinalizer allows Reconcile to clean up Azure resources associated with the AzureManagedControlPlane before
	// removing it from the apiserver.
	ManagedClusterFinalizer = "azuremanagedcontrolplane.infrastructure.cluster.x-k8s.io"

	// PrivateDNSZoneModeSystem represents mode System for azuremanagedcontrolplane.
	PrivateDNSZoneModeSystem string = "System"

	// PrivateDNSZoneModeNone represents mode None for azuremanagedcontrolplane.
	PrivateDNSZoneModeNone string = "None"
)

// AzureManagedClusterSpec defines the desired state of AzureManagedCluster.
type AzureManagedClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// Version defines the desired Kubernetes version.
	// +kubebuilder:validation:MinLength:=2
	Version string `json:"version"`

	// ResourceGroupName is the name of the Azure resource group for this AKS Cluster.
	ResourceGroupName string `json:"resourceGroupName"`

	// NodeResourceGroupName is the name of the resource group
	// containing cluster IaaS resources. Will be populated to default
	// in webhook.
	// +optional
	NodeResourceGroupName string `json:"nodeResourceGroupName,omitempty"`

	// VirtualNetwork describes the vnet for the AKS cluster. Will be created if it does not exist.
	// +optional
	VirtualNetwork AKSVirtualNetwork `json:"virtualNetwork,omitempty"`

	// SubscriptionID is the GUID of the Azure subscription to hold this cluster.
	// +optional
	SubscriptionID string `json:"subscriptionID,omitempty"`

	// Location is a string matching one of the canonical Azure region names. Examples: "westus2", "eastus".
	Location string `json:"location"`

	// AdditionalTags is an optional set of tags to add to Azure resources managed by the Azure provider, in addition to the
	// ones added by default.
	// +optional
	AdditionalTags Tags `json:"additionalTags,omitempty"`

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

	// DNSServiceIP is an IP address assigned to the Kubernetes DNS service.
	// It must be within the Kubernetes service address range specified in serviceCidr.
	// +optional
	DNSServiceIP *string `json:"dnsServiceIP,omitempty"`

	// LoadBalancerSKU is the SKU of the loadBalancer to be provisioned.
	// +kubebuilder:validation:Enum=Basic;Standard
	// +optional
	LoadBalancerSKU *string `json:"loadBalancerSKU,omitempty"`

	// IdentityRef is a reference to a AzureClusterIdentity to be used when reconciling this cluster
	// +optional
	IdentityRef *corev1.ObjectReference `json:"identityRef,omitempty"`

	// AadProfile is Azure Active Directory configuration to integrate with AKS for aad authentication.
	// +optional
	AADProfile *AADProfile `json:"aadProfile,omitempty"`

	// AddonProfiles are the profiles of managed cluster add-on.
	// +optional
	AddonProfiles []AddonProfile `json:"addonProfiles,omitempty"`

	// LoadBalancerProfile is the profile of the cluster load balancer.
	// +optional
	LoadBalancerProfile *LoadBalancerProfile `json:"loadBalancerProfile,omitempty"`
}

// AzureManagedClusterStatus defines the observed state of AzureManagedCluster.
type AzureManagedClusterStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`
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

// AddonProfile represents a managed cluster add-on.
type AddonProfile struct {
	// Name - The name of the managed cluster add-on.
	Name string `json:"name"`

	// Config - Key-value pairs for configuring the add-on.
	// +optional
	Config map[string]string `json:"config,omitempty"`

	// Enabled - Whether the add-on is enabled or not.
	Enabled bool `json:"enabled"`
}

// LoadBalancerProfile - Profile of the cluster load balancer.
type LoadBalancerProfile struct {
	// Load balancer profile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs.
	// By default the AKS cluster automatically creates a public IP in the AKS-managed infrastructure resource group and assigns it to the load balancer outbound pool.
	// Alternatively, you can assign your own custom public IP or public IP prefix at cluster creation time.
	// See https://docs.microsoft.com/en-us/azure/aks/load-balancer-standard#provide-your-own-outbound-public-ips-or-prefixes

	// ManagedOutboundIPs - Desired managed outbound IPs for the cluster load balancer.
	// +optional
	ManagedOutboundIPs *int32 `json:"managedOutboundIPs,omitempty"`

	// OutboundIPPrefixes - Desired outbound IP Prefix resources for the cluster load balancer.
	// +optional
	OutboundIPPrefixes []string `json:"outboundIPPrefixes,omitempty"`

	// OutboundIPs - Desired outbound IP resources for the cluster load balancer.
	// +optional
	OutboundIPs []string `json:"outboundIPs,omitempty"`

	// AllocatedOutboundPorts - Desired number of allocated SNAT ports per VM. Allowed values must be in the range of 0 to 64000 (inclusive). The default value is 0 which results in Azure dynamically allocating ports.
	// +optional
	AllocatedOutboundPorts *int32 `json:"allocatedOutboundPorts,omitempty"`

	// IdleTimeoutInMinutes - Desired outbound flow idle timeout in minutes. Allowed values must be in the range of 4 to 120 (inclusive). The default value is 30 minutes.
	// +optional
	IdleTimeoutInMinutes *int32 `json:"idleTimeoutInMinutes,omitempty"`
}

// AKSVirtualNetwork describes a virtual network required to provision AKS clusters.
type AKSVirtualNetwork struct {
	Name      string `json:"name"`
	CIDRBlock string `json:"cidrBlock"`
	// +optional
	Subnet AKSSubnet `json:"subnet,omitempty"`
	// ResourceGroup is the name of the Azure resource group for the VNet and Subnet.
	// +optional
	ResourceGroup string `json:"resourceGroup,omitempty"`
}

// AKSSubnet describes a subnet for an AKS cluster.
type AKSSubnet struct {
	Name      string `json:"name"`
	CIDRBlock string `json:"cidrBlock"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=azuremanagedclusters,scope=Namespaced,categories=cluster-api,shortName=amc
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// AzureManagedCluster is the Schema for the azuremanagedclusters API.
type AzureManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureManagedClusterSpec   `json:"spec,omitempty"`
	Status AzureManagedClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureManagedClusterList contains a list of AzureManagedClusters.
type AzureManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureManagedCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureManagedCluster{}, &AzureManagedClusterList{})
}
