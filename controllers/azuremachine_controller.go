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
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureMachineReconciler reconciles a AzureMachine object
type AzureMachineReconciler struct {
	client.Client
	Log                       logr.Logger
	Recorder                  record.EventRecorder
	ReconcileTimeout          time.Duration
	createAzureMachineService azureMachineServiceCreator
}

type azureMachineServiceCreator func(machineScope *scope.MachineScope) (*azureMachineService, error)

// NewAzureMachineReconciler returns a new AzureMachineReconciler instance
func NewAzureMachineReconciler(client client.Client, log logr.Logger, recorder record.EventRecorder, reconcileTimeout time.Duration) *AzureMachineReconciler {
	amr := &AzureMachineReconciler{
		Client:           client,
		Log:              log,
		Recorder:         recorder,
		ReconcileTimeout: reconcileTimeout,
	}

	amr.createAzureMachineService = newAzureMachineService

	return amr
}

// SetupWithManager initializes this controller with a manager.
func (r *AzureMachineReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	log := r.Log.WithValues("controller", "AzureMachine")
	// create mapper to transform incoming AzureClusters into AzureMachine requests
	azureClusterToAzureMachinesMapper, err := AzureClusterToAzureMachinesMapper(r.Client, mgr.GetScheme(), log)
	if err != nil {
		return errors.Wrapf(err, "failed to create AzureCluster to AzureMachines mapper")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.AzureMachine{}).
		WithEventFilter(predicates.ResourceNotPaused(log)). // don't queue reconcile if resource is paused
		// watch for changes in CAPI Machine resources
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("AzureMachine")),
			},
		).
		// watch for changes in AzureCluster
		Watches(
			&source.Kind{Type: &infrav1.AzureCluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: azureClusterToAzureMachinesMapper,
			},
		).
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	azureMachineMapper, err := util.ClusterToObjectsMapper(r.Client, &infrav1.AzureMachineList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to create mapper for Cluster to AzureMachines")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: azureMachineMapper,
		},
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

// Reconcile idempotently gets, creates, and updates a machine.
func (r *AzureMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	logger := r.Log.WithValues("namespace", req.Namespace, "azureMachine", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachineReconciler.Reconcile",
		trace.WithAttributes(
			label.String("namespace", req.Namespace),
			label.String("name", req.Name),
			label.String("kind", "AzureMachine"),
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

	if machineScope.AzureMachine.Spec.AvailabilityZone.ID != nil {
		message := "AvailabilityZone is deprecated, use FailureDomain instead"
		machineScope.Info(message)
		r.Recorder.Eventf(clusterScope.AzureCluster, corev1.EventTypeWarning, "DeprecatedField", message)

		// Set FailureDomain if it is not set.
		if machineScope.AzureMachine.Spec.FailureDomain == nil {
			machineScope.V(2).Info("Failure domain not set, setting with value from AvailabilityZone.ID")
			machineScope.AzureMachine.Spec.FailureDomain = machineScope.AzureMachine.Spec.AvailabilityZone.ID
		}
	}

	ams, err := r.createAzureMachineService(machineScope)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create azure machine service")
	}

	err = ams.Reconcile(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create AzureMachineService")
	}

	if err := ams.Reconcile(ctx); err != nil {

		// This means that a VM was created and managed by this controller, but is not present anymore.
		// In this case, we mark it as failed and leave it to MHC for remediation
		if errors.As(err, &azure.VMDeletedError{}) {
			r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "VMDeleted", errors.Wrapf(err, "failed to reconcile AzureMachine").Error())
			conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.VMProvisionFailedReason, clusterv1.ConditionSeverityError, err.Error())
			machineScope.SetFailureReason(capierrors.UpdateMachineError)
			machineScope.SetFailureMessage(err)
			machineScope.SetNotReady()
			return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile AzureMachine")
		}

		// Handle transient and terminal errors
		var reconcileError azure.ReconcileError
		if errors.As(err, &reconcileError) {
			r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to reconcile AzureMachine").Error())
			conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.VMProvisionFailedReason, clusterv1.ConditionSeverityError, err.Error())

			if reconcileError.IsTerminal() {
				machineScope.Error(err, "failed to reconcile AzureMachine", "name", machineScope.Name())
				return reconcile.Result{}, nil
			}

			if reconcileError.IsTransient() {
				machineScope.Error(err, "failed to reconcile AzureMachine", "name", machineScope.Name())
				return reconcile.Result{RequeueAfter: reconcileError.RequeueAfter()}, nil
			}

			return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile AzureMachine")
		}

		r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "Error creating new AzureMachine", errors.Wrapf(err, "failed to reconcile AzureMachine").Error())
		conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.VMProvisionFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile AzureMachine")
	}

	switch machineScope.VMState() {
	case infrav1.VMStateSucceeded:
		machineScope.V(2).Info("VM is running", "id", machineScope.GetVMID())
		conditions.MarkTrue(machineScope.AzureMachine, infrav1.VMRunningCondition)
		machineScope.SetReady()
	case infrav1.VMStateCreating:
		machineScope.V(2).Info("VM is creating", "id", machineScope.GetVMID())
		conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.VMNCreatingReason, clusterv1.ConditionSeverityInfo, "")
		machineScope.SetNotReady()
	case infrav1.VMStateUpdating:
		machineScope.V(2).Info("VM is updating", "id", machineScope.GetVMID())
		conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.VMNUpdatingReason, clusterv1.ConditionSeverityInfo, "")
		machineScope.SetNotReady()
	case infrav1.VMStateDeleting:
		machineScope.Info("Unexpected VM deletion", "id", machineScope.GetVMID())
		r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "UnexpectedVMDeletion", "Unexpected Azure VM deletion")
		conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.VMDDeletingReason, clusterv1.ConditionSeverityWarning, "")
		machineScope.SetNotReady()
	case infrav1.VMStateFailed:
		machineScope.Error(errors.New("Failed to create or update VM"), "VM is in failed state", "id", machineScope.GetVMID())
		r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "FailedVMState", "Azure VM is in failed state")
		machineScope.SetFailureReason(capierrors.UpdateMachineError)
		machineScope.SetFailureMessage(errors.Errorf("Azure VM state is %s", machineScope.VMState()))
		conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, infrav1.VMProvisionFailedReason, clusterv1.ConditionSeverityWarning, "")
		machineScope.SetNotReady()
	default:
		machineScope.V(2).Info("VM state is undefined", "id", machineScope.GetVMID())
		conditions.MarkUnknown(machineScope.AzureMachine, infrav1.VMRunningCondition, "", "")
		machineScope.SetNotReady()
	}

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

	if ShouldDeleteIndividualResources(ctx, clusterScope) {
		machineScope.Info("Deleting AzureMachine")
		ams, err := r.createAzureMachineService(machineScope)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to create azure machine service")
		}

		if err := ams.Delete(ctx); err != nil {
			r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "Error deleting AzureMachine", errors.Wrapf(err, "error deleting AzureMachine %s/%s", clusterScope.Namespace(), clusterScope.ClusterName()).Error())
			conditions.MarkFalse(machineScope.AzureMachine, infrav1.VMRunningCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return reconcile.Result{}, errors.Wrapf(err, "error deleting AzureMachine %s/%s", clusterScope.Namespace(), clusterScope.ClusterName())
		}
	}
	defer func() {
		if reterr == nil {
			// VM is deleted so remove the finalizer.
			controllerutil.RemoveFinalizer(machineScope.AzureMachine, infrav1.MachineFinalizer)
		}
	}()

	return reconcile.Result{}, nil
}
