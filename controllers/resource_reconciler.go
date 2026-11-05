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
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/mutators"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const (
	ownedKindsAnnotation = "sigs.k8s.io/cluster-api-provider-azure-owned-aso-kinds"
	ownedKindsSep        = ";"
)

// ResourceReconciler reconciles a set of arbitrary ASO resources.
type ResourceReconciler struct {
	client.Client
	resources     []*unstructured.Unstructured
	owner         ResourceReconcilerOwner
	watcher       ResourceWatcher
	childrenFirst bool
}

// ResourceWatcher watches ASO resources managed by a ResourceReconciler.
type ResourceWatcher interface {
	Watch(log logr.Logger, obj client.Object, handler handler.EventHandler, p ...predicate.Predicate) error
}

// ResourceReconcilerOwner is the CAPZ resource whose status records managed ASO resources.
type ResourceReconcilerOwner interface {
	client.Object
	SetResourceStatuses([]infrav1.ResourceStatus)
}

// ResourceReconcilerOption configures a ResourceReconciler.
type ResourceReconcilerOption func(*ResourceReconciler)

// NewResourceReconciler creates a reconciler for the specified ASO resources.
func NewResourceReconciler(c client.Client, owner ResourceReconcilerOwner, watcher ResourceWatcher, resources []*unstructured.Unstructured, options ...ResourceReconcilerOption) *ResourceReconciler {
	r := &ResourceReconciler{
		Client:    c,
		resources: resources,
		owner:     owner,
		watcher:   watcher,
	}
	for _, option := range options {
		option(r)
	}
	return r
}

// WithChildrenFirst configures the ResourceReconciler to delete child resources
// before their dependencies, using ASO owner references, ARM IDs, and
// Kubernetes resource references in resource specifications to determine the
// hierarchy.
func WithChildrenFirst() ResourceReconcilerOption {
	return func(r *ResourceReconciler) {
		r.childrenFirst = true
	}
}

// Reconcile creates or updates the specified resources.
func (r *ResourceReconciler) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.Reconcile")
	defer done()
	log.V(4).Info("reconciling resources")
	return r.reconcile(ctx)
}

// Delete deletes the specified resources.
func (r *ResourceReconciler) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.Delete")
	defer done()
	log.V(4).Info("deleting resources")

	// Delete is a special case of a normal reconciliation which is equivalent to all resources from spec
	// being deleted.
	r.resources = nil
	return r.reconcile(ctx)
}

// Pause pauses reconciliation of the specified resources.
func (r *ResourceReconciler) Pause(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.Pause")
	defer done()
	log.V(4).Info("pausing resources")

	err := mutators.Pause(ctx, r.resources)
	if err != nil {
		if errors.As(err, &mutators.Incompatible{}) {
			err = reconcile.TerminalError(err)
		}
		return err
	}

	for _, spec := range r.resources {
		spec.SetNamespace(r.owner.GetNamespace())
		gvk := spec.GroupVersionKind()
		log.V(4).Info("pausing resource", "resource", klog.KObj(spec), "resourceVersion", gvk.GroupVersion(), "resourceKind", gvk.Kind)
		err := r.Apply(ctx, client.ApplyConfigurationFromUnstructured(spec), client.FieldOwner("capz-manager"))
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to patch resource: %w", err)
		}
	}

	return nil
}

