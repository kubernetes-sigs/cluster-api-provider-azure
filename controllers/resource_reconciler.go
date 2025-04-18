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

	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/mutators"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// ResourceReconciler reconciles a set of arbitrary ASO resources.
type ResourceReconciler struct {
	client.Client
	resources []*unstructured.Unstructured
	owner     resourceStatusObject
	watcher   watcher
}

type watcher interface {
	Watch(log logr.Logger, obj client.Object, handler handler.EventHandler, p ...predicate.Predicate) error
}

type resourceStatusObject interface {
	client.Object
	GetResourceStatuses() []infrav1alpha.ResourceStatus
	SetResourceStatuses([]infrav1alpha.ResourceStatus)
}

// Reconcile creates or updates the specified resources.
func (r *ResourceReconciler) Reconcile(ctx context.Context) (bool, error) {
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
	_, err := r.reconcile(ctx)
	return err
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

	_, observedResources, _ := partitionResources(r.resources, r.owner.GetResourceStatuses())

	for _, spec := range observedResources {
		gvk := spec.GroupVersionKind()
		log.V(4).Info("pausing resource", "resource", klog.KObj(spec), "resourceVersion", gvk.GroupVersion(), "resourceKind", gvk.Kind)
		err := r.Patch(ctx, spec, client.Apply, client.FieldOwner("capz-manager"))
		if err != nil {
			return fmt.Errorf("failed to patch resource: %w", err)
		}
	}

	return nil
}

func (r *ResourceReconciler) reconcile(ctx context.Context) (bool, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.reconcile")
	defer done()

	var newResourceStatuses []infrav1alpha.ResourceStatus

	unobservedResources, observedResources, deletedResources := partitionResources(r.resources, r.owner.GetResourceStatuses())

	// Newly-defined resources in the CAPZ spec are first recorded in the status without performing a patch
	// that would create the resource. CAPZ only patches resources that have already been recorded in status
	// to ensure no resources are orphaned.
	for _, spec := range unobservedResources {
		newResourceStatuses = append(newResourceStatuses, infrav1alpha.ResourceStatus{
			Resource: statusResource(spec),
			Ready:    false,
		})
	}

	for _, spec := range observedResources {
		spec.SetNamespace(r.owner.GetNamespace())

		if err := controllerutil.SetControllerReference(r.owner, spec, r.Scheme()); err != nil {
			return false, fmt.Errorf("failed to set owner reference: %w", err)
		}

		if err := r.watcher.Watch(log, spec, handler.EnqueueRequestForOwner(r.Client.Scheme(), r.Client.RESTMapper(), r.owner)); err != nil {
			return false, fmt.Errorf("failed to watch resource: %w", err)
		}

		gvk := spec.GroupVersionKind()
		log.V(4).Info("applying resource", "resource", klog.KObj(spec), "resourceVersion", gvk.GroupVersion(), "resourceKind", gvk.Kind)
		err := r.Patch(ctx, spec, client.Apply, client.FieldOwner("capz-manager"), client.ForceOwnership)
		if err != nil {
			return false, fmt.Errorf("failed to apply resource: %w", err)
		}

		ready, err := readyStatus(ctx, spec)
		if err != nil {
			return false, fmt.Errorf("failed to get ready status: %w", err)
		}
		newResourceStatuses = append(newResourceStatuses, infrav1alpha.ResourceStatus{
			Resource: statusResource(spec),
			Ready:    ready,
		})
	}

	for _, status := range deletedResources {
		updatedStatus, err := r.deleteResource(ctx, status.Resource)
		if err != nil {
			return false, err
		}
		if updatedStatus != nil {
			newResourceStatuses = append(newResourceStatuses, *updatedStatus)
		}
	}

	r.owner.SetResourceStatuses(newResourceStatuses)

	return len(unobservedResources) > 0, nil
}

func (r *ResourceReconciler) deleteResource(ctx context.Context, resource infrav1alpha.StatusResource) (*infrav1alpha.ResourceStatus, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.deleteResource")
	defer done()

	spec := &unstructured.Unstructured{}
	spec.SetGroupVersionKind(schema.GroupVersionKind{Group: resource.Group, Version: resource.Version, Kind: resource.Kind})
	spec.SetNamespace(r.owner.GetNamespace())
	spec.SetName(resource.Name)

	log = log.WithValues("resource", klog.KObj(spec), "resourceVersion", spec.GroupVersionKind().GroupVersion(), "resourceKind", spec.GetKind())

	log.V(4).Info("deleting resource")
	err := r.Client.Delete(ctx, spec)
	if apierrors.IsNotFound(err) {
		log.V(4).Info("resource has been deleted")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to delete resource: %w", err)
	}

	err = r.Client.Get(ctx, client.ObjectKeyFromObject(spec), spec)
	if apierrors.IsNotFound(err) {
		log.V(4).Info("resource has been deleted")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}
	ready, err := readyStatus(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to get ready status: %w", err)
	}

	return &infrav1alpha.ResourceStatus{
		Resource: resource,
		Ready:    ready,
	}, nil
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
		condition, ok := el.(map[string]interface{})
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

// partitionResources splits sets of resources defined in spec and status into three groups:
// - unobservedResources exist in spec but not status.
// - observedResources exist in both spec and status.
// - deletedResources exist in status but not spec.
func partitionResources(
	specs []*unstructured.Unstructured,
	statuses []infrav1alpha.ResourceStatus,
) (
	unobservedResources []*unstructured.Unstructured,
	observedResources []*unstructured.Unstructured,
	deletedResources []infrav1alpha.ResourceStatus,
) {
specs:
	for _, spec := range specs {
		for _, status := range statuses {
			if statusRefersToResource(status, spec) {
				observedResources = append(observedResources, spec)
				continue specs
			}
		}
		unobservedResources = append(unobservedResources, spec)
	}

statuses:
	for _, status := range statuses {
		for _, resource := range specs {
			if statusRefersToResource(status, resource) {
				continue statuses
			}
		}
		deletedResources = append(deletedResources, status)
	}
	return
}

func statusResource(resource *unstructured.Unstructured) infrav1alpha.StatusResource {
	gvk := resource.GroupVersionKind()
	return infrav1alpha.StatusResource{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
		Name:    resource.GetName(),
	}
}

func statusRefersToResource(status infrav1alpha.ResourceStatus, resource *unstructured.Unstructured) bool {
	gvk := resource.GroupVersionKind()
	// Version is not a stable property of a particular resource. The API version of an ASO resource may
	// change in the CAPZ spec from v1 to v2 but that still represents the same underlying resource.
	return status.Resource.Group == gvk.Group &&
		status.Resource.Kind == gvk.Kind &&
		status.Resource.Name == resource.GetName()
}
