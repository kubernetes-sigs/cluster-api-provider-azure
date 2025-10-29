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

package scope

import (
	"reflect"
	"testing"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
)

func TestNewManagedMachinePoolScope(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	input := ManagedMachinePoolScopeParams{
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster1",
				Namespace: "default",
			},
		},
		ControlPlane: &infrav1.AzureManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster1",
				Namespace: "default",
			},
			Spec: infrav1.AzureManagedControlPlaneSpec{
				AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
					SubscriptionID: "00000000-0000-0000-0000-000000000000",
				},
			},
		},
		ManagedMachinePool: ManagedMachinePool{
			MachinePool:      getMachinePool("pool0"),
			InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
		},
	}

	g := NewWithT(t)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(input.MachinePool, input.InfraMachinePool, input.ControlPlane).Build()
	input.Client = fakeClient
	_, err := NewManagedMachinePoolScope(t.Context(), input)
	g.Expect(err).To(Succeed())
}

func TestManagedMachinePoolScope_Autoscaling(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedMachinePoolScope
		Expected azure.ASOResourceSpecGetter[genruntime.MetaObject]
	}{
		{
			Name: "Without Autoscaling",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: &agentpools.AgentPoolSpec{

				Name:         "pool0",
				AzureName:    "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With Autoscaling",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithScaling("pool1", 2, 10),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:              "pool1",
				AzureName:         "pool1",
				SKU:               "Standard_D2s_v3",
				Mode:              "User",
				Cluster:           "cluster1",
				Replicas:          1,
				EnableAutoScaling: true,
				MinCount:          ptr.To(2),
				MaxCount:          ptr.To(10),
				VnetSubnetID:      "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			agentPool := c.Scope.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_NodeLabels(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedMachinePoolScope
		Expected azure.ASOResourceSpecGetter[genruntime.MetaObject]
	}{
		{
			Name: "Without node labels",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				AzureName:    "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With node labels",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool: getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithLabels("pool1", map[string]string{
					"custom": "default",
				}),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:      "pool1",
				AzureName: "pool1",
				SKU:       "Standard_D2s_v3",
				Mode:      "System",
				Cluster:   "cluster1",
				Replicas:  1,
				NodeLabels: map[string]string{
					"custom": "default",
				},
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			agentPool := c.Scope.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_AdditionalTags(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedMachinePoolScope
		Expected azure.ASOResourceSpecGetter[genruntime.MetaObject]
	}{
		{
			Name: "Without additional tags",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				AzureName:    "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With additional tags",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool: getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithAdditionalTags("pool1", map[string]string{
					"environment": "production",
				}),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:      "pool1",
				AzureName: "pool1",
				SKU:       "Standard_D2s_v3",
				Mode:      "System",
				Cluster:   "cluster1",
				Replicas:  1,
				AdditionalTags: map[string]string{
					"environment": "production",
				},
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			agentPool := c.Scope.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_MaxPods(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedMachinePoolScope
		Expected azure.ASOResourceSpecGetter[genruntime.MetaObject]
	}{
		{
			Name: "Without MaxPods",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				AzureName:    "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With MaxPods",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithMaxPods("pool1", 12),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				AzureName:    "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "System",
				Cluster:      "cluster1",
				Replicas:     1,
				MaxPods:      ptr.To(12),
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			agentPool := c.Scope.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_Taints(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedMachinePoolScope
		Expected azure.ASOResourceSpecGetter[genruntime.MetaObject]
	}{
		{
			Name: "Without taints",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: &agentpools.AgentPoolSpec{

				Name:         "pool0",
				AzureName:    "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With taints",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool: getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithTaints("pool1", infrav1.Taints{
					infrav1.Taint{
						Key:    "key1",
						Value:  "value1",
						Effect: "NoSchedule",
					},
				}),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				AzureName:    "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "User",
				Cluster:      "cluster1",
				Replicas:     1,
				NodeTaints:   []string{"key1=value1:NoSchedule"},
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			agentPool := c.Scope.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_OSDiskType(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedMachinePoolScope
		Expected azure.ASOResourceSpecGetter[genruntime.MetaObject]
	}{
		{
			Name: "Without OsDiskType",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				AzureName:    "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With OsDiskType",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithOsDiskType("pool1", string(asocontainerservicev1.OSDiskType_Ephemeral)),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				AzureName:    "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "User",
				Cluster:      "cluster1",
				Replicas:     1,
				OsDiskType:   ptr.To(string(asocontainerservicev1.OSDiskType_Ephemeral)),
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			agentPool := c.Scope.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_SubnetName(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedMachinePoolScope
		Expected azure.ASOResourceSpecGetter[genruntime.MetaObject]
	}{
		{
			Name: "Without Vnet and SubnetName",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				AzureName:    "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With Vnet and Without SubnetName",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
							VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
								Name: "my-vnet",
								ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
									Subnet: infrav1.ManagedControlPlaneSubnet{
										Name: "my-vnet-subnet",
									},
								},
								ResourceGroup: "my-resource-group",
							},
						},
					},
				},
				MachinePool:      getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePool("pool1", infrav1.NodePoolModeUser),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				AzureName:    "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "User",
				Cluster:      "cluster1",
				Replicas:     1,
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-vnet-subnet",
			},
		},
		{
			Name: "With Vnet and With SubnetName",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
							VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
								Name: "my-vnet",
								ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
									Subnet: infrav1.ManagedControlPlaneSubnet{
										Name: "my-vnet-subnet",
									},
								},
								ResourceGroup: "my-resource-group",
							},
						},
					},
				},
				MachinePool:      getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithSubnetName("pool1", ptr.To("my-subnet")),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				AzureName:    "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "User",
				Cluster:      "cluster1",
				Replicas:     1,
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			c.Scope.SetSubnetName()
			agentPool := c.Scope.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_KubeletDiskType(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedMachinePoolScope
		Expected azure.ASOResourceSpecGetter[genruntime.MetaObject]
	}{
		{
			Name: "Without KubeletDiskType",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				AzureName:    "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With KubeletDiskType",
			Scope: &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				MachinePool:      getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithKubeletDiskType("pool1", (*infrav1.KubeletDiskType)(ptr.To("Temporary"))),
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:            "pool1",
				AzureName:       "pool1",
				SKU:             "Standard_D2s_v3",
				Mode:            "User",
				Cluster:         "cluster1",
				Replicas:        1,
				KubeletDiskType: (*infrav1.KubeletDiskType)(ptr.To("Temporary")),
				VnetSubnetID:    "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			agentPool := c.Scope.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_EnablePreviewFeatures(t *testing.T) {
	cases := []struct {
		Name           string
		previewEnabled *bool
		Expected       bool
	}{
		{
			Name:           "Without EnablePreviewFeatures",
			previewEnabled: nil,
			Expected:       false,
		},
		{
			Name:           "With EnablePreviewFeatures false",
			previewEnabled: ptr.To(false),
			Expected:       false,
		},
		{
			Name:           "With EnablePreviewFeatures true",
			previewEnabled: ptr.To(true),
			Expected:       true,
		},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			s := &ManagedMachinePoolScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							EnablePreviewFeatures: c.previewEnabled,
						},
					},
				},
				MachinePool:      &expv1.MachinePool{},
				InfraMachinePool: &infrav1.AzureManagedMachinePool{},
			}
			agentPoolGetter := s.AgentPoolSpec()
			agentPool, ok := agentPoolGetter.(*agentpools.AgentPoolSpec)
			g.Expect(ok).To(BeTrue())
			g.Expect(agentPool.Preview).To(Equal(c.Expected))
			g.Expect(s.IsPreviewEnabled()).To(Equal(c.Expected))
		})
	}
}

