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

package converters

import (
	"github.com/Azure/azure-sdk-for-go/services/preview/containerservice/mgmt/2022-03-02-preview/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

// AgentPoolToManagedClusterAgentPoolProfile converts a AgentPoolSpec to an Azure SDK ManagedClusterAgentPoolProfile used in managedcluster reconcile.
func AgentPoolToManagedClusterAgentPoolProfile(pool azure.AgentPoolSpec) containerservice.ManagedClusterAgentPoolProfile {
	profile := containerservice.ManagedClusterAgentPoolProfile{
		Name:                &pool.Name,
		VMSize:              &pool.SKU,
		OsType:              containerservice.OSTypeLinux,
		OsDiskSizeGB:        &pool.OSDiskSizeGB,
		Count:               &pool.Replicas,
		Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
		OrchestratorVersion: pool.Version,
		VnetSubnetID:        &pool.VnetSubnetID,
		Mode:                containerservice.AgentPoolMode(pool.Mode),
		EnableAutoScaling:   pool.EnableAutoScaling,
		MaxCount:            pool.MaxCount,
		MinCount:            pool.MinCount,
		NodeTaints:          &pool.NodeTaints,
		AvailabilityZones:   &pool.AvailabilityZones,
		MaxPods:             pool.MaxPods,
		OsDiskType:          containerservice.OSDiskType(to.String(pool.OsDiskType)),
		NodeLabels:          pool.NodeLabels,
		EnableUltraSSD:      pool.EnableUltraSSD,
		EnableFIPS:          pool.EnableFIPS,
		EnableNodePublicIP:  pool.EnableNodePublicIP,
	}
	if pool.ScaleSetPriority != nil {
		profile.ScaleSetPriority = containerservice.ScaleSetPriority(*pool.ScaleSetPriority)
	}
	return profile
}

// AgentPoolToContainerServiceAgentPool converts a AgentPoolSpec to an Azure SDK AgentPool used in agentpool reconcile.
func AgentPoolToContainerServiceAgentPool(pool azure.AgentPoolSpec) containerservice.AgentPool {
	containerSvcAgentPool := containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			VMSize:              &pool.SKU,
			OsType:              containerservice.OSTypeLinux,
			OsDiskSizeGB:        &pool.OSDiskSizeGB,
			Count:               &pool.Replicas,
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			OrchestratorVersion: pool.Version,
			VnetSubnetID:        &pool.VnetSubnetID,
			Mode:                containerservice.AgentPoolMode(pool.Mode),
			EnableAutoScaling:   pool.EnableAutoScaling,
			MaxCount:            pool.MaxCount,
			MinCount:            pool.MinCount,
			NodeTaints:          &pool.NodeTaints,
			AvailabilityZones:   &pool.AvailabilityZones,
			MaxPods:             pool.MaxPods,
			OsDiskType:          containerservice.OSDiskType(to.String(pool.OsDiskType)),
			NodeLabels:          pool.NodeLabels,
			EnableUltraSSD:      pool.EnableUltraSSD,
			EnableFIPS:          pool.EnableFIPS,
			EnableNodePublicIP:  pool.EnableNodePublicIP,
		},
	}
	if pool.ScaleSetPriority != nil {
		containerSvcAgentPool.ScaleSetPriority = containerservice.ScaleSetPriority(*pool.ScaleSetPriority)
	}
	return containerSvcAgentPool
}
