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

package agentpools

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2022-03-01/containerservice"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
)

// KubeletConfig defines the set of kubelet configurations for nodes in pools.
type KubeletConfig struct {
	// CPUManagerPolicy - CPU Manager policy to use.
	CPUManagerPolicy *string
	// CPUCfsQuota - Enable CPU CFS quota enforcement for containers that specify CPU limits.
	CPUCfsQuota *bool
	// CPUCfsQuotaPeriod - Sets CPU CFS quota period value.
	CPUCfsQuotaPeriod *string
	// ImageGcHighThreshold - The percent of disk usage after which image garbage collection is always run.
	ImageGcHighThreshold *int32
	// ImageGcLowThreshold - The percent of disk usage before which image garbage collection is never run.
	ImageGcLowThreshold *int32
	// TopologyManagerPolicy - Topology Manager policy to use.
	TopologyManagerPolicy *string
	// AllowedUnsafeSysctls - Allowlist of unsafe sysctls or unsafe sysctl patterns (ending in `*`).
	AllowedUnsafeSysctls *[]string
	// FailSwapOn - If set to true it will make the Kubelet fail to start if swap is enabled on the node.
	FailSwapOn *bool
	// ContainerLogMaxSizeMB - The maximum size (e.g. 10Mi) of container log file before it is rotated.
	ContainerLogMaxSizeMB *int32
	// ContainerLogMaxFiles - The maximum number of container log files that can be present for a container. The number must be ≥ 2.
	ContainerLogMaxFiles *int32
	// PodMaxPids - The maximum number of processes per pod.
	PodMaxPids *int32
}

// AgentPoolSpec contains agent pool specification details.
type AgentPoolSpec struct {
	// Name is the name of agent pool.
	Name string

	// ResourceGroup is the name of the Azure resource group for the AKS Cluster.
	ResourceGroup string

	// Cluster is the name of the AKS cluster.
	Cluster string

	// Version defines the desired Kubernetes version.
	Version *string

	// SKU defines the Azure VM size for the agent pool VMs.
	SKU string

	// Replicas is the number of desired machines.
	Replicas int32

	// OSDiskSizeGB is the OS disk size in GB for every machine in this agent pool.
	OSDiskSizeGB int32

	// VnetSubnetID is the Azure Resource ID for the subnet which should contain nodes.
	VnetSubnetID string

	// Mode represents mode of an agent pool. Possible values include: 'System', 'User'.
	Mode string

	//  Maximum number of nodes for auto-scaling
	MaxCount *int32 `json:"maxCount,omitempty"`

	// Minimum number of nodes for auto-scaling
	MinCount *int32 `json:"minCount,omitempty"`

	// Node labels - labels for all of the nodes present in node pool
	NodeLabels map[string]*string `json:"nodeLabels,omitempty"`

	// NodeTaints specifies the taints for nodes present in this agent pool.
	NodeTaints []string `json:"nodeTaints,omitempty"`

	// EnableAutoScaling - Whether to enable auto-scaler
	EnableAutoScaling *bool `json:"enableAutoScaling,omitempty"`

	// AvailabilityZones represents the Availability zones for nodes in the AgentPool.
	AvailabilityZones []string

	// MaxPods specifies the kubelet --max-pods configuration for the agent pool.
	MaxPods *int32 `json:"maxPods,omitempty"`

	// OsDiskType specifies the OS disk type for each node in the pool. Allowed values are 'Ephemeral' and 'Managed'.
	OsDiskType *string `json:"osDiskType,omitempty"`

	// EnableUltraSSD enables the storage type UltraSSD_LRS for the agent pool.
	EnableUltraSSD *bool `json:"enableUltraSSD,omitempty"`

	// OSType specifies the operating system for the node pool. Allowed values are 'Linux' and 'Windows'
	OSType *string `json:"osType,omitempty"`

	// Headers is the list of headers to add to the HTTP requests to update this resource.
	Headers map[string]string

	// EnableNodePublicIP controls whether or not nodes in the agent pool each have a public IP address.
	EnableNodePublicIP *bool `json:"enableNodePublicIP,omitempty"`

	// NodePublicIPPrefixID specifies the public IP prefix resource ID which VM nodes should use IPs from.
	NodePublicIPPrefixID *string `json:"nodePublicIPPrefixID,omitempty"`

	// ScaleSetPriority specifies the ScaleSetPriority for the node pool. Allowed values are 'Spot' and 'Regular'
	ScaleSetPriority *string `json:"scaleSetPriority,omitempty"`

	// KubeletConfig specifies the kubelet configurations for nodes.
	KubeletConfig *KubeletConfig `json:"kubeletConfig,omitempty"`

	// KubeletDiskType specifies the kubelet disk type for each node in the pool. Allowed values are 'OS' and 'Temporary'
	KubeletDiskType *infrav1.KubeletDiskType `json:"kubeletDiskType,omitempty"`

	// AdditionalTags is an optional set of tags to add to Azure resources managed by the Azure provider, in addition to the ones added by default.
	AdditionalTags infrav1.Tags
}

