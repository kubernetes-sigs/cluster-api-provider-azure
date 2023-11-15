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

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20230201"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/managedclusters"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManagedControlPlaneScope_OutboundType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	explicitOutboundType := infrav1.ManagedControlPlaneOutboundTypeUserDefinedRouting
	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected bool
	}{
		{
			Name: "With Explicit OutboundType defined",
			Input: ManagedControlPlaneScopeParams{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
						OutboundType:   &explicitOutboundType,
					},
				},
			},
			Expected: false,
		},
		{
			Name: "Without OutboundType defined",
			Input: ManagedControlPlaneScopeParams{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						SubscriptionID: "00000000-0000-0000-0000-000000000000",
					},
				},
			},
			Expected: true,
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
			managedCluster := s.ManagedClusterSpec()
			result := managedCluster.(*managedclusters.ManagedClusterSpec).OutboundType == nil
			g.Expect(result).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_PoolVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected []azure.ASOResourceSpecGetter[*asocontainerservicev1.ManagedClustersAgentPool]
		Err      string
	}{
		{
			Name: "Without Version",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: []azure.ASOResourceSpecGetter[*asocontainerservicev1.ManagedClustersAgentPool]{
				&agentpools.AgentPoolSpec{
					Name:         "pool0",
					AzureName:    "pool0",
					Namespace:    "default",
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePoolWithVersion("pool0", "v1.21.1"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: []azure.ASOResourceSpecGetter[*asocontainerservicev1.ManagedClustersAgentPool]{
				&agentpools.AgentPoolSpec{
					Name:         "pool0",
					AzureName:    "pool0",
					Namespace:    "default",
					SKU:          "Standard_D2s_v3",
					Mode:         "System",
					Replicas:     1,
					Version:      ptr.To("1.21.1"),
					Cluster:      "cluster1",
					VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				},
			},
		},
		{
			Name: "With bad version",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePoolWithVersion("pool0", "v1.21.1"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Err: "MachinePool version cannot be greater than the AzureManagedControlPlane version",
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
			agentPools, err := s.GetAllAgentPoolSpecs()
			if err != nil {
				g.Expect(err.Error()).To(Equal(c.Err))
			} else {
				g.Expect(agentPools).To(Equal(c.Expected))
			}
		})
	}
}

func TestManagedControlPlaneScope_AddonProfiles(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected []managedclusters.AddonProfile
	}{
		{
			Name: "Without add-ons",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: nil,
		},
		{
			Name: "With add-ons",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: []managedclusters.AddonProfile{
				{Name: "addon1", Config: nil, Enabled: false},
				{Name: "addon2", Config: map[string]string{"k1": "v1", "k2": "v2"}, Enabled: true},
			},
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
			managedCluster := s.ManagedClusterSpec()
			g.Expect(managedCluster.(*managedclusters.ManagedClusterSpec).AddonProfiles).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_OSType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected []azure.ASOResourceSpecGetter[*asocontainerservicev1.ManagedClustersAgentPool]
		Err      string
	}{
		{
			Name: "with Linux and Windows pools",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
					{
						MachinePool:      getMachinePool("pool1"),
						InfraMachinePool: getLinuxAzureMachinePool("pool1"),
					},
					{
						MachinePool:      getMachinePool("pool2"),
						InfraMachinePool: getWindowsAzureMachinePool("pool2"),
					},
				},
			},
			Expected: []azure.ASOResourceSpecGetter[*asocontainerservicev1.ManagedClustersAgentPool]{
				&agentpools.AgentPoolSpec{
					Name:         "pool0",
					AzureName:    "pool0",
					Namespace:    "default",
					SKU:          "Standard_D2s_v3",
					Mode:         "System",
					Replicas:     1,
					Cluster:      "cluster1",
					VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				},
				&agentpools.AgentPoolSpec{
					Name:         "pool1",
					AzureName:    "pool1",
					Namespace:    "default",
					SKU:          "Standard_D2s_v3",
					Mode:         "User",
					Replicas:     1,
					Cluster:      "cluster1",
					VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
					OSType:       ptr.To(azure.LinuxOS),
				},
				&agentpools.AgentPoolSpec{
					Name:         "pool2",
					AzureName:    "pool2",
					Namespace:    "default",
					SKU:          "Standard_D2s_v3",
					Mode:         "User",
					Replicas:     1,
					Cluster:      "cluster1",
					VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
					OSType:       ptr.To(azure.WindowsOS),
				},
			},
		},
		{
			Name: "system pool required",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getLinuxAzureMachinePool("pool0"),
					},
					{
						MachinePool:      getMachinePool("pool1"),
						InfraMachinePool: getWindowsAzureMachinePool("pool1"),
					},
				},
			},
			Err: "failed to fetch azuremanagedMachine pool with mode:System, require at least 1 system node pool",
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
			agentPools, err := s.GetAllAgentPoolSpecs()
			if err != nil {
				g.Expect(err.Error()).To(Equal(c.Err))
			} else {
				g.Expect(agentPools).To(Equal(c.Expected))
			}
		})
	}
}

