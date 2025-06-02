/*
Copyright 2025 The Kubernetes Authors.

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/controllers"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav2exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/coalescing"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AROMachinePoolReconciler reconciles an AROMachinePool object.
type AROMachinePoolReconciler struct {
	client.Client
	Timeouts                    reconciler.Timeouts
	WatchFilterValue            string
	CredentialCache             azure.CredentialCache
	createAROMachinePoolService aroMachinePoolServiceCreator
}

type aroMachinePoolServiceCreator func(aroMachinePoolScope *scope.AROMachinePoolScope, apiCallTimeout time.Duration) (*aroMachinePoolService, error)

// NewAROMachinePoolReconciler returns a new AROMachinePoolReconciler instance.
func NewAROMachinePoolReconciler(client client.Client, _ record.EventRecorder, timeouts reconciler.Timeouts, watchFilterValue string, credCache azure.CredentialCache) *AROMachinePoolReconciler {
	ampr := &AROMachinePoolReconciler{
		Client: client,
		//Recorder:         recorder,
		Timeouts:         timeouts,
		WatchFilterValue: watchFilterValue,
		CredentialCache:  credCache,
	}

	ampr.createAROMachinePoolService = newAROMachinePoolService

	return ampr
}

// SetupWithManager sets up the controller with the Manager.
func (ampr *AROMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controllers.Options) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROMachinePoolReconciler.SetupWithManager",
		tele.KVP("controller", infrav2exp.AROMachinePoolKind),
	)
	defer done()

	var r reconcile.Reconciler = ampr
	if options.Cache != nil {
		r = coalescing.NewReconciler(ampr, options.Cache, log)
	}

	aroMachinePool := &infrav2exp.AROMachinePool{}
	// create mapper to transform incoming AroControlPlanes into AROMachinePool requests
	aroControlPlaneMapper, err := AROControlPlaneToAROMachinePoolsMapper(ctx, ampr.Client, mgr.GetScheme(), log)
	if err != nil {
		return errors.Wrap(err, "failed to create AROControlPlane to AROMachinePools mapper")
	}

	aroMachinePoolMapper, err := util.ClusterToTypedObjectsMapper(ampr.Client, &infrav2exp.AROMachinePoolList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrap(err, "failed to create mapper for Cluster to AROMachinePools")
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options.Options).
		For(aroMachinePool).
		WithEventFilter(predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, ampr.WatchFilterValue)).
		// watch for changes in CAPI MachinePool resources
		Watches(
			&clusterv1.MachinePool{},
			handler.EnqueueRequestsFromMapFunc(AROMachinePoolToInfrastructureMapFunc(infrav2exp.GroupVersion.WithKind("AROMachinePool"), log)),
		).
		// watch for changes in AROControlPlanes
		Watches(
			&cplane.AROControlPlane{},
			handler.EnqueueRequestsFromMapFunc(aroControlPlaneMapper),
		).
		// Add a watch on clusterv1.Cluster object for pause/unpause & ready notifications.
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(aroMachinePoolMapper),
			builder.WithPredicates(
				predicates.ClusterPausedTransitions(mgr.GetScheme(), log),
				predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, ampr.WatchFilterValue),
			),
		).
		Complete(r)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools/finalizers,verbs=update
// +kubebuilder:rbac:groups=redhatopenshift.azure.com,resources=hcpopenshiftclustersnodepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redhatopenshift.azure.com,resources=hcpopenshiftclustersnodepools/status,verbs=get;list;watch

// Reconcile idempotently gets, creates, and updates a machine pool.
func (ampr *AROMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, ampr.Timeouts.DefaultedLoopTimeout())
	defer cancel()

	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AROMachinePoolReconciler.Reconcile",
		tele.KVP("namespace", req.Namespace),
		tele.KVP("name", req.Name),
		tele.KVP("kind", infrav2exp.AROMachinePoolKind),
	)
	defer done()

	// Fetch the AROMachinePool instance
	infraPool := &infrav2exp.AROMachinePool{}
	err := ampr.Get(ctx, req.NamespacedName, infraPool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Handle deletion early - before requiring owner references
	// During deletion, the MachinePool may already be deleted
	if !infraPool.DeletionTimestamp.IsZero() {
		return ampr.reconcileDeleteWithoutScope(ctx, log, infraPool)
	}

	// Fetch the owning MachinePool.
	ownerPool, err := controllers.GetOwnerMachinePool(ctx, ampr.Client, infraPool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if ownerPool == nil {
		log.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	// Fetch the Cluster.
	ownerCluster, err := util.GetOwnerCluster(ctx, ampr.Client, ownerPool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if ownerCluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("ownerCluster", ownerCluster.Name)

	// Check if ControlPlaneRef is defined
	if !ownerCluster.Spec.ControlPlaneRef.IsDefined() {
		log.Info("Cluster ControlPlaneRef is not yet defined")
		return reconcile.Result{}, nil
	}

	// Fetch the corresponding control plane which has all the interesting data.
	controlPlane := &cplane.AROControlPlane{}
	controlPlaneName := client.ObjectKey{
		Namespace: ownerCluster.Namespace,
		Name:      ownerCluster.Spec.ControlPlaneRef.Name,
	}
	if err := ampr.Client.Get(ctx, controlPlaneName, controlPlane); err != nil {
		return reconcile.Result{}, err
	}

	// Upon first create of an AKS service, the node pools are provided to the CreateOrUpdate call. After the initial
	// create of the control plane and node pools, the control plane will transition to initialized. After the control
	// plane is initialized, we can proceed to reconcile aro machine pools.
	if controlPlane.Status.Initialization == nil || !controlPlane.Status.Initialization.ControlPlaneInitialized {
		log.Info("AROControlPlane is not initialized")
		return reconcile.Result{}, nil
	}

	// create the aro control plane scope
	aroControlPlaneScope, err := scope.NewAROControlPlaneScope(ctx, scope.AROControlPlaneScopeParams{
		Client:          ampr.Client,
		ControlPlane:    controlPlane,
		Cluster:         ownerCluster,
		Timeouts:        ampr.Timeouts,
		CredentialCache: ampr.CredentialCache,
	})
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create AROControlPlane scope")
	}

	// Create the scope.
	acpScope, err := scope.NewAROMachinePoolScope(ctx, scope.AROMachinePoolScopeParams{
		Client:               ampr.Client,
		Cluster:              ownerCluster,
		MachinePool:          ownerPool,
		ControlPlane:         controlPlane,
		AROMachinePool:       infraPool,
		Cache:                nil,
		AROControlPlaneScope: aroControlPlaneScope,
		Timeouts:             ampr.Timeouts,
		CredentialCache:      ampr.CredentialCache,
	})
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create AROMachinePool scope")
	}

	// Always patch when exiting so we can persist changes to finalizers and status
	defer func() {
		if err := acpScope.PatchObject(ctx); err != nil && resultErr == nil {
			resultErr = err
			result = reconcile.Result{}
		}
		if err := acpScope.PatchCAPIMachinePoolObject(ctx); err != nil && resultErr == nil {
			resultErr = err
			result = reconcile.Result{}
		}
	}()

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(ownerCluster, infraPool) {
		log.Info("AROMachinePool or linked Cluster is marked as paused. Won't reconcile normally")
		return ampr.reconcilePause(ctx, acpScope)
	}

	// Handle non-deleted clusters
	return ampr.reconcileNormal(ctx, acpScope)
}

func (ampr *AROMachinePoolReconciler) reconcileNormal(ctx context.Context, scope *scope.AROMachinePoolScope) (reconcile.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AROMachinePoolReconciler.reconcileNormal")
	defer done()

	log.Info("Reconciling AROMachinePool")

	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	needsPatch := controllerutil.AddFinalizer(scope.InfraMachinePool, infrav2exp.AROMachinePoolFinalizer)
	// Register the block-move annotation immediately to avoid moving un-paused ASO resources
	needsPatch = controllers.AddBlockMoveAnnotation(scope.InfraMachinePool) || needsPatch
	if needsPatch {
		if err := scope.PatchObject(ctx); err != nil {
			return reconcile.Result{}, err
		}
	}

	svc, err := ampr.createAROMachinePoolService(scope, ampr.Timeouts.DefaultedAzureServiceReconcileTimeout())
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create an AROMachinePoolService")
	}

	if err := svc.Reconcile(ctx); err != nil {
		scope.SetAgentPoolReady(false)
		// Always set the error condition to ensure validation errors are surfaced
		conditions.Set(scope.InfraMachinePool, metav1.Condition{
			Type:    string(infrav1.AgentPoolsReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.FailedReason,
			Message: err.Error(),
		})

		// Handle transient and terminal errors
		log := log.WithValues("name", scope.InfraMachinePool.Name, "namespace", scope.InfraMachinePool.Namespace)
		var reconcileError azure.ReconcileError
		if errors.As(err, &reconcileError) {
			if reconcileError.IsTerminal() {
				log.Error(err, "failed to reconcile AROMachinePool")
				return reconcile.Result{}, nil
			}

			if reconcileError.IsTransient() {
				log.V(4).Info("requeuing due to transient failure", "error", err)
				if scope.InfraMachinePool.Status.ProvisioningState == infrav1.UpdatingReason {
					scope.SetAgentPoolReady(true)
				}
				return reconcile.Result{RequeueAfter: reconcileError.RequeueAfter()}, nil
			}

			return reconcile.Result{}, errors.Wrap(err, "failed to reconcile AROMachinePool")
		}

		return reconcile.Result{}, errors.Wrapf(err, "error creating AROMachinePool %s/%s", scope.InfraMachinePool.Namespace, scope.InfraMachinePool.Name)
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	scope.SetAgentPoolReady(true)
	conditions.Set(scope.InfraMachinePool, metav1.Condition{
		Type:   string(infrav1.AgentPoolsReadyCondition),
		Status: metav1.ConditionTrue,
		Reason: "Succeeded",
	})
	return reconcile.Result{}, nil
}

func (ampr *AROMachinePoolReconciler) reconcilePause(ctx context.Context, scope *scope.AROMachinePoolScope) (reconcile.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AROMachinePool.reconcilePause")
	defer done()

	log.Info("Reconciling AROMachinePool pause")

	svc, err := ampr.createAROMachinePoolService(scope, ampr.Timeouts.DefaultedAzureServiceReconcileTimeout())
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create an AROMachinePoolService")
	}

	if err := svc.Pause(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error pausing AROMachinePool %s/%s", scope.InfraMachinePool.Namespace, scope.InfraMachinePool.Name)
	}
	controllers.RemoveBlockMoveAnnotation(scope.InfraMachinePool)

	return reconcile.Result{}, nil
}

// reconcileDeleteWithoutScope handles deletion when owner references may not be available.
// This can happen when the MachinePool or Cluster has already been deleted.
func (ampr *AROMachinePoolReconciler) reconcileDeleteWithoutScope(ctx context.Context, log logr.Logger, infraPool *infrav2exp.AROMachinePool) (reconcile.Result, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.AROMachinePoolReconciler.reconcileDeleteWithoutScope")
	defer done()

	log.Info("Reconciling AROMachinePool delete without full scope")

	// If the AROMachinePool is being deleted and we can't get the owner references,
	// we'll just remove the finalizer. The Azure resources may have already been cleaned up
	// when the cluster was deleted, or they'll be cleaned up by Azure's garbage collection.
	// This prevents the AROMachinePool from being stuck in deletion when its owner is gone.

	patchHelper, err := patch.NewHelper(infraPool, ampr.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create patch helper")
	}

	controllerutil.RemoveFinalizer(infraPool, infrav2exp.AROMachinePoolFinalizer)

	if err := patchHelper.Patch(ctx, infraPool); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to patch AROMachinePool")
	}

	log.Info("Successfully removed finalizer from AROMachinePool")
	return reconcile.Result{}, nil
}
