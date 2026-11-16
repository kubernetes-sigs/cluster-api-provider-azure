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

	asonetworkv120201101 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	asonetworkv120220701 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asoannotations "github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

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
	ctx := t.Context()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1.AddToScheme,
		asoresourcesv1.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())

	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1.AzureASOManagedCluster{})
	}

	t.Run("empty resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		r := &ResourceReconciler{
			resources: []*unstructured.Unstructured{},
			owner:     &infrav1.AzureASOManagedCluster{},
		}
		err := r.Reconcile(ctx)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("acknowledge new types", func(t *testing.T) {
		g := NewGomegaWithT(t)

		w := &FakeWatcher{}

		asoManagedCluster := &infrav1.AzureASOManagedCluster{}

		r := &ResourceReconciler{
			Client: fakeClientBuilder().Build(),
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
			owner:   asoManagedCluster,
			watcher: w,
		}

		err := r.Reconcile(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(w.watching).To(BeEmpty())
		g.Expect(asoManagedCluster.Annotations).To(HaveKeyWithValue(ownedKindsAnnotation, getOwnedKindsValue([]schema.GroupVersionKind{asoresourcesv1.GroupVersion.WithKind("ResourceGroup")})))

		resourcesStatuses := asoManagedCluster.Status.Resources
		g.Expect(resourcesStatuses).To(HaveLen(2))
		g.Expect(resourcesStatuses[0].Resource.Name).To(Equal("rg1"))
		g.Expect(resourcesStatuses[0].Ready).To(BeFalse())
		g.Expect(resourcesStatuses[1].Resource.Name).To(Equal("rg2"))
		g.Expect(resourcesStatuses[1].Ready).To(BeFalse())

		resourceGroups := new(asoresourcesv1.ResourceGroupList)
		g.Expect(r.List(ctx, resourceGroups)).To(Succeed())
		g.Expect(resourceGroups.Items).To(BeEmpty(), "Resources should not have been created")
	})

	t.Run("create resources with acknowledged types", func(t *testing.T) {
		g := NewGomegaWithT(t)

		asoManagedCluster := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					ownedKindsAnnotation: getOwnedKindsValue([]schema.GroupVersionKind{asoresourcesv1.GroupVersion.WithKind("ResourceGroup")}),
				},
			},
		}

		w := &FakeWatcher{}

		r := &ResourceReconciler{
			Client: fakeClientBuilder().Build(),
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
					Status: asoresourcesv1.ResourceGroup_STATUS{
						Conditions: []conditions.Condition{
							{
								Type:   conditions.ConditionTypeReady,
								Status: metav1.ConditionFalse,
							},
						},
					},
				}),
			},
			owner:   asoManagedCluster,
			watcher: w,
		}

		err := r.Reconcile(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(w.watching).To(HaveKey("ResourceGroup.resources.azure.com"))
		g.Expect(asoManagedCluster.Annotations).To(HaveKeyWithValue(ownedKindsAnnotation, getOwnedKindsValue([]schema.GroupVersionKind{asoresourcesv1.GroupVersion.WithKind("ResourceGroup")})))

		resourcesStatuses := asoManagedCluster.Status.Resources
		g.Expect(resourcesStatuses).To(HaveLen(2))
		g.Expect(resourcesStatuses[0].Resource.Name).To(Equal("rg1"))
		g.Expect(resourcesStatuses[0].Ready).To(BeTrue())
		g.Expect(resourcesStatuses[1].Resource.Name).To(Equal("rg2"))
		g.Expect(resourcesStatuses[1].Ready).To(BeFalse())

		resourceGroups := new(asoresourcesv1.ResourceGroupList)
		g.Expect(r.List(ctx, resourceGroups)).To(Succeed())
		g.Expect(resourceGroups.Items).To(ConsistOf(
			HaveField("Name", "rg1"),
			HaveField("Name", "rg2"),
		), "Expected ResourceGroups should have been created")
	})

	t.Run("delete stale resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		owner := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Annotations: map[string]string{
					ownedKindsAnnotation: getOwnedKindsValue([]schema.GroupVersionKind{asoresourcesv1.GroupVersion.WithKind("ResourceGroup")}),
				},
			},
		}
		ownerGVK, err := apiutil.GVKForObject(owner, s)
		g.Expect(err).NotTo(HaveOccurred())
		controlledByOwner := []metav1.OwnerReference{
			*metav1.NewControllerRef(owner, ownerGVK),
		}

		objs := []client.Object{
			&asoresourcesv1.ResourceGroup{
				TypeMeta: rgTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:            "rg0",
					Namespace:       owner.Namespace,
					OwnerReferences: controlledBy(g, s, owner),
				},
			},
			&asoresourcesv1.ResourceGroup{
				TypeMeta: rgTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:            "rg1",
					Namespace:       owner.Namespace,
					OwnerReferences: controlledBy(g, s, owner),
				},
			},
			&asoresourcesv1.ResourceGroup{
				TypeMeta: rgTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:            "rg2",
					Namespace:       owner.Namespace,
					OwnerReferences: controlledBy(g, s, owner),
				},
			},
			&asoresourcesv1.ResourceGroup{
				TypeMeta: rgTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:            "rg3",
					Namespace:       owner.Namespace,
					OwnerReferences: controlledByOwner,
					Finalizers:      []string{"still deleting"},
				},
			},
		}

		r := &ResourceReconciler{
			Client: fakeClientBuilder().
				WithObjects(objs...).
				Build(),
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

		err = r.Reconcile(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(owner.Annotations).To(HaveKeyWithValue(ownedKindsAnnotation, getOwnedKindsValue([]schema.GroupVersionKind{asoresourcesv1.GroupVersion.WithKind("ResourceGroup")})))

		resourcesStatuses := owner.Status.Resources
		g.Expect(resourcesStatuses).To(HaveLen(3))
		// rg0 should be deleted and gone
		g.Expect(resourcesStatuses[0].Resource.Name).To(Equal("rg1"))
		g.Expect(resourcesStatuses[1].Resource.Name).To(Equal("rg2"))
		g.Expect(resourcesStatuses[2].Resource.Name).To(Equal("rg3"))

		err = r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "rg0"}, &asoresourcesv1.ResourceGroup{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "err is not a NotFound error")

		err = r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "rg1"}, &asoresourcesv1.ResourceGroup{})
		g.Expect(err).NotTo(HaveOccurred())

		err = r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "rg2"}, &asoresourcesv1.ResourceGroup{})
		g.Expect(err).NotTo(HaveOccurred())

		rg3 := &asoresourcesv1.ResourceGroup{}
		err = r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "rg3"}, rg3)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(rg3.GetDeletionTimestamp().IsZero()).To(BeFalse(), "resource does not have expected deletion timestamp")
	})
}

