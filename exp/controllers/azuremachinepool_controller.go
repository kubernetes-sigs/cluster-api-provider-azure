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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/coalescing"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	infracontroller "sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	// AzureMachinePoolReconciler reconciles an AzureMachinePool object.
	AzureMachinePoolReconciler struct {
		client.Client
		Log                           logr.Logger
		Scheme                        *runtime.Scheme
		Recorder                      record.EventRecorder
		ReconcileTimeout              time.Duration
		WatchFilterValue              string
		createAzureMachinePoolService azureMachinePoolServiceCreator
	}

	// annotationReaderWriter provides an interface to read and write annotations.
	annotationReaderWriter interface {
		GetAnnotations() map[string]string
		SetAnnotations(annotations map[string]string)
	}
)

type azureMachinePoolServiceCreator func(machinePoolScope *scope.MachinePoolScope) (*azureMachinePoolService, error)

// NewAzureMachinePoolReconciler returns a new AzureMachinePoolReconciler instance.
func NewAzureMachinePoolReconciler(client client.Client, log logr.Logger, recorder record.EventRecorder, reconcileTimeout time.Duration, watchFilterValue string) *AzureMachinePoolReconciler {
	ampr := &AzureMachinePoolReconciler{
		Client:           client,
		Log:              log,
		Recorder:         recorder,
		ReconcileTimeout: reconcileTimeout,
		WatchFilterValue: watchFilterValue,
	}

	ampr.createAzureMachinePoolService = newAzureMachinePoolService

	return ampr
}

