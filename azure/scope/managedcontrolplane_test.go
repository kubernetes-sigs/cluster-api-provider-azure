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
	"reflect"
	"testing"

	asokubernetesconfigurationv1 "github.com/Azure/azure-service-operator/v2/api/kubernetesconfiguration/v1api20230501"
	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aksextensions"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/managedclusters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/privateendpoints"
)

func TestNewManagedControlPlaneScope(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	input := ManagedControlPlaneScopeParams{
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster1",
				Namespace: "default",
			},
		},
		ControlPlane: &infrav1.AzureManagedControlPlane{
			Spec: infrav1.AzureManagedControlPlaneSpec{
				AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
					SubscriptionID: "00000000-0000-0000-0000-000000000000",
					IdentityRef: &corev1.ObjectReference{
						Name:      "fake-identity",
						Namespace: "default",
						Kind:      "AzureClusterIdentity",
					},
				},
			},
		},
		CredentialCache: azure.NewCredentialCache(),
	}
	fakeIdentity := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type:     infrav1.ServicePrincipal,
			ClientID: fakeClientID,
			TenantID: fakeTenantID,
		},
	}
	fakeSecret := &corev1.Secret{Data: map[string][]byte{"clientSecret": []byte("fooSecret")}}
	initObjects := []runtime.Object{input.ControlPlane, fakeIdentity, fakeSecret}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	input.Client = fakeClient
	_, err := NewManagedControlPlaneScope(context.TODO(), input)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestManagedControlPlaneScope_OutboundType(t *testing.T) {
	explicitOutboundType := infrav1.ManagedControlPlaneOutboundTypeUserDefinedRouting
	cases := []struct {
		Name     string
		Scope    *ManagedControlPlaneScope
		Expected bool
	}{
		{
			Name: "With Explicit OutboundType defined",
			Scope: &ManagedControlPlaneScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							OutboundType: &explicitOutboundType,
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "Without OutboundType defined",
			Scope: &ManagedControlPlaneScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{},
					},
				},
			},
			Expected: true,
		},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)

			managedCluster := c.Scope.ManagedClusterSpec()
			result := managedCluster.(*managedclusters.ManagedClusterSpec).OutboundType == nil
			g.Expect(result).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_PoolVersion(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedControlPlaneScope
		Expected []azure.ASOResourceSpecGetter[genruntime.MetaObject]
		Err      string
	}{
		{
			Name: "Without Version",
			Scope: &ManagedControlPlaneScope{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: []azure.ASOResourceSpecGetter[genruntime.MetaObject]{
				&agentpools.AgentPoolSpec{
					Name:         "pool0",
					AzureName:    "pool0",
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
			Scope: &ManagedControlPlaneScope{
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
							Version:        "v1.22.0",
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePoolWithVersion("pool0", "v1.21.1"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			Expected: []azure.ASOResourceSpecGetter[genruntime.MetaObject]{
				&agentpools.AgentPoolSpec{
					Name:         "pool0",
					AzureName:    "pool0",
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
			Scope: &ManagedControlPlaneScope{
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
							Version: "v1.20.1",
						},
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
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)

			agentPools, err := c.Scope.GetAllAgentPoolSpecs()
			if err != nil {
				g.Expect(err.Error()).To(Equal(c.Err))
			} else {
				g.Expect(agentPools).To(Equal(c.Expected))
			}
		})
	}
}

func TestManagedControlPlaneScope_AddonProfiles(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedControlPlaneScope
		Expected []managedclusters.AddonProfile
	}{
		{
			Name: "Without add-ons",
			Scope: &ManagedControlPlaneScope{
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
					Spec: infrav1.AzureManagedControlPlaneSpec{},
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
			Scope: &ManagedControlPlaneScope{
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
							AddonProfiles: []infrav1.AddonProfile{
								{Name: "addon1", Config: nil, Enabled: false},
								{Name: "addon2", Config: map[string]string{"k1": "v1", "k2": "v2"}, Enabled: true},
							},
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
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			managedCluster := c.Scope.ManagedClusterSpec()
			g.Expect(managedCluster.(*managedclusters.ManagedClusterSpec).AddonProfiles).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_OSType(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedControlPlaneScope
		Expected []azure.ASOResourceSpecGetter[genruntime.MetaObject]
		Err      string
	}{
		{
			Name: "with Linux and Windows pools",
			Scope: &ManagedControlPlaneScope{
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
							Version:        "v1.20.1",
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
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
			Expected: []azure.ASOResourceSpecGetter[genruntime.MetaObject]{
				&agentpools.AgentPoolSpec{
					Name:         "pool0",
					AzureName:    "pool0",
					SKU:          "Standard_D2s_v3",
					Mode:         "System",
					Replicas:     1,
					Cluster:      "cluster1",
					VnetSubnetID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups//providers/Microsoft.Network/virtualNetworks//subnets/",
				},
				&agentpools.AgentPoolSpec{
					Name:         "pool1",
					AzureName:    "pool1",
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
			Scope: &ManagedControlPlaneScope{
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
							Version:        "v1.20.1",
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
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
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			agentPools, err := c.Scope.GetAllAgentPoolSpecs()
			if err != nil {
				g.Expect(err.Error()).To(Equal(c.Err))
			} else {
				g.Expect(agentPools).To(Equal(c.Expected))
			}
		})
	}
}

func TestManagedControlPlaneScope_IsVnetManagedCache(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	cases := []struct {
		Name     string
		Scope    *ManagedControlPlaneScope
		Expected bool
	}{
		{
			Name: "no Cache value",
			Scope: &ManagedControlPlaneScope{
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
							Version:        "v1.20.1",
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
							IdentityRef: &corev1.ObjectReference{
								Name:      "fake-identity",
								Namespace: "default",
								Kind:      "AzureClusterIdentity",
							},
						},
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
				cache: &ManagedControlPlaneCache{
					isVnetManaged: nil,
				},
			},
			Expected: false,
		},
		{
			Name: "with Cache value of true",
			Scope: &ManagedControlPlaneScope{
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
							Version:        "v1.20.1",
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
							IdentityRef: &corev1.ObjectReference{
								Name:      "fake-identity",
								Namespace: "default",
								Kind:      "AzureClusterIdentity",
							},
						},
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
				cache: &ManagedControlPlaneCache{
					isVnetManaged: ptr.To(true),
				},
			},
			Expected: true,
		},
		{
			Name: "with Cache value of false",
			Scope: &ManagedControlPlaneScope{
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
							Version:        "v1.20.1",
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
							IdentityRef: &corev1.ObjectReference{
								Name:      "fake-identity",
								Namespace: "default",
								Kind:      "AzureClusterIdentity",
							},
						},
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
				cache: &ManagedControlPlaneCache{
					isVnetManaged: ptr.To(false),
				},
			},
			Expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			initObjects := []runtime.Object{c.Scope.ControlPlane}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

			c.Scope.Client = fakeClient
			isVnetManaged := c.Scope.IsVnetManaged()
			g.Expect(isVnetManaged).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_AADProfile(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedControlPlaneScope
		Expected *managedclusters.AADProfile
	}{
		{
			Name: "Without AADProfile",
			Scope: &ManagedControlPlaneScope{
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
					Spec: infrav1.AzureManagedControlPlaneSpec{},
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
			Scope: &ManagedControlPlaneScope{
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
							AADProfile: &infrav1.AADProfile{
								Managed:             true,
								AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
							},
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
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			managedClusterGetter := c.Scope.ManagedClusterSpec()
			managedCluster, ok := managedClusterGetter.(*managedclusters.ManagedClusterSpec)
			g.Expect(ok).To(BeTrue())
			g.Expect(managedCluster.AADProfile).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_DisableLocalAccounts(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedControlPlaneScope
		Expected *bool
	}{
		{
			Name: "Without DisableLocalAccounts",
			Scope: &ManagedControlPlaneScope{
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
					Spec: infrav1.AzureManagedControlPlaneSpec{},
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
			Scope: &ManagedControlPlaneScope{
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
							DisableLocalAccounts: ptr.To(true),
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
			Expected: nil,
		},
		{
			Name: "With AAdProfile and With DisableLocalAccounts",
			Scope: &ManagedControlPlaneScope{
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
							AADProfile: &infrav1.AADProfile{
								Managed:             true,
								AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
							},
							DisableLocalAccounts: ptr.To(true),
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
			Expected: ptr.To(true),
		},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			managedClusterGetter := c.Scope.ManagedClusterSpec()
			managedCluster, ok := managedClusterGetter.(*managedclusters.ManagedClusterSpec)
			g.Expect(ok).To(BeTrue())
			g.Expect(managedCluster.DisableLocalAccounts).To(Equal(c.Expected))
		})
	}
}

func TestIsAADEnabled(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedControlPlaneScope
		Expected bool
	}{
		{
			Name: "AAD is not enabled",
			Scope: &ManagedControlPlaneScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{},
				},
			},
			Expected: false,
		},
		{
			Name: "AAdProfile is enabled",
			Scope: &ManagedControlPlaneScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							AADProfile: &infrav1.AADProfile{
								Managed: true,
							},
						},
					},
				},
			},
			Expected: true,
		},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			aadEnabled := c.Scope.IsAADEnabled()
			g.Expect(aadEnabled).To(Equal(c.Expected))
		})
	}
}

func TestAreLocalAccountsDisabled(t *testing.T) {
	cases := []struct {
		Name     string
		Scope    *ManagedControlPlaneScope
		Expected bool
	}{
		{
			Name: "DisbaleLocalAccount is not enabled",
			Scope: &ManagedControlPlaneScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{},
				},
			},
			Expected: false,
		},
		{
			Name: "With AAdProfile and Without DisableLocalAccounts",
			Scope: &ManagedControlPlaneScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							AADProfile: &infrav1.AADProfile{
								Managed: true,
							},
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "With AAdProfile and With DisableLocalAccounts",
			Scope: &ManagedControlPlaneScope{
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							AADProfile: &infrav1.AADProfile{
								Managed: true,
							},
							DisableLocalAccounts: ptr.To(true),
						},
					},
				},
			},
			Expected: true,
		},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)
			localAccountsDisabled := c.Scope.AreLocalAccountsDisabled()
			g.Expect(localAccountsDisabled).To(Equal(c.Expected))
		})
	}
}