func Test_getManagedMachinePoolVersion(t *testing.T) {
	cases := []struct {
		name                string
		managedControlPlane *infrav1.AzureManagedControlPlane
		machinePool         *expv1.MachinePool
		expected            *string
	}{
		{
			name:                "Empty configs",
			managedControlPlane: nil,
			machinePool:         nil,
			expected:            nil,
		},
		{
			name:                "Empty mp",
			managedControlPlane: &infrav1.AzureManagedControlPlane{},
			machinePool:         nil,
			expected:            nil,
		},
		{
			name:                "Only machine pool is available",
			managedControlPlane: nil,
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: ptr.To("v1.15.0"),
						},
					},
				},
			},
			expected: ptr.To("1.15.0"),
		},
		{
			name:                "Only machine pool is available and cp is nil",
			managedControlPlane: nil,
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: ptr.To("v1.15.0"),
						},
					},
				},
			},
			expected: ptr.To("1.15.0"),
		},
		{
			name: "mcp.status.autoUpgradeVersion > mp.spec.template.spec.version",
			managedControlPlane: &infrav1.AzureManagedControlPlane{
				Status: infrav1.AzureManagedControlPlaneStatus{
					AutoUpgradeVersion: "1.20.3",
				},
			},
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: ptr.To("v1.15.0"),
						},
					},
				},
			},
			expected: ptr.To("1.20.3"),
		},
		{
			name: "suffix + mcp.status.autoUpgradeVersion > mp.spec.template.spec.version",
			managedControlPlane: &infrav1.AzureManagedControlPlane{
				Status: infrav1.AzureManagedControlPlaneStatus{
					AutoUpgradeVersion: "v1.20.3",
				},
			},
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: ptr.To("v1.15.0"),
						},
					},
				},
			},
			expected: ptr.To("1.20.3"),
		},
		{
			name: "mcp.status.autoUpgradeVersion < mp.spec.template.spec.version",
			managedControlPlane: &infrav1.AzureManagedControlPlane{
				Status: infrav1.AzureManagedControlPlaneStatus{
					AutoUpgradeVersion: "v1.20.3",
				},
			},
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: ptr.To("v1.21.0"),
						},
					},
				},
			},
			expected: ptr.To("1.21.0"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := NewWithT(t)
			v := getManagedMachinePoolVersion(c.managedControlPlane, c.machinePool)
			if c.expected != nil {
				g.Expect(*v).To(Equal(*c.expected))
			} else {
				g.Expect(v).To(BeNil())
			}
		})
	}
}