func (r *ResourceReconciler) reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.reconcile")
	defer done()

	var newResourceStatuses []infrav1.ResourceStatus

	ownedKindsValue := r.owner.GetAnnotations()[ownedKindsAnnotation]
	ownedKinds, err := parseOwnedKinds(ownedKindsValue)
	if err != nil {
		return fmt.Errorf("failed to parse %s annotation: %s", ownedKindsAnnotation, ownedKindsValue)
	}

	ownedObjs, err := r.ownedObjs(ctx, ownedKinds)
	if err != nil {
		return fmt.Errorf("failed to get owned objects: %w", err)
	}

	unrecordedTypeResources, recordedTypeResources, toBeDeletedResources := partitionResources(ownedKinds, r.resources, ownedObjs)

	// Newly-defined types in the CAPZ spec are first recorded in the annotation without performing a
	// patch that would create resources of that type. CAPZ only patches resources whose kinds have
	// already been recorded to ensure no resources are orphaned.
	for _, spec := range unrecordedTypeResources {
		newResourceStatuses = append(newResourceStatuses, infrav1.ResourceStatus{
			Resource: statusResource(spec),
			Ready:    false,
		})
	}

	for _, spec := range recordedTypeResources {
		spec.SetNamespace(r.owner.GetNamespace())

		if err := controllerutil.SetControllerReference(r.owner, spec, r.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}

		if r.watcher != nil {
			toWatch := meta.AsPartialObjectMetadata(spec)
			toWatch.APIVersion = spec.GetAPIVersion()
			toWatch.Kind = spec.GetKind()
			if err := r.watcher.Watch(log, toWatch, handler.EnqueueRequestForOwner(r.Client.Scheme(), r.Client.RESTMapper(), r.owner)); err != nil {
				return fmt.Errorf("failed to watch resource: %w", err)
			}
		}

		gvk := spec.GroupVersionKind()
		log.V(4).Info("applying resource", "resource", klog.KObj(spec), "resourceVersion", gvk.GroupVersion(), "resourceKind", gvk.Kind)
		err := r.Apply(ctx, client.ApplyConfigurationFromUnstructured(spec), client.FieldOwner("capz-manager"), client.ForceOwnership)
		if err != nil {
			return fmt.Errorf("failed to apply resource: %w", err)
		}

		ready, err := readyStatus(ctx, spec)
		if err != nil {
			return fmt.Errorf("failed to get ready status: %w", err)
		}
		newResourceStatuses = append(newResourceStatuses, infrav1.ResourceStatus{
			Resource: statusResource(spec),
			Ready:    ready,
		})
	}

	var deferred sets.Set[types.UID]
	if r.childrenFirst {
		resourcesForOrdering, err := r.resourcesForOrdering(ctx, toBeDeletedResources)
		if err != nil {
			return fmt.Errorf("failed to get resources for deletion ordering: %w", err)
		}
		deferred = deferShallowerResources(toBeDeletedResources, resourcesForOrdering)
	}
	for _, obj := range toBeDeletedResources {
		if deferred.Has(obj.UID) {
			gvk := obj.GroupVersionKind()
			newResourceStatuses = append(newResourceStatuses, infrav1.ResourceStatus{
				Resource: infrav1.StatusResource{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
					Name:    obj.Name,
				},
				Ready: false,
			})
			continue
		}
		newStatus, err := r.deleteResource(ctx, obj)
		if err != nil {
			return fmt.Errorf("failed to delete %s %s/%s", obj.GroupVersionKind(), obj.Namespace, obj.Name)
		}
		if newStatus != nil {
			newResourceStatuses = append(newResourceStatuses, *newStatus)
		}
	}

	newOwnedKinds := []schema.GroupVersionKind{}
	for _, status := range newResourceStatuses {
		gvk := schema.GroupVersionKind{
			Group:   status.Resource.Group,
			Version: status.Resource.Version,
			Kind:    status.Resource.Kind,
		}
		if !slices.Contains(newOwnedKinds, gvk) {
			newOwnedKinds = append(newOwnedKinds, gvk)
		}
	}
	annotations := r.owner.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[ownedKindsAnnotation] = getOwnedKindsValue(newOwnedKinds)
	if annotations[ownedKindsAnnotation] == "" {
		delete(annotations, ownedKindsAnnotation)
	}
	r.owner.SetAnnotations(annotations)

	r.owner.SetResourceStatuses(newResourceStatuses)

	return nil
}

// resourcesForOrdering loads the full ASO objects for resources being deleted.
// Their specs contain ARM IDs and Kubernetes resource references for
// dependencies that are not represented by owner references.
func (r *ResourceReconciler) resourcesForOrdering(ctx context.Context, resources []*metav1.PartialObjectMetadata) (map[types.UID]*unstructured.Unstructured, error) {
	objects := make(map[types.UID]*unstructured.Unstructured, len(resources))
	for _, resource := range resources {
		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(resource.GroupVersionKind())
		if err := r.Get(ctx, client.ObjectKeyFromObject(resource), object); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("failed to get %s %s/%s: %w", resource.GroupVersionKind(), resource.Namespace, resource.Name, err)
		}
		objects[resource.UID] = object
	}
	return objects, nil
}

