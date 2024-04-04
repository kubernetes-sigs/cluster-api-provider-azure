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
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha1"
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

func (w *FakeWatcher) Watch(_ logr.Logger, obj runtime.Object, _ handler.EventHandler, _ ...predicate.Predicate) error {
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
		infrav1exp.AddToScheme,
		asoresourcesv1.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())

	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1exp.AzureASOManagedCluster{})
	}

	t.Run("empty resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		r := &ResourceReconciler{
			resources: []*unstructured.Unstructured{},
			owner:     &infrav1exp.AzureASOManagedCluster{},
		}

		g.Expect(r.Reconcile(ctx)).To(Succeed())
	})

	t.Run("reconcile several resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		w := &FakeWatcher{}
		c := fakeClientBuilder().
			Build()

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
				}),
				rgJSON(g, s, &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rg2",
					},
				}),
			},
			owner:   &infrav1exp.AzureASOManagedCluster{},
			watcher: w,
		}

		g.Expect(r.Reconcile(ctx)).To(Succeed())
		g.Expect(w.watching).To(HaveKey("ResourceGroup.resources.azure.com"))
		g.Expect(unpatchedRGs).To(BeEmpty()) // all expected resources were patched
	})
}

func TestResourceReconcilerDelete(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1exp.AddToScheme,
		asoresourcesv1.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())

	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1exp.AzureASOManagedCluster{})
	}

	t.Run("empty resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		r := &ResourceReconciler{
			resources: []*unstructured.Unstructured{},
			owner:     &infrav1exp.AzureASOManagedCluster{},
		}

		g.Expect(r.Delete(ctx)).To(Succeed())
	})

	t.Run("delete several resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		owner := &infrav1exp.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
			},
		}

		objs := []client.Object{
			&asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rg1",
					Namespace: owner.Namespace,
				},
			},
			// simulating rg2 already having been deleted
		}

		c := fakeClientBuilder().
			WithObjects(objs...).
			Build()

		r := &ResourceReconciler{
			Client: &FakeClient{
				Client: c,
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
			owner: owner,
		}

		g.Expect(r.Delete(ctx)).To(Succeed())
		g.Expect(apierrors.IsNotFound(r.Client.Get(ctx, client.ObjectKey{Namespace: owner.Namespace, Name: "rg1"}, &asoresourcesv1.ResourceGroup{}))).To(BeTrue())
		g.Expect(apierrors.IsNotFound(r.Client.Get(ctx, client.ObjectKey{Namespace: owner.Namespace, Name: "rg2"}, &asoresourcesv1.ResourceGroup{}))).To(BeTrue())
	})
}

func rgJSON(g Gomega, scheme *runtime.Scheme, rg *asoresourcesv1.ResourceGroup) *unstructured.Unstructured {
	rg.SetGroupVersionKind(asoresourcesv1.GroupVersion.WithKind("ResourceGroup"))
	u := &unstructured.Unstructured{}
	g.Expect(scheme.Convert(rg, u, nil)).To(Succeed())
	return u
}
