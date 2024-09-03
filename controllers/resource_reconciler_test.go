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

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1alpha "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type FakeClient struct {
	client.Client
	// Override the Patch method because controller-runtime's doesn't really support
	// server-side apply, so we make our own dollar store version:
	// https://github.com/kubernetes-sigs/controller-runtime/issues/2341
	patchFunc func(context.Context, client.Object, client.Patch, ...client.PatchOption) error
}

func (c *FakeClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if c.patchFunc == nil {
		return c.Client.Patch(ctx, obj, patch, opts...)
	}
	return c.patchFunc(ctx, obj, patch, opts...)
}

type FakeWatcher struct {
	watching map[string]struct{}
}

func (w *FakeWatcher) Watch(_ logr.Logger, obj client.Object, _ handler.EventHandler, _ ...predicate.Predicate) error {
	if w.watching == nil {
		w.watching = make(map[string]struct{})
	}
	w.watching[obj.GetObjectKind().GroupVersionKind().GroupKind().String()] = struct{}{}
	return nil
}

func TestResourceReconcilerReconcile(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1alpha.AddToScheme,
		asoresourcesv1.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())

	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1alpha.AzureASOManagedCluster{})
	}

	t.Run("empty resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		r := &ResourceReconciler{
			resources: []*unstructured.Unstructured{},
			owner:     &infrav1alpha.AzureASOManagedCluster{},
		}

		g.Expect(r.Reconcile(ctx)).To(Succeed())
	})

	t.Run("reconcile several resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		w := &FakeWatcher{}
		c := fakeClientBuilder().
			Build()

		asoManagedCluster := &infrav1alpha.AzureASOManagedCluster{}

		unpatchedRGs := map[string]struct{}{
			"rg1": {},
			"rg2": {},
		}
		r := &ResourceReconciler{
			Client: &FakeClient{
				Client: c,
				patchFunc: func(ctx context.Context, o client.Object, p client.Patch, po ...client.PatchOption) error {
					g.Expect(unpatchedRGs).To(HaveKey(o.GetName()))
					delete(unpatchedRGs, o.GetName())
					return nil
				},
			},
			resources: []*unstructured.Unstructured{
				rgJSON(g, s, &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rg1",
					},
					// Status normally wouldn't be defined here. This simulates the server response after a PATCH.
					Status: asoresourcesv1.ResourceGroup_STATUS{
						Conditions: []conditions.Condition{
							{
								Type:   conditions.ConditionTypeReady,
								Status: metav1.ConditionTrue,
							},
						},
					},
				}),
				rgJSON(g, s, &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rg2",
					},
				}),
			},
			owner:   asoManagedCluster,
			watcher: w,
		}

		g.Expect(r.Reconcile(ctx)).To(Succeed())
		g.Expect(w.watching).To(HaveKey("ResourceGroup.resources.azure.com"))
		g.Expect(unpatchedRGs).To(BeEmpty()) // all expected resources were patched

		resourcesStatuses := asoManagedCluster.Status.Resources
		g.Expect(resourcesStatuses).To(HaveLen(2))
		g.Expect(resourcesStatuses[0].Resource.Name).To(Equal("rg1"))
		g.Expect(resourcesStatuses[0].Ready).To(BeTrue())
		g.Expect(resourcesStatuses[1].Resource.Name).To(Equal("rg2"))
		g.Expect(resourcesStatuses[1].Ready).To(BeFalse())
	})

	t.Run("delete stale resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		owner := &infrav1alpha.AzureASOManagedCluster{
			Status: infrav1alpha.AzureASOManagedClusterStatus{
				Resources: []infrav1alpha.ResourceStatus{
					rgStatus("rg0"),
					rgStatus("rg1"),
					rgStatus("rg2"),
					rgStatus("rg3"),
				},
			},
		}

		objs := []client.Object{
			&asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rg0",
					Namespace: owner.Namespace,
				},
			},
			&asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rg1",
					Namespace: owner.Namespace,
				},
			},
			&asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rg2",
					Namespace: owner.Namespace,
				},
			},
			&asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "rg3",
					Namespace:  owner.Namespace,
					Finalizers: []string{"still deleting"},
				},
			},
		}

		c := fakeClientBuilder().
			WithObjects(objs...).
			Build()

		r := &ResourceReconciler{
			Client: &FakeClient{
				Client: c,
				patchFunc: func(ctx context.Context, o client.Object, p client.Patch, po ...client.PatchOption) error {
					return nil
				},
			},
			resources: []*unstructured.Unstructured{
				rgJSON(g, s, &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rg1",
					},
				}),
				rgJSON(g, s, &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rg2",
					},
				}),
			},
			owner:   owner,
			watcher: &FakeWatcher{},
		}

		g.Expect(r.Reconcile(ctx)).To(Succeed())

		resourcesStatuses := owner.Status.Resources
		g.Expect(resourcesStatuses).To(HaveLen(3))
		// rg0 should be deleted and gone
		g.Expect(resourcesStatuses[0].Resource.Name).To(Equal("rg1"))
		g.Expect(resourcesStatuses[1].Resource.Name).To(Equal("rg2"))
		g.Expect(resourcesStatuses[2].Resource.Name).To(Equal("rg3"))
	})
}