func TestResourceReconcilerPause(t *testing.T) {
	ctx := t.Context()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1.AddToScheme,
		asoresourcesv1.AddToScheme,
		asonetworkv120220701.AddToScheme,
		asonetworkv120201101.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())

	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1.AzureASOManagedCluster{})
	}

	t.Run("empty resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		r := &ResourceReconciler{
			resources: []*unstructured.Unstructured{},
			owner:     &infrav1.AzureASOManagedCluster{},
		}

		g.Expect(r.Pause(ctx)).To(Succeed())
	})

	t.Run("pause several resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		owner := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Annotations: map[string]string{
					ownedKindsAnnotation: getOwnedKindsValue([]schema.GroupVersionKind{asoresourcesv1.GroupVersion.WithKind("ResourceGroup")}),
				},
			},
		}

		objs := []client.Object{
			&asoresourcesv1.ResourceGroup{
				TypeMeta: rgTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:            "rg1",
					Namespace:       owner.Namespace,
					OwnerReferences: controlledBy(g, s, owner),
				},
			},
			&asoresourcesv1.ResourceGroup{
				TypeMeta: rgTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:            "rg2",
					Namespace:       owner.Namespace,
					OwnerReferences: controlledBy(g, s, owner),
				},
			},
			&asoresourcesv1.ResourceGroup{
				TypeMeta: rgTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:            "deleted from spec",
					Namespace:       owner.Namespace,
					OwnerReferences: controlledBy(g, s, owner),
					Finalizers:      []string{"still deleting"},
				},
			},
		}

		r := &ResourceReconciler{
			Client: fakeClientBuilder().
				WithObjects(objs...).
				Build(),
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
				rgJSON(g, s, &asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "not-yet-created",
					},
				}),
			},
			owner: owner,
		}

		g.Expect(r.Pause(ctx)).To(Succeed())

		haveNameAndAnnotations := func(name string, haveAnnotations types.GomegaMatcher) types.GomegaMatcher {
			return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
				"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Name":        Equal(name),
					"Annotations": haveAnnotations,
				}),
			})
		}

		resourceGroups := new(asoresourcesv1.ResourceGroupList)
		g.Expect(r.List(ctx, resourceGroups)).To(Succeed())
		g.Expect(resourceGroups.Items).To(ConsistOf(
			haveNameAndAnnotations("deleted from spec", BeEmpty()),
			haveNameAndAnnotations("not-yet-created", HaveKeyWithValue(asoannotations.ReconcilePolicy, string(asoannotations.ReconcilePolicySkip))),
			haveNameAndAnnotations("rg1", HaveKeyWithValue(asoannotations.ReconcilePolicy, string(asoannotations.ReconcilePolicySkip))),
			haveNameAndAnnotations("rg2", HaveKeyWithValue(asoannotations.ReconcilePolicy, string(asoannotations.ReconcilePolicySkip))),
		), "Expected ResourceGroups should have been updated")
	})
}