type fakeVnetDescriber struct{}

func (f fakeVnetDescriber) IsManaged(ctx context.Context) (bool, error) {
	return false, nil
}

func TestManagedControlPlaneScope_IsVnetManagedCache(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected bool
	}{
		{
			Name: "no Cache value",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
					{
						MachinePool:      getMachinePool("pool1"),
						InfraMachinePool: getLinuxAzureMachinePool("pool1"),
					},
				},
				VnetDescriber: fakeVnetDescriber{},
			},
			Expected: false,
		},
		{
			Name: "with Cache value of true",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
					{
						MachinePool:      getMachinePool("pool1"),
						InfraMachinePool: getLinuxAzureMachinePool("pool1"),
					},
				},
				Cache: &ManagedControlPlaneCache{
					isVnetManaged: ptr.To(true),
				},
			},
			Expected: true,
		},
		{
			Name: "with Cache value of false",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
					{
						MachinePool:      getMachinePool("pool1"),
						InfraMachinePool: getLinuxAzureMachinePool("pool1"),
					},
				},
				Cache: &ManagedControlPlaneCache{
					isVnetManaged: ptr.To(false),
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

func TestManagedControlPlaneScope_AADProfile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected *managedclusters.AADProfile
	}{
		{
			Name: "Without AADProfile",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: nil,
		},
		{
			Name: "With AADProfile",
			Input: ManagedControlPlaneScopeParams{
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
						AADProfile: &infrav1.AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
					},
				},
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: &managedclusters.AADProfile{
				Managed:             true,
				EnableAzureRBAC:     true,
				AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
			},
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
			managedClusterGetter := s.ManagedClusterSpec()
			managedCluster, ok := managedClusterGetter.(*managedclusters.ManagedClusterSpec)
			g.Expect(ok).To(BeTrue())
			g.Expect(managedCluster.AADProfile).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_DisableLocalAccounts(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected *bool
	}{
		{
			Name: "Without DisableLocalAccounts",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: nil,
		},
		{
			Name: "Without AAdProfile and With DisableLocalAccounts",
			Input: ManagedControlPlaneScopeParams{
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
						SubscriptionID:       "00000000-0000-0000-0000-000000000000",
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: nil,
		},
		{
			Name: "With AAdProfile and With DisableLocalAccounts",
			Input: ManagedControlPlaneScopeParams{
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
						AADProfile: &infrav1.AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: ptr.To[bool](true),
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
			managedClusterGetter := s.ManagedClusterSpec()
			managedCluster, ok := managedClusterGetter.(*managedclusters.ManagedClusterSpec)
			g.Expect(ok).To(BeTrue())
			g.Expect(managedCluster.DisableLocalAccounts).To(Equal(c.Expected))
		})
	}
}

func TestIsAADEnabled(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected bool
	}{
		{
			Name: "AAD is not enabled",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: false,
		},
		{
			Name: "AAdProfile and With DisableLocalAccounts",
			Input: ManagedControlPlaneScopeParams{
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
						AADProfile: &infrav1.AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: true,
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
			aadEnabled := s.IsAADEnabled()
			g.Expect(aadEnabled).To(Equal(c.Expected))
		})
	}
}

func TestAreLocalAccountsDisabled(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected bool
	}{
		{
			Name: "DisbaleLocalAccount is not enabled",
			Input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: false,
		},
		{
			Name: "With AAdProfile and Without DisableLocalAccounts",
			Input: ManagedControlPlaneScopeParams{
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
						AADProfile: &infrav1.AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
					},
				},
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: false,
		},
		{
			Name: "With AAdProfile and With DisableLocalAccounts",
			Input: ManagedControlPlaneScopeParams{
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
						AADProfile: &infrav1.AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: true,
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
			localAccountsDisabled := s.AreLocalAccountsDisabled()
			g.Expect(localAccountsDisabled).To(Equal(c.Expected))
		})
	}
}
