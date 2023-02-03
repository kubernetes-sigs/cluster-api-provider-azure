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

package scope

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManagedControlPlaneScope_Autoscaling(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected azure.AgentPoolSpec
	}{
		{
			Name: "Without Autoscaling",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: azure.AgentPoolSpec{

				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With Autoscaling",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithScaling("pool1", 2, 10),
				PatchTarget:      getAzureMachinePoolWithScaling("pool1", 2, 10),
			},
			Expected: azure.AgentPoolSpec{
				Name:              "pool1",
				SKU:               "Standard_D2s_v3",
				Mode:              "User",
				Cluster:           "cluster1",
				Replicas:          1,
				EnableAutoScaling: to.BoolPtr(true),
				MinCount:          to.Int32Ptr(2),
				MaxCount:          to.Int32Ptr(10),
				VnetSubnetID:      "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedControlPlaneScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			g.Expect(agentPool).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_NodeLabels(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected azure.AgentPoolSpec
	}{
		{
			Name: "Without node labels",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: azure.AgentPoolSpec{

				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With node labels",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool: getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithLabels("pool1", map[string]string{
					"custom": "default",
				}),
				PatchTarget: getAzureMachinePoolWithLabels("pool1", map[string]string{
					"custom": "default",
				}),
			},
			Expected: azure.AgentPoolSpec{
				Name:     "pool1",
				SKU:      "Standard_D2s_v3",
				Mode:     "System",
				Cluster:  "cluster1",
				Replicas: 1,
				NodeLabels: map[string]*string{
					"custom": to.StringPtr("default"),
				},
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedControlPlaneScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			g.Expect(agentPool).To(Equal(c.Expected))
			agentPools, err := s.GetAllAgentPoolSpecs(context.TODO())
			g.Expect(err).To(Succeed())
			g.Expect(agentPools[0].NodeLabels).To(Equal(c.Expected.NodeLabels))
		})
	}
}

func TestManagedControlPlaneScope_PoolVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected []azure.AgentPoolSpec
		Err      string
	}{
		{
			Name: "Without Version",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: []azure.AgentPoolSpec{
				{
					Name:         "pool0",
					SKU:          "Standard_D2s_v3",
					Replicas:     1,
					Mode:         "System",
					Cluster:      "cluster1",
					VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				},
			},
		},
		{
			Name: "With Version",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
						Version:        "v1.22.0",
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				MachinePool:      getMachinePoolWithVersion("pool0", "v1.21.1"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: []azure.AgentPoolSpec{
				{
					Name:         "pool0",
					SKU:          "Standard_D2s_v3",
					Mode:         "System",
					Replicas:     1,
					Version:      to.StringPtr("1.21.1"),
					Cluster:      "cluster1",
					VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				},
			},
		},
		{
			Name: "With bad version",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
						Version:        "v1.20.1",
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				MachinePool:      getMachinePoolWithVersion("pool0", "v1.21.1"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Err: "MachinePool version cannot be greater than the AzureManagedControlPlane version",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedControlPlaneScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPools, err := s.GetAllAgentPoolSpecs(context.TODO())
			if err != nil {
				g.Expect(err.Error()).To(Equal(c.Err))
			} else {
				g.Expect(agentPools).To(Equal(c.Expected))
			}
		})
	}
}

func TestManagedControlPlaneScope_MaxPods(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected azure.AgentPoolSpec
	}{
		{
			Name: "Without MaxPods",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: azure.AgentPoolSpec{
				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With MaxPods",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithMaxPods("pool1", 12),
				PatchTarget:      getAzureMachinePoolWithScaling("pool1", 2, 10),
			},
			Expected: azure.AgentPoolSpec{
				Name:         "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "System",
				Cluster:      "cluster1",
				Replicas:     1,
				MaxPods:      to.Int32Ptr(12),
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedControlPlaneScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			g.Expect(agentPool).To(Equal(c.Expected))
			agentPools, err := s.GetAllAgentPoolSpecs(context.TODO())
			g.Expect(err).To(Succeed())
			g.Expect(agentPools[0].MaxPods).To(Equal(c.Expected.MaxPods))
		})
	}
}