func TestManagedControlPlaneScope_PrivateEndpointSpecs(t *testing.T) {
	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected []azure.ASOResourceSpecGetter[*asonetworkv1.PrivateEndpoint]
		Err      string
	}{
		{
			Name: "returns empty private endpoints list if no subnets are specified",
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
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{},
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
			},
			Expected: make([]azure.ASOResourceSpecGetter[*asonetworkv1.PrivateEndpoint], 0),
		},
		{
			Name: "returns empty private endpoints list if no private endpoints are specified",
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
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000000",
							VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
								ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
									Subnet: infrav1.ManagedControlPlaneSubnet{
										PrivateEndpoints: infrav1.PrivateEndpoints{},
									},
								},
							},
						},
					},
				},
			},
			Expected: make([]azure.ASOResourceSpecGetter[*asonetworkv1.PrivateEndpoint], 0),
		},
		{
			Name: "returns list of private endpoint specs if private endpoints are specified",
			Input: ManagedControlPlaneScopeParams{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-cluster",
						Namespace: "dummy-ns",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-cluster",
						Namespace: "dummy-ns",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							SubscriptionID: "00000000-0000-0000-0000-000000000001",
							VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
								ResourceGroup: "dummy-rg",
								Name:          "vnet1",
								ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
									Subnet: infrav1.ManagedControlPlaneSubnet{
										Name: "subnet1",
										PrivateEndpoints: infrav1.PrivateEndpoints{
											{
												Name:     "my-private-endpoint",
												Location: "westus2",
												PrivateLinkServiceConnections: []infrav1.PrivateLinkServiceConnection{
													{
														Name:                 "my-pls-connection",
														PrivateLinkServiceID: "my-pls-id",
														GroupIDs: []string{
															"my-group-id-1",
														},
														RequestMessage: "my-request-message",
													},
												},
												CustomNetworkInterfaceName: "my-custom-nic",
												PrivateIPAddresses: []string{
													"IP1",
													"IP2",
												},
												ApplicationSecurityGroups: []string{
													"ASG1",
													"ASG2",
												},
												ManualApproval: true,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			Expected: []azure.ASOResourceSpecGetter[*asonetworkv1.PrivateEndpoint]{
				&privateendpoints.PrivateEndpointSpec{
					Name:                       "my-private-endpoint",
					ResourceGroup:              "dummy-rg",
					Location:                   "westus2",
					CustomNetworkInterfaceName: "my-custom-nic",
					PrivateIPAddresses: []string{
						"IP1",
						"IP2",
					},
					SubnetID: "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/dummy-rg/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/subnet1",
					ApplicationSecurityGroups: []string{
						"ASG1",
						"ASG2",
					},
					ClusterName: "my-cluster",
					PrivateLinkServiceConnections: []privateendpoints.PrivateLinkServiceConnection{
						{
							Name:                 "my-pls-connection",
							RequestMessage:       "my-request-message",
							PrivateLinkServiceID: "my-pls-id",
							GroupIDs: []string{
								"my-group-id-1",
							},
						},
					},
					ManualApproval: true,
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			s := &ManagedControlPlaneScope{
				ControlPlane: c.Input.ControlPlane,
				Cluster:      c.Input.Cluster,
			}
			if got := s.PrivateEndpointSpecs(); !reflect.DeepEqual(got, c.Expected) {
				t.Errorf("PrivateEndpointSpecs() = %s, want %s", specArrayToString(got), specArrayToString(c.Expected))
			}
		})
	}
}

func TestManagedControlPlaneScope_AKSExtensionSpecs(t *testing.T) {
	cases := []struct {
		Name     string
		Input    ManagedControlPlaneScopeParams
		Expected []azure.ASOResourceSpecGetter[*asokubernetesconfigurationv1.Extension]
		Err      string
	}{
		{
			Name: "returns empty AKS extensions list if no extensions are specified",
			Input: ManagedControlPlaneScopeParams{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "dummy-ns",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-cluster",
						Namespace: "dummy-ns",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{},
					},
				},
			},
		},
		{
			Name: "returns list of AKS extensions if extensions are specified",
			Input: ManagedControlPlaneScopeParams{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-cluster",
						Namespace: "dummy-ns",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-cluster",
						Namespace: "dummy-ns",
					},
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							Extensions: []infrav1.AKSExtension{
								{
									Name:                    "my-extension",
									AutoUpgradeMinorVersion: ptr.To(true),
									ConfigurationSettings: map[string]string{
										"my-key": "my-value",
									},
									ExtensionType: ptr.To("my-extension-type"),
									ReleaseTrain:  ptr.To("my-release-train"),
									Version:       ptr.To("my-version"),
									Plan: &infrav1.ExtensionPlan{
										Name:      "my-plan-name",
										Product:   "my-product",
										Publisher: "my-publisher",
									},
									AKSAssignedIdentityType: infrav1.AKSAssignedIdentitySystemAssigned,
									Identity:                infrav1.ExtensionIdentitySystemAssigned,
								},
							},
						},
					},
				},
			},
			Expected: []azure.ASOResourceSpecGetter[*asokubernetesconfigurationv1.Extension]{
				&aksextensions.AKSExtensionSpec{
					Name:                    "my-extension",
					Namespace:               "dummy-ns",
					AutoUpgradeMinorVersion: ptr.To(true),
					ConfigurationSettings: map[string]string{
						"my-key": "my-value",
					},
					ExtensionType: ptr.To("my-extension-type"),
					ReleaseTrain:  ptr.To("my-release-train"),
					Version:       ptr.To("my-version"),
					Owner:         "/subscriptions//resourceGroups//providers/Microsoft.ContainerService/managedClusters/my-cluster",
					Plan: &infrav1.ExtensionPlan{
						Name:      "my-plan-name",
						Product:   "my-product",
						Publisher: "my-publisher",
					},
					AKSAssignedIdentityType: infrav1.AKSAssignedIdentitySystemAssigned,
					ExtensionIdentity:       infrav1.ExtensionIdentitySystemAssigned,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			s := &ManagedControlPlaneScope{
				ControlPlane: c.Input.ControlPlane,
				Cluster:      c.Input.Cluster,
			}
			if got := s.AKSExtensionSpecs(); !reflect.DeepEqual(got, c.Expected) {
				t.Errorf("AKSExtensionSpecs() = %s, want %s", specArrayToString(got), specArrayToString(c.Expected))
			}
		})
	}
}

