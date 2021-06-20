/*
Copyright 2020 The Kubernetes Authors.

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
	"time"

	infraexpv1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/feature"

	aadpodv1 "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/util/identity"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/system"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AzureIdentityReconciler reconciles Azure identity objects.
type AzureIdentityReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
	WatchFilterValue string
}

// SetupWithManager initializes this controller with a manager.
func (r *AzureIdentityReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := r.Log.WithValues("controller", "AzureIdentity")
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.AzureCluster{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	// Add a watch on infraexpv1.AzureManagedControlPlane if aks is enabled.
	if feature.Gates.Enabled(feature.AKS) {
		if err = c.Watch(
			&source.Kind{Type: &infraexpv1.AzureManagedControlPlane{}},
			&handler.EnqueueRequestForObject{},
			predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue),
		); err != nil {
			return errors.Wrap(err, "failed adding a watch for ready clusters")
		}
	}

	// Add a watch on clusterv1.Cluster object for unpause notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("AzureCluster"))),
		predicates.ClusterUnpaused(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=aadpodidentity.k8s.io,resources=azureidentities;azureidentities/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=aadpodidentity.k8s.io,resources=azureidentitybindings;azureidentitybindings/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

// Reconcile reconciles the Azure identity.
func (r *AzureIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "azureIdentity", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureIdentityReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("namespace", req.Namespace),
			attribute.String("name", req.Name),
			attribute.String("kind", "AzureCluster"),
		))
	defer span.End()

	// identityOwner is the resource that created the identity. This could be either an AzureCluster or AzureManagedControlPlane (if AKS is enabled).
	// check for AzureManagedControlPlane first and if it is not found, check for AzureManagedControlPlane.
	var identityOwner interface{}

	// Fetch the AzureCluster instance
	azureCluster := &infrav1.AzureCluster{}
	identityOwner = azureCluster
	err := r.Get(ctx, req.NamespacedName, azureCluster)
	if err != nil && apierrors.IsNotFound(err) {
		if feature.Gates.Enabled(feature.AKS) {
			// Fetch the AzureManagedControlPlane instance
			azureManagedControlPlane := &infraexpv1.AzureManagedControlPlane{}
			identityOwner = azureManagedControlPlane
			err = r.Get(ctx, req.NamespacedName, azureManagedControlPlane)
			if err != nil && apierrors.IsNotFound(err) {
				r.Recorder.Eventf(azureCluster, corev1.EventTypeNormal, "AzureClusterObjectNotFound",
					fmt.Sprintf("AzureCluster object %s/%s not found", req.Namespace, req.Name))
				r.Recorder.Eventf(azureManagedControlPlane, corev1.EventTypeNormal, "AzureManagedControlPlaneObjectNotFound",
					fmt.Sprintf("AzureManagedControlPlane object %s/%s not found", req.Namespace, req.Name))
				log.Info("object was not found")
				return reconcile.Result{}, nil
			}
		} else {
			r.Recorder.Eventf(azureCluster, corev1.EventTypeNormal, "AzureClusterObjectNotFound", err.Error())
			log.Info("object was not found")
			return reconcile.Result{}, nil
		}
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	// get all the bindings
	var bindings aadpodv1.AzureIdentityBindingList
	if err := r.List(ctx, &bindings, client.InNamespace(system.GetManagerNamespace())); err != nil {
		return ctrl.Result{}, err
	}

	bindingsToDelete := []aadpodv1.AzureIdentityBinding{}
	for _, b := range bindings.Items {
		log = log.WithValues("azureidentitybinding", b.Name)

		binding := b
		clusterName := binding.ObjectMeta.Labels[clusterv1.ClusterLabelName]
		clusterNamespace := binding.ObjectMeta.Labels[infrav1.ClusterLabelNamespace]

		key := client.ObjectKey{Name: clusterName, Namespace: clusterNamespace}
		var expectedIdentityName string

		// only delete bindings when the identity owner type is not found.
		// we should not delete an identity when azureCluster is not found because it could have been created by AzureManagedControlPlane.
		switch identityOwner.(type) {
		case infrav1.AzureCluster:
			azCluster := &infrav1.AzureCluster{}
			if err := r.Get(ctx, key, azCluster); err != nil {
				if apierrors.IsNotFound(err) {
					bindingsToDelete = append(bindingsToDelete, b)
					continue
				} else {
					return ctrl.Result{}, errors.Wrap(err, "failed to get AzureCluster")
				}
			}
			expectedIdentityName = identity.GetAzureIdentityName(azCluster.Name, azCluster.Namespace, azCluster.Spec.IdentityRef.Name)
		case infraexpv1.AzureManagedControlPlane:
			azManagedControlPlane := &infraexpv1.AzureManagedControlPlane{}
			if err := r.Get(ctx, key, azManagedControlPlane); err != nil {
				if apierrors.IsNotFound(err) {
					bindingsToDelete = append(bindingsToDelete, b)
					continue
				} else {
					return ctrl.Result{}, errors.Wrap(err, "failed to get AzureManagedControlPlane")
				}
			}
			expectedIdentityName = identity.GetAzureIdentityName(azManagedControlPlane.Name, azManagedControlPlane.Namespace,
				azManagedControlPlane.Spec.IdentityRef.Name)
		}

		if binding.Spec.AzureIdentity != expectedIdentityName {
			bindingsToDelete = append(bindingsToDelete, b)
		}
	}

	// delete bindings and identites no longer used by a cluster
	for _, bindingToDelete := range bindingsToDelete {
		binding := bindingToDelete
		identityName := binding.Spec.AzureIdentity
		if err := r.Client.Delete(ctx, &binding); err != nil {
			r.Recorder.Eventf(azureCluster, corev1.EventTypeWarning, "Error reconciling AzureIdentity", err.Error())
			log.Error(err, "failed to delete AzureIdentityBinding")
			return ctrl.Result{}, err
		}
		azureIdentity := &aadpodv1.AzureIdentity{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: identityName, Namespace: system.GetManagerNamespace()}, azureIdentity); err != nil {
			log.Error(err, "failed to fetch AzureIdentity")
			return ctrl.Result{}, err
		}
		if err := r.Client.Delete(ctx, azureIdentity); err != nil {
			r.Recorder.Eventf(azureCluster, corev1.EventTypeWarning, "Error reconciling AzureIdentity", err.Error())
			log.Error(err, "failed to delete AzureIdentity")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
