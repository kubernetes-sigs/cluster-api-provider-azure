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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/net"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// AzureManagedControlPlaneTemplateResourceSpec specifies an Azure managed control plane template resource.
type AzureManagedControlPlaneTemplateResourceSpec struct {
	// MachineTemplate contains information about how machines
	// should be shaped when creating or updating a control plane.
	// +optional
	MachineTemplate *AzureManagedControlPlaneTemplateMachineTemplate `json:"machineTemplate,omitempty"`

	// Version defines the desired Kubernetes version.
	// +kubebuilder:validation:MinLength:=2
	Version string `json:"version"`

	// VirtualNetwork describes the vnet for the AKS cluster. Will be created if it does not exist.
	// +optional
	VirtualNetwork ManagedControlPlaneVirtualNetworkTemplate `json:"virtualNetwork,omitempty"`

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

	// Outbound configuration used by Nodes.
	// +kubebuilder:validation:Enum=loadBalancer;managedNATGateway;userAssignedNATGateway;userDefinedRouting
	// +optional
	OutboundType *ManagedControlPlaneOutboundType `json:"outboundType,omitempty"`

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

	// SKU is the SKU of the AKS to be provisioned.
	// +optional
	SKU *AKSSku `json:"sku,omitempty"`

	// LoadBalancerProfile is the profile of the cluster load balancer.
	// +optional
	LoadBalancerProfile *LoadBalancerProfile `json:"loadBalancerProfile,omitempty"`

	// APIServerAccessProfile is the access profile for AKS API server.
	// +optional
	APIServerAccessProfile *APIServerAccessProfileTemplate `json:"apiServerAccessProfile,omitempty"`

	// AutoscalerProfile is the parameters to be applied to the cluster-autoscaler when enabled
	// +optional
	AutoScalerProfile *AutoScalerProfile `json:"autoscalerProfile,omitempty"`
}

