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
	"context"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2022-03-01/containerservice"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManagedMachinePoolScope_Autoscaling(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedMachinePoolScopeParams
		Expected azure.ResourceSpecGetter
	}{
		{
			Name: "Without Autoscaling",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				},
			},
			Expected: &agentpools.AgentPoolSpec{

				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
		{
			Name: "With Autoscaling",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePoolWithScaling("pool1", 2, 10),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:              "pool1",
				SKU:               "Standard_D2s_v3",
				Mode:              "User",
				Cluster:           "cluster1",
				Replicas:          1,
				EnableAutoScaling: pointer.Bool(true),
				MinCount:          pointer.Int32(2),
				MaxCount:          pointer.Int32(10),
				VnetSubnetID:      "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:           map[string]string{},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedMachinePoolScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_NodeLabels(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedMachinePoolScopeParams
		Expected azure.ResourceSpecGetter
	}{
		{
			Name: "Without node labels",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
		{
			Name: "With node labels",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool: getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePoolWithLabels("pool1", map[string]string{
						"custom": "default",
					}),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:     "pool1",
				SKU:      "Standard_D2s_v3",
				Mode:     "System",
				Cluster:  "cluster1",
				Replicas: 1,
				NodeLabels: map[string]*string{
					"custom": pointer.String("default"),
				},
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedMachinePoolScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_AdditionalTags(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedMachinePoolScopeParams
		Expected azure.ResourceSpecGetter
	}{
		{
			Name: "Without additional tags",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
		{
			Name: "With additional tags",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool: getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePoolWithAdditionalTags("pool1", map[string]string{
						"environment": "production",
					}),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:     "pool1",
				SKU:      "Standard_D2s_v3",
				Mode:     "System",
				Cluster:  "cluster1",
				Replicas: 1,
				AdditionalTags: map[string]string{
					"environment": "production",
				},
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedMachinePoolScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_MaxPods(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedMachinePoolScopeParams
		Expected azure.ResourceSpecGetter
	}{
		{
			Name: "Without MaxPods",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
		{
			Name: "With MaxPods",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePoolWithMaxPods("pool1", 12),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "System",
				Cluster:      "cluster1",
				Replicas:     1,
				MaxPods:      pointer.Int32(12),
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedMachinePoolScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_Taints(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedMachinePoolScopeParams
		Expected azure.ResourceSpecGetter
	}{
		{
			Name: "Without taints",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				},
			},
			Expected: &agentpools.AgentPoolSpec{

				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
		{
			Name: "With taints",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool: getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePoolWithTaints("pool1", infrav1.Taints{
						infrav1.Taint{
							Key:    "key1",
							Value:  "value1",
							Effect: "NoSchedule",
						},
					}),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "User",
				Cluster:      "cluster1",
				Replicas:     1,
				NodeTaints:   []string{"key1=value1:NoSchedule"},
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedMachinePoolScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_OSDiskType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedMachinePoolScopeParams
		Expected azure.ResourceSpecGetter
	}{
		{
			Name: "Without OsDiskType",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
		{
			Name: "With OsDiskType",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePoolWithOsDiskType("pool1", string(containerservice.OSDiskTypeEphemeral)),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "User",
				Cluster:      "cluster1",
				Replicas:     1,
				OsDiskType:   pointer.String(string(containerservice.OSDiskTypeEphemeral)),
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedMachinePoolScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_SubnetName(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedMachinePoolScopeParams
		Expected azure.ResourceSpecGetter
	}{
		{
			Name: "Without Vnet and SubnetName",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
		{
			Name: "With Vnet and Without SubnetName",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							Name: "my-vnet",
							Subnet: infrav1.ManagedControlPlaneSubnet{
								Name: "my-vnet-subnet",
							},
							ResourceGroup: "my-resource-group",
						},
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePool("pool1", infrav1.NodePoolModeUser),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "User",
				Cluster:      "cluster1",
				Replicas:     1,
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-vnet-subnet",
				Headers:      map[string]string{},
			},
		},
		{
			Name: "With Vnet and With SubnetName",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							Name: "my-vnet",
							Subnet: infrav1.ManagedControlPlaneSubnet{
								Name: "my-vnet-subnet",
							},
							ResourceGroup: "my-resource-group",
						},
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePoolWithSubnetName("pool1", pointer.StringPtr("my-subnet")),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "User",
				Cluster:      "cluster1",
				Replicas:     1,
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet",
				Headers:      map[string]string{},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedMachinePoolScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			s.SetSubnetName()
			agentPool := s.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
			}
		})
	}
}

func TestManagedMachinePoolScope_KubeletDiskType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedMachinePoolScopeParams
		Expected azure.ResourceSpecGetter
	}{
		{
			Name: "Without KubeletDiskType",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:      map[string]string{},
			},
		},
		{
			Name: "With KubeletDiskType",
			Input: ManagedMachinePoolScopeParams{
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
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePoolWithKubeletDiskType("pool1", (*infrav1.KubeletDiskType)(pointer.String("Temporary"))),
				},
			},
			Expected: &agentpools.AgentPoolSpec{
				Name:            "pool1",
				SKU:             "Standard_D2s_v3",
				Mode:            "User",
				Cluster:         "cluster1",
				Replicas:        1,
				KubeletDiskType: (*infrav1.KubeletDiskType)(pointer.String("Temporary")),
				VnetSubnetID:    "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				Headers:         map[string]string{},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedMachinePoolScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			if !reflect.DeepEqual(c.Expected, agentPool) {
				t.Errorf("Got difference between expected result and result:\n%s", cmp.Diff(c.Expected, agentPool))
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
			Mode: string(mode),
			SKU:  "Standard_D2s_v3",
			Name: pointer.String(name),
		},
	}
}

func getAzureMachinePoolWithScaling(name string, min, max int32) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.Scaling = &infrav1.ManagedMachinePoolScaling{
		MinSize: pointer.Int32(min),
		MaxSize: pointer.Int32(max),
	}
	return managedPool
}

func getAzureMachinePoolWithMaxPods(name string, maxPods int32) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeSystem)
	managedPool.Spec.MaxPods = pointer.Int32(maxPods)
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
	managedPool.Spec.OsDiskType = pointer.String(osDiskType)
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
	managedPool.Spec.OSType = pointer.String(azure.LinuxOS)
	return managedPool
}

func getWindowsAzureMachinePool(name string) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.OSType = pointer.String(azure.WindowsOS)
	return managedPool
}

func getMachinePoolWithVersion(name, version string) *expv1.MachinePool {
	machine := getMachinePool(name)
	machine.Spec.Template.Spec.Version = pointer.String(version)
	return machine
}
