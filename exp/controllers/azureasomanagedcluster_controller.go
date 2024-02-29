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

	infracontroller "sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var errInvalidControlPlaneKind = errors.New("AzureASOManagedCluster cannot be used without AzureASOManagedControlPlane")

// AzureASOManagedClusterReconciler reconciles a AzureASOManagedCluster object.
type AzureASOManagedClusterReconciler struct {
	client.Client
	WatchFilterValue string
}

// SetupWithManager sets up the controller with the Manager.
func (r *AzureASOManagedClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedClusterReconciler.SetupWithManager",
		tele.KVP("controller", infrav1exp.AzureASOManagedClusterKind),
	)
	defer done()

	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1exp.AzureASOManagedCluster{}).
		WithEventFilter(predicates.ResourceHasFilterLabel(log, r.WatchFilterValue)).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(log)).
		// Watch clusters for pause/unpause notifications
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(
				util.ClusterToInfrastructureMapFunc(ctx, infrav1exp.GroupVersion.WithKind(infrav1exp.AzureASOManagedClusterKind), mgr.GetClient(), &infrav1exp.AzureASOManagedCluster{}),
			),
			builder.WithPredicates(
				predicates.ResourceHasFilterLabel(log, r.WatchFilterValue),
				infracontroller.ClusterUpdatePauseChange(log),
			),
		).
		Build(r)
	if err != nil {
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureasomanagedclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureasomanagedclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureasomanagedclusters/finalizers,verbs=update

// Reconcile reconciles an AzureASOManagedCluster.
func (r *AzureASOManagedClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, resultErr error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedClusterReconciler.Reconcile",
		tele.KVP("namespace", req.Namespace),
		tele.KVP("name", req.Name),
		tele.KVP("kind", infrav1exp.AzureASOManagedClusterKind),
	)
	defer done()

	asoManagedCluster := &infrav1exp.AzureASOManagedCluster{}
	err := r.Get(ctx, req.NamespacedName, asoManagedCluster)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchHelper, err := patch.NewHelper(asoManagedCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patch helper: %w", err)
	}
	defer func() {
		err := patchHelper.Patch(ctx, asoManagedCluster)
		if err != nil && resultErr == nil {
			resultErr = err
			result = ctrl.Result{}
		}
	}()

	cluster, err := util.GetOwnerCluster(ctx, r.Client, asoManagedCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	if cluster != nil && cluster.Spec.Paused ||
		annotations.HasPaused(asoManagedCluster) {
		return r.reconcilePaused(ctx, asoManagedCluster, cluster)
	}

	if !asoManagedCluster.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(ctx, asoManagedCluster)
	}

	return r.reconcileNormal(ctx, asoManagedCluster, cluster)
}

func (r *AzureASOManagedClusterReconciler) reconcileNormal(ctx context.Context, asoManagedCluster *infrav1exp.AzureASOManagedCluster, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	//nolint:all // ctx will be used soon
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedClusterReconciler.reconcileNormal",
	)
	defer done()
	log.V(4).Info("reconciling normally")

	if cluster == nil {
		log.V(4).Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}
	if cluster.Spec.ControlPlaneRef == nil ||
		cluster.Spec.ControlPlaneRef.APIVersion != infrav1exp.GroupVersion.Identifier() ||
		cluster.Spec.ControlPlaneRef.Kind != infrav1exp.AzureASOManagedControlPlaneKind {
		return ctrl.Result{}, reconcile.TerminalError(errInvalidControlPlaneKind)
	}

	needsPatch := controllerutil.AddFinalizer(asoManagedCluster, clusterv1.ClusterFinalizer)
	if needsPatch {
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

//nolint:unparam // these parameters will be used soon enough
func (r *AzureASOManagedClusterReconciler) reconcilePaused(ctx context.Context, asoManagedCluster *infrav1exp.AzureASOManagedCluster, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	//nolint:all // ctx will be used soon
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedClusterReconciler.reconcilePaused",
	)
	defer done()
	log.V(4).Info("reconciling pause")

	return ctrl.Result{}, nil
}

//nolint:unparam // these parameters will be used soon enough
func (r *AzureASOManagedClusterReconciler) reconcileDelete(ctx context.Context, asoManagedCluster *infrav1exp.AzureASOManagedCluster) (ctrl.Result, error) {
	//nolint:all // ctx will be used soon
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedClusterReconciler.reconcileDelete",
	)
	defer done()
	log.V(4).Info("reconciling delete")

	controllerutil.RemoveFinalizer(asoManagedCluster, clusterv1.ClusterFinalizer)
	return ctrl.Result{}, nil
}