func TestResourceReconcilerDelete(t *testing.T) {
	ctx := t.Context()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1.AddToScheme,
		asoresourcesv1.AddToScheme,
		asonetworkv120220701.AddToScheme,
		asonetworkv120201101.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())

	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1.AzureASOManagedCluster{})
	}

	t.Run("empty resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		r := &ResourceReconciler{
			resources: []*unstructured.Unstructured{},
			owner:     &infrav1.AzureASOManagedCluster{},
		}

		g.Expect(r.Delete(ctx)).To(Succeed())
	})

	t.Run("delete several resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		owner := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Annotations: map[string]string{
					ownedKindsAnnotation: getOwnedKindsValue([]schema.GroupVersionKind{asoresourcesv1.GroupVersion.WithKind("ResourceGroup")}),
				},
			},
		}

		objs := []client.Object{
			&asoresourcesv1.ResourceGroup{
				TypeMeta: rgTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:            "still-deleting",
					Namespace:       owner.Namespace,
					OwnerReferences: controlledBy(g, s, owner),
					Finalizers: []string{
						"ASO finalizer",
					},
				},
			},
			&asoresourcesv1.ResourceGroup{
				TypeMeta: rgTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:            "just-deleted",
					Namespace:       owner.Namespace,
					OwnerReferences: controlledBy(g, s, owner),
				},
			},
		}

		r := &ResourceReconciler{
			Client: fakeClientBuilder().
				WithObjects(objs...).
				Build(),
			owner: owner,
		}

		g.Expect(r.Delete(ctx)).To(Succeed())
		g.Expect(owner.Annotations).To(HaveKeyWithValue(ownedKindsAnnotation, getOwnedKindsValue([]schema.GroupVersionKind{asoresourcesv1.GroupVersion.WithKind("ResourceGroup")})))
		g.Expect(apierrors.IsNotFound(r.Client.Get(ctx, client.ObjectKey{Namespace: owner.Namespace, Name: "just-deleted"}, &asoresourcesv1.ResourceGroup{}))).To(BeTrue())
		stillDeleting := &asoresourcesv1.ResourceGroup{}
		g.Expect(r.Client.Get(ctx, client.ObjectKey{Namespace: owner.Namespace, Name: "still-deleting"}, stillDeleting)).To(Succeed())
		g.Expect(stillDeleting.GetDeletionTimestamp().IsZero()).To(BeFalse())

		g.Expect(owner.Status.Resources).To(HaveLen(1))
		g.Expect(owner.Status.Resources[0].Resource.Name).To(Equal("still-deleting"))
		g.Expect(owner.Status.Resources[0].Ready).To(BeFalse())
	})

	t.Run("delete resources in ARM reference order", func(t *testing.T) {
		g := NewGomegaWithT(t)

		owner := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				UID:       "owner-uid",
				Annotations: map[string]string{
					ownedKindsAnnotation: getOwnedKindsValue([]schema.GroupVersionKind{
						asoresourcesv1.GroupVersion.WithKind("ResourceGroup"),
						asonetworkv120220701.GroupVersion.WithKind("PrivateEndpoint"),
						asonetworkv120201101.GroupVersion.WithKind("VirtualNetwork"),
					}),
				},
			},
		}
		resourceGroup := &asoresourcesv1.ResourceGroup{
			TypeMeta: rgTypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       owner.Namespace,
				Name:            "rg",
				UID:             "rg-uid",
				OwnerReferences: controlledBy(g, s, owner),
			},
		}
		vnetID := "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet"

		vnet := &unstructured.Unstructured{Object: map[string]any{}}
		vnet.SetGroupVersionKind(asonetworkv120201101.GroupVersion.WithKind("VirtualNetwork"))
		vnet.SetNamespace(owner.Namespace)
		vnet.SetName("vnet")
		vnet.SetUID("vnet-uid")
		vnet.SetOwnerReferences(append(controlledBy(g, s, owner), metav1.OwnerReference{UID: "rg-uid"}))
		g.Expect(unstructured.SetNestedField(vnet.Object, vnetID, "status", "id")).To(Succeed())

		privateEndpoint := &unstructured.Unstructured{Object: map[string]any{}}
		privateEndpoint.SetGroupVersionKind(asonetworkv120220701.GroupVersion.WithKind("PrivateEndpoint"))
		privateEndpoint.SetNamespace(owner.Namespace)
		privateEndpoint.SetName("private-endpoint")
		privateEndpoint.SetUID("private-endpoint-uid")
		privateEndpoint.SetOwnerReferences(append(controlledBy(g, s, owner), metav1.OwnerReference{UID: "rg-uid"}))
		// A PrivateEndpoint is owned by a ResourceGroup in ASO, but depends on
		// a subnet embedded in a VirtualNetwork's ARM resource ID.
		g.Expect(unstructured.SetNestedField(privateEndpoint.Object, vnetID+"/subnets/subnet", "spec", "subnet", "reference", "armId")).To(Succeed())

		clientWithDeleteOrder := &recordingDeleteClient{Client: fakeClientBuilder().WithObjects(resourceGroup, vnet, privateEndpoint).Build()}
		r := NewResourceReconciler(clientWithDeleteOrder, owner, nil, nil, WithChildrenFirst())
		ownedKinds, err := parseOwnedKinds(owner.Annotations[ownedKindsAnnotation])
		g.Expect(err).NotTo(HaveOccurred())
		ownedObjects, err := r.ownedObjs(ctx, ownedKinds)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ownedObjects).To(HaveLen(3))
		objectsForOrdering, err := r.resourcesForOrdering(ctx, ownedObjects)
		g.Expect(err).NotTo(HaveOccurred())
		deferred, err := deferShallowerResources(ownedObjects, objectsForOrdering)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(deferred.Has("vnet-uid")).To(BeTrue())
		g.Expect(deferred.Has("rg-uid")).To(BeTrue())

		g.Expect(r.Delete(ctx)).To(Succeed())
		g.Expect(clientWithDeleteOrder.deleted).To(Equal([]string{"PrivateEndpoint/private-endpoint"}))
		remainingVNet := vnet.DeepCopy()
		g.Expect(r.Get(ctx, client.ObjectKeyFromObject(vnet), remainingVNet)).To(Succeed())
		g.Expect(owner.Status.Resources).To(HaveLen(2))
		remainingNames := []string{owner.Status.Resources[0].Resource.Name, owner.Status.Resources[1].Resource.Name}
		g.Expect(remainingNames).To(ConsistOf("vnet", "rg"))
	})

	t.Run("delete resources in Kubernetes reference order", func(t *testing.T) {
		g := NewGomegaWithT(t)

		owner := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				UID:       "owner-uid",
				Annotations: map[string]string{
					ownedKindsAnnotation: getOwnedKindsValue([]schema.GroupVersionKind{
						asoresourcesv1.GroupVersion.WithKind("ResourceGroup"),
						asonetworkv120201101.GroupVersion.WithKind("VirtualNetwork"),
						asonetworkv120201101.GroupVersion.WithKind("VirtualNetworksSubnet"),
						asonetworkv120220701.GroupVersion.WithKind("PrivateEndpoint"),
					}),
				},
			},
		}
		resourceGroup := &asoresourcesv1.ResourceGroup{
			TypeMeta: rgTypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       owner.Namespace,
				Name:            "rg",
				UID:             "rg-uid",
				OwnerReferences: controlledBy(g, s, owner),
			},
		}
		resourceGroupOwner := metav1.OwnerReference{UID: "rg-uid"}
		vnet := &unstructured.Unstructured{Object: map[string]any{}}
		vnet.SetGroupVersionKind(asonetworkv120201101.GroupVersion.WithKind("VirtualNetwork"))
		vnet.SetNamespace(owner.Namespace)
		vnet.SetName("vnet")
		vnet.SetUID("vnet-uid")
		vnet.SetOwnerReferences(append(controlledBy(g, s, owner), resourceGroupOwner))

		subnet := &unstructured.Unstructured{Object: map[string]any{}}
		subnet.SetGroupVersionKind(asonetworkv120201101.GroupVersion.WithKind("VirtualNetworksSubnet"))
		subnet.SetNamespace(owner.Namespace)
		subnet.SetName("subnet")
		subnet.SetUID("subnet-uid")
		subnet.SetOwnerReferences(append(controlledBy(g, s, owner), metav1.OwnerReference{UID: "vnet-uid"}))

		privateEndpoint := &unstructured.Unstructured{Object: map[string]any{}}
		privateEndpoint.SetGroupVersionKind(asonetworkv120220701.GroupVersion.WithKind("PrivateEndpoint"))
		privateEndpoint.SetNamespace(owner.Namespace)
		privateEndpoint.SetName("private-endpoint")
		privateEndpoint.SetUID("private-endpoint-uid")
		privateEndpoint.SetOwnerReferences(append(controlledBy(g, s, owner), resourceGroupOwner))
		g.Expect(unstructured.SetNestedField(privateEndpoint.Object, asonetworkv120201101.GroupVersion.Group, "spec", "subnet", "reference", "group")).To(Succeed())
		g.Expect(unstructured.SetNestedField(privateEndpoint.Object, "VirtualNetworksSubnet", "spec", "subnet", "reference", "kind")).To(Succeed())
		g.Expect(unstructured.SetNestedField(privateEndpoint.Object, "subnet", "spec", "subnet", "reference", "name")).To(Succeed())

		clientWithDeleteOrder := &recordingDeleteClient{Client: fakeClientBuilder().WithObjects(resourceGroup, vnet, subnet, privateEndpoint).Build()}
		r := NewResourceReconciler(clientWithDeleteOrder, owner, nil, nil, WithChildrenFirst())

		g.Expect(r.Delete(ctx)).To(Succeed())
		g.Expect(clientWithDeleteOrder.deleted).To(Equal([]string{"PrivateEndpoint/private-endpoint"}))
		for _, object := range []*unstructured.Unstructured{vnet, subnet} {
			remaining := object.DeepCopy()
			g.Expect(r.Get(ctx, client.ObjectKeyFromObject(object), remaining)).To(Succeed())
		}
		remainingResourceGroup := &asoresourcesv1.ResourceGroup{}
		g.Expect(r.Get(ctx, client.ObjectKeyFromObject(resourceGroup), remainingResourceGroup)).To(Succeed())
		g.Expect(owner.Status.Resources).To(HaveLen(3))
	})

	t.Run("done deleting", func(t *testing.T) {
		g := NewGomegaWithT(t)

		owner := &infrav1.AzureASOManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Annotations: map[string]string{
					ownedKindsAnnotation: getOwnedKindsValue([]schema.GroupVersionKind{asoresourcesv1.GroupVersion.WithKind("ResourceGroup")}),
				},
			},
		}

		r := &ResourceReconciler{
			Client: fakeClientBuilder().Build(),
			owner:  owner,
		}

		g.Expect(r.Delete(ctx)).To(Succeed())

		g.Expect(owner.Annotations).NotTo(HaveKey(ownedKindsAnnotation))
		g.Expect(owner.Status.Resources).To(BeEmpty())
	})
}

