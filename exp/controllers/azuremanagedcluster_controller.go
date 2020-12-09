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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureManagedClusterReconciler reconciles a AzureManagedCluster object
type AzureManagedClusterReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
}

// SetupWithManager initializes this controller with a manager.
func (r *AzureManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	log := r.Log.WithValues("controller", "AzureManagedCluster")
	azManagedCluster := &infrav1exp.AzureManagedCluster{}
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(azManagedCluster).
		WithEventFilter(predicates.ResourceNotPaused(log)). // don't queue reconcile if resource is paused
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: util.ClusterToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureManagedCluster")),
		},
		predicates.ClusterUnpaused(log),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}
	return nil
}

// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremanagedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremanagedclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile idempotently gets, creates, and updates a managed cluster.
func (r *AzureManagedClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "azureManagedCluster", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureManagedClusterReconciler.Reconcile",
		trace.WithAttributes(
			label.String("namespace", req.Namespace),
			label.String("name", req.Name),
			label.String("kind", "AzureManagedCluster"),
		))
	defer span.End()

	// Fetch the AzureManagedCluster instance
	aksCluster := &infrav1exp.AzureManagedCluster{}
	err := r.Get(ctx, req.NamespacedName, aksCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, aksCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	controlPlane := &infrav1exp.AzureManagedControlPlane{}
	controlPlaneRef := types.NamespacedName{
		Name:      cluster.Spec.ControlPlaneRef.Name,
		Namespace: cluster.Namespace,
	}

	log = log.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, aksCluster) {
		log.Info("AzureManagedCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	if err := r.Get(ctx, controlPlaneRef, controlPlane); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get control plane ref")
	}

	log = log.WithValues("controlPlane", controlPlaneRef.Name)

	patchhelper, err := patch.NewHelper(aksCluster, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to init patch helper")
	}

	// Match whatever the control plane says. We should also enqueue
	// requests from control plane to infra cluster to keep this accurate
	aksCluster.Status.Ready = controlPlane.Status.Ready
	aksCluster.Spec.ControlPlaneEndpoint = controlPlane.Spec.ControlPlaneEndpoint

	if err := patchhelper.Patch(ctx, aksCluster); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Successfully reconciled")

	return reconcile.Result{}, nil
}
