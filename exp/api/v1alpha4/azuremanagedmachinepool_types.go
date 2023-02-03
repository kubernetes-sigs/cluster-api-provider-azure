/*
Copyright 2021 The Kubernetes Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capierrors "sigs.k8s.io/cluster-api/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

const (
	// LabelAgentPoolMode represents mode of an agent pool. Possible values include: System, User.
	LabelAgentPoolMode = "azuremanagedmachinepool.infrastructure.cluster.x-k8s.io/agentpoolmode"

	// NodePoolModeSystem represents mode system for azuremachinepool.
	NodePoolModeSystem NodePoolMode = "System"

	// NodePoolModeUser represents mode user for azuremachinepool.
	NodePoolModeUser NodePoolMode = "User"
)

// NodePoolMode enumerates the values for agent pool mode.
type NodePoolMode string

// AzureManagedMachinePoolSpec defines the desired state of AzureManagedMachinePool.
type AzureManagedMachinePoolSpec struct {

	// Name - name of the agent pool. If not specified, CAPZ uses the name of the CR as the agent pool name.
	// +optional
	Name *string `json:"name,omitempty"`

	// Mode - represents mode of an agent pool. Possible values include: System, User.
	// +kubebuilder:validation:Enum=System;User
	Mode string `json:"mode"`

	// SKU is the size of the VMs in the node pool.
	SKU string `json:"sku"`

	// OSDiskSizeGB is the disk size for every machine in this agent pool.
	// If you specify 0, it will apply the default osDisk size according to the vmSize specified.
	OSDiskSizeGB *int32 `json:"osDiskSizeGB,omitempty"`

	// MaxCount - Maximum number of nodes for auto-scaling
	// +optional
	MaxCount *int32 `json:"maxCount,omitempty"`

	// MinCount - Minimum number of nodes for auto-scaling
	// +optional
	MinCount *int32 `json:"minCount,omitempty"`

	// EnableAutoScaling - Whether to enable auto-scaler
	// +optional
	EnableAutoScaling *bool `json:"enableAutoScaling,omitempty"`

	// EnableNodePublicIP - Enable public IP for nodes
	// +optional
	EnableNodePublicIP *bool `json:"enableNodePublicIP,omitempty"`

	// EnableFIPS - Whether to use FIPS enabled OS
	// +optional
	EnableFIPS *bool `json:"enableFIPS,omitempty"`

	// OsDiskType - OS disk type to be used for machines in a given agent pool. Allowed values are 'Ephemeral' and 'Managed'. If unspecified, defaults to 'Ephemeral' when the VM supports ephemeral OS and has a cache disk larger than the requested OSDiskSizeGB. Otherwise, defaults to 'Managed'. May not be changed after creation. Possible values include: 'Managed', 'Ephemeral'
	// +kubebuilder:validation:Enum=Managed;Ephemeral
	// +optional
	OsDiskType *string `json:"osDiskType,omitempty"`

	// NodeLabels - Agent pool node labels to be persisted across all nodes in agent pool.
	// Disable conversion gen as the upstream version uses map[string]string
	// +k8s:conversion-gen=false
	// +optional
	NodeLabels map[string]*string `json:"nodeLabels,omitempty"`

	// NodeTaints - Taints added to new nodes during node pool create and scale. For example, key=value:NoSchedule.
	// +optional
	NodeTaints []string `json:"nodeTaints,omitempty"`

	// VnetSubnetID - VNet SubnetID specifies the VNet's subnet identifier for nodes and maybe pods
	// +optional
	VnetSubnetID *string `json:"vnetSubnetID,omitempty"`

	// AvailabilityZones - Availability zones for nodes. Must use VirtualMachineScaleSets AgentPoolType.
	// +optional
	AvailabilityZones []string `json:"availabilityZones,omitempty"`

	// ScaleSetPriority - ScaleSetPriority to be used to specify virtual machine scale set priority. Default to regular. Possible values include: 'Spot', 'Regular'
	// +kubebuilder:validation:Enum=Regular;Spot
	// +optional
	ScaleSetPriority *string `json:"scaleSetPriority,omitempty"`

	// MaxPods - Maximum number of pods that can run on a node.
	// +optional
	MaxPods *int32 `json:"maxPods,omitempty"`

	// KubeletConfig - KubeletConfig specifies the configuration of kubelet on agent nodes.
	// +optional
	KubeletConfig *KubeletConfig `json:"kubeletConfig,omitempty"`

	// ProviderIDList is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderIDList []string `json:"providerIDList,omitempty"`

	// AdditionalTags is an optional set of tags to add to an instance, in addition to the ones added by default by the
	// Azure provider. If both the AzureCluster and the AzureMachine specify the same tag name with different values, the
	// AzureMachine's value takes precedence.
	// +optional
	AdditionalTags infrav1.Tags `json:"additionalTags,omitempty"`
}

// KubeletConfig kubelet configurations of agent nodes.
type KubeletConfig struct {
	// CPUManagerPolicy - CPU Manager policy to use.
	// +kubebuilder:validation:Enum=none;static
	// +optional
	CPUManagerPolicy *string `json:"cpuManagerPolicy,omitempty"`

	// CPUCfsQuota - Enable CPU CFS quota enforcement for containers that specify CPU limits.
	// +optional
	CPUCfsQuota *bool `json:"cpuCfsQuota,omitempty"`

	// CPUCfsQuotaPeriod - Sets CPU CFS quota period value.
	// +optional
	CPUCfsQuotaPeriod *string `json:"cpuCfsQuotaPeriod,omitempty"`

	// ImageGcHighThreshold - The percent of disk usage after which image garbage collection is always run.
	// +optional
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	ImageGcHighThreshold *int32 `json:"imageGcHighThreshold,omitempty"`

	// ImageGcLowThreshold - The percent of disk usage before which image garbage collection is never run.
	// +optional
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	ImageGcLowThreshold *int32 `json:"imageGcLowThreshold,omitempty"`

	// TopologyManagerPolicy - Topology Manager policy to use.
	// +kubebuilder:validation:Enum=none;best-effort;restricted;single-numa-node
	// +optional
	TopologyManagerPolicy *string `json:"topologyManagerPolicy,omitempty"`

	// AllowedUnsafeSysctls - Allowlist of unsafe sysctls or unsafe sysctl patterns (ending in `*`).
	// +optional
	AllowedUnsafeSysctls *[]string `json:"allowedUnsafeSysctls,omitempty"`

	// FailSwapOn - If set to true it will make the Kubelet fail to start if swap is enabled on the node.
	// +optional
	FailSwapOn *bool `json:"failSwapOn,omitempty"`

	// ContainerLogMaxSizeMB - The maximum size (e.g. 10Mi) of container log file before it is rotated.
	// +optional
	ContainerLogMaxSizeMB *int32 `json:"containerLogMaxSizeMB,omitempty"`

	// ContainerLogMaxFiles - The maximum number of container log files that can be present for a container. The number must be ≥ 2.
	// +optional
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty"`

	// PodMaxPids - The maximum number of processes per pod.
	// +optional
	PodMaxPids *int32 `json:"podMaxPids,omitempty"`
}

// AzureManagedMachinePoolStatus defines the observed state of AzureManagedMachinePool.
type AzureManagedMachinePoolStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Replicas is the most recently observed number of replicas.
	// +optional
	Replicas int32 `json:"replicas"`

	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	ErrorReason *capierrors.MachineStatusError `json:"errorReason,omitempty"`

	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=azuremanagedmachinepools,scope=Namespaced,categories=cluster-api,shortName=ammp
// +kubebuilder:subresource:status

// AzureManagedMachinePool is the Schema for the azuremanagedmachinepools API.
type AzureManagedMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureManagedMachinePoolSpec   `json:"spec,omitempty"`
	Status AzureManagedMachinePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureManagedMachinePoolList contains a list of AzureManagedMachinePools.
type AzureManagedMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureManagedMachinePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureManagedMachinePool{}, &AzureManagedMachinePoolList{})
}
