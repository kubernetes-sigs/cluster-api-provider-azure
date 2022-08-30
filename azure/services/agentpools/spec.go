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
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

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
func (s *AgentPoolSpec) Parameters(existing interface{}) (params interface{}, err error) {
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
			},
		}

		// When autoscaling is set, the count of the nodes differ based on the autoscaler and should not depend on the
		// count present in MachinePool or AzureManagedMachinePool, hence we should not make an update API call based
		// on difference in count.
		if to.Bool(s.EnableAutoScaling) {
			normalizedProfile.Count = existingProfile.Count
		}

		// Compute a diff to check if we require an update
		diff := cmp.Diff(normalizedProfile, existingProfile)
		if diff == "" {
			// agent pool is up to date, nothing to do
			return nil, nil
		}
	}

	var availabilityZones *[]string
	if len(s.AvailabilityZones) > 0 {
		availabilityZones = &s.AvailabilityZones
	}
	var replicas *int32
	if s.Replicas > 0 {
		replicas = &s.Replicas
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

	return containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:   availabilityZones,
			Count:               replicas,
			EnableAutoScaling:   s.EnableAutoScaling,
			EnableUltraSSD:      s.EnableUltraSSD,
			MaxCount:            s.MaxCount,
			MaxPods:             s.MaxPods,
			MinCount:            s.MinCount,
			Mode:                containerservice.AgentPoolMode(s.Mode),
			NodeLabels:          s.NodeLabels,
			NodeTaints:          nodeTaints,
			OrchestratorVersion: s.Version,
			OsDiskSizeGB:        &s.OSDiskSizeGB,
			OsDiskType:          containerservice.OSDiskType(to.String(s.OsDiskType)),
			OsType:              containerservice.OSType(to.String(s.OSType)),
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:              sku,
			VnetSubnetID:        vnetSubnetID,
		},
	}, nil
}
