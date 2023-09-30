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
	"testing"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20230201"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

func Test_AgentPoolToManagedClusterAgentPoolProfile(t *testing.T) {
	cases := []struct {
		name   string
		pool   *asocontainerservicev1.ManagedClustersAgentPool
		expect func(*GomegaWithT, asocontainerservicev1.ManagedClusterAgentPoolProfile)
	}{
		{
			name: "Should set all values correctly",
			pool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					AzureName:           "agentpool1",
					VmSize:              ptr.To("Standard_D2s_v3"),
					OsType:              ptr.To(asocontainerservicev1.OSType_Linux),
					OsDiskSizeGB:        ptr.To[asocontainerservicev1.ContainerServiceOSDisk](100),
					Count:               ptr.To(2),
					Type:                ptr.To(asocontainerservicev1.AgentPoolType_VirtualMachineScaleSets),
					OrchestratorVersion: ptr.To("1.22.6"),
					VnetSubnetReference: &genruntime.ResourceReference{
						ARMID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-123/providers/Microsoft.Network/virtualNetworks/vnet-123/subnets/subnet-123",
					},
					Mode:              ptr.To(asocontainerservicev1.AgentPoolMode_User),
					EnableAutoScaling: ptr.To(true),
					MaxCount:          ptr.To(5),
					MinCount:          ptr.To(2),
					NodeTaints:        []string{"key1=value1:NoSchedule"},
					AvailabilityZones: []string{"zone1"},
					MaxPods:           ptr.To(60),
					OsDiskType:        ptr.To(asocontainerservicev1.OSDiskType_Managed),
					NodeLabels: map[string]string{
						"custom": "default",
					},
					Tags: map[string]string{
						"custom": "default",
					},
					EnableFIPS:             ptr.To(true),
					EnableEncryptionAtHost: ptr.To(true),
				},
			},

			expect: func(g *GomegaWithT, result asocontainerservicev1.ManagedClusterAgentPoolProfile) {
				g.Expect(result).To(Equal(asocontainerservicev1.ManagedClusterAgentPoolProfile{
					Name:                ptr.To("agentpool1"),
					VmSize:              ptr.To("Standard_D2s_v3"),
					OsType:              ptr.To(asocontainerservicev1.OSType_Linux),
					OsDiskSizeGB:        ptr.To[asocontainerservicev1.ContainerServiceOSDisk](100),
					Count:               ptr.To(2),
					Type:                ptr.To(asocontainerservicev1.AgentPoolType_VirtualMachineScaleSets),
					OrchestratorVersion: ptr.To("1.22.6"),
					VnetSubnetReference: &genruntime.ResourceReference{
						ARMID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-123/providers/Microsoft.Network/virtualNetworks/vnet-123/subnets/subnet-123",
					},
					Mode:              ptr.To(asocontainerservicev1.AgentPoolMode_User),
					EnableAutoScaling: ptr.To(true),
					MaxCount:          ptr.To(5),
					MinCount:          ptr.To(2),
					NodeTaints:        []string{"key1=value1:NoSchedule"},
					AvailabilityZones: []string{"zone1"},
					MaxPods:           ptr.To(60),
					OsDiskType:        ptr.To(asocontainerservicev1.OSDiskType_Managed),
					NodeLabels: map[string]string{
						"custom": "default",
					},
					Tags: map[string]string{
						"custom": "default",
					},
					EnableFIPS:             ptr.To(true),
					EnableEncryptionAtHost: ptr.To(true),
				}))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result := AgentPoolToManagedClusterAgentPoolProfile(c.pool)
			c.expect(g, result)
		})
	}
}
