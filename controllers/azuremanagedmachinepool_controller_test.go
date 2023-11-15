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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
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
	type pausingReconciler struct {
		*mock_azure.MockReconciler
		*mock_azure.MockPauser
	}

	os.Setenv(auth.ClientID, "fooClient")
	os.Setenv(auth.ClientSecret, "fooSecret")
	os.Setenv(auth.TenantID, "fooTenant")
	os.Setenv(auth.SubscriptionID, "fooSubscription")

	cases := []struct {
		name   string
		Setup  func(cb *fake.ClientBuilder, reconciler pausingReconciler, agentpools *mock_agentpools.MockAgentPoolScopeMockRecorder, nodelister *MockNodeListerMockRecorder)
		Verify func(g *WithT, result ctrl.Result, err error)
	}{
		{
			name: "Reconcile succeed",
			Setup: func(cb *fake.ClientBuilder, reconciler pausingReconciler, agentpools *mock_agentpools.MockAgentPoolScopeMockRecorder, nodelister *MockNodeListerMockRecorder) {
				cluster, azManagedCluster, azManagedControlPlane, ammp, mp := newReadyAzureManagedMachinePoolCluster()
				fakeAgentPoolSpec := fakeAgentPool()
				providerIDs := []string{"azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myresourcegroupname/providers/Microsoft.Compute/virtualMachineScaleSets/myScaleSetName/virtualMachines/156"}
				fakeVirtualMachineScaleSet := fakeVirtualMachineScaleSet()
				fakeVirtualMachineScaleSetVM := fakeVirtualMachineScaleSetVM()

				reconciler.MockReconciler.EXPECT().Reconcile(gomock2.AContext()).Return(nil)
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
			name: "Reconcile pause",
			Setup: func(cb *fake.ClientBuilder, reconciler pausingReconciler, agentpools *mock_agentpools.MockAgentPoolScopeMockRecorder, nodelister *MockNodeListerMockRecorder) {
				cluster, azManagedCluster, azManagedControlPlane, ammp, mp := newReadyAzureManagedMachinePoolCluster()
				cluster.Spec.Paused = true

				reconciler.MockPauser.EXPECT().Pause(gomock2.AContext()).Return(nil)

				cb.WithObjects(cluster, azManagedCluster, azManagedControlPlane, ammp, mp)
			},
			Verify: func(g *WithT, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			name: "Reconcile delete",
			Setup: func(cb *fake.ClientBuilder, reconciler pausingReconciler, _ *mock_agentpools.MockAgentPoolScopeMockRecorder, _ *MockNodeListerMockRecorder) {
				cluster, azManagedCluster, azManagedControlPlane, ammp, mp := newReadyAzureManagedMachinePoolCluster()
				reconciler.MockReconciler.EXPECT().Delete(gomock2.AContext()).Return(nil)
				ammp.DeletionTimestamp = &metav1.Time{
					Time: time.Now(),
				}
				cb.WithObjects(cluster, azManagedCluster, azManagedControlPlane, ammp, mp)
			},
			Verify: func(g *WithT, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			name: "Reconcile delete transient error",
			Setup: func(cb *fake.ClientBuilder, reconciler pausingReconciler, agentpools *mock_agentpools.MockAgentPoolScopeMockRecorder, _ *MockNodeListerMockRecorder) {
				cluster, azManagedCluster, azManagedControlPlane, ammp, mp := newReadyAzureManagedMachinePoolCluster()
				reconciler.MockReconciler.EXPECT().Delete(gomock2.AContext()).Return(azure.WithTransientError(errors.New("transient"), 76*time.Second))
				agentpools.Name()
				ammp.DeletionTimestamp = &metav1.Time{
					Time: time.Now(),
				}
				cb.WithObjects(cluster, azManagedCluster, azManagedControlPlane, ammp, mp)
			},
			Verify: func(g *WithT, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result.RequeueAfter).To(Equal(76 * time.Second))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var (
				g          = NewWithT(t)
				mockCtrl   = gomock.NewController(t)
				reconciler = pausingReconciler{
					MockReconciler: mock_azure.NewMockReconciler(mockCtrl),
					MockPauser:     mock_azure.NewMockPauser(mockCtrl),
				}
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
				cb = fake.NewClientBuilder().
					WithStatusSubresource(
						&infrav1.AzureManagedMachinePool{},
					).
					WithScheme(scheme)
			)
			defer mockCtrl.Finish()

			c.Setup(cb, reconciler, agentpools.EXPECT(), nodelister.EXPECT())
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
			Name:       "foo-ammp",
			Namespace:  "foobar",
			Finalizers: []string{"test"},
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
		AzureName:         "fake-agent-pool-name",
		ResourceGroup:     "fake-rg",
		Cluster:           "fake-cluster",
		AvailabilityZones: []string{"fake-zone"},
		EnableAutoScaling: true,
		EnableUltraSSD:    ptr.To(true),
		KubeletDiskType:   (*infrav1.KubeletDiskType)(ptr.To("fake-kubelet-disk-type")),
		MaxCount:          ptr.To(5),
		MaxPods:           ptr.To(10),
		MinCount:          ptr.To(1),
		Mode:              "fake-mode",
		NodeLabels:        map[string]string{"fake-label": "fake-value"},
		NodeTaints:        []string{"fake-taint"},
		OSDiskSizeGB:      2,
		OsDiskType:        ptr.To("fake-os-disk-type"),
		OSType:            ptr.To("fake-os-type"),
		Replicas:          1,
		SKU:               "fake-sku",
		Version:           ptr.To("fake-version"),
		VnetSubnetID:      "fake-vnet-subnet-id",
		AdditionalTags:    infrav1.Tags{"fake": "tag"},
	}

	for _, change := range changes {
		change(&pool)
	}

	return pool
}

func fakeVirtualMachineScaleSetVM() []armcompute.VirtualMachineScaleSetVM {
	virtualMachineScaleSetVM := []armcompute.VirtualMachineScaleSetVM{
		{
			InstanceID: ptr.To("0"),
			ID:         ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroupName/providers/Microsoft.Compute/virtualMachineScaleSets/myScaleSetName/virtualMachines/156"),
			Name:       ptr.To("vm0"),
			Zones:      []*string{ptr.To("zone0")},
			Properties: &armcompute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: ptr.To("Succeeded"),
				OSProfile: &armcompute.OSProfile{
					ComputerName: ptr.To("instance-000000"),
				},
			},
		},
	}
	return virtualMachineScaleSetVM
}

func fakeVirtualMachineScaleSet() []armcompute.VirtualMachineScaleSet {
	tags := map[string]*string{
		"foo":      ptr.To("bazz"),
		"poolName": ptr.To("fake-agent-pool-name"),
	}
	zones := []string{"zone0", "zone1"}
	virtualMachineScaleSet := []armcompute.VirtualMachineScaleSet{
		{
			SKU: &armcompute.SKU{
				Name:     ptr.To("skuName"),
				Tier:     ptr.To("skuTier"),
				Capacity: ptr.To[int64](2),
			},
			Zones:    azure.PtrSlice(&zones),
			ID:       ptr.To("vmssID"),
			Name:     ptr.To("vmssName"),
			Location: ptr.To("westus2"),
			Tags:     tags,
			Properties: &armcompute.VirtualMachineScaleSetProperties{
				SinglePlacementGroup: ptr.To(false),
				ProvisioningState:    ptr.To("Succeeded"),
			},
		},
	}
	return virtualMachineScaleSet
}
