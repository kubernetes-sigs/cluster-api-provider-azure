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
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAzureASOManagedMachinePoolReconcile(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1exp.AddToScheme,
		clusterv1.AddToScheme,
		expv1.AddToScheme,
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
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedMachinePool)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))
	})
}