func (r *ResourceReconciler) deleteResource(ctx context.Context, resource *metav1.PartialObjectMetadata) (*infrav1.ResourceStatus, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.deleteResource")
	defer done()

	log = log.WithValues("resource", klog.KObj(resource), "resourceVersion", resource.APIVersion, "resourceKind", resource.Kind)

	log.V(4).Info("deleting resource")
	err := r.Client.Delete(ctx, resource)
	if apierrors.IsNotFound(err) {
		log.V(4).Info("resource has been deleted")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to delete resource: %w", err)
	}

	err = r.Client.Get(ctx, client.ObjectKeyFromObject(resource), resource)
	if apierrors.IsNotFound(err) {
		log.V(4).Info("resource has been deleted")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	gvk := resource.GroupVersionKind()
	return &infrav1.ResourceStatus{
		Resource: infrav1.StatusResource{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
			Name:    resource.Name,
		},
		Ready: false,
	}, nil
}

func (r *ResourceReconciler) ownedObjs(ctx context.Context, ownedTypes sets.Set[metav1.TypeMeta]) ([]*metav1.PartialObjectMetadata, error) {
	var ownedObjs []*metav1.PartialObjectMetadata

	for typeMeta := range ownedTypes {
		objs := &metav1.PartialObjectMetadataList{TypeMeta: typeMeta}
		err := r.List(ctx, objs, client.InNamespace(r.owner.GetNamespace()))
		if err != nil {
			return nil, fmt.Errorf("failed to list %s %s: %w", typeMeta.APIVersion, typeMeta.Kind, err)
		}

		for _, obj := range objs.Items {
			controller := metav1.GetControllerOfNoCopy(&obj)
			if controller != nil && controller.UID == r.owner.GetUID() {
				ownedObjs = append(ownedObjs, &obj)
			}
		}
	}

	return ownedObjs, nil
}

func readyStatus(ctx context.Context, u *unstructured.Unstructured) (bool, error) {
	_, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.readyStatus")
	defer done()

	statusConditions, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	for _, el := range statusConditions {
		condition, ok := el.(map[string]any)
		if !ok {
			continue
		}
		condType, found, err := unstructured.NestedString(condition, "type")
		if !found || err != nil || condType != conditions.ConditionTypeReady {
			continue
		}

		observedGen, _, err := unstructured.NestedInt64(condition, "observedGeneration")
		if err != nil {
			return false, err
		}
		if observedGen < u.GetGeneration() {
			log.V(4).Info("waiting for ASO to reconcile the resource")
			return false, nil
		}

		readyStatus, _, err := unstructured.NestedString(condition, "status")
		if err != nil {
			return false, err
		}
		return readyStatus == string(metav1.ConditionTrue), nil
	}

	// no ready condition is set
	return false, nil
}

// partitionResources splits the sets of resources in spec and the current set
// of owned, existing resources into three groups:
// - unrecordedTypeResources are of a type not yet known to this owning CAPZ resource.
// - recordedTypeResources are of a type already known to this owning CAPZ resource.
// - toBeDeletedResources exist but are not defined in spec.
func partitionResources(
	ownedKinds sets.Set[metav1.TypeMeta],
	specs []*unstructured.Unstructured,
	ownedObjs []*metav1.PartialObjectMetadata,
) (
	unrecordedTypeResources []*unstructured.Unstructured,
	recordedTypeResources []*unstructured.Unstructured,
	toBeDeletedResources []*metav1.PartialObjectMetadata,
) {
	for _, spec := range specs {
		typeMeta := metav1.TypeMeta{
			APIVersion: spec.GetAPIVersion(),
			Kind:       spec.GetKind(),
		}
		if ownedKinds.Has(typeMeta) {
			recordedTypeResources = append(recordedTypeResources, spec)
		} else {
			unrecordedTypeResources = append(unrecordedTypeResources, spec)
		}
	}

	for _, owned := range ownedObjs {
		if !slices.ContainsFunc(specs, metadataRefersToResource(owned)) {
			toBeDeletedResources = append(toBeDeletedResources, owned)
		}
	}
	return
}

func statusResource(resource *unstructured.Unstructured) infrav1.StatusResource {
	gvk := resource.GroupVersionKind()
	return infrav1.StatusResource{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
		Name:    resource.GetName(),
	}
}

// deferShallowerResources returns the set of UIDs that should NOT be deleted
// this cycle. It computes the depth of each resource from owner references and
// ARM IDs referenced by its spec, then allows only the deepest level to proceed.
func deferShallowerResources(resources []*metav1.PartialObjectMetadata, objects map[types.UID]*unstructured.Unstructured) sets.Set[types.UID] {
	pending := make(map[types.UID]*metav1.PartialObjectMetadata, len(resources))
	for _, obj := range resources {
		pending[obj.UID] = obj
	}

	dependencies := make(map[types.UID]sets.Set[types.UID], len(resources))
	resourcesByReference := make(map[resourceReferenceKey]types.UID, len(resources))
	for _, resource := range resources {
		dependencies[resource.UID] = sets.New[types.UID]()
		gvk := resource.GroupVersionKind()
		resourcesByReference[resourceReferenceKey{
			group: gvk.Group,
			kind:  gvk.Kind,
			name:  resource.Name,
		}] = resource.UID
		for i := range resource.OwnerReferences {
			ref := &resource.OwnerReferences[i]
			if ref.Controller != nil && *ref.Controller {
				continue
			}
			if _, inSet := pending[ref.UID]; inSet {
				dependencies[resource.UID].Insert(ref.UID)
			}
		}
	}

	// Match referenced ARM IDs to the most specific managed resource ID. A
	// reference to a child resource (for example, a subnet ID) is therefore
	// treated as a dependency on its managed parent (the virtual network).
	resourceIDs := make(map[string]types.UID, len(objects))
	for uid, object := range objects {
		id, found, err := unstructured.NestedString(object.Object, "status", "id")
		if err == nil && found && id != "" {
			if id = normalizeARMID(id); id != "" {
				resourceIDs[id] = uid
			}
		}
	}
	for uid, object := range objects {
		spec, found, err := unstructured.NestedFieldNoCopy(object.Object, "spec")
		if err != nil || !found {
			continue
		}
		visitARMIDs(spec, func(id string) {
			id = normalizeARMID(id)
			var bestMatch string
			var dependency types.UID
			for resourceID, candidate := range resourceIDs {
				if candidate == uid || !isARMIDParent(resourceID, id) {
					continue
				}
				if len(resourceID) > len(bestMatch) {
					bestMatch = resourceID
					dependency = candidate
				}
			}
			if bestMatch != "" {
				dependencies[uid].Insert(dependency)
			}
		})
		visitKubernetesResourceReferences(spec, func(reference resourceReferenceKey) {
			if dependency, found := resourcesByReference[reference]; found && dependency != uid {
				dependencies[uid].Insert(dependency)
			}
		})
	}

	depth := make(map[types.UID]int, len(resources))
	var computeDepth func(types.UID) int
	computeDepth = func(uid types.UID) int {
		if d, ok := depth[uid]; ok {
			return d
		}
		depth[uid] = 0 // guard against cycles
		maxParentDepth := -1
		for dependency := range dependencies[uid] {
			if pd := computeDepth(dependency); pd > maxParentDepth {
				maxParentDepth = pd
			}
		}
		depth[uid] = maxParentDepth + 1
		return depth[uid]
	}

	maxDepth := 0
	for uid := range pending {
		if d := computeDepth(uid); d > maxDepth {
			maxDepth = d
		}
	}

	deferred := sets.New[types.UID]()
	for uid, d := range depth {
		if d < maxDepth {
			deferred.Insert(uid)
		}
	}
	return deferred
}

func normalizeARMID(id string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(id)), "/")
}

