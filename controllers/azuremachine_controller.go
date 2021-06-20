/*
Copyright 2019 The Kubernetes Authors.

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
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureMachineReconciler reconciles an AzureMachine object.
type AzureMachineReconciler struct {
	client.Client
	Log                       logr.Logger
	Recorder                  record.EventRecorder
	ReconcileTimeout          time.Duration
	WatchFilterValue          string
	createAzureMachineService azureMachineServiceCreator
}

type azureMachineServiceCreator func(machineScope *scope.MachineScope) (*azureMachineService, error)

// NewAzureMachineReconciler returns a new AzureMachineReconciler instance.
func NewAzureMachineReconciler(client client.Client, log logr.Logger, recorder record.EventRecorder, reconcileTimeout time.Duration, watchFilterValue string) *AzureMachineReconciler {
	amr := &AzureMachineReconciler{
		Client:           client,
		Log:              log,
		Recorder:         recorder,
		ReconcileTimeout: reconcileTimeout,
		WatchFilterValue: watchFilterValue,
	}

	amr.createAzureMachineService = newAzureMachineService

	return amr
}

// SetupWithManager initializes this controller with a manager.
func (r *AzureMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := r.Log.WithValues("controller", "AzureMachine")
	// create mapper to transform incoming AzureClusters into AzureMachine requests
	azureClusterToAzureMachinesMapper, err := AzureClusterToAzureMachinesMapper(ctx, r.Client, &infrav1.AzureMachineList{}, mgr.GetScheme(), log)
	if err != nil {
		return errors.Wrap(err, "failed to create AzureCluster to AzureMachines mapper")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.AzureMachine{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		// watch for changes in CAPI Machine resources
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("AzureMachine"))),
		).
		// watch for changes in AzureCluster
		Watches(
			&source.Kind{Type: &infrav1.AzureCluster{}},
			handler.EnqueueRequestsFromMapFunc(azureClusterToAzureMachinesMapper),
		).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	azureMachineMapper, err := util.ClusterToObjectsMapper(r.Client, &infrav1.AzureMachineList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrap(err, "failed to create mapper for Cluster to AzureMachines")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(azureMachineMapper),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

// Reconcile idempotently gets, creates, and updates a machine.
func (r *AzureMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	logger := r.Log.WithValues("namespace", req.Namespace, "azureMachine", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachineReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("namespace", req.Namespace),
			attribute.String("name", req.Name),
			attribute.String("kind", "AzureMachine"),
		))
	defer span.End()

	// Fetch the AzureMachine VM.
	azureMachine := &infrav1.AzureMachine{}
	err := r.Get(ctx, req.NamespacedName, azureMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, azureMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		r.Recorder.Eventf(azureMachine, corev1.EventTypeNormal, "Machine controller dependency not yet met", "Machine Controller has not yet set OwnerRef")
		logger.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		r.Recorder.Eventf(azureMachine, corev1.EventTypeNormal, "Unable to get cluster from metadata", "Machine is missing cluster label or cluster does not exist")
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, azureMachine) {
		logger.Info("AzureMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	azureClusterName := client.ObjectKey{
		Namespace: azureMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	azureCluster := &infrav1.AzureCluster{}
	if err := r.Client.Get(ctx, azureClusterName, azureCluster); err != nil {
		r.Recorder.Eventf(azureMachine, corev1.EventTypeNormal, "AzureCluster unavailable", "AzureCluster is not available yet")
		logger.Info("AzureCluster is not available yet")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("AzureCluster", azureCluster.Name)

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client:       r.Client,
		Logger:       logger,
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	if err != nil {
		r.Recorder.Eventf(azureCluster, corev1.EventTypeWarning, "Error creating the cluster scope", err.Error())
		return reconcile.Result{}, err
	}

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Logger:       logger,
		Client:       r.Client,
		Machine:      machine,
		AzureMachine: azureMachine,
		ClusterScope: clusterScope,
	})
	if err != nil {
		r.Recorder.Eventf(azureMachine, corev1.EventTypeWarning, "Error creating the machine scope", err.Error())
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any AzureMachine changes.
	defer func() {
		if err := machineScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !azureMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope, clusterScope)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, machineScope, clusterScope)
}

func (r *AzureMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachineReconciler.reconcileNormal")
	defer span.End()

	machineScope.Info("Reconciling AzureMachine")
	// If the AzureMachine is in an error state, return early.
	if machineScope.AzureMachine.Status.FailureReason != nil || machineScope.AzureMachine.Status.FailureMessage != nil {
		machineScope.Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the AzureMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machineScope.AzureMachine, infrav1.MachineFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := machineScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if !clusterScope.Cluster.Status.InfrastructureReady {
		machineScope.Info("Cluster infrastructure is not ready yet")
		conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		machineScope.Info("Bootstrap data secret reference is not yet available")
		conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{}, nil
	}

	ams, err := r.createAzureMachineService(machineScope)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create azure machine service")
	}

	if err := ams.Reconcile(ctx); err != nil {
		// This means that a VM was created and managed by this controller, but is not present anymore.
		// In this case, we mark it as failed and leave it to MHC for remediation
		if errors.As(err, &azure.VMDeletedError{}) {
			r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "VMDeleted", errors.Wrap(err, "failed to reconcile AzureMachine").Error())
			conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.VMProvisionFailedReason, clusterv1.ConditionSeverityError, err.Error())
			machineScope.SetFailureReason(capierrors.UpdateMachineError)
			machineScope.SetFailureMessage(err)
			machineScope.SetNotReady()
			return reconcile.Result{}, errors.Wrap(err, "failed to reconcile AzureMachine")
		}

		// Handle transient and terminal errors
		var reconcileError azure.ReconcileError
		if errors.As(err, &reconcileError) {
			if reconcileError.IsTerminal() {
				r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to reconcile AzureMachine").Error())
				machineScope.Error(err, "failed to reconcile AzureMachine", "name", machineScope.Name())
				machineScope.SetFailureReason(capierrors.CreateMachineError)
				machineScope.SetFailureMessage(err)
				machineScope.SetNotReady()
				machineScope.SetVMState(infrav1.Failed)
				return reconcile.Result{}, nil
			}

			if reconcileError.IsTransient() {
				machineScope.Error(err, "transient failure to reconcile AzureMachine, retrying", "name", machineScope.Name())
				machineScope.SetNotReady()
				return reconcile.Result{RequeueAfter: reconcileError.RequeueAfter()}, nil
			}
		}
		r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to reconcile AzureMachine").Error())
		conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.VMProvisionFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, errors.Wrap(err, "failed to reconcile AzureMachine")
	}

	machineScope.SetReady()

	return reconcile.Result{}, nil
}

func (r *AzureMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (_ reconcile.Result, reterr error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachineReconciler.reconcileDelete")
	defer span.End()

	machineScope.Info("Handling deleted AzureMachine")

	conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := machineScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	defer func() {
		if reterr == nil {
			machineScope.Info("Removing finalizer from AzureMachine")
			controllerutil.RemoveFinalizer(machineScope.AzureMachine, infrav1.MachineFinalizer)
		}
	}()

	if ShouldDeleteIndividualResources(ctx, clusterScope) {
		machineScope.Info("Deleting AzureMachine")
		ams, err := r.createAzureMachineService(machineScope)
		if err != nil {
			reterr = errors.Wrap(err, "failed to create azure machine service")
			return
		}

		if err := ams.Delete(ctx); err != nil {
			r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "Error deleting AzureMachine", errors.Wrapf(err, "error deleting AzureMachine %s/%s", clusterScope.Namespace(), clusterScope.ClusterName()).Error())
			conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			reterr = errors.Wrapf(err, "error deleting AzureMachine %s/%s", clusterScope.Namespace(), clusterScope.ClusterName())
			return
		}
	} else {
		machineScope.Info("Skipping AzureMachine Deletion; will delete whole resource group.")
	}

	return reconcile.Result{}, reterr
}