func TestResourceReconcilerPause(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1alpha.AddToScheme,
		asoresourcesv1.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())

	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1alpha.AzureASOManagedCluster{})
	}

	t.Run("empty resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		r := &ResourceReconciler{
			resources: []*unstructured.Unstructured{},
			owner:     &infrav1alpha.AzureASOManagedCluster{},
		}

		g.Expect(r.Pause(ctx)).To(Succeed())
	})

	t.Run("pause several resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		c := fakeClientBuilder().
			Build()

		asoManagedCluster := &infrav1alpha.AzureASOManagedCluster{}

		var patchedRGs []string
		r := &ResourceReconciler{
			Client: &FakeClient{
				Client: c,
				patchFunc: func(ctx context.Context, o client.Object, p client.Patch, po ...client.PatchOption) error {
					g.Expect(o.GetAnnotations()).To(HaveKeyWithValue(annotations.ReconcilePolicy, string(annotations.ReconcilePolicySkip)))
					patchedRGs = append(patchedRGs, o.GetName())
					return nil
				},
			},
			resources: []*unstructured.Unstructured{
				rgJSON(g, s, &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rg1",
					},
				}),
				rgJSON(g, s, &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rg2",
					},
				}),
			},
			owner: asoManagedCluster,
		}

		g.Expect(r.Pause(ctx)).To(Succeed())
		g.Expect(patchedRGs).To(ConsistOf("rg1", "rg2"))
	})
}

func TestResourceReconcilerDelete(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1alpha.AddToScheme,
		asoresourcesv1.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())

	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1alpha.AzureASOManagedCluster{})
	}

	t.Run("empty resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		r := &ResourceReconciler{
			resources: []*unstructured.Unstructured{},
			owner:     &infrav1alpha.AzureASOManagedCluster{},
		}

		g.Expect(r.Delete(ctx)).To(Succeed())
	})

	t.Run("delete several resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		owner := &infrav1alpha.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
			},
			Status: infrav1alpha.AzureASOManagedClusterStatus{
				Resources: []infrav1alpha.ResourceStatus{
					rgStatus("still-deleting"),
					rgStatus("already-gone"),
				},
			},
		}

		objs := []client.Object{
			&asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "still-deleting",
					Namespace: owner.Namespace,
					Finalizers: []string{
						"ASO finalizer",
					},
				},
			},
		}

		c := fakeClientBuilder().
			WithObjects(objs...).
			Build()

		r := &ResourceReconciler{
			Client: &FakeClient{
				Client: c,
			},
			owner: owner,
		}

		g.Expect(r.Delete(ctx)).To(Succeed())
		g.Expect(apierrors.IsNotFound(r.Client.Get(ctx, client.ObjectKey{Namespace: owner.Namespace, Name: "already-gone"}, &asoresourcesv1.ResourceGroup{}))).To(BeTrue())
		stillDeleting := &asoresourcesv1.ResourceGroup{}
		g.Expect(r.Client.Get(ctx, client.ObjectKey{Namespace: owner.Namespace, Name: "still-deleting"}, stillDeleting)).To(Succeed())
		g.Expect(stillDeleting.GetDeletionTimestamp().IsZero()).To(BeFalse())

		g.Expect(owner.Status.Resources).To(HaveLen(1))
		g.Expect(owner.Status.Resources[0].Resource.Name).To(Equal("still-deleting"))
		g.Expect(owner.Status.Resources[0].Ready).To(BeFalse())
	})
}

func TestReadyStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("unstructured", func(t *testing.T) {
		tests := []struct {
			name          string
			object        *unstructured.Unstructured
			expectedReady bool
		}{
			{
				name:          "empty object",
				object:        &unstructured.Unstructured{Object: make(map[string]interface{})},
				expectedReady: false,
			},
			{
				name: "empty status.conditions",
				object: &unstructured.Unstructured{Object: map[string]interface{}{
					"status": map[string]interface{}{
						"conditions": []interface{}{},
					},
				}},
				expectedReady: false,
			},
			{
				name: "status.conditions wrong type",
				object: &unstructured.Unstructured{Object: map[string]interface{}{
					"status": map[string]interface{}{
						"conditions": []interface{}{
							int64(0),
						},
					},
				}},
				expectedReady: false,
			},
			{
				name: "non-Ready type status.conditions",
				object: &unstructured.Unstructured{Object: map[string]interface{}{
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type": "not" + conditions.ConditionTypeReady,
							},
						},
					},
				}},
				expectedReady: false,
			},
			{
				name: "observedGeneration not up to date",
				object: &unstructured.Unstructured{Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":               conditions.ConditionTypeReady,
								"observedGeneration": int64(0),
							},
						},
					},
				}},
				expectedReady: false,
			},
			{
				name: "status is not defined",
				object: &unstructured.Unstructured{Object: map[string]interface{}{
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":    conditions.ConditionTypeReady,
								"message": "a message",
							},
						},
					},
				}},
				expectedReady: false,
			},
			{
				name: "status is not True",
				object: &unstructured.Unstructured{Object: map[string]interface{}{
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":    conditions.ConditionTypeReady,
								"status":  "not-" + string(metav1.ConditionTrue),
								"message": "a message",
							},
						},
					},
				}},
				expectedReady: false,
			},
			{
				name: "status is True",
				object: &unstructured.Unstructured{Object: map[string]interface{}{
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":   "not-" + conditions.ConditionTypeReady,
								"status": "not-" + string(metav1.ConditionTrue),
							},
							map[string]interface{}{
								"type":   conditions.ConditionTypeReady,
								"status": string(metav1.ConditionTrue),
							},
							map[string]interface{}{
								"type":   "not-" + conditions.ConditionTypeReady,
								"status": "not-" + string(metav1.ConditionTrue),
							},
						},
					},
				}},
				expectedReady: true,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				g := NewGomegaWithT(t)

				ready, err := readyStatus(ctx, test.object)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(Equal(test.expectedReady))
			})
		}
	})

	// These tests verify readyStatus() on an actual ASO typed object to ensure the unstructured assertions
	// work on the actual structure of ASO objects.
	t.Run("ResourceGroup", func(t *testing.T) {
		tests := []struct {
			name          string
			conditions    conditions.Conditions
			expectedReady bool
		}{
			{
				name:          "empty conditions",
				conditions:    nil,
				expectedReady: false,
			},
			{
				name: "not ready conditions",
				conditions: conditions.Conditions{
					{
						Type:    conditions.ConditionTypeReady,
						Status:  metav1.ConditionFalse,
						Message: "a message",
					},
					{
						Type:    "not-" + conditions.ConditionTypeReady,
						Status:  metav1.ConditionTrue,
						Message: "another message",
					},
				},
				expectedReady: false,
			},
			{
				name: "ready conditions",
				conditions: conditions.Conditions{
					{
						Type:    "not-" + conditions.ConditionTypeReady,
						Status:  metav1.ConditionTrue,
						Message: "another message",
					},
					{
						Type:    conditions.ConditionTypeReady,
						Status:  metav1.ConditionTrue,
						Message: "a message",
					},
					{
						Type:    "not-" + conditions.ConditionTypeReady,
						Status:  metav1.ConditionTrue,
						Message: "another message",
					},
				},
				expectedReady: true,
			},
		}

		s := runtime.NewScheme()
		NewGomegaWithT(t).Expect(asoresourcesv1.AddToScheme(s)).To(Succeed())

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				g := NewGomegaWithT(t)

				rg := &asoresourcesv1.ResourceGroup{
					Status: asoresourcesv1.ResourceGroup_STATUS{
						Conditions: test.conditions,
					},
				}
				u := &unstructured.Unstructured{}
				g.Expect(s.Convert(rg, u, nil)).To(Succeed())

				ready, err := readyStatus(ctx, u)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(Equal(test.expectedReady))
			})
		}
	})
}

func rgJSON(g Gomega, scheme *runtime.Scheme, rg *asoresourcesv1.ResourceGroup) *unstructured.Unstructured {
	rg.SetGroupVersionKind(asoresourcesv1.GroupVersion.WithKind("ResourceGroup"))
	u := &unstructured.Unstructured{}
	g.Expect(scheme.Convert(rg, u, nil)).To(Succeed())
	return u
}

func rgStatus(name string) infrav1alpha.ResourceStatus {
	return infrav1alpha.ResourceStatus{
		Resource: infrav1alpha.StatusResource{
			Group:   asoresourcesv1.GroupVersion.Group,
			Version: asoresourcesv1.GroupVersion.Version,
			Kind:    "ResourceGroup",
			Name:    name,
		},
	}
}
