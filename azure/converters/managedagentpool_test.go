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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2022-03-01/containerservice"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func Test_AgentPoolToManagedClusterAgentPoolProfile(t *testing.T) {
	cases := []struct {
		name   string
		pool   containerservice.AgentPool
		expect func(*GomegaWithT, containerservice.ManagedClusterAgentPoolProfile)
	}{
		{
			name: "Should set all values correctly",
			pool: containerservice.AgentPool{
				Name: ptr.To("agentpool1"),
				ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
					VMSize:              ptr.To("Standard_D2s_v3"),
					OsType:              azure.LinuxOS,
					OsDiskSizeGB:        ptr.To[int32](100),
					Count:               ptr.To[int32](2),
					Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
					OrchestratorVersion: ptr.To("1.22.6"),
					VnetSubnetID:        ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-123/providers/Microsoft.Network/virtualNetworks/vnet-123/subnets/subnet-123"),
					Mode:                containerservice.AgentPoolModeUser,
					EnableAutoScaling:   ptr.To(true),
					MaxCount:            ptr.To[int32](5),
					MinCount:            ptr.To[int32](2),
					NodeTaints:          &[]string{"key1=value1:NoSchedule"},
					AvailabilityZones:   &[]string{"zone1"},
					MaxPods:             ptr.To[int32](60),
					OsDiskType:          containerservice.OSDiskTypeManaged,
					NodeLabels: map[string]*string{
						"custom": ptr.To("default"),
					},
					Tags: map[string]*string{
						"custom": ptr.To("default"),
					},
					EnableFIPS: ptr.To(true),
				},
			},

			expect: func(g *GomegaWithT, result containerservice.ManagedClusterAgentPoolProfile) {
				g.Expect(result).To(Equal(containerservice.ManagedClusterAgentPoolProfile{
					Name:                ptr.To("agentpool1"),
					VMSize:              ptr.To("Standard_D2s_v3"),
					OsType:              azure.LinuxOS,
					OsDiskSizeGB:        ptr.To[int32](100),
					Count:               ptr.To[int32](2),
					Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
					OrchestratorVersion: ptr.To("1.22.6"),
					VnetSubnetID:        ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-123/providers/Microsoft.Network/virtualNetworks/vnet-123/subnets/subnet-123"),
					Mode:                containerservice.AgentPoolModeUser,
					EnableAutoScaling:   ptr.To(true),
					MaxCount:            ptr.To[int32](5),
					MinCount:            ptr.To[int32](2),
					NodeTaints:          &[]string{"key1=value1:NoSchedule"},
					AvailabilityZones:   &[]string{"zone1"},
					MaxPods:             ptr.To[int32](60),
					OsDiskType:          containerservice.OSDiskTypeManaged,
					NodeLabels: map[string]*string{
						"custom": ptr.To("default"),
					},
					Tags: map[string]*string{
						"custom": ptr.To("default"),
					},
					EnableFIPS: ptr.To(true),
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
