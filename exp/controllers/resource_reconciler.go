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
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ResourceReconciler reconciles a set of arbitrary ASO resources.
type ResourceReconciler struct {
	client.Client
	resources []*unstructured.Unstructured
	owner     resourceStatusObject
	watcher   watcher
}

type watcher interface {
	Watch(log logr.Logger, obj runtime.Object, handler handler.EventHandler, p ...predicate.Predicate) error
}

type resourceStatusObject interface {
	client.Object
	// methods will be added here which help track the status of resources.
}

// Reconcile creates or updates the specified resources.
func (r *ResourceReconciler) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.Reconcile")
	defer done()
	log.V(4).Info("reconciling resources")

	for _, spec := range r.resources {
		gvk := spec.GroupVersionKind()
		spec.SetNamespace(r.owner.GetNamespace())

		log := log.WithValues("resource", klog.KObj(spec), "resourceVersion", gvk.GroupVersion(), "resourceKind", gvk.Kind)

		if err := controllerutil.SetControllerReference(r.owner, spec, r.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}

		if err := r.watcher.Watch(log, spec, handler.EnqueueRequestForOwner(r.Client.Scheme(), r.Client.RESTMapper(), r.owner)); err != nil {
			return fmt.Errorf("failed to watch resource: %w", err)
		}

		log.V(4).Info("applying resource")
		err := r.Patch(ctx, spec, client.Apply, client.FieldOwner("capz-manager"), client.ForceOwnership)
		if err != nil {
			return fmt.Errorf("failed to apply resource: %w", err)
		}
	}

	return nil
}

// Delete deletes the specified resources.
func (r *ResourceReconciler) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.ResourceReconciler.Delete")
	defer done()
	log.V(4).Info("deleting resources")

	for _, spec := range r.resources {
		spec.SetNamespace(r.owner.GetNamespace())
		gvk := spec.GroupVersionKind()

		log := log.WithValues("resource", klog.KObj(spec), "resourceVersion", gvk.GroupVersion(), "resourceKind", gvk.Kind)

		log.V(4).Info("deleting resource")
		err := r.Client.Delete(ctx, spec)
		if apierrors.IsNotFound(err) {
			log.V(4).Info("resource has been deleted")
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to delete resource: %w", err)
		}
	}

	return nil
}
