/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools/mock_agentpools"
	gomock2 "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAzureManagedMachinePoolReconcile(t *testing.T) {
	os.Setenv(auth.ClientID, "fooClient")
	os.Setenv(auth.ClientSecret, "fooSecret")
	os.Setenv(auth.TenantID, "fooTenant")
	os.Setenv(auth.SubscriptionID, "fooSubscription")

	cases := []struct {
		name   string
		Setup  func(cb *fake.ClientBuilder, reconciler *mock_azure.MockReconcilerMockRecorder, agentpools *mock_agentpools.MockAgentPoolScopeMockRecorder, nodelister *MockNodeListerMockRecorder)
		Verify func(g *WithT, result ctrl.Result, err error)
	}{
		{
			name: "Reconcile succeed",
			Setup: func(cb *fake.ClientBuilder, reconciler *mock_azure.MockReconcilerMockRecorder, agentpools *mock_agentpools.MockAgentPoolScopeMockRecorder, nodelister *MockNodeListerMockRecorder) {
				cluster, azManagedCluster, azManagedControlPlane, ammp, mp := newReadyAzureManagedMachinePoolCluster()
				fakeAgentPoolSpec := fakeAgentPool()
				providerIDs := []string{"azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myresourcegroupname/providers/Microsoft.Compute/virtualMachineScaleSets/myScaleSetName/virtualMachines/156"}
				fakeVirtualMachineScaleSet := fakeVirtualMachineScaleSet()
				fakeVirtualMachineScaleSetVM := fakeVirtualMachineScaleSetVM()

				reconciler.Reconcile(gomock2.AContext()).Return(nil)
				agentpools.SetSubnetName()
				agentpools.AgentPoolSpec().Return(&fakeAgentPoolSpec)
				agentpools.NodeResourceGroup().Return("fake-rg")
				agentpools.SetAgentPoolProviderIDList(providerIDs)
				agentpools.SetAgentPoolReplicas(int32(len(providerIDs))).Return()
				agentpools.SetAgentPoolReady(true).Return()

				nodelister.List(gomock2.AContext(), "fake-rg").Return(fakeVirtualMachineScaleSet, nil)
				nodelister.ListInstances(gomock2.AContext(), "fake-rg", "vmssName").Return(fakeVirtualMachineScaleSetVM, nil)

				cb.WithObjects(cluster, azManagedCluster, azManagedControlPlane, ammp, mp)
			},
			Verify: func(g *WithT, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			name: "Reconcile delete",
			Setup: func(cb *fake.ClientBuilder, reconciler *mock_azure.MockReconcilerMockRecorder, _ *mock_agentpools.MockAgentPoolScopeMockRecorder, _ *MockNodeListerMockRecorder) {
				cluster, azManagedCluster, azManagedControlPlane, ammp, mp := newReadyAzureManagedMachinePoolCluster()
				reconciler.Delete(gomock2.AContext()).Return(nil)
				ammp.DeletionTimestamp = &metav1.Time{
					Time: time.Now(),
				}
				cb.WithObjects(cluster, azManagedCluster, azManagedControlPlane, ammp, mp)
			},
			Verify: func(g *WithT, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var (
				g          = NewWithT(t)
				mockCtrl   = gomock.NewController(t)
				reconciler = mock_azure.NewMockReconciler(mockCtrl)
				agentpools = mock_agentpools.NewMockAgentPoolScope(mockCtrl)
				nodelister = NewMockNodeLister(mockCtrl)
				scheme     = func() *runtime.Scheme {
					s := runtime.NewScheme()
					for _, addTo := range []func(s *runtime.Scheme) error{
						scheme.AddToScheme,
						clusterv1.AddToScheme,
						expv1.AddToScheme,
						infrav1.AddToScheme,
					} {
						g.Expect(addTo(s)).To(Succeed())
					}

					return s
				}()
				cb = fake.NewClientBuilder().WithScheme(scheme)
			)
			defer mockCtrl.Finish()

			c.Setup(cb, reconciler.EXPECT(), agentpools.EXPECT(), nodelister.EXPECT())
			controller := NewAzureManagedMachinePoolReconciler(cb.Build(), nil, 30*time.Second, "foo")
			controller.createAzureManagedMachinePoolService = func(_ *scope.ManagedMachinePoolScope) (*azureManagedMachinePoolService, error) {
				return &azureManagedMachinePoolService{
					scope:         agentpools,
					agentPoolsSvc: reconciler,
					scaleSetsSvc:  nodelister,
				}, nil
			}
			res, err := controller.Reconcile(context.TODO(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "foo-ammp",
					Namespace: "foobar",
				},
			})
			c.Verify(g, res, err)
		})
	}
}