func TestManagedControlPlaneScope_OSDiskType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected azure.AgentPoolSpec
	}{
		{
			Name: "Without OsDiskType",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: azure.AgentPoolSpec{
				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With OsDiskType",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithOsDiskType("pool1", string(containerservice.OSDiskTypeEphemeral)),
				PatchTarget:      getAzureMachinePoolWithOsDiskType("pool1", string(containerservice.OSDiskTypeEphemeral)),
			},
			Expected: azure.AgentPoolSpec{
				Name:         "pool1",
				SKU:          "Standard_D2s_v3",
				Mode:         "User",
				Cluster:      "cluster1",
				Replicas:     1,
				OsDiskType:   to.StringPtr(string(containerservice.OSDiskTypeEphemeral)),
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedControlPlaneScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			g.Expect(agentPool).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_Taints(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected azure.AgentPoolSpec
	}{
		{
			Name: "Without taints",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: azure.AgentPoolSpec{

				Name:         "pool0",
				SKU:          "Standard_D2s_v3",
				Replicas:     1,
				Mode:         "System",
				Cluster:      "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With taints",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool: getMachinePool("pool1"),
				InfraMachinePool: getAzureMachinePoolWithTaints("pool1", infrav1.Taints{
					infrav1.Taint{
						Key:    "key1",
						Value:  "value1",
						Effect: "NoSchedule",
					},
				}),
				PatchTarget: getAzureMachinePoolWithTaints("pool1", infrav1.Taints{
					infrav1.Taint{
						Key:    "key1",
						Value:  "value1",
						Effect: "NoSchedule",
					},
				}),
			},
			Expected: azure.AgentPoolSpec{
				Name:         "pool1",
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
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedControlPlaneScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			agentPool := s.AgentPoolSpec()
			g.Expect(agentPool).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_AddonProfiles(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = capiv1exp.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected azure.ManagedClusterSpec
	}{
		{
			Name: "Without add-ons",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: azure.ManagedClusterSpec{
				Name:         "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
			},
		},
		{
			Name: "With add-ons",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
						AddonProfiles: []infrav1.AddonProfile{
							{Name: "addon1", Config: nil, Enabled: false},
							{Name: "addon2", Config: map[string]string{"k1": "v1", "k2": "v2"}, Enabled: true},
						},
					},
				},
				MachinePool:      getMachinePool("pool0"),
				InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
				PatchTarget:      getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
			},
			Expected: azure.ManagedClusterSpec{
				Name:         "cluster1",
				VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				AddonProfiles: []azure.AddonProfile{
					{Name: "addon1", Config: nil, Enabled: false},
					{Name: "addon2", Config: map[string]string{"k1": "v1", "k2": "v2"}, Enabled: true},
				},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.MachinePool, c.Input.InfraMachinePool, c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedControlPlaneScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			managedCluster, err := s.ManagedClusterSpec()
			g.Expect(err).To(Succeed())
			g.Expect(managedCluster).To(Equal(c.Expected))
		})
	}
}

func getAzureMachinePool(name string, mode infrav1.NodePoolMode) *infrav1.AzureManagedMachinePool {
	return &infrav1.AzureManagedMachinePool{
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
		Spec: infrav1.AzureManagedMachinePoolSpec{
			Mode: string(mode),
			SKU:  "Standard_D2s_v3",
			Name: to.StringPtr(name),
		},
	}
}

func getAzureMachinePoolWithScaling(name string, min, max int32) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.Scaling = &infrav1.ManagedMachinePoolScaling{
		MinSize: to.Int32Ptr(min),
		MaxSize: to.Int32Ptr(max),
	}
	return managedPool
}

func getAzureMachinePoolWithMaxPods(name string, maxPods int32) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeSystem)
	managedPool.Spec.MaxPods = to.Int32Ptr(maxPods)
	return managedPool
}

func getAzureMachinePoolWithTaints(name string, taints infrav1.Taints) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.Taints = taints
	return managedPool
}

func getAzureMachinePoolWithOsDiskType(name string, osDiskType string) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeUser)
	managedPool.Spec.OsDiskType = to.StringPtr(osDiskType)
	return managedPool
}

func getAzureMachinePoolWithLabels(name string, nodeLabels map[string]string) *infrav1.AzureManagedMachinePool {
	managedPool := getAzureMachinePool(name, infrav1.NodePoolModeSystem)
	managedPool.Spec.NodeLabels = nodeLabels
	return managedPool
}

func getMachinePool(name string) *capiv1exp.MachinePool {
	return &capiv1exp.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "cluster1",
			},
		},
		Spec: capiv1exp.MachinePoolSpec{
			ClusterName: "cluster1",
		},
	}
}

func getMachinePoolWithVersion(name, version string) *capiv1exp.MachinePool {
	machine := getMachinePool(name)
	machine.Spec.Template.Spec.Version = to.StringPtr(version)
	return machine
}

func TestManagedControlPlaneScope_IsVnetManagedCache(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected bool
	}{
		{
			Name: "no Cache value",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
						Version:        "v1.20.1",
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
			},
			Expected: false,
		},
		{
			Name: "with Cache value of true",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
						Version:        "v1.20.1",
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				Cache: &ManagedControlPlaneCache{
					isVnetManaged: to.BoolPtr(true),
				},
			},
			Expected: true,
		},
		{
			Name: "with Cache value of false",
			Input: ManagedControlPlaneScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
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
						Version:        "v1.20.1",
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
				Cache: &ManagedControlPlaneCache{
					isVnetManaged: to.BoolPtr(false),
				},
			},
			Expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(c.Input.ControlPlane).Build()
			c.Input.Client = fakeClient
			s, err := NewManagedControlPlaneScope(context.TODO(), c.Input)
			g.Expect(err).To(Succeed())
			isVnetManaged := s.IsVnetManaged()
			g.Expect(isVnetManaged).To(Equal(c.Expected))
		})
	}
}