// SetupWithManager initializes this controller with a manager.
func (ampr *AzureMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options infracontroller.Options) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachinePoolReconciler.SetupWithManager")
	defer span.End()

	log := ampr.Log.WithValues("controller", "AzureMachinePool")
	var r reconcile.Reconciler = ampr
	if options.Cache != nil {
		r = coalescing.NewReconciler(ampr, options.Cache, log)
	}

	// create mapper to transform incoming AzureClusters into AzureMachinePool requests
	azureClusterMapper, err := AzureClusterToAzureMachinePoolsMapper(ctx, ampr.Client, mgr.GetScheme(), log)
	if err != nil {
		return errors.Wrapf(err, "failed to create AzureCluster to AzureMachinePools mapper")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options.Options).
		For(&infrav1exp.AzureMachinePool{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), ampr.WatchFilterValue)).
		// watch for changes in CAPI MachinePool resources
		Watches(
			&source.Kind{Type: &capiv1exp.MachinePool{}},
			handler.EnqueueRequestsFromMapFunc(MachinePoolToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureMachinePool"), log)),
		).
		// watch for changes in AzureCluster resources
		Watches(
			&source.Kind{Type: &infrav1.AzureCluster{}},
			handler.EnqueueRequestsFromMapFunc(azureClusterMapper),
		).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	if err := c.Watch(
		&source.Kind{Type: &infrav1exp.AzureMachinePoolMachine{}},
		handler.EnqueueRequestsFromMapFunc(AzureMachinePoolMachineMapper(mgr.GetScheme(), log)),
		MachinePoolMachineHasStateOrVersionChange(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for AzureMachinePoolMachine")
	}

	azureMachinePoolMapper, err := util.ClusterToObjectsMapper(ampr.Client, &infrav1exp.AzureMachinePoolList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrap(err, "failed to create mapper for Cluster to AzureMachines")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(azureMachinePoolMapper),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepoolmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepoolmachines/status,verbs=get
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

// Reconcile idempotently gets, creates, and updates a machine pool.
func (ampr *AzureMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(ampr.ReconcileTimeout))
	defer cancel()

	logger := ampr.Log.WithValues("namespace", req.Namespace, "azureMachinePool", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachinePoolReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("namespace", req.Namespace),
			attribute.String("name", req.Name),
			attribute.String("kind", "AzureMachinePool"),
		))
	defer span.End()

	azMachinePool := &infrav1exp.AzureMachinePool{}
	err := ampr.Get(ctx, req.NamespacedName, azMachinePool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the CAPI MachinePool.
	machinePool, err := infracontroller.GetOwnerMachinePool(ctx, ampr.Client, azMachinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machinePool == nil {
		logger.V(2).Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("machinePool", machinePool.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, ampr.Client, machinePool.ObjectMeta)
	if err != nil {
		logger.V(2).Info("MachinePool is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, azMachinePool) {
		logger.V(2).Info("AzureMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	azureClusterName := client.ObjectKey{
		Namespace: azMachinePool.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	azureCluster := &infrav1.AzureCluster{}
	if err := ampr.Client.Get(ctx, azureClusterName, azureCluster); err != nil {
		logger.V(2).Info("AzureCluster is not available yet")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("AzureCluster", azureCluster.Name)

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client:       ampr.Client,
		Logger:       logger,
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Create the machine pool scope
	machinePoolScope, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Logger:           logger,
		Client:           ampr.Client,
		MachinePool:      machinePool,
		AzureMachinePool: azMachinePool,
		ClusterScope:     clusterScope,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any AzureMachine changes.
	defer func() {
		if err := machinePoolScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machine pools
	if !azMachinePool.ObjectMeta.DeletionTimestamp.IsZero() {
		return ampr.reconcileDelete(ctx, machinePoolScope, clusterScope)
	}

	// Handle non-deleted machine pools
	return ampr.reconcileNormal(ctx, machinePoolScope, clusterScope)
}

func (ampr *AzureMachinePoolReconciler) reconcileNormal(ctx context.Context, machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (_ reconcile.Result, reterr error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachinePoolReconciler.reconcileNormal")
	defer span.End()

	machinePoolScope.Info("Reconciling AzureMachinePool")
	// If the AzureMachine is in an error state, return early.
	if machinePoolScope.AzureMachinePool.Status.FailureReason != nil || machinePoolScope.AzureMachinePool.Status.FailureMessage != nil {
		machinePoolScope.Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the AzureMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machinePoolScope.AzureMachinePool, capiv1exp.MachinePoolFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := machinePoolScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if !clusterScope.Cluster.Status.InfrastructureReady {
		machinePoolScope.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machinePoolScope.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		machinePoolScope.Info("Bootstrap data secret reference is not yet available")
		return reconcile.Result{}, nil
	}

	ams, err := ampr.createAzureMachinePoolService(machinePoolScope)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed creating a newAzureMachinePoolService")
	}

	if err := ams.Reconcile(ctx); err != nil {
		// Handle transient and terminal errors
		var reconcileError azure.ReconcileError
		if errors.As(err, &reconcileError) {
			if reconcileError.IsTerminal() {
				machinePoolScope.Error(err, "failed to reconcile AzureMachinePool", "name", machinePoolScope.Name())
				return reconcile.Result{}, nil
			}

			if reconcileError.IsTransient() {
				machinePoolScope.Error(err, "failed to reconcile AzureMachinePool", "name", machinePoolScope.Name())
				return reconcile.Result{RequeueAfter: reconcileError.RequeueAfter()}, nil
			}

			return reconcile.Result{}, errors.Wrap(err, "failed to reconcile AzureMachinePool")
		}

		return reconcile.Result{}, err
	}

	machinePoolScope.V(2).Info("Scale Set reconciled", "id",
		machinePoolScope.ProviderID(), "state", machinePoolScope.ProvisioningState())

	switch machinePoolScope.ProvisioningState() {
	case infrav1.Deleting:
		machinePoolScope.Info("Unexpected scale set deletion", "id", machinePoolScope.ProviderID())
		ampr.Recorder.Eventf(machinePoolScope.AzureMachinePool, corev1.EventTypeWarning, "UnexpectedVMDeletion", "Unexpected Azure scale set deletion")
	case infrav1.Failed:
		err := ams.Delete(ctx)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to delete scale set in a failed state")
		}
		return reconcile.Result{}, errors.Wrap(err, "Scale set deleted, retry creating in next reconcile")
	}

	if machinePoolScope.NeedsRequeue() {
		return reconcile.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}

	return reconcile.Result{}, nil
}

func (ampr *AzureMachinePoolReconciler) reconcileDelete(ctx context.Context, machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachinePoolReconciler.reconcileDelete")
	defer span.End()

	machinePoolScope.V(2).Info("handling deleted AzureMachinePool")

	if infracontroller.ShouldDeleteIndividualResources(ctx, clusterScope) {
		amps, err := ampr.createAzureMachinePoolService(machinePoolScope)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed creating a new AzureMachinePoolService")
		}

		machinePoolScope.V(4).Info("deleting AzureMachinePool resource individually")
		if err := amps.Delete(ctx); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "error deleting AzureMachinePool %s/%s", clusterScope.Namespace(), machinePoolScope.Name())
		}
	}

	// Delete succeeded, remove finalizer
	machinePoolScope.V(4).Info("removing finalizer for AzureMachinePool")
	controllerutil.RemoveFinalizer(machinePoolScope.AzureMachinePool, capiv1exp.MachinePoolFinalizer)
	return reconcile.Result{}, nil
}