type recordingDeleteClient struct {
	client.Client
	deleted []string
}

// Delete records the deleted object's kind and name before forwarding the request.
func (c *recordingDeleteClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleted = append(c.deleted, obj.GetObjectKind().GroupVersionKind().Kind+"/"+obj.GetName())
	return c.Client.Delete(ctx, obj, opts...)
}

func TestReadyStatus(t *testing.T) {
	ctx := t.Context()

	t.Run("unstructured", func(t *testing.T) {
		tests := []struct {
			name          string
			object        *unstructured.Unstructured
			expectedReady bool
		}{
			{
				name:          "empty object",
				object:        &unstructured.Unstructured{Object: make(map[string]any)},
				expectedReady: false,
			},
			{
				name: "empty status.conditions",
				object: &unstructured.Unstructured{Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{},
					},
				}},
				expectedReady: false,
			},
			{
				name: "status.conditions wrong type",
				object: &unstructured.Unstructured{Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							int64(0),
						},
					},
				}},
				expectedReady: false,
			},
			{
				name: "non-Ready type status.conditions",
				object: &unstructured.Unstructured{Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type": "not" + conditions.ConditionTypeReady,
							},
						},
					},
				}},
				expectedReady: false,
			},
			{
				name: "observedGeneration not up to date",
				object: &unstructured.Unstructured{Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(1),
					},
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
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
				object: &unstructured.Unstructured{Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
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
				object: &unstructured.Unstructured{Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
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
				object: &unstructured.Unstructured{Object: map[string]any{
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":   "not-" + conditions.ConditionTypeReady,
								"status": "not-" + string(metav1.ConditionTrue),
							},
							map[string]any{
								"type":   conditions.ConditionTypeReady,
								"status": string(metav1.ConditionTrue),
							},
							map[string]any{
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

func controlledBy(g *GomegaWithT, s *runtime.Scheme, owner client.Object) []metav1.OwnerReference {
	ownerGVK, err := apiutil.GVKForObject(owner, s)
	g.Expect(err).NotTo(HaveOccurred())
	return []metav1.OwnerReference{
		*metav1.NewControllerRef(owner, ownerGVK),
	}
}

func TestDeferShallowerResources(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	partialResource := func(uid, name string, ownerReferences ...metav1.OwnerReference) *metav1.PartialObjectMetadata {
		return &metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{
				UID:             k8stypes.UID(uid),
				Name:            name,
				OwnerReferences: ownerReferences,
			},
		}
	}
	resourceObject := func(name, kind string, status, spec any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "example.azure.com/v1",
			"kind":       kind,
			"metadata":   map[string]any{"name": name},
			"status":     status,
			"spec":       spec,
		}}
	}
	withID := func(name, kind, id string, spec any) *unstructured.Unstructured {
		return resourceObject(name, kind, map[string]any{"id": id}, spec)
	}
	vnetID := "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet"
	subnetID := vnetID + "/subnets/subnet"

	rg := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{UID: "rg-uid", Name: "rg"},
	}
	vnet := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			UID:  "vnet-uid",
			Name: "vnet",
			OwnerReferences: []metav1.OwnerReference{
				{UID: "controller-uid", Controller: boolPtr(true)},
				{UID: "rg-uid"},
			},
		},
	}
	nsg := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			UID:  "nsg-uid",
			Name: "nsg",
			OwnerReferences: []metav1.OwnerReference{
				{UID: "controller-uid", Controller: boolPtr(true)},
				{UID: "rg-uid"},
			},
		},
	}
	subnet := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			UID:  "subnet-uid",
			Name: "subnet",
			OwnerReferences: []metav1.OwnerReference{
				{UID: "controller-uid", Controller: boolPtr(true)},
				{UID: "vnet-uid"},
			},
		},
	}

	tests := []struct {
		name             string
		resources        []*metav1.PartialObjectMetadata
		objects          map[k8stypes.UID]*unstructured.Unstructured
		expectedDeferred []string
		expectedError    string
		checkCycle       bool
	}{
		{
			name:             "all levels present: only deepest (subnet) proceeds",
			resources:        []*metav1.PartialObjectMetadata{rg, vnet, nsg, subnet},
			expectedDeferred: []string{"rg", "vnet", "nsg"},
		},
		{
			name:             "without subnet: nsg and vnet are deepest, rg deferred",
			resources:        []*metav1.PartialObjectMetadata{rg, vnet, nsg},
			expectedDeferred: []string{"rg"},
		},
		{
			name:             "only roots: nothing deferred",
			resources:        []*metav1.PartialObjectMetadata{rg},
			expectedDeferred: []string{},
		},
		{
			name:             "siblings only (no hierarchy): nothing deferred",
			resources:        []*metav1.PartialObjectMetadata{vnet, nsg},
			expectedDeferred: []string{},
		},
		{
			name:             "empty input: nothing deferred",
			resources:        nil,
			expectedDeferred: []string{},
		},
		{
			name:          "wrong-typed status.id returns an error",
			resources:     []*metav1.PartialObjectMetadata{partialResource("bad-uid", "bad")},
			objects:       map[k8stypes.UID]*unstructured.Unstructured{"bad-uid": resourceObject("bad", "VirtualNetwork", map[string]any{"id": 123}, nil)},
			expectedError: "failed to read status.id",
		},
		{
			name: "ARM references use the longest matching resource ID",
			resources: []*metav1.PartialObjectMetadata{
				partialResource("vnet-uid", "vnet"),
				partialResource("subnet-uid", "subnet"),
				partialResource("consumer-uid", "consumer"),
			},
			objects: map[k8stypes.UID]*unstructured.Unstructured{
				"vnet-uid":     withID("vnet", "VirtualNetwork", vnetID, nil),
				"subnet-uid":   withID("subnet", "VirtualNetworksSubnet", subnetID, map[string]any{"virtualNetworkID": vnetID}),
				"consumer-uid": withID("consumer", "PrivateEndpoint", "", map[string]any{"subnetID": subnetID}),
			},
			expectedDeferred: []string{"vnet", "subnet"},
		},
		{
			name: "ARM ID matching ignores case and trailing slashes",
			resources: []*metav1.PartialObjectMetadata{
				partialResource("vnet-uid", "vnet"),
				partialResource("consumer-uid", "consumer"),
			},
			objects: map[k8stypes.UID]*unstructured.Unstructured{
				"vnet-uid":     withID("vnet", "VirtualNetwork", " /SUBSCRIPTIONS/SUB/RESOURCEGROUPS/RG/PROVIDERS/MICROSOFT.NETWORK/VIRTUALNETWORKS/VNET/ ", nil),
				"consumer-uid": withID("consumer", "PrivateEndpoint", "", map[string]any{"subnetID": "/subscriptions/sub/resourcegroups/rg/providers/microsoft.network/virtualnetworks/vnet/subnets/subnet/"}),
			},
			expectedDeferred: []string{"vnet"},
		},
		{
			name: "self-references are ignored",
			resources: []*metav1.PartialObjectMetadata{
				partialResource("root-uid", "root"),
				partialResource("self-uid", "self"),
			},
			objects: map[k8stypes.UID]*unstructured.Unstructured{
				"root-uid": withID("root", "VirtualNetwork", "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/root", nil),
				"self-uid": withID("self", "VirtualNetwork", "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/self", map[string]any{"selfID": "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/self/subnets/subnet"}),
			},
			expectedDeferred: []string{},
		},
		{
			name: "dependency cycles defer some resources but leave one deletable",
			resources: []*metav1.PartialObjectMetadata{
				partialResource("left-uid", "left", metav1.OwnerReference{UID: "right-uid"}),
				partialResource("right-uid", "right", metav1.OwnerReference{UID: "left-uid"}),
			},
			checkCycle: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			deferred, err := deferShallowerResources(tc.resources, tc.objects)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			if tc.checkCycle {
				g.Expect(deferred).ToNot(BeEmpty())
				g.Expect(len(deferred)).To(BeNumerically("<", len(tc.resources)))
				return
			}
			var deferredNames []string
			for _, r := range tc.resources {
				if deferred.Has(r.UID) {
					deferredNames = append(deferredNames, r.Name)
				}
			}
			if len(tc.expectedDeferred) == 0 {
				g.Expect(deferredNames).To(BeEmpty())
			} else {
				g.Expect(deferredNames).To(ConsistOf(tc.expectedDeferred))
			}
		})
	}
}

var rgTypeMeta = metav1.TypeMeta{
	APIVersion: asoresourcesv1.GroupVersion.Identifier(),
	Kind:       "ResourceGroup",
}

func rgJSON(g Gomega, scheme *runtime.Scheme, rg *asoresourcesv1.ResourceGroup) *unstructured.Unstructured {
	rg.SetGroupVersionKind(asoresourcesv1.GroupVersion.WithKind("ResourceGroup"))
	u := &unstructured.Unstructured{}
	g.Expect(scheme.Convert(rg, u, nil)).To(Succeed())
	return u
}
