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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	infracontroller "sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/coalescing"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AzureManagedMachinePoolReconciler reconciles an AzureManagedMachinePool object.
type AzureManagedMachinePoolReconciler struct {
	client.Client
	Recorder                             record.EventRecorder
	ReconcileTimeout                     time.Duration
	WatchFilterValue                     string
	createAzureManagedMachinePoolService azureManagedMachinePoolServiceCreator
}

type azureManagedMachinePoolServiceCreator func(managedMachinePoolScope *scope.ManagedMachinePoolScope) (*azureManagedMachinePoolService, error)

// NewAzureManagedMachinePoolReconciler returns a new AzureManagedMachinePoolReconciler instance.
func NewAzureManagedMachinePoolReconciler(client client.Client, recorder record.EventRecorder, reconcileTimeout time.Duration, watchFilterValue string) *AzureManagedMachinePoolReconciler {
	ampr := &AzureManagedMachinePoolReconciler{
		Client:           client,
		Recorder:         recorder,
		ReconcileTimeout: reconcileTimeout,
		WatchFilterValue: watchFilterValue,
	}

	ampr.createAzureManagedMachinePoolService = newAzureManagedMachinePoolService

	return ampr
}

// SetupWithManager initializes this controller with a manager.
func (ammpr *AzureManagedMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options infracontroller.Options) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureManagedMachinePoolReconciler.SetupWithManager",
		tele.KVP("controller", "AzureManagedMachinePool"),
	)
	defer done()

	var r reconcile.Reconciler = ammpr
	if options.Cache != nil {
		r = coalescing.NewReconciler(ammpr, options.Cache, log)
	}

	azManagedMachinePool := &infrav1exp.AzureManagedMachinePool{}
	// create mapper to transform incoming AzureManagedControlPlanes into AzureManagedMachinePool requests
	azureManagedControlPlaneMapper, err := AzureManagedControlPlaneToAzureManagedMachinePoolsMapper(ctx, ammpr.Client, mgr.GetScheme(), log)
	if err != nil {
		return errors.Wrap(err, "failed to create AzureManagedControlPlane to AzureManagedMachinePools mapper")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options.Options).
		For(azManagedMachinePool).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, ammpr.WatchFilterValue)).
		// watch for changes in CAPI MachinePool resources
		Watches(
			&source.Kind{Type: &clusterv1exp.MachinePool{}},
			handler.EnqueueRequestsFromMapFunc(MachinePoolToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureManagedMachinePool"), log)),
		).
		// watch for changes in AzureManagedControlPlanes
		Watches(
			&source.Kind{Type: &infrav1exp.AzureManagedControlPlane{}},
			handler.EnqueueRequestsFromMapFunc(azureManagedControlPlaneMapper),
		).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureManagedMachinePool"))),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
		predicates.ResourceNotPausedAndHasFilterLabel(log, ammpr.WatchFilterValue),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch

// Reconcile idempotently gets, creates, and updates a machine pool.
func (ammpr *AzureManagedMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(ammpr.ReconcileTimeout))
	defer cancel()

	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AzureManagedMachinePoolReconciler.Reconcile",
		tele.KVP("namespace", req.Namespace),
		tele.KVP("name", req.Name),
		tele.KVP("kind", "AzureManagedMachinePool"),
	)
	defer done()

	// Fetch the AzureManagedMachinePool instance
	infraPool := &infrav1exp.AzureManagedMachinePool{}
	err := ammpr.Get(ctx, req.NamespacedName, infraPool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the owning MachinePool.
	ownerPool, err := infracontroller.GetOwnerMachinePool(ctx, ammpr.Client, infraPool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if ownerPool == nil {
		log.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	// Fetch the Cluster.
	ownerCluster, err := util.GetOwnerCluster(ctx, ammpr.Client, ownerPool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if ownerCluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("ownerCluster", ownerCluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(ownerCluster, infraPool) {
		log.Info("AzureManagedMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	// Fetch the corresponding control plane which has all the interesting data.
	controlPlane := &infrav1exp.AzureManagedControlPlane{}
	controlPlaneName := client.ObjectKey{
		Namespace: ownerCluster.Spec.ControlPlaneRef.Namespace,
		Name:      ownerCluster.Spec.ControlPlaneRef.Name,
	}
	if err := ammpr.Client.Get(ctx, controlPlaneName, controlPlane); err != nil {
		return reconcile.Result{}, err
	}

	// Upon first create of an AKS service, the node pools are provided to the CreateOrUpdate call. After the initial
	// create of the control plane and node pools, the control plane will transition to initialized. After the control
	// plane is initialized, we can proceed to reconcile managed machine pools.
	if !controlPlane.Status.Initialized {
		log.Info("AzureManagedControlPlane is not initialized")
		return reconcile.Result{}, nil
	}

	// create the managed control plane scope
	managedControlPlaneScope, err := scope.NewManagedControlPlaneScope(ctx, scope.ManagedControlPlaneScopeParams{
		Client:       ammpr.Client,
		ControlPlane: controlPlane,
		Cluster:      ownerCluster,
	})
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create ManagedControlPlane scope")
	}

	// Create the scope.
	mcpScope, err := scope.NewManagedMachinePoolScope(ctx, scope.ManagedMachinePoolScopeParams{
		Client:       ammpr.Client,
		ControlPlane: controlPlane,
		Cluster:      ownerCluster,
		ManagedMachinePool: scope.ManagedMachinePool{
			MachinePool:      ownerPool,
			InfraMachinePool: infraPool,
		},
		ManagedControlPlaneScope: managedControlPlaneScope,
	})
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create ManagedMachinePool scope")
	}

	// Always patch when exiting so we can persist changes to finalizers and status
	defer func() {
		if err := mcpScope.PatchObject(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !infraPool.DeletionTimestamp.IsZero() {
		return ammpr.reconcileDelete(ctx, mcpScope)
	}

	// Handle non-deleted clusters
	return ammpr.reconcileNormal(ctx, mcpScope)
}

func (ammpr *AzureManagedMachinePoolReconciler) reconcileNormal(ctx context.Context, scope *scope.ManagedMachinePoolScope) (reconcile.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AzureManagedMachinePoolReconciler.reconcileNormal")
	defer done()

	log.Info("Reconciling AzureManagedMachinePool")

	// If the AzureManagedMachinePool doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(scope.InfraMachinePool, infrav1.ClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := scope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	svc, err := ammpr.createAzureManagedMachinePoolService(scope)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create an AzureManageMachinePoolService")
	}

	if err := svc.Reconcile(ctx); err != nil {
		// Handle transient and terminal errors
		log := log.WithValues("name", scope.InfraMachinePool.Name, "namespace", scope.InfraMachinePool.Namespace)
		var reconcileError azure.ReconcileError
		if errors.As(err, &reconcileError) {
			if reconcileError.IsTerminal() {
				log.Error(err, "failed to reconcile AzureManagedMachinePool")
				return reconcile.Result{}, nil
			}

			if reconcileError.IsTransient() {
				log.V(4).Info("requeuing due to transient transient failure", "error", err)
				return reconcile.Result{RequeueAfter: reconcileError.RequeueAfter()}, nil
			}

			return reconcile.Result{}, errors.Wrap(err, "failed to reconcile AzureManagedMachinePool")
		}

		return reconcile.Result{}, errors.Wrapf(err, "error creating AzureManagedMachinePool %s/%s", scope.InfraMachinePool.Namespace, scope.InfraMachinePool.Name)
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	scope.InfraMachinePool.Status.Ready = true
	ammpr.Recorder.Eventf(scope.InfraMachinePool, corev1.EventTypeNormal, "AzureManagedMachinePool available", "node pool successfully reconciled")
	return reconcile.Result{}, nil
}

func (ammpr *AzureManagedMachinePoolReconciler) reconcileDelete(ctx context.Context, scope *scope.ManagedMachinePoolScope) (reconcile.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AzureManagedMachinePoolReconciler.reconcileDelete")
	defer done()

	log.Info("Reconciling AzureManagedMachinePool delete")

	if !scope.Cluster.DeletionTimestamp.IsZero() {
		// Cluster was deleted, skip machine pool deletion and let AKS delete the whole cluster.
		// So, remove the finalizer.
		controllerutil.RemoveFinalizer(scope.InfraMachinePool, infrav1.ClusterFinalizer)
	} else {
		svc, err := ammpr.createAzureManagedMachinePoolService(scope)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to create an AzureManageMachinePoolService")
		}

		if err := svc.Delete(ctx); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "error deleting AzureManagedMachinePool %s/%s", scope.InfraMachinePool.Namespace, scope.InfraMachinePool.Name)
		}
		// Machine pool successfully deleted, remove the finalizer.
		controllerutil.RemoveFinalizer(scope.InfraMachinePool, infrav1.ClusterFinalizer)
	}

	if err := scope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