func TestManagedControlPlaneScope_AutoUpgradeProfile(t *testing.T) {
	cases := []struct {
		name     string
		input    ManagedControlPlaneScopeParams
		expected *managedclusters.ManagedClusterAutoUpgradeProfile
	}{
		{
			name: "Without AutoUpgradeProfile",
			input: ManagedControlPlaneScopeParams{
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
				ManagedMachinePools: []ManagedMachinePool{
					{
						MachinePool:      getMachinePool("pool0"),
						InfraMachinePool: getAzureMachinePool("pool0", infrav1.NodePoolModeSystem),
					},
				},
			},
			expected: nil,
		},
		{
			name: "With AutoUpgradeProfile UpgradeChannelNodeImage",
			input: ManagedControlPlaneScopeParams{
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
							AutoUpgradeProfile: &infrav1.ManagedClusterAutoUpgradeProfile{
								UpgradeChannel: ptr.To(infrav1.UpgradeChannelNodeImage),
							},
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
			expected: &managedclusters.ManagedClusterAutoUpgradeProfile{
				UpgradeChannel: ptr.To(infrav1.UpgradeChannelNodeImage),
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := NewWithT(t)
			s := &ManagedControlPlaneScope{
				ControlPlane: c.input.ControlPlane,
				Cluster:      c.input.Cluster,
			}
			managedClusterGetter := s.ManagedClusterSpec()
			managedCluster, ok := managedClusterGetter.(*managedclusters.ManagedClusterSpec)
			g.Expect(ok).To(BeTrue())
			g.Expect(managedCluster.AutoUpgradeProfile).To(Equal(c.expected))
		})
	}
}

func TestManagedControlPlaneScope_GroupSpecs(t *testing.T) {
	cases := []struct {
		name     string
		input    ManagedControlPlaneScopeParams
		expected []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]
	}{
		{
			name: "virtualNetwork belongs to a different resource group",
			input: ManagedControlPlaneScopeParams{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							ResourceGroupName: "dummy-rg",
							VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
								ResourceGroup: "different-rg",
							},
						},
					},
				},
			},
			expected: []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
				&groups.GroupSpec{
					Name:           "dummy-rg",
					AzureName:      "dummy-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
				&groups.GroupSpec{
					Name:           "different-rg",
					AzureName:      "different-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
		{
			name: "virtualNetwork belongs to a same resource group",
			input: ManagedControlPlaneScopeParams{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							ResourceGroupName: "dummy-rg",
							VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
								ResourceGroup: "dummy-rg",
							},
						},
					},
				},
			},
			expected: []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
				&groups.GroupSpec{
					Name:           "dummy-rg",
					AzureName:      "dummy-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
		{
			name: "virtualNetwork resource group not specified",
			input: ManagedControlPlaneScopeParams{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							ResourceGroupName: "dummy-rg",
							VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
								Name: "vnet1",
							},
						},
					},
				},
			},
			expected: []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
				&groups.GroupSpec{
					Name:           "dummy-rg",
					AzureName:      "dummy-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
		{
			name: "virtualNetwork belongs to different resource group with non-k8s name",
			input: ManagedControlPlaneScopeParams{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
				},
				ControlPlane: &infrav1.AzureManagedControlPlane{
					Spec: infrav1.AzureManagedControlPlaneSpec{
						AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
							ResourceGroupName: "dummy-rg",
							VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
								ResourceGroup: "my_custom_rg",
								Name:          "vnet1",
							},
						},
					},
				},
			},
			expected: []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
				&groups.GroupSpec{
					Name:           "dummy-rg",
					AzureName:      "dummy-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
				&groups.GroupSpec{
					Name:           "my-custom-rg",
					AzureName:      "my_custom_rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &ManagedControlPlaneScope{
				ControlPlane: c.input.ControlPlane,
				Cluster:      c.input.Cluster,
			}
			if got := s.GroupSpecs(); !reflect.DeepEqual(got, c.expected) {
				t.Errorf("GroupSpecs() = %s, want %s", specArrayToString(got), specArrayToString(c.expected))
			}
		})
	}
}
