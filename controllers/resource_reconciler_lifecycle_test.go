/*
Copyright 2026 The Kubernetes Authors.

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
	"slices"
	"testing"

	asonetworkv120201101 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	asonetworkv120220701 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asostoragev1api20230101 "github.com/Azure/azure-service-operator/v2/api/storage/v1api20230101"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// The tests in this file drive ResourceReconciler.Delete across several
// reconciles, with the test standing in for ASO between them. A single Delete
// call only shows that the first batch is chosen correctly; these show that
// the deletion carries on, layer by layer, until nothing is left.

const testVNetID = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet"

// deleteLifecycle runs ResourceReconciler.Delete repeatedly against one fake
// API server. Like the controllers, it builds a new ResourceReconciler for
// every reconcile, so nothing carries over between rounds except the owner and
// the objects in the API server.
type deleteLifecycle struct {
	t       *testing.T
	g       *WithT
	scheme  *runtime.Scheme
	client  *recordingDeleteClient
	owner   *infrav1.AzureASOManagedCluster
	options []ResourceReconcilerOption
}

// newDeleteLifecycle creates objs as resources that the owner controls and
// that ASO is managing: each one carries the ASO finalizer, so a DELETE leaves
// it in place with a deletion timestamp until the test finishes its deletion.
func newDeleteLifecycle(t *testing.T, options []ResourceReconcilerOption, objs ...client.Object) *deleteLifecycle {
	t.Helper()
	g := NewGomegaWithT(t)

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1.AddToScheme,
		asoresourcesv1.AddToScheme,
		asonetworkv120201101.AddToScheme,
		asonetworkv120220701.AddToScheme,
		asostoragev1api20230101.AddToScheme,
	)
	g.Expect(sb.AddToScheme(s)).To(Succeed())

	owner := &infrav1.AzureASOManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "owner",
			UID:       "owner-uid",
		},
	}

	var ownedKinds []schema.GroupVersionKind
	for _, obj := range objs {
		gvk, err := apiutil.GVKForObject(obj, s)
		g.Expect(err).NotTo(HaveOccurred())
		if !slices.Contains(ownedKinds, gvk) {
			ownedKinds = append(ownedKinds, gvk)
		}
		obj.SetNamespace(owner.Namespace)
		obj.SetOwnerReferences(append(controlledBy(g, s, owner), obj.GetOwnerReferences()...))
		obj.SetFinalizers([]string{genruntime.ReconcilerFinalizer})
	}
	owner.Annotations = map[string]string{ownedKindsAnnotation: getOwnedKindsValue(ownedKinds)}

	return &deleteLifecycle{
		t:      t,
		g:      g,
		scheme: s,
		client: &recordingDeleteClient{
			Client: fakeclient.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build(),
		},
		owner:   owner,
		options: options,
	}
}

// asoObjectMeta returns metadata for an ASO resource named name. ASO records a
// resource's spec.owner as an owner reference, which asoOwners stands in for.
func asoObjectMeta(name string, asoOwners ...client.Object) metav1.ObjectMeta {
	objectMeta := metav1.ObjectMeta{
		Name: name,
		UID:  k8stypes.UID(name + "-uid"),
	}
	for _, asoOwner := range asoOwners {
		objectMeta.OwnerReferences = append(objectMeta.OwnerReferences, metav1.OwnerReference{
			Name: asoOwner.GetName(),
			UID:  asoOwner.GetUID(),
		})
	}
	return objectMeta
}

// reconcileDelete runs one Delete with a new ResourceReconciler and returns
// the resources it sent a DELETE for, as recordingDeleteClient names them. A
// resource that is already being deleted is listed again each time Delete
// repeats the request.
func (l *deleteLifecycle) reconcileDelete() []string {
	l.client.deleted = nil
	r := NewResourceReconciler(l.client, l.owner, nil, nil, l.options...)
	l.g.Expect(r.Delete(l.t.Context())).To(Succeed())
	return l.client.deleted
}

// reconcileDeleteUntilDone reconciles until the owner has no resources left,
// calling step after each round so the test can act as ASO. It returns the
// DELETE requests sent in each round and fails if the deletion has not
// finished after maxRounds.
func (l *deleteLifecycle) reconcileDeleteUntilDone(maxRounds int, step func(round int)) [][]string {
	var rounds [][]string
	for round := range maxRounds {
		deleted := l.reconcileDelete()
		if len(l.owner.Status.Resources) == 0 {
			l.g.Expect(deleted).To(BeEmpty())
			l.g.Expect(l.owner.Annotations).NotTo(HaveKey(ownedKindsAnnotation))
			return rounds
		}
		rounds = append(rounds, deleted)
		step(round)
	}
	l.g.Expect(l.owner.Status.Resources).To(BeEmpty(), "deletion did not finish in %d reconciles; DELETE requests per reconcile: %v", maxRounds, rounds)
	return rounds
}

// restart discards everything a controller restart would lose. The owner is
// read back as the API server would return it, and the next reconcile gets a
// new client and a new ResourceReconciler.
func (l *deleteLifecycle) restart() {
	l.owner = l.owner.DeepCopy()
	l.client = &recordingDeleteClient{Client: l.client.Client}
}

// get reads the current state of obj and reports whether it still exists.
func (l *deleteLifecycle) get(obj client.Object) (client.Object, bool) {
	current, ok := obj.DeepCopyObject().(client.Object)
	l.g.Expect(ok).To(BeTrue())
	err := l.client.Get(l.t.Context(), client.ObjectKeyFromObject(obj), current)
	if client.IgnoreNotFound(err) == nil && err != nil {
		return nil, false
	}
	l.g.Expect(err).NotTo(HaveOccurred())
	return current, true
}

func (l *deleteLifecycle) id(obj client.Object) string {
	gvk, err := apiutil.GVKForObject(obj, l.scheme)
	l.g.Expect(err).NotTo(HaveOccurred())
	return gvk.Kind + "/" + obj.GetName()
}

func (l *deleteLifecycle) isDeleting(obj client.Object) bool {
	current, found := l.get(obj)
	l.g.Expect(found).To(BeTrue(), "%s no longer exists", l.id(obj))
	return !current.GetDeletionTimestamp().IsZero()
}

// expectNotDeleting asserts that objs exist and have never been sent a DELETE.
func (l *deleteLifecycle) expectNotDeleting(objs ...client.Object) {
	for _, obj := range objs {
		l.g.Expect(l.isDeleting(obj)).To(BeFalse(), "%s should not be deleting yet", l.id(obj))
	}
}

// expectDeleting asserts that objs are waiting for ASO to finish deleting them.
func (l *deleteLifecycle) expectDeleting(objs ...client.Object) {
	for _, obj := range objs {
		l.g.Expect(l.isDeleting(obj)).To(BeTrue(), "%s should be deleting", l.id(obj))
	}
}

// expectPending asserts that the owner's status lists exactly objs, none of
// them ready.
func (l *deleteLifecycle) expectPending(objs ...client.Object) {
	var expected, actual []string
	for _, obj := range objs {
		expected = append(expected, l.id(obj))
	}
	for _, status := range l.owner.Status.Resources {
		l.g.Expect(status.Ready).To(BeFalse())
		actual = append(actual, status.Resource.Kind+"/"+status.Resource.Name)
	}
	l.g.Expect(actual).To(ConsistOf(expected))
}

// finishDeleting acts as ASO completing the Azure delete of objs: ASO removes
// its finalizer and the API server drops the object.
func (l *deleteLifecycle) finishDeleting(objs ...client.Object) {
	for _, obj := range objs {
		current, found := l.get(obj)
		l.g.Expect(found).To(BeTrue(), "%s no longer exists", l.id(obj))
		l.g.Expect(current.GetDeletionTimestamp().IsZero()).To(BeFalse(), "ASO only removes its finalizer from %s once it is being deleted", l.id(obj))
		current.SetFinalizers(nil)
		l.g.Expect(l.client.Update(l.t.Context(), current)).To(Succeed())
		_, found = l.get(obj)
		l.g.Expect(found).To(BeFalse(), "%s should be gone", l.id(obj))
	}
}

// expectDone asserts that one more reconcile finds nothing left to delete and
// clears the owner's record of its resources.
func (l *deleteLifecycle) expectDone() {
	l.g.Expect(l.reconcileDelete()).To(BeEmpty())
	l.g.Expect(l.owner.Status.Resources).To(BeEmpty())
	l.g.Expect(l.owner.Annotations).NotTo(HaveKey(ownedKindsAnnotation))
}

// armIDChain returns a PrivateEndpoint that depends on a VirtualNetwork only
// through the ARM ID of a subnet embedded in it. ASO makes both of them
// children of the ResourceGroup, so owner references alone would let the
// VirtualNetwork be deleted alongside the PrivateEndpoint.
func armIDChain() (*asoresourcesv1.ResourceGroup, *asonetworkv120201101.VirtualNetwork, *asonetworkv120220701.PrivateEndpoint) {
	resourceGroup := &asoresourcesv1.ResourceGroup{ObjectMeta: asoObjectMeta("rg")}
	vnet := &asonetworkv120201101.VirtualNetwork{
		ObjectMeta: asoObjectMeta("vnet", resourceGroup),
		Status:     asonetworkv120201101.VirtualNetwork_STATUS{Id: ptr.To(testVNetID)},
	}
	privateEndpoint := &asonetworkv120220701.PrivateEndpoint{
		ObjectMeta: asoObjectMeta("private-endpoint", resourceGroup),
		Spec: asonetworkv120220701.PrivateEndpoint_Spec{
			Subnet: &asonetworkv120220701.Subnet_PrivateEndpoint_SubResourceEmbedded{
				Reference: &genruntime.ResourceReference{ARMID: testVNetID + "/subnets/subnet"},
			},
		},
	}
	return resourceGroup, vnet, privateEndpoint
}

// kubernetesReferenceChain returns a PrivateEndpoint that depends on a
// VirtualNetworksSubnet by its Kubernetes group, kind, and name.
func kubernetesReferenceChain() (*asoresourcesv1.ResourceGroup, *asonetworkv120201101.VirtualNetwork, *asonetworkv120201101.VirtualNetworksSubnet, *asonetworkv120220701.PrivateEndpoint) {
	resourceGroup := &asoresourcesv1.ResourceGroup{ObjectMeta: asoObjectMeta("rg")}
	vnet := &asonetworkv120201101.VirtualNetwork{ObjectMeta: asoObjectMeta("vnet", resourceGroup)}
	subnet := &asonetworkv120201101.VirtualNetworksSubnet{ObjectMeta: asoObjectMeta("subnet", vnet)}
	privateEndpoint := &asonetworkv120220701.PrivateEndpoint{
		ObjectMeta: asoObjectMeta("private-endpoint", resourceGroup),
		Spec: asonetworkv120220701.PrivateEndpoint_Spec{
			Subnet: &asonetworkv120220701.Subnet_PrivateEndpoint_SubResourceEmbedded{
				Reference: &genruntime.ResourceReference{
					Group: asonetworkv120201101.GroupVersion.Group,
					Kind:  "VirtualNetworksSubnet",
					Name:  subnet.Name,
				},
			},
		},
	}
	return resourceGroup, vnet, subnet, privateEndpoint
}

func TestResourceReconcilerDeleteLifecycle_ARMIDChain(t *testing.T) {
	g := NewGomegaWithT(t)
	resourceGroup, vnet, privateEndpoint := armIDChain()
	l := newDeleteLifecycle(t, []ResourceReconcilerOption{WithChildrenFirst()}, resourceGroup, vnet, privateEndpoint)

	g.Expect(l.reconcileDelete()).To(ConsistOf("PrivateEndpoint/private-endpoint"))
	l.expectDeleting(privateEndpoint)
	l.expectNotDeleting(vnet, resourceGroup)
	l.expectPending(privateEndpoint, vnet, resourceGroup)

	// ASO has not finished deleting the PrivateEndpoint, so another reconcile
	// must not move on to the resources it depends on.
	g.Expect(l.reconcileDelete()).To(ConsistOf("PrivateEndpoint/private-endpoint"))
	l.expectNotDeleting(vnet, resourceGroup)
	l.expectPending(privateEndpoint, vnet, resourceGroup)

	l.finishDeleting(privateEndpoint)
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetwork/vnet"))
	l.expectDeleting(vnet)
	l.expectNotDeleting(resourceGroup)
	l.expectPending(vnet, resourceGroup)

	l.finishDeleting(vnet)
	g.Expect(l.reconcileDelete()).To(ConsistOf("ResourceGroup/rg"))
	l.expectDeleting(resourceGroup)
	l.expectPending(resourceGroup)

	l.finishDeleting(resourceGroup)
	l.expectDone()
}

func TestResourceReconcilerDeleteLifecycle_KubernetesReferenceChain(t *testing.T) {
	g := NewGomegaWithT(t)
	resourceGroup, vnet, subnet, privateEndpoint := kubernetesReferenceChain()
	l := newDeleteLifecycle(t, []ResourceReconcilerOption{WithChildrenFirst()}, resourceGroup, vnet, subnet, privateEndpoint)

	g.Expect(l.reconcileDelete()).To(ConsistOf("PrivateEndpoint/private-endpoint"))
	l.expectNotDeleting(subnet, vnet, resourceGroup)
	l.expectPending(privateEndpoint, subnet, vnet, resourceGroup)

	l.finishDeleting(privateEndpoint)
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetworksSubnet/subnet"))
	l.expectDeleting(subnet)
	l.expectNotDeleting(vnet, resourceGroup)

	// A subnet that still holds its finalizer keeps the VirtualNetwork waiting.
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetworksSubnet/subnet"))
	l.expectNotDeleting(vnet, resourceGroup)
	l.expectPending(subnet, vnet, resourceGroup)

	l.finishDeleting(subnet)
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetwork/vnet"))
	l.expectNotDeleting(resourceGroup)

	l.finishDeleting(vnet)
	g.Expect(l.reconcileDelete()).To(ConsistOf("ResourceGroup/rg"))

	l.finishDeleting(resourceGroup)
	l.expectDone()
}

// ASO only reports that a resource type cannot be deleted from Azure after it
// has been asked to delete one, so the condition is not there on the first
// reconcile. The parent must be released once it shows up.
func TestResourceReconcilerDeleteLifecycle_UnsupportedDeleteReportedAfterDelete(t *testing.T) {
	g := NewGomegaWithT(t)
	account := &asostoragev1api20230101.StorageAccount{ObjectMeta: asoObjectMeta("account")}
	blobService := &asostoragev1api20230101.StorageAccountsBlobService{ObjectMeta: asoObjectMeta("blob-service", account)}
	l := newDeleteLifecycle(t, []ResourceReconcilerOption{WithChildrenFirst()}, account, blobService)

	g.Expect(l.reconcileDelete()).To(ConsistOf("StorageAccountsBlobService/blob-service"))
	l.expectDeleting(blobService)
	l.expectNotDeleting(account)
	l.expectPending(blobService, account)

	// ASO refuses the delete and keeps its finalizer on the BlobService until
	// the StorageAccount is gone.
	current, found := l.get(blobService)
	g.Expect(found).To(BeTrue())
	reported, ok := current.(*asostoragev1api20230101.StorageAccountsBlobService)
	g.Expect(ok).To(BeTrue())
	reported.Status.Conditions = conditions.Conditions{{
		Type:   conditions.ConditionTypeReady,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonDeletionNotSupported.Name,
	}}
	g.Expect(l.client.Update(l.t.Context(), reported)).To(Succeed())

	// Repeating the DELETE for the BlobService is harmless.
	g.Expect(l.reconcileDelete()).To(ConsistOf("StorageAccount/account", "StorageAccountsBlobService/blob-service"))
	l.expectDeleting(account, blobService)
	l.expectPending(blobService, account)

	// With the StorageAccount gone from Azure, ASO finds the BlobService gone
	// as well and releases it.
	l.finishDeleting(account)
	g.Expect(l.reconcileDelete()).To(ConsistOf("StorageAccountsBlobService/blob-service"))
	l.expectPending(blobService)

	l.finishDeleting(blobService)
	l.expectDone()
}

// This test describes the current ordering rather than a requirement. Depth is
// compared across every resource being deleted, so a resource waits for deeper
// resources it has no relationship with. If ordering ever becomes
// per-dependency, change this test on purpose.
func TestResourceReconcilerDeleteLifecycle_GlobalDepthBarrier(t *testing.T) {
	g := NewGomegaWithT(t)
	deepResourceGroup := &asoresourcesv1.ResourceGroup{ObjectMeta: asoObjectMeta("deep-rg")}
	vnet := &asonetworkv120201101.VirtualNetwork{ObjectMeta: asoObjectMeta("vnet", deepResourceGroup)}
	subnet := &asonetworkv120201101.VirtualNetworksSubnet{ObjectMeta: asoObjectMeta("subnet", vnet)}
	shallowResourceGroup := &asoresourcesv1.ResourceGroup{ObjectMeta: asoObjectMeta("shallow-rg")}
	securityGroup := &asonetworkv120201101.NetworkSecurityGroup{ObjectMeta: asoObjectMeta("nsg", shallowResourceGroup)}
	l := newDeleteLifecycle(t, []ResourceReconcilerOption{WithChildrenFirst()}, deepResourceGroup, vnet, subnet, shallowResourceGroup, securityGroup)

	// Nothing depends on the NetworkSecurityGroup and it shares no dependency
	// with the subnet, but it sits one level above the deepest resource.
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetworksSubnet/subnet"))
	l.expectNotDeleting(securityGroup, vnet, shallowResourceGroup, deepResourceGroup)

	l.finishDeleting(subnet)
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetwork/vnet", "NetworkSecurityGroup/nsg"))
	l.expectNotDeleting(shallowResourceGroup, deepResourceGroup)

	// The shallow ResourceGroup has lost its only child, and still waits for
	// the VirtualNetwork in the other branch.
	l.finishDeleting(securityGroup)
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetwork/vnet"))
	l.expectNotDeleting(shallowResourceGroup, deepResourceGroup)

	l.finishDeleting(vnet)
	g.Expect(l.reconcileDelete()).To(ConsistOf("ResourceGroup/deep-rg", "ResourceGroup/shallow-rg"))

	l.finishDeleting(deepResourceGroup, shallowResourceGroup)
	l.expectDone()
}

// Without WithChildrenFirst every resource is deleted at once and ASO is left
// to work out the order.
func TestResourceReconcilerDeleteLifecycle_ChildrenFirstDisabled(t *testing.T) {
	g := NewGomegaWithT(t)
	resourceGroup, vnet, privateEndpoint := armIDChain()
	l := newDeleteLifecycle(t, nil, resourceGroup, vnet, privateEndpoint)

	g.Expect(l.reconcileDelete()).To(ConsistOf("ResourceGroup/rg", "VirtualNetwork/vnet", "PrivateEndpoint/private-endpoint"))
	l.expectDeleting(resourceGroup, vnet, privateEndpoint)
	l.expectPending(resourceGroup, vnet, privateEndpoint)

	l.finishDeleting(privateEndpoint, vnet, resourceGroup)
	l.expectDone()
}

// A controller that restarts partway through a deletion has only the owner and
// the cluster to go on. The ordering has to come out the same.
func TestResourceReconcilerDeleteLifecycle_Restart(t *testing.T) {
	g := NewGomegaWithT(t)
	resourceGroup, vnet, subnet, privateEndpoint := kubernetesReferenceChain()
	l := newDeleteLifecycle(t, []ResourceReconcilerOption{WithChildrenFirst()}, resourceGroup, vnet, subnet, privateEndpoint)

	g.Expect(l.reconcileDelete()).To(ConsistOf("PrivateEndpoint/private-endpoint"))
	l.finishDeleting(privateEndpoint)
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetworksSubnet/subnet"))

	// Restart while ASO is still deleting the subnet.
	l.restart()
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetworksSubnet/subnet"))
	l.expectNotDeleting(vnet, resourceGroup)
	l.expectPending(subnet, vnet, resourceGroup)

	// Restart again, this time between two layers.
	l.finishDeleting(subnet)
	l.restart()
	g.Expect(l.reconcileDelete()).To(ConsistOf("VirtualNetwork/vnet"))
	l.expectNotDeleting(resourceGroup)

	l.finishDeleting(vnet)
	l.restart()
	g.Expect(l.reconcileDelete()).To(ConsistOf("ResourceGroup/rg"))

	l.finishDeleting(resourceGroup)
	l.restart()
	l.expectDone()
}