// AzureManagedControlPlaneTemplateMachineTemplate specifies an Azure managed control plane template.
type AzureManagedControlPlaneTemplateMachineTemplate struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`

	// NodeDrainTimeout is the total amount of time that the controller will spend on draining a controlplane node
	// The default value is 0, meaning that the node can be drained without any time limitations.
	// NOTE: NodeDrainTimeout is different from `kubectl drain --timeout`
	// +optional
	NodeDrainTimeout *metav1.Duration `json:"nodeDrainTimeout,omitempty"`

	// NodeVolumeDetachTimeout is the total amount of time that the controller will spend on waiting for all volumes
	// to be detached. The default value is 0, meaning that the volumes can be detached without any time limitations.
	// +optional
	NodeVolumeDetachTimeout *metav1.Duration `json:"nodeVolumeDetachTimeout,omitempty"`

	// NodeDeletionTimeout defines how long the machine controller will attempt to delete the Node that the Machine
	// hosts after the Machine is marked for deletion. A duration of 0 will retry deletion indefinitely.
	// If no value is provided, the default value for this property of the Machine resource will be used.
	// +optional
	NodeDeletionTimeout *metav1.Duration `json:"nodeDeletionTimeout,omitempty"`
}

// AzureManagedMachinePoolTemplateResourceSpec specifies an Azure managed control plane template resource.
type AzureManagedMachinePoolTemplateResourceSpec struct {
	// AdditionalTags is an optional set of tags to add to Azure resources managed by the
	// Azure provider, in addition to the ones added by default.
	// +optional
	AdditionalTags Tags `json:"additionalTags,omitempty"`

	// Name - name of the agent pool. If not specified, CAPZ uses the name of the CR as the agent pool name.
	// Immutable.
	// +optional
	Name *string `json:"name,omitempty"`

	// Mode - represents mode of an agent pool. Possible values include: System, User.
	// +kubebuilder:validation:Enum=System;User
	Mode string `json:"mode"`

	// SKU is the size of the VMs in the node pool.
	// Immutable.
	SKU string `json:"sku"`

	// OSDiskSizeGB is the disk size for every machine in this agent pool.
	// If you specify 0, it will apply the default osDisk size according to the vmSize specified.
	// Immutable.
	// +optional
	OSDiskSizeGB *int32 `json:"osDiskSizeGB,omitempty"`

	// AvailabilityZones - Availability zones for nodes. Must use VirtualMachineScaleSets AgentPoolType.
	// Immutable.
	// +optional
	AvailabilityZones []string `json:"availabilityZones,omitempty"`

	// Node labels - labels for all of the nodes present in node pool.
	// See also [AKS doc].
	//
	// [AKS doc]: https://learn.microsoft.com/azure/aks/use-labels
	// +optional
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`

	// Taints specifies the taints for nodes present in this agent pool.
	// See also [AKS doc].
	//
	// [AKS doc]: https://learn.microsoft.com/azure/aks/use-multiple-node-pools#setting-node-pool-taints
	// +optional
	Taints Taints `json:"taints,omitempty"`

	// Scaling specifies the autoscaling parameters for the node pool.
	// +optional
	Scaling *ManagedMachinePoolScaling `json:"scaling,omitempty"`

	// MaxPods specifies the kubelet `--max-pods` configuration for the node pool.
	// Immutable.
	// See also [AKS doc], [K8s doc].
	//
	// [AKS doc]: https://learn.microsoft.com/azure/aks/configure-azure-cni#configure-maximum---new-clusters
	// [K8s doc]: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/
	// +optional
	MaxPods *int32 `json:"maxPods,omitempty"`

	// OsDiskType specifies the OS disk type for each node in the pool. Allowed values are 'Ephemeral' and 'Managed' (default).
	// Immutable.
	// See also [AKS doc].
	//
	// [AKS doc]: https://learn.microsoft.com/azure/aks/cluster-configuration#ephemeral-os
	// +kubebuilder:validation:Enum=Ephemeral;Managed
	// +kubebuilder:default=Managed
	// +optional
	OsDiskType *string `json:"osDiskType,omitempty"`

	// EnableUltraSSD enables the storage type UltraSSD_LRS for the agent pool.
	// Immutable.
	// +optional
	EnableUltraSSD *bool `json:"enableUltraSSD,omitempty"`

	// OSType specifies the virtual machine operating system. Default to Linux. Possible values include: 'Linux', 'Windows'.
	// 'Windows' requires the AzureManagedControlPlane's `spec.networkPlugin` to be `azure`.
	// Immutable.
	// See also [AKS doc].
	//
	// [AKS doc]: https://learn.microsoft.com/rest/api/aks/agent-pools/create-or-update?tabs=HTTP#ostype
	// +kubebuilder:validation:Enum=Linux;Windows
	// +optional
	OSType *string `json:"osType,omitempty"`

	// EnableNodePublicIP controls whether or not nodes in the pool each have a public IP address.
	// Immutable.
	// +optional
	EnableNodePublicIP *bool `json:"enableNodePublicIP,omitempty"`

	// NodePublicIPPrefixID specifies the public IP prefix resource ID which VM nodes should use IPs from.
	// Immutable.
	// +optional
	NodePublicIPPrefixID *string `json:"nodePublicIPPrefixID,omitempty"`

	// ScaleSetPriority specifies the ScaleSetPriority value. Default to Regular. Possible values include: 'Regular', 'Spot'
	// Immutable.
	// +kubebuilder:validation:Enum=Regular;Spot
	// +optional
	ScaleSetPriority *string `json:"scaleSetPriority,omitempty"`

	// ScaleDownMode affects the cluster autoscaler behavior. Default to Delete. Possible values include: 'Deallocate', 'Delete'
	// +kubebuilder:validation:Enum=Deallocate;Delete
	// +kubebuilder:default=Delete
	// +optional
	ScaleDownMode *string `json:"scaleDownMode,omitempty"`

	// SpotMaxPrice defines max price to pay for spot instance. Possible values are any decimal value greater than zero or -1.
	// If you set the max price to be -1, the VM won't be evicted based on price. The price for the VM will be the current price
	// for spot or the price for a standard VM, which ever is less, as long as there's capacity and quota available.
	// +optional
	SpotMaxPrice *resource.Quantity `json:"spotMaxPrice,omitempty"`

	// KubeletConfig specifies the kubelet configurations for nodes.
	// Immutable.
	// +optional
	KubeletConfig *KubeletConfig `json:"kubeletConfig,omitempty"`

	// KubeletDiskType specifies the kubelet disk type. Default to OS. Possible values include: 'OS', 'Temporary'.
	// Requires Microsoft.ContainerService/KubeletDisk preview feature to be set.
	// Immutable.
	// See also [AKS doc].
	//
	// [AKS doc]: https://learn.microsoft.com/rest/api/aks/agent-pools/create-or-update?tabs=HTTP#kubeletdisktype
	// +kubebuilder:validation:Enum=OS;Temporary
	// +optional
	KubeletDiskType *KubeletDiskType `json:"kubeletDiskType,omitempty"`

	// LinuxOSConfig specifies the custom Linux OS settings and configurations.
	// Immutable.
	// +optional
	LinuxOSConfig *LinuxOSConfig `json:"linuxOSConfig,omitempty"`

	// SubnetName specifies the Subnet where the MachinePool will be placed
	// Immutable.
	// +optional
	SubnetName *string `json:"subnetName,omitempty"`

	// EnableFIPS indicates whether FIPS is enabled on the node pool.
	// Immutable.
	// +optional
	EnableFIPS *bool `json:"enableFIPS,omitempty"`
}

