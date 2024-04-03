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

	infracontroller "sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	utilexp "sigs.k8s.io/cluster-api/exp/util"
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

// AzureASOManagedMachinePoolReconciler reconciles a AzureASOManagedMachinePool object.
type AzureASOManagedMachinePoolReconciler struct {
	client.Client
	WatchFilterValue string
}

// SetupWithManager sets up the controller with the Manager.
func (r *AzureASOManagedMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	_, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedMachinePoolReconciler.SetupWithManager",
		tele.KVP("controller", infrav1exp.AzureASOManagedMachinePoolKind),
	)
	defer done()

	clusterToAzureASOManagedMachinePools, err := util.ClusterToTypedObjectsMapper(mgr.GetClient(), &infrav1exp.AzureASOManagedMachinePoolList{}, mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("failed to get Cluster to AzureASOManagedMachinePool mapper: %w", err)
	}

	_, err = ctrl.NewControllerManagedBy(mgr).
		For(&infrav1exp.AzureASOManagedMachinePool{}).
		WithEventFilter(predicates.ResourceHasFilterLabel(log, r.WatchFilterValue)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToAzureASOManagedMachinePools),
			builder.WithPredicates(
				predicates.ResourceHasFilterLabel(log, r.WatchFilterValue),
				predicates.Any(log,
					predicates.ClusterControlPlaneInitialized(log),
					infracontroller.ClusterUpdatePauseChange(log),
				),
			),
		).
		Watches(
			&expv1.MachinePool{},
			handler.EnqueueRequestsFromMapFunc(utilexp.MachinePoolToInfrastructureMapFunc(
				infrav1exp.GroupVersion.WithKind(infrav1exp.AzureASOManagedMachinePoolKind), log),
			),
			builder.WithPredicates(
				predicates.ResourceHasFilterLabel(log, r.WatchFilterValue),
			),
		).
		Build(r)
	if err != nil {
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureasomanagedmachinepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureasomanagedmachinepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureasomanagedmachinepools/finalizers,verbs=update

// Reconcile reconciles an AzureASOManagedMachinePool.
func (r *AzureASOManagedMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, resultErr error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedMachinePoolReconciler.Reconcile",
		tele.KVP("namespace", req.Namespace),
		tele.KVP("name", req.Name),
		tele.KVP("kind", infrav1exp.AzureASOManagedMachinePoolKind),
	)
	defer done()

	asoManagedMachinePool := &infrav1exp.AzureASOManagedMachinePool{}
	err := r.Get(ctx, req.NamespacedName, asoManagedMachinePool)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchHelper, err := patch.NewHelper(asoManagedMachinePool, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patch helper: %w", err)
	}
	defer func() {
		err := patchHelper.Patch(ctx, asoManagedMachinePool)
		if err != nil && resultErr == nil {
			resultErr = err
			result = ctrl.Result{}
		}
	}()

	machinePool, err := utilexp.GetOwnerMachinePool(ctx, r.Client, asoManagedMachinePool.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machinePool == nil {
		log.V(4).Info("Waiting for MachinePool Controller to set OwnerRef on AzureASOManagedMachinePool")
		return ctrl.Result{}, nil
	}

	machinePoolBefore := machinePool.DeepCopy()
	defer func() {
		// Skip using a patch helper here because we will never modify the MachinePool status.
		err := r.Patch(ctx, machinePool, client.MergeFrom(machinePoolBefore))
		if err != nil && resultErr == nil {
			resultErr = err
			result = ctrl.Result{}
		}
	}()

	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("AzureASOManagedMachinePool owner MachinePool is missing cluster label or cluster does not exist: %w", err)
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Waiting for MachinePool controller to set %s label on MachinePool", clusterv1.ClusterNameLabel))
		return ctrl.Result{}, nil
	}
	if cluster.Spec.ControlPlaneRef == nil ||
		cluster.Spec.ControlPlaneRef.APIVersion != infrav1exp.GroupVersion.Identifier() ||
		cluster.Spec.ControlPlaneRef.Kind != infrav1exp.AzureASOManagedControlPlaneKind {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("AzureASOManagedMachinePool cannot be used without AzureASOManagedControlPlane"))
	}

	if annotations.IsPaused(cluster, asoManagedMachinePool) {
		return r.reconcilePause(ctx, asoManagedMachinePool, cluster)
	}

	if !asoManagedMachinePool.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, asoManagedMachinePool, cluster)
	}

	return r.reconcileNormal(ctx, asoManagedMachinePool, machinePool, cluster)
}

//nolint:unparam // these parameters will be used soon enough
func (r *AzureASOManagedMachinePoolReconciler) reconcileNormal(ctx context.Context, asoManagedMachinePool *infrav1exp.AzureASOManagedMachinePool, machinePool *expv1.MachinePool, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	//nolint:all // ctx will be used soon
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedMachinePoolReconciler.reconcileNormal",
	)
	defer done()
	log.V(4).Info("reconciling normally")

	needsPatch := controllerutil.AddFinalizer(asoManagedMachinePool, clusterv1.ClusterFinalizer)
	if needsPatch {
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

//nolint:unparam // these parameters will be used soon enough
func (r *AzureASOManagedMachinePoolReconciler) reconcilePause(ctx context.Context, asoManagedMachinePool *infrav1exp.AzureASOManagedMachinePool, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	//nolint:all // ctx will be used soon
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedMachinePoolReconciler.reconcilePaused",
	)
	defer done()
	log.V(4).Info("reconciling pause")

	return ctrl.Result{}, nil
}

//nolint:unparam // these parameters will be used soon enough
func (r *AzureASOManagedMachinePoolReconciler) reconcileDelete(ctx context.Context, asoManagedMachinePool *infrav1exp.AzureASOManagedMachinePool, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	//nolint:all // ctx will be used soon
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureASOManagedMachinePoolReconciler.reconcileDelete",
	)
	defer done()
	log.V(4).Info("reconciling delete")

	controllerutil.RemoveFinalizer(asoManagedMachinePool, clusterv1.ClusterFinalizer)
	return ctrl.Result{}, nil
}
