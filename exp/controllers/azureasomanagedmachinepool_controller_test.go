/*
Copyright 2024 The Kubernetes Authors.

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
	"encoding/json"
	"testing"
	"time"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type FakeClusterTracker struct {
	getClientFunc func(context.Context, types.NamespacedName) (client.Client, error)
}

func (c *FakeClusterTracker) GetClient(ctx context.Context, name types.NamespacedName) (client.Client, error) {
	if c.getClientFunc == nil {
		return nil, nil
	}
	return c.getClientFunc(ctx, name)
}

func TestAzureASOManagedMachinePoolReconcile(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1exp.AddToScheme,
		clusterv1.AddToScheme,
		expv1.AddToScheme,
		asocontainerservicev1.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())
	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1exp.AzureASOManagedMachinePool{})
	}

	t.Run("AzureASOManagedMachinePool does not exist", func(t *testing.T) {
		g := NewGomegaWithT(t)

		c := fakeClientBuilder().
			Build()
		r := &AzureASOManagedMachinePoolReconciler{
			Client: c,
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "doesn't", Name: "exist"}})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))
	})

	t.Run("MachinePool does not exist", func(t *testing.T) {
		g := NewGomegaWithT(t)

		asoManagedMachinePool := &infrav1exp.AzureASOManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ammp",
				Namespace: "ns",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: expv1.GroupVersion.Identifier(),
						Kind:       "MachinePool",
						Name:       "mp",
					},
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedMachinePool).
			Build()
		r := &AzureASOManagedMachinePoolReconciler{
			Client: c,
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedMachinePool)})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("machinepools.cluster.x-k8s.io \"mp\" not found"))
		g.Expect(result).To(Equal(ctrl.Result{}))
	})

	t.Run("Cluster does not exist", func(t *testing.T) {
		g := NewGomegaWithT(t)

		asoManagedMachinePool := &infrav1exp.AzureASOManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ammp",
				Namespace: "ns",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: expv1.GroupVersion.Identifier(),
						Kind:       "MachinePool",
						Name:       "mp",
					},
				},
			},
		}
		machinePool := &expv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mp",
				Namespace: asoManagedMachinePool.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: "cluster",
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedMachinePool, machinePool).
			Build()
		r := &AzureASOManagedMachinePoolReconciler{
			Client: c,
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedMachinePool)})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("clusters.cluster.x-k8s.io \"cluster\" not found"))
		g.Expect(result).To(Equal(ctrl.Result{}))
	})

	t.Run("adds a finalizer", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneRef: &corev1.ObjectReference{
					APIVersion: infrav1exp.GroupVersion.Identifier(),
					Kind:       infrav1exp.AzureASOManagedControlPlaneKind,
				},
			},
		}
		asoManagedMachinePool := &infrav1exp.AzureASOManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ammp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: expv1.GroupVersion.Identifier(),
						Kind:       "MachinePool",
						Name:       "mp",
					},
				},
			},
		}
		machinePool := &expv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mp",
				Namespace: cluster.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: "cluster",
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedMachinePool, machinePool, cluster).
			Build()
		r := &AzureASOManagedMachinePoolReconciler{
			Client: c,
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedMachinePool)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{Requeue: true}))

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedMachinePool), asoManagedMachinePool)).To(Succeed())
		g.Expect(asoManagedMachinePool.GetFinalizers()).To(ContainElement(clusterv1.ClusterFinalizer))
	})

	t.Run("reconciles resources that are not ready", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneRef: &corev1.ObjectReference{
					APIVersion: infrav1exp.GroupVersion.Identifier(),
					Kind:       infrav1exp.AzureASOManagedControlPlaneKind,
				},
			},
		}
		asoManagedMachinePool := &infrav1exp.AzureASOManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ammp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: expv1.GroupVersion.Identifier(),
						Kind:       "MachinePool",
						Name:       "mp",
					},
				},
				Finalizers: []string{
					clusterv1.ClusterFinalizer,
				},
			},
			Spec: infrav1exp.AzureASOManagedMachinePoolSpec{
				AzureASOManagedMachinePoolTemplateResourceSpec: infrav1exp.AzureASOManagedMachinePoolTemplateResourceSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: apJSON(g, &asocontainerservicev1.ManagedClustersAgentPool{
								ObjectMeta: metav1.ObjectMeta{
									Name: "ap",
								},
							}),
						},
					},
				},
			},
		}
		machinePool := &expv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mp",
				Namespace: cluster.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: "cluster",
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedMachinePool, machinePool, cluster).
			Build()
		r := &AzureASOManagedMachinePoolReconciler{
			Client: c,
			newResourceReconciler: func(asoManagedMachinePool *infrav1exp.AzureASOManagedMachinePool, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					owner: asoManagedMachinePool,
					reconcileFunc: func(ctx context.Context, o client.Object) error {
						asoManagedMachinePool.SetResourceStatuses([]infrav1exp.ResourceStatus{
							{Ready: true},
							{Ready: false},
							{Ready: true},
						})
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedMachinePool)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))
	})

	t.Run("successfully reconciles normally", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneRef: &corev1.ObjectReference{
					APIVersion: infrav1exp.GroupVersion.Identifier(),
					Kind:       infrav1exp.AzureASOManagedControlPlaneKind,
				},
			},
		}
		asoManagedCluster := &asocontainerservicev1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mc",
				Namespace: cluster.Namespace,
			},
			Status: asocontainerservicev1.ManagedCluster_STATUS{
				NodeResourceGroup: ptr.To("MC_rg"),
			},
		}
		asoAgentPool := &asocontainerservicev1.ManagedClustersAgentPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ap",
				Namespace: cluster.Namespace,
			},
			Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
				AzureName: "pool1",
				Owner: &genruntime.KnownResourceReference{
					Name: asoManagedCluster.Name,
				},
			},
			Status: asocontainerservicev1.ManagedClusters_AgentPool_STATUS{
				Count: ptr.To(3),
			},
		}
		asoManagedMachinePool := &infrav1exp.AzureASOManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ammp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: expv1.GroupVersion.Identifier(),
						Kind:       "MachinePool",
						Name:       "mp",
					},
				},
				Finalizers: []string{
					clusterv1.ClusterFinalizer,
				},
			},
			Spec: infrav1exp.AzureASOManagedMachinePoolSpec{
				AzureASOManagedMachinePoolTemplateResourceSpec: infrav1exp.AzureASOManagedMachinePoolTemplateResourceSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: apJSON(g, asoAgentPool),
						},
					},
				},
			},
		}
		machinePool := &expv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mp",
				Namespace: cluster.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: "cluster",
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedMachinePool, machinePool, cluster, asoAgentPool, asoManagedCluster).
			Build()
		r := &AzureASOManagedMachinePoolReconciler{
			Client: c,
			newResourceReconciler: func(_ *infrav1exp.AzureASOManagedMachinePool, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					reconcileFunc: func(ctx context.Context, o client.Object) error {
						return nil
					},
				}
			},
			Tracker: &FakeClusterTracker{
				getClientFunc: func(_ context.Context, _ types.NamespacedName) (client.Client, error) {
					return fakeclient.NewClientBuilder().
						WithObjects(
							&corev1.Node{
								ObjectMeta: metav1.ObjectMeta{
									Name:   "node1",
									Labels: expectedNodeLabels(asoAgentPool.AzureName(), *asoManagedCluster.Status.NodeResourceGroup),
								},
								Spec: corev1.NodeSpec{
									ProviderID: "azure://node1",
								},
							},
							&corev1.Node{
								ObjectMeta: metav1.ObjectMeta{
									Name:   "node2",
									Labels: expectedNodeLabels(asoAgentPool.AzureName(), *asoManagedCluster.Status.NodeResourceGroup),
								},
								Spec: corev1.NodeSpec{
									ProviderID: "azure://node2",
								},
							},
							&corev1.Node{
								ObjectMeta: metav1.ObjectMeta{
									Name: "no-labels",
								},
								Spec: corev1.NodeSpec{
									ProviderID: "azure://node3",
								},
							},
						).
						Build(), nil
				},
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedMachinePool)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		g.Expect(r.Get(ctx, client.ObjectKeyFromObject(asoManagedMachinePool), asoManagedMachinePool)).To(Succeed())
		g.Expect(asoManagedMachinePool.Spec.ProviderIDList).To(ConsistOf("azure://node1", "azure://node2"))
		g.Expect(asoManagedMachinePool.Status.Replicas).To(Equal(int32(3)))
	})

	t.Run("successfully reconciles pause", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				Paused: true,
				ControlPlaneRef: &corev1.ObjectReference{
					APIVersion: infrav1exp.GroupVersion.Identifier(),
					Kind:       infrav1exp.AzureASOManagedControlPlaneKind,
				},
			},
		}
		asoManagedMachinePool := &infrav1exp.AzureASOManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ammp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: expv1.GroupVersion.Identifier(),
						Kind:       "MachinePool",
						Name:       "mp",
					},
				},
			},
		}
		machinePool := &expv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mp",
				Namespace: cluster.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: "cluster",
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedMachinePool, machinePool, cluster).
			Build()
		r := &AzureASOManagedMachinePoolReconciler{
			Client: c,
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedMachinePool)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))
	})

	t.Run("successfully reconciles delete", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneRef: &corev1.ObjectReference{
					APIVersion: infrav1exp.GroupVersion.Identifier(),
					Kind:       infrav1exp.AzureASOManagedControlPlaneKind,
				},
			},
		}
		asoManagedMachinePool := &infrav1exp.AzureASOManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ammp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: expv1.GroupVersion.Identifier(),
						Kind:       "MachinePool",
						Name:       "mp",
					},
				},
				DeletionTimestamp: &metav1.Time{Time: time.Date(1, 0, 0, 0, 0, 0, 0, time.UTC)},
				Finalizers: []string{
					clusterv1.ClusterFinalizer,
				},
			},
		}
		machinePool := &expv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mp",
				Namespace: cluster.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: "cluster",
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedMachinePool, machinePool, cluster).
			Build()
		r := &AzureASOManagedMachinePoolReconciler{
			Client: c,
			newResourceReconciler: func(_ *infrav1exp.AzureASOManagedMachinePool, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					deleteFunc: func(ctx context.Context, o client.Object) error {
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedMachinePool)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))
	})
}

func apJSON(g Gomega, ap *asocontainerservicev1.ManagedClustersAgentPool) []byte {
	ap.SetGroupVersionKind(asocontainerservicev1.GroupVersion.WithKind("ManagedClustersAgentPool"))
	j, err := json.Marshal(ap)
	g.Expect(err).NotTo(HaveOccurred())
	return j
}