// ResourceName returns the name of the agent pool.
func (s *AgentPoolSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *AgentPoolSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for agent pools.
func (s *AgentPoolSpec) OwnerResourceName() string {
	return s.Cluster
}

// CustomHeaders returns custom headers to be added to the Azure API calls.
func (s *AgentPoolSpec) CustomHeaders() map[string]string {
	return s.Headers
}

// Parameters returns the parameters for the agent pool.
func (s *AgentPoolSpec) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	nodeLabels := s.NodeLabels
	if existing != nil {
		existingPool, ok := existing.(containerservice.AgentPool)
		if !ok {
			return nil, errors.Errorf("%T is not a containerservice.AgentPool", existing)
		}

		// agent pool already exists
		ps := *existingPool.ManagedClusterAgentPoolProfileProperties.ProvisioningState
		if ps != string(infrav1.Canceled) && ps != string(infrav1.Failed) && ps != string(infrav1.Succeeded) {
			msg := fmt.Sprintf("Unable to update existing agent pool in non terminal state. Agent pool must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: %s", ps)
			return nil, azure.WithTransientError(errors.New(msg), 20*time.Second)
		}

		// Normalize individual agent pools to diff in case we need to update
		existingProfile := containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				Count:               existingPool.Count,
				OrchestratorVersion: existingPool.OrchestratorVersion,
				Mode:                existingPool.Mode,
				EnableAutoScaling:   existingPool.EnableAutoScaling,
				MinCount:            existingPool.MinCount,
				MaxCount:            existingPool.MaxCount,
				NodeLabels:          existingPool.NodeLabels,
				NodeTaints:          existingPool.NodeTaints,
				Tags:                existingPool.Tags,
				KubeletConfig:       existingPool.KubeletConfig,
			},
		}

		normalizedProfile := containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				Count:               &s.Replicas,
				OrchestratorVersion: s.Version,
				Mode:                containerservice.AgentPoolMode(s.Mode),
				EnableAutoScaling:   s.EnableAutoScaling,
				MinCount:            s.MinCount,
				MaxCount:            s.MaxCount,
				NodeLabels:          s.NodeLabels,
				NodeTaints:          &s.NodeTaints,
				Tags:                converters.TagsToMap(s.AdditionalTags),
			},
		}
		if len(*normalizedProfile.NodeTaints) == 0 {
			normalizedProfile.NodeTaints = nil
		}

		if s.KubeletConfig != nil {
			normalizedProfile.KubeletConfig = &containerservice.KubeletConfig{
				CPUManagerPolicy:      s.KubeletConfig.CPUManagerPolicy,
				CPUCfsQuota:           s.KubeletConfig.CPUCfsQuota,
				CPUCfsQuotaPeriod:     s.KubeletConfig.CPUCfsQuotaPeriod,
				ImageGcHighThreshold:  s.KubeletConfig.ImageGcHighThreshold,
				ImageGcLowThreshold:   s.KubeletConfig.ImageGcLowThreshold,
				TopologyManagerPolicy: s.KubeletConfig.TopologyManagerPolicy,
				FailSwapOn:            s.KubeletConfig.FailSwapOn,
				ContainerLogMaxSizeMB: s.KubeletConfig.ContainerLogMaxSizeMB,
				ContainerLogMaxFiles:  s.KubeletConfig.ContainerLogMaxFiles,
				PodMaxPids:            s.KubeletConfig.PodMaxPids,
				AllowedUnsafeSysctls:  s.KubeletConfig.AllowedUnsafeSysctls,
			}
		}

		// When autoscaling is set, the count of the nodes differ based on the autoscaler and should not depend on the
		// count present in MachinePool or AzureManagedMachinePool, hence we should not make an update API call based
		// on difference in count.
		if pointer.BoolDeref(s.EnableAutoScaling, false) {
			normalizedProfile.Count = existingProfile.Count
		}

		// Compute a diff to check if we require an update
		diff := cmp.Diff(normalizedProfile, existingProfile)
		if diff == "" {
			// agent pool is up to date, nothing to do
			return nil, nil
		}
		// We do a just-in-time merge of existent kubernetes.azure.com-prefixed labels
		// So that we don't unintentionally delete them
		// See https://github.com/Azure/AKS/issues/3152
		if normalizedProfile.NodeLabels != nil {
			nodeLabels = mergeSystemNodeLabels(normalizedProfile.NodeLabels, existingPool.NodeLabels)
		}
	}

	var availabilityZones *[]string
	if len(s.AvailabilityZones) > 0 {
		availabilityZones = &s.AvailabilityZones
	}
	var nodeTaints *[]string
	if len(s.NodeTaints) > 0 {
		nodeTaints = &s.NodeTaints
	}
	var sku *string
	if s.SKU != "" {
		sku = &s.SKU
	}
	var vnetSubnetID *string
	if s.VnetSubnetID != "" {
		vnetSubnetID = &s.VnetSubnetID
	}

	var kubeletConfig *containerservice.KubeletConfig
	if s.KubeletConfig != nil {
		kubeletConfig = &containerservice.KubeletConfig{
			CPUManagerPolicy:      s.KubeletConfig.CPUManagerPolicy,
			CPUCfsQuota:           s.KubeletConfig.CPUCfsQuota,
			CPUCfsQuotaPeriod:     s.KubeletConfig.CPUCfsQuotaPeriod,
			ImageGcHighThreshold:  s.KubeletConfig.ImageGcHighThreshold,
			ImageGcLowThreshold:   s.KubeletConfig.ImageGcLowThreshold,
			TopologyManagerPolicy: s.KubeletConfig.TopologyManagerPolicy,
			FailSwapOn:            s.KubeletConfig.FailSwapOn,
			ContainerLogMaxSizeMB: s.KubeletConfig.ContainerLogMaxSizeMB,
			ContainerLogMaxFiles:  s.KubeletConfig.ContainerLogMaxFiles,
			PodMaxPids:            s.KubeletConfig.PodMaxPids,
			AllowedUnsafeSysctls:  s.KubeletConfig.AllowedUnsafeSysctls,
		}
	}

	agentPool := containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:    availabilityZones,
			Count:                &s.Replicas,
			EnableAutoScaling:    s.EnableAutoScaling,
			EnableUltraSSD:       s.EnableUltraSSD,
			KubeletConfig:        kubeletConfig,
			KubeletDiskType:      containerservice.KubeletDiskType(pointer.StringDeref((*string)(s.KubeletDiskType), "")),
			MaxCount:             s.MaxCount,
			MaxPods:              s.MaxPods,
			MinCount:             s.MinCount,
			Mode:                 containerservice.AgentPoolMode(s.Mode),
			NodeLabels:           nodeLabels,
			NodeTaints:           nodeTaints,
			OrchestratorVersion:  s.Version,
			OsDiskSizeGB:         &s.OSDiskSizeGB,
			OsDiskType:           containerservice.OSDiskType(pointer.StringDeref(s.OsDiskType, "")),
			OsType:               containerservice.OSType(pointer.StringDeref(s.OSType, "")),
			ScaleSetPriority:     containerservice.ScaleSetPriority(pointer.StringDeref(s.ScaleSetPriority, "")),
			Type:                 containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:               sku,
			VnetSubnetID:         vnetSubnetID,
			EnableNodePublicIP:   s.EnableNodePublicIP,
			NodePublicIPPrefixID: s.NodePublicIPPrefixID,
			Tags:                 *azure.StringMapPtr(s.AdditionalTags),
		},
	}

	return agentPool, nil
}

// mergeSystemNodeLabels appends any kubernetes.azure.com-prefixed labels from the AKS label set
// into the local capz label set.
func mergeSystemNodeLabels(capz, aks map[string]*string) map[string]*string {
	ret := capz
	// Look for labels returned from the AKS node pool API that begin with kubernetes.azure.com
	for aksNodeLabelKey := range aks {
		if azureutil.IsAzureSystemNodeLabelKey(aksNodeLabelKey) {
			ret[aksNodeLabelKey] = aks[aksNodeLabelKey]
		}
	}
	return ret
}