func isARMIDParent(resourceID, referenceID string) bool {
	return referenceID == resourceID || strings.HasPrefix(referenceID, resourceID+"/")
}

func visitARMIDs(value any, visit func(string)) {
	switch value := value.(type) {
	case string:
		if strings.HasPrefix(strings.TrimSpace(value), "/") {
			visit(value)
		}
	case []any:
		for _, element := range value {
			visitARMIDs(element, visit)
		}
	case map[string]any:
		for _, element := range value {
			visitARMIDs(element, visit)
		}
	}
}

type resourceReferenceKey struct {
	group string
	kind  string
	name  string
}

func visitKubernetesResourceReferences(value any, visit func(resourceReferenceKey)) {
	switch value := value.(type) {
	case []any:
		for _, element := range value {
			visitKubernetesResourceReferences(element, visit)
		}
	case map[string]any:
		group, groupFound := value["group"].(string)
		kind, kindFound := value["kind"].(string)
		name, nameFound := value["name"].(string)
		armID, _ := value["armId"].(string)
		if groupFound && kindFound && nameFound && group != "" && kind != "" && name != "" && armID == "" {
			visit(resourceReferenceKey{group: group, kind: kind, name: name})
		}
		for _, element := range value {
			visitKubernetesResourceReferences(element, visit)
		}
	}
}

func metadataRefersToResource(metadata *metav1.PartialObjectMetadata) func(*unstructured.Unstructured) bool {
	return func(u *unstructured.Unstructured) bool {
		// Version is not a stable property of a particular resource. The API version of an ASO resource may
		// change in the CAPZ spec from v1 to v2 but that still represents the same underlying resource.
		return metadata.GroupVersionKind().GroupKind() == u.GroupVersionKind().GroupKind() &&
			metadata.Name == u.GetName()
	}
}

func parseOwnedKinds(value string) (sets.Set[metav1.TypeMeta], error) {
	ownedKinds := sets.Set[metav1.TypeMeta]{}
	if value == "" {
		return nil, nil
	}
	for ownedKind := range strings.SplitSeq(value, ownedKindsSep) {
		gvk, _ := schema.ParseKindArg(ownedKind)
		if gvk == nil {
			return nil, fmt.Errorf("invalid field %q: expected Kind.version.group", ownedKind)
		}
		ownedKinds.Insert(metav1.TypeMeta{
			APIVersion: gvk.GroupVersion().Identifier(),
			Kind:       gvk.Kind,
		})
	}
	return ownedKinds, nil
}

func getOwnedKindsValue(ownedKinds []schema.GroupVersionKind) string {
	fields := make([]string, 0, len(ownedKinds))
	for _, gvk := range ownedKinds {
		fields = append(fields, strings.Join([]string{gvk.Kind, gvk.Version, gvk.Group}, "."))
	}
	return strings.Join(fields, ownedKindsSep)
}