// APIServerAccessProfileTemplate specifies an API server access profile template.
type APIServerAccessProfileTemplate struct {
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

// ManagedControlPlaneVirtualNetworkTemplate specifies a managed control plane virtual network template.
type ManagedControlPlaneVirtualNetworkTemplate struct {
	Name      string `json:"name"`
	CIDRBlock string `json:"cidrBlock"`
	// +optional
	Subnet ManagedControlPlaneSubnet `json:"subnet,omitempty"`
}

// AzureManagedClusterTemplateResourceSpec specifies an Azure managed cluster template resource.
type AzureManagedClusterTemplateResourceSpec struct{}

// AzureClusterTemplateResourceSpec specifies an Azure cluster template resource.
type AzureClusterTemplateResourceSpec struct {
	AzureClusterClassSpec `json:",inline"`

	// NetworkSpec encapsulates all things related to Azure network.
	// +optional
	NetworkSpec NetworkTemplateSpec `json:"networkSpec,omitempty"`

	// BastionSpec encapsulates all things related to the Bastions in the cluster.
	// +optional
	BastionSpec BastionTemplateSpec `json:"bastionSpec,omitempty"`
}

// NetworkTemplateSpec specifies a network template.
type NetworkTemplateSpec struct {
	NetworkClassSpec `json:",inline"`

	// Vnet is the configuration for the Azure virtual network.
	// +optional
	Vnet VnetTemplateSpec `json:"vnet,omitempty"`

	// Subnets is the configuration for the control-plane subnet and the node subnet.
	// +optional
	Subnets SubnetTemplatesSpec `json:"subnets,omitempty"`

	// APIServerLB is the configuration for the control-plane load balancer.
	// +optional
	APIServerLB LoadBalancerClassSpec `json:"apiServerLB,omitempty"`

	// NodeOutboundLB is the configuration for the node outbound load balancer.
	// +optional
	NodeOutboundLB *LoadBalancerClassSpec `json:"nodeOutboundLB,omitempty"`

	// ControlPlaneOutboundLB is the configuration for the control-plane outbound load balancer.
	// This is different from APIServerLB, and is used only in private clusters (optionally) for enabling outbound traffic.
	// +optional
	ControlPlaneOutboundLB *LoadBalancerClassSpec `json:"controlPlaneOutboundLB,omitempty"`
}

// GetControlPlaneSubnetTemplate returns the cluster control plane subnet template.
func (n *NetworkTemplateSpec) GetControlPlaneSubnetTemplate() (SubnetTemplateSpec, error) {
	for _, sn := range n.Subnets {
		if sn.Role == SubnetControlPlane {
			return sn, nil
		}
	}
	return SubnetTemplateSpec{}, errors.Errorf("no subnet template found with role %s", SubnetControlPlane)
}

// UpdateControlPlaneSubnetTemplate updates the cluster control plane subnet template.
func (n *NetworkTemplateSpec) UpdateControlPlaneSubnetTemplate(subnet SubnetTemplateSpec) {
	for i, sn := range n.Subnets {
		if sn.Role == SubnetControlPlane {
			n.Subnets[i] = subnet
		}
	}
}

// VnetTemplateSpec defines the desired state of a virtual network.
type VnetTemplateSpec struct {
	VnetClassSpec `json:",inline"`

	// Peerings defines a list of peerings of the newly created virtual network with existing virtual networks.
	// +optional
	Peerings VnetPeeringsTemplateSpec `json:"peerings,omitempty"`
}

// VnetPeeringsTemplateSpec defines a list of peerings of the newly created virtual network with existing virtual networks.
type VnetPeeringsTemplateSpec []VnetPeeringClassSpec

// SubnetTemplateSpec specifies a template for a subnet.
type SubnetTemplateSpec struct {
	SubnetClassSpec `json:",inline"`

	// SecurityGroup defines the NSG (network security group) that should be attached to this subnet.
	// +optional
	SecurityGroup SecurityGroupClass `json:"securityGroup,omitempty"`

	// NatGateway associated with this subnet.
	// +optional
	NatGateway NatGatewayClassSpec `json:"natGateway,omitempty"`
}

// IsNatGatewayEnabled returns true if the NAT gateway is enabled.
func (s SubnetTemplateSpec) IsNatGatewayEnabled() bool {
	return s.NatGateway.Name != ""
}

// IsIPv6Enabled returns whether or not IPv6 is enabled on the subnet.
func (s SubnetTemplateSpec) IsIPv6Enabled() bool {
	for _, cidr := range s.CIDRBlocks {
		if net.IsIPv6CIDRString(cidr) {
			return true
		}
	}
	return false
}

// SubnetTemplatesSpec specifies a list of subnet templates.
// +listType=map
// +listMapKey=name
type SubnetTemplatesSpec []SubnetTemplateSpec

// BastionTemplateSpec specifies a template for a bastion host.
type BastionTemplateSpec struct {
	// +optional
	AzureBastion *AzureBastionTemplateSpec `json:"azureBastion,omitempty"`
}

// AzureBastionTemplateSpec specifies a template for an Azure Bastion host.
type AzureBastionTemplateSpec struct {
	// +optional
	Subnet SubnetTemplateSpec `json:"subnet,omitempty"`
}
