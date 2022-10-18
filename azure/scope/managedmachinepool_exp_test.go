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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManagedMachinePoolScope_Autoscaling(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1exp.NodePoolModeSystem),
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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
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
				EnableAutoScaling: to.BoolPtr(true),
				MinCount:          to.Int32Ptr(2),
				MaxCount:          to.Int32Ptr(10),
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
	_ = infrav1exp.AddToScheme(scheme)

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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1exp.NodePoolModeSystem),
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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
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
					"custom": to.StringPtr("default"),
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
	_ = infrav1exp.AddToScheme(scheme)

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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1exp.NodePoolModeSystem),
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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
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
				MaxPods:      to.Int32Ptr(12),
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
	_ = infrav1exp.AddToScheme(scheme)

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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1exp.NodePoolModeSystem),
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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool: getMachinePool("pool1"),
					InfraMachinePool: getAzureMachinePoolWithTaints("pool1", infrav1exp.Taints{
						infrav1exp.Taint{
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
	_ = infrav1exp.AddToScheme(scheme)

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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				ManagedMachinePool: ManagedMachinePool{
					MachinePool:      getMachinePool("pool0"),
					InfraMachinePool: getAzureMachinePool("pool0", infrav1exp.NodePoolModeSystem),
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
				ControlPlane: &infrav1exp.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: infrav1exp.AzureManagedControlPlaneSpec{
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
				OsDiskType:   to.StringPtr(string(containerservice.OSDiskTypeEphemeral)),
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

func getAzureMachinePool(name string, mode infrav1exp.NodePoolMode) *infrav1exp.AzureManagedMachinePool {
	return &infrav1exp.AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "cluster1",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "MachinePool",
					Name:       name,
				},
			},
		},
		Spec: infrav1exp.AzureManagedMachinePoolSpec{
			Mode: string(mode),
			SKU:  "Standard_D2s_v3",
			Name: to.StringPtr(name),
		},
	}
}

func getAzureMachinePoolWithScaling(name string, min, max int32) *infrav1exp.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1exp.NodePoolModeUser)
	managedPool.Spec.Scaling = &infrav1exp.ManagedMachinePoolScaling{
		MinSize: to.Int32Ptr(min),
		MaxSize: to.Int32Ptr(max),
	}
	return managedPool
}

func getAzureMachinePoolWithMaxPods(name string, maxPods int32) *infrav1exp.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1exp.NodePoolModeSystem)
	managedPool.Spec.MaxPods = to.Int32Ptr(maxPods)
	return managedPool
}

func getAzureMachinePoolWithTaints(name string, taints infrav1exp.Taints) *infrav1exp.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1exp.NodePoolModeUser)
	managedPool.Spec.Taints = taints
	return managedPool
}

func getAzureMachinePoolWithOsDiskType(name string, osDiskType string) *infrav1exp.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1exp.NodePoolModeUser)
	managedPool.Spec.OsDiskType = to.StringPtr(osDiskType)
	return managedPool
}

func getAzureMachinePoolWithLabels(name string, nodeLabels map[string]string) *infrav1exp.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1exp.NodePoolModeSystem)
	managedPool.Spec.NodeLabels = nodeLabels
	return managedPool
}

func getMachinePool(name string) *expv1.MachinePool {
	return &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "cluster1",
			},
		},
		Spec: expv1.MachinePoolSpec{
			ClusterName: "cluster1",
		},
	}
}

func getLinuxAzureMachinePool(name string) *infrav1exp.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1exp.NodePoolModeUser)
	managedPool.Spec.OSType = to.StringPtr(azure.LinuxOS)
	return managedPool
}

func getWindowsAzureMachinePool(name string) *infrav1exp.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1exp.NodePoolModeUser)
	managedPool.Spec.OSType = to.StringPtr(azure.WindowsOS)
	return managedPool
}

func getMachinePoolWithVersion(name, version string) *expv1.MachinePool {
	machine := getMachinePool(name)
	machine.Spec.Template.Spec.Version = to.StringPtr(version)
	return machine
}