func newReadyAzureManagedMachinePoolCluster() (*clusterv1.Cluster, *infrav1.AzureManagedCluster, *infrav1.AzureManagedControlPlane, *infrav1.AzureManagedMachinePool, *expv1.MachinePool) {
	// AzureManagedCluster
	azManagedCluster := &infrav1.AzureManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-azManagedCluster",
			Namespace: "foobar",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "foo-cluster",
					Kind:       "Cluster",
					APIVersion: "cluster.x-k8s.io/v1beta1",
				},
			},
		},
		Spec: infrav1.AzureManagedClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: "foo.bar",
				Port: 123,
			},
		},
	}
	// AzureManagedControlPlane
	azManagedControlPlane := &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-azManagedControlPlane",
			Namespace: "foobar",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "foo-cluster",
					Kind:       "Cluster",
					APIVersion: "cluster.x-k8s.io/v1beta1",
				},
			},
		},
		Spec: infrav1.AzureManagedControlPlaneSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: "foo.bar",
				Port: 123,
			},
		},
		Status: infrav1.AzureManagedControlPlaneStatus{
			Ready:       true,
			Initialized: true,
		},
	}
	// Cluster
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-cluster",
			Namespace: "foobar",
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Kind:       "AzureManagedControlPlane",
				Name:       azManagedControlPlane.Name,
				Namespace:  azManagedControlPlane.Namespace,
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Kind:       "AzureManagedCluster",
				Name:       azManagedCluster.Name,
				Namespace:  azManagedCluster.Namespace,
			},
		},
	}
	// AzureManagedMachinePool
	ammp := &infrav1.AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-ammp",
			Namespace: "foobar",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "foo-mp1",
					Kind:       "MachinePool",
					APIVersion: "cluster.x-k8s.io/v1beta1",
				},
			},
		},
	}
	// MachinePool
	mp := &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-mp1",
			Namespace: "foobar",
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": cluster.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "Cluster",
					Name:       "foo-cluster",
				},
			},
		},
		Spec: expv1.MachinePoolSpec{
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName: cluster.Name,
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
						Kind:       "AzureManagedMachinePool",
						Name:       ammp.Name,
						Namespace:  ammp.Namespace,
					},
				},
			},
		},
	}

	return cluster, azManagedCluster, azManagedControlPlane, ammp, mp
}

func fakeAgentPool(changes ...func(*agentpools.AgentPoolSpec)) agentpools.AgentPoolSpec {
	pool := agentpools.AgentPoolSpec{
		Name:              "fake-agent-pool-name",
		ResourceGroup:     "fake-rg",
		Cluster:           "fake-cluster",
		AvailabilityZones: []string{"fake-zone"},
		EnableAutoScaling: true,
		EnableUltraSSD:    pointer.Bool(true),
		KubeletDiskType:   (*infrav1.KubeletDiskType)(pointer.String("fake-kubelet-disk-type")),
		MaxCount:          pointer.Int32(5),
		MaxPods:           pointer.Int32(10),
		MinCount:          pointer.Int32(1),
		Mode:              "fake-mode",
		NodeLabels:        map[string]*string{"fake-label": pointer.String("fake-value")},
		NodeTaints:        []string{"fake-taint"},
		OSDiskSizeGB:      2,
		OsDiskType:        pointer.String("fake-os-disk-type"),
		OSType:            pointer.String("fake-os-type"),
		Replicas:          1,
		SKU:               "fake-sku",
		Version:           pointer.String("fake-version"),
		VnetSubnetID:      "fake-vnet-subnet-id",
		Headers:           map[string]string{"fake-header": "fake-value"},
		AdditionalTags:    infrav1.Tags{"fake": "tag"},
	}

	for _, change := range changes {
		change(&pool)
	}

	return pool
}

func fakeVirtualMachineScaleSetVM() []compute.VirtualMachineScaleSetVM {
	virtualMachineScaleSetVM := []compute.VirtualMachineScaleSetVM{
		{
			InstanceID: pointer.String("0"),
			ID:         pointer.String("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroupName/providers/Microsoft.Compute/virtualMachineScaleSets/myScaleSetName/virtualMachines/156"),
			Name:       pointer.String("vm0"),
			Zones:      &[]string{"zone0"},
			VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: pointer.String(string(compute.ProvisioningState1Succeeded)),
				OsProfile: &compute.OSProfile{
					ComputerName: pointer.String("instance-000000"),
				},
			},
		},
	}
	return virtualMachineScaleSetVM
}

func fakeVirtualMachineScaleSet() []compute.VirtualMachineScaleSet {
	tags := map[string]*string{
		"foo":      pointer.String("bazz"),
		"poolName": pointer.String("fake-agent-pool-name"),
	}
	zones := []string{"zone0", "zone1"}
	virtualMachineScaleSet := []compute.VirtualMachineScaleSet{
		{
			Sku: &compute.Sku{
				Name:     pointer.String("skuName"),
				Tier:     pointer.String("skuTier"),
				Capacity: pointer.Int64(2),
			},
			Zones:    &zones,
			ID:       pointer.String("vmssID"),
			Name:     pointer.String("vmssName"),
			Location: pointer.String("westus2"),
			Tags:     tags,
			VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
				SinglePlacementGroup: pointer.Bool(false),
				ProvisioningState:    pointer.String(string(compute.ProvisioningState1Succeeded)),
			},
		},
	}
	return virtualMachineScaleSet
}
