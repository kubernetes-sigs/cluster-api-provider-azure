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
	"testing"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20230201"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestParameters(t *testing.T) {
	t.Run("no existing agent pool", func(t *testing.T) {
		g := NewGomegaWithT(t)

		spec := &AgentPoolSpec{
			Name:                 "name",
			Namespace:            "namespace",
			AzureName:            "azure name",
			ResourceGroup:        "rg",
			Cluster:              "cluster",
			Version:              ptr.To("version"),
			SKU:                  "sku",
			Replicas:             1,
			OSDiskSizeGB:         2,
			VnetSubnetID:         "vnet subnet id",
			Mode:                 "mode",
			MaxCount:             ptr.To(3),
			MinCount:             ptr.To(4),
			NodeLabels:           map[string]string{"node": "labels"},
			NodeTaints:           []string{"node taints"},
			EnableAutoScaling:    true,
			AvailabilityZones:    []string{"zones"},
			MaxPods:              ptr.To(5),
			OsDiskType:           ptr.To("disk type"),
			EnableUltraSSD:       ptr.To(false),
			OSType:               ptr.To("os type"),
			EnableNodePublicIP:   ptr.To(true),
			NodePublicIPPrefixID: "public IP prefix ID",
			ScaleSetPriority:     ptr.To("scaleset priority"),
			ScaleDownMode:        ptr.To("scale down mode"),
			SpotMaxPrice:         ptr.To(resource.MustParse("123")),
			KubeletConfig: &KubeletConfig{
				CPUManagerPolicy: ptr.To("cpu manager policy"),
			},
			KubeletDiskType: ptr.To(infrav1.KubeletDiskType("kubelet disk type")),
			AdditionalTags:  map[string]string{"additional": "tags"},
			LinuxOSConfig: &infrav1.LinuxOSConfig{
				Sysctls: &infrav1.SysctlConfig{
					FsNrOpen: ptr.To(6),
				},
			},
			EnableFIPS:             ptr.To(true),
			EnableEncryptionAtHost: ptr.To(false),
		}
		expected := &asocontainerservicev1.ManagedClustersAgentPool{
			Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
				AzureName: "azure name",
				Owner: &genruntime.KnownResourceReference{
					Name: "cluster",
				},
				AvailabilityZones:      []string{"zones"},
				Count:                  ptr.To(1),
				EnableAutoScaling:      ptr.To(true),
				EnableUltraSSD:         ptr.To(false),
				EnableEncryptionAtHost: ptr.To(false),
				KubeletDiskType:        ptr.To(asocontainerservicev1.KubeletDiskType("kubelet disk type")),
				MaxCount:               ptr.To(3),
				MaxPods:                ptr.To(5),
				MinCount:               ptr.To(4),
				Mode:                   ptr.To(asocontainerservicev1.AgentPoolMode("mode")),
				NodeLabels:             map[string]string{"node": "labels"},
				NodeTaints:             []string{"node taints"},
				OrchestratorVersion:    ptr.To("version"),
				OsDiskSizeGB:           ptr.To(asocontainerservicev1.ContainerServiceOSDisk(2)),
				OsDiskType:             ptr.To(asocontainerservicev1.OSDiskType("disk type")),
				OsType:                 ptr.To(asocontainerservicev1.OSType("os type")),
				ScaleSetPriority:       ptr.To(asocontainerservicev1.ScaleSetPriority("scaleset priority")),
				ScaleDownMode:          ptr.To(asocontainerservicev1.ScaleDownMode("scale down mode")),
				Type:                   ptr.To(asocontainerservicev1.AgentPoolType_VirtualMachineScaleSets),
				EnableNodePublicIP:     ptr.To(true),
				Tags:                   map[string]string{"additional": "tags"},
				EnableFIPS:             ptr.To(true),
				KubeletConfig: &asocontainerservicev1.KubeletConfig{
					CpuManagerPolicy: ptr.To("cpu manager policy"),
				},
				VmSize:       ptr.To("sku"),
				SpotMaxPrice: ptr.To(ptr.To(resource.MustParse("123")).AsApproximateFloat64()),
				VnetSubnetReference: &genruntime.ResourceReference{
					ARMID: "vnet subnet id",
				},
				NodePublicIPPrefixReference: &genruntime.ResourceReference{
					ARMID: "public IP prefix ID",
				},
				LinuxOSConfig: &asocontainerservicev1.LinuxOSConfig{
					Sysctls: &asocontainerservicev1.SysctlConfig{
						FsNrOpen: ptr.To(6),
					},
				},
			},
		}

		actual, err := spec.Parameters(context.Background(), nil)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(cmp.Diff(actual, expected)).To(BeEmpty())
	})

	t.Run("with existing agent pool", func(t *testing.T) {
		g := NewGomegaWithT(t)

		spec := &AgentPoolSpec{
			AzureName:         "managed by CAPZ",
			Replicas:          3,
			EnableAutoScaling: true,
		}
		existing := &asocontainerservicev1.ManagedClustersAgentPool{
			Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
				AzureName: "set by the user",
				PowerState: &asocontainerservicev1.PowerState{
					Code: ptr.To(asocontainerservicev1.PowerState_Code("set by the user")),
				},
			},
			Status: asocontainerservicev1.ManagedClusters_AgentPool_STATUS{
				Count: ptr.To(1212),
			},
		}

		actual, err := spec.Parameters(context.Background(), existing)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(actual.Spec.AzureName).To(Equal("managed by CAPZ"))
		g.Expect(actual.Spec.Count).To(Equal(ptr.To(1212)))
		g.Expect(actual.Spec.PowerState.Code).To(Equal(ptr.To(asocontainerservicev1.PowerState_Code("set by the user"))))
	})
}
