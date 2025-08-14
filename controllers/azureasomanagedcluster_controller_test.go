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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

type fakeResourceReconciler struct {
	owner         client.Object
	reconcileFunc func(context.Context, client.Object) error
	pauseFunc     func(context.Context, client.Object) error
	deleteFunc    func(context.Context, client.Object) error
}

func (r *fakeResourceReconciler) Reconcile(ctx context.Context) error {
	if r.reconcileFunc == nil {
		return nil
	}
	return r.reconcileFunc(ctx, r.owner)
}

func (r *fakeResourceReconciler) Pause(ctx context.Context) error {
	if r.pauseFunc == nil {
		return nil
	}
	return r.pauseFunc(ctx, r.owner)
}

func (r *fakeResourceReconciler) Delete(ctx context.Context) error {
	if r.deleteFunc == nil {
		return nil
	}
	return r.deleteFunc(ctx, r.owner)
}

func TestAzureASOManagedClusterReconcile(t *testing.T) {
	ctx := t.Context()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1.AddToScheme,
		clusterv1.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())

	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1.AzureASOManagedCluster{})
	}

	t.Run("AzureASOManagedCluster does not exist", func(t *testing.T) {
		g := NewGomegaWithT(t)

		c := fakeClientBuilder().
			Build()
		r := &AzureASOManagedClusterReconciler{
			Client: c,
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "doesn't", Name: "exist"}})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))
	})

	t.Run("Cluster does not exist", func(t *testing.T) {
		g := NewGomegaWithT(t)

		asoManagedCluster := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amc",
				Namespace: "ns",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       "cluster",
					},
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedCluster).
			Build()
		r := &AzureASOManagedClusterReconciler{
			Client: c,
		}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedCluster)})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("adds a finalizer and block-move annotation", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				ControlPlaneRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1somethingelse",
					Kind:       infrav1.AzureASOManagedControlPlaneKind,
				},
			},
		}
		asoManagedCluster := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amc",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedCluster).
			Build()
		r := &AzureASOManagedClusterReconciler{
			Client: c,
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedCluster)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{Requeue: true}))

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedCluster), asoManagedCluster)).To(Succeed())
		g.Expect(asoManagedCluster.GetFinalizers()).To(ContainElement(clusterv1.ClusterFinalizer))
		g.Expect(asoManagedCluster.GetAnnotations()).To(HaveKey(clusterctlv1.BlockMoveAnnotation))
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
					APIVersion: infrav1.GroupVersion.Identifier(),
					Kind:       infrav1.AzureASOManagedControlPlaneKind,
				},
			},
		}
		asoManagedCluster := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amc",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
				Finalizers: []string{
					clusterv1.ClusterFinalizer,
				},
				Annotations: map[string]string{
					clusterctlv1.BlockMoveAnnotation: "true",
				},
			},
			Status: infrav1.AzureASOManagedClusterStatus{
				Ready: true,
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedCluster).
			Build()
		r := &AzureASOManagedClusterReconciler{
			Client: c,
			newResourceReconciler: func(asoManagedCluster *infrav1.AzureASOManagedCluster, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					owner: asoManagedCluster,
					reconcileFunc: func(ctx context.Context, o client.Object) error {
						asoManagedCluster.SetResourceStatuses([]infrav1.ResourceStatus{
							{Ready: true},
							{Ready: false},
							{Ready: true},
						})
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedCluster)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		g.Expect(r.Get(ctx, client.ObjectKeyFromObject(asoManagedCluster), asoManagedCluster)).To(Succeed())
		g.Expect(asoManagedCluster.Status.Ready).To(BeFalse())
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
					APIVersion: infrav1.GroupVersion.Identifier(),
					Kind:       infrav1.AzureASOManagedControlPlaneKind,
					Name:       "amcp",
					Namespace:  "ns",
				},
			},
		}
		asoManagedCluster := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amc",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
				Finalizers: []string{
					clusterv1.ClusterFinalizer,
				},
				Annotations: map[string]string{
					clusterctlv1.BlockMoveAnnotation: "true",
				},
			},
			Status: infrav1.AzureASOManagedClusterStatus{
				Ready: false,
			},
		}
		asoManagedControlPlane := &infrav1.AzureASOManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amcp",
				Namespace: cluster.Namespace,
			},
			Status: infrav1.AzureASOManagedControlPlaneStatus{
				ControlPlaneEndpoint: clusterv1.APIEndpoint{Host: "endpoint"},
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedCluster, asoManagedControlPlane).
			Build()
		r := &AzureASOManagedClusterReconciler{
			Client: c,
			newResourceReconciler: func(_ *infrav1.AzureASOManagedCluster, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					reconcileFunc: func(ctx context.Context, o client.Object) error {
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedCluster)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedCluster), asoManagedCluster)).To(Succeed())
		g.Expect(asoManagedCluster.Spec.ControlPlaneEndpoint.Host).To(Equal("endpoint"))
		g.Expect(asoManagedCluster.Status.Ready).To(BeTrue())
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
			},
		}
		asoManagedCluster := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amc",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
				Annotations: map[string]string{
					clusterctlv1.BlockMoveAnnotation: "true",
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedCluster).
			Build()
		r := &AzureASOManagedClusterReconciler{
			Client: c,
			newResourceReconciler: func(_ *infrav1.AzureASOManagedCluster, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					pauseFunc: func(_ context.Context, _ client.Object) error {
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedCluster)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedCluster), asoManagedCluster)).To(Succeed())
		g.Expect(asoManagedCluster.GetAnnotations()).NotTo(HaveKey(clusterctlv1.BlockMoveAnnotation))
	})

	t.Run("successfully reconciles in-progress delete", func(t *testing.T) {
		g := NewGomegaWithT(t)

		asoManagedCluster := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amc",
				Namespace: "ns",
				Finalizers: []string{
					clusterv1.ClusterFinalizer,
				},
				DeletionTimestamp: &metav1.Time{Time: time.Date(1, 0, 0, 0, 0, 0, 0, time.UTC)},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedCluster).
			Build()
		r := &AzureASOManagedClusterReconciler{
			Client: c,
			newResourceReconciler: func(asoManagedCluster *infrav1.AzureASOManagedCluster, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					owner: asoManagedCluster,
					deleteFunc: func(ctx context.Context, o client.Object) error {
						asoManagedCluster.SetResourceStatuses([]infrav1.ResourceStatus{
							{
								Resource: infrav1.StatusResource{
									Name: "still-deleting",
								},
							},
						})
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedCluster)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		err = c.Get(ctx, client.ObjectKeyFromObject(asoManagedCluster), asoManagedCluster)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(asoManagedCluster.GetFinalizers()).To(ContainElement(clusterv1.ClusterFinalizer))
	})

	t.Run("successfully reconciles finished delete", func(t *testing.T) {
		g := NewGomegaWithT(t)

		asoManagedCluster := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amc",
				Namespace: "ns",
				Finalizers: []string{
					clusterv1.ClusterFinalizer,
				},
				DeletionTimestamp: &metav1.Time{Time: time.Date(1, 0, 0, 0, 0, 0, 0, time.UTC)},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedCluster).
			Build()
		r := &AzureASOManagedClusterReconciler{
			Client: c,
			newResourceReconciler: func(_ *infrav1.AzureASOManagedCluster, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					deleteFunc: func(ctx context.Context, o client.Object) error {
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedCluster)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		err = c.Get(ctx, client.ObjectKeyFromObject(asoManagedCluster), asoManagedCluster)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
}