func getAzureMachinePool(name string, mode infrav1.NodePoolMode) *infrav1.AzureManagedMachinePool {
	return &infrav1.AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "cluster1",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "MachinePool",
					Name:       name,
				},
			},
		},
		Spec: infrav1.AzureManagedMachinePoolSpec{
			AzureManagedMachinePoolClassSpec: infrav1.AzureManagedMachinePoolClassSpec{
				Mode: string(mode),
				SKU:  "Standard_D2s_v3",
				Name: ptr.To(name),
			},
		},
	}
}

func getAzureMachinePoolWithScaling(name string, minVal, maxVal int) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.Scaling = &infrav1.ManagedMachinePoolScaling{
		MinSize: ptr.To(minVal),
		MaxSize: ptr.To(maxVal),
	}
	return managedPool
}

func getAzureMachinePoolWithMaxPods(name string, maxPods int) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeSystem)
	managedPool.Spec.MaxPods = ptr.To(maxPods)
	return managedPool
}

func getAzureMachinePoolWithTaints(name string, taints infrav1.Taints) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.Taints = taints
	return managedPool
}

func getAzureMachinePoolWithSubnetName(name string, subnetName *string) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.SubnetName = subnetName
	return managedPool
}

func getAzureMachinePoolWithOsDiskType(name string, osDiskType string) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.OsDiskType = ptr.To(osDiskType)
	return managedPool
}

func getAzureMachinePoolWithKubeletDiskType(name string, kubeletDiskType *infrav1.KubeletDiskType) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.KubeletDiskType = kubeletDiskType
	return managedPool
}

func getAzureMachinePoolWithLabels(name string, nodeLabels map[string]string) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeSystem)
	managedPool.Spec.NodeLabels = nodeLabels
	return managedPool
}

func getAzureMachinePoolWithAdditionalTags(name string, additionalTags infrav1.Tags) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeSystem)
	managedPool.Spec.AdditionalTags = additionalTags
	return managedPool
}

func getMachinePool(name string) *expv1.MachinePool {
	return &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "cluster1",
			},
		},
		Spec: expv1.MachinePoolSpec{
			ClusterName: "cluster1",
		},
	}
}

func getLinuxAzureMachinePool(name string) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.OSType = ptr.To(azure.LinuxOS)
	return managedPool
}

func getWindowsAzureMachinePool(name string) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.OSType = ptr.To(azure.WindowsOS)
	return managedPool
}

func getMachinePoolWithVersion(name, version string) *expv1.MachinePool {
	machine := getMachinePool(name)
	machine.Spec.Template.Spec.Version = ptr.To(version)
	return machine
}
