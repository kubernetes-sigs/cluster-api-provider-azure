/*
Copyright The Kubernetes Authors.

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
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
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aci"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	azureContainerInstanceMachineReconcilerFactory func(*scope.ContainerInstanceMachineScope) azure.Reconciler

	// AzureContainerInstanceMachineReconciler reconciles a AzureContainerInstanceMachine object
	AzureContainerInstanceMachineReconciler struct {
		client.Client
		Log               logr.Logger
		Scheme            *runtime.Scheme
		Recorder          record.EventRecorder
		ReconcileTimeout  time.Duration
		reconcilerFactory azureContainerInstanceMachineReconcilerFactory
	}

	azureContainerInstanceMachineReconciler struct {
		Scope      *scope.ContainerInstanceMachineScope
		aciService *aci.Service
	}
)

// NewAzureContainerInstanceMachineReconciler creates a new AzureContainerInstanceMachineController to handle updates to Azure Container Instance Machines
func NewAzureContainerInstanceMachineReconciler(c client.Client, log logr.Logger, recorder record.EventRecorder, reconcileTimeout time.Duration) *AzureContainerInstanceMachineReconciler {
	return &AzureContainerInstanceMachineReconciler{
		Client:            c,
		Log:               log,
		Recorder:          recorder,
		ReconcileTimeout:  reconcileTimeout,
		reconcilerFactory: newAzureMachinePoolMachineReconciler,
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azurecontainerinstancemachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azurecontainerinstancemachines/status,verbs=get;update;patch

func (r *AzureContainerInstanceMachineReconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	log := r.Log.WithValues("controller", "AzureMachine")
	// create mapper to transform incoming AzureClusters into AzureMachine requests
	azureClusterToAzureContainerInstanceMachinesMapper, err := AzureClusterToAzureContainerInstanceMachinesMapper(r.Client, mgr.GetScheme(), log)
	if err != nil {
		return errors.Wrapf(err, "failed to create AzureClusterToAzureContainerInstanceMachines mapper")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&infrav1.AzureContainerInstanceMachine{}).
		WithEventFilter(predicates.ResourceNotPaused(log)). // don't queue reconcile if resource is paused
		// watch for changes in CAPI Machine resources
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("AzureContainerInstanceMachine")),
			},
		).
		// watch for changes in AzureCluster
		Watches(
			&source.Kind{Type: &infrav1.AzureCluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: azureClusterToAzureContainerInstanceMachinesMapper,
			},
		).
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	azureContainerInstanceMachineMapper, err := util.ClusterToObjectsMapper(r.Client, &infrav1.AzureContainerInstanceMachineList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to create mapper for Cluster to AzureContainerInstanceMachine")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: azureContainerInstanceMachineMapper,
		},
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}

	return nil
}

func (r *AzureContainerInstanceMachineReconciler) Reconcile(req ctrl.Request) (result ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	logger := r.Log.WithValues("namespace", req.Namespace, "azureContainerInstanceMachine", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureContainerInstanceMachineReconciler.Reconcile",
		trace.WithAttributes(
			label.String("namespace", req.Namespace),
			label.String("name", req.Name),
			label.String("kind", "AzureContainerInstanceMachine"),
		))
	defer span.End()

	aciMachine := &infrav1.AzureContainerInstanceMachine{}
	err := r.Get(ctx, req.NamespacedName, aciMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, aciMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		r.Recorder.Eventf(aciMachine, corev1.EventTypeNormal, "Machine controller dependency not yet met", "Machine Controller has not yet set OwnerRef")
		logger.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		r.Recorder.Eventf(aciMachine, corev1.EventTypeNormal, "Unable to get cluster from metadata", "Machine is missing cluster label or cluster does not exist")
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, aciMachine) {
		logger.Info("AzureContainerInstanceMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	azureClusterName := client.ObjectKey{
		Namespace: aciMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	azureCluster := &infrav1.AzureCluster{}
	if err := r.Client.Get(ctx, azureClusterName, azureCluster); err != nil {
		r.Recorder.Eventf(aciMachine, corev1.EventTypeNormal, "AzureCluster unavailable", "AzureCluster is not available yet")
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
	aciScope, err := scope.NewContainerInstanceMachineScope(scope.ContainerInstanceMachineScopeParams{
		Logger:       logger,
		Client:       r.Client,
		Machine:      machine,
		ACIMachine:   aciMachine,
		ClusterScope: clusterScope,
	})
	if err != nil {
		r.Recorder.Eventf(aciMachine, corev1.EventTypeWarning, "Error creating the ACI machine scope", err.Error())
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any AzureContainerInstanceMachine changes.
	defer func() {
		if err := aciScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !aciMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, aciScope, clusterScope)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, aciScope, clusterScope)
}

func (r *AzureContainerInstanceMachineReconciler) reconcileNormal(ctx context.Context, aciScope *scope.ContainerInstanceMachineScope, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureMachineReconciler.reconcileNormal")
	defer span.End()

	aciScope.Info("Reconciling AzureContainerInstanceMachine")
	// If the AzureContainerInstanceMachine is in an error state, return early.
	if aciScope.ACIMachine.Status.FailureReason != nil || aciScope.ACIMachine.Status.FailureMessage != nil {
		aciScope.Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the ACIMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(aciScope.ACIMachine, infrav1.MachineFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := aciScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if !clusterScope.Cluster.Status.InfrastructureReady {
		aciScope.Info("Cluster infrastructure is not ready yet")
		conditions.MarkFalse(aciScope.ACIMachine, infrav1.VMRunningCondition, infrav1.WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if aciScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		aciScope.Info("Bootstrap data secret reference is not yet available")
		conditions.MarkFalse(aciScope.ACIMachine, infrav1.VMRunningCondition, infrav1.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{}, nil
	}

	innerReconciler := r.reconcilerFactory(aciScope)
	if err := innerReconciler.Reconcile(ctx); err != nil {

		// Handle transient and terminal errors
		var reconcileError azure.ReconcileError
		if errors.As(err, &reconcileError) {
			r.Recorder.Eventf(aciScope.ACIMachine, corev1.EventTypeWarning, "ReconcileError", errors.Wrapf(err, "failed to reconcile AzureContainerInstanceMachine").Error())
			conditions.MarkFalse(aciScope.ACIMachine, infrav1.VMRunningCondition, infrav1.VMProvisionFailedReason, clusterv1.ConditionSeverityError, err.Error())

			if reconcileError.IsTerminal() {
				aciScope.Error(err, "failed to reconcile AzureContainerInstanceMachine")
				return reconcile.Result{}, nil
			}

			if reconcileError.IsTransient() {
				aciScope.V(4).Info("failed to reconcile AzureContainerInstanceMachine")
				return reconcile.Result{RequeueAfter: reconcileError.RequeueAfter()}, nil
			}

			return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile AzureContainerInstanceMachine")
		}

		r.Recorder.Eventf(aciScope.ACIMachine, corev1.EventTypeWarning, "Error creating new AzureContainerInstanceMachine", errors.Wrapf(err, "failed to reconcile AzureContainerInstanceMachine").Error())
		conditions.MarkFalse(aciScope.ACIMachine, infrav1.VMRunningCondition, infrav1.VMProvisionFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile AzureContainerInstanceMachine")
	}

	return reconcile.Result{}, nil
}

func (r *AzureContainerInstanceMachineReconciler) reconcileDelete(ctx context.Context, aciScope *scope.ContainerInstanceMachineScope, clusterScope *scope.ClusterScope) (_ reconcile.Result, reterr error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureContainerInstanceMachineReconciler.reconcileDelete")
	defer span.End()

	aciScope.Info("Handling deleted AzureContainerInstanceMachine")

	conditions.MarkFalse(aciScope.ACIMachine, infrav1.VMRunningCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := aciScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if ShouldDeleteIndividualResources(ctx, clusterScope) {
		aciScope.Info("Deleting AzureContainerInstanceMachineReconciler")
		innerReconciler := r.reconcilerFactory(aciScope)
		if err := innerReconciler.Delete(ctx); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to delete")
		}
	}

	defer func() {
		if reterr == nil {
			// VM is deleted so remove the finalizer.
			controllerutil.RemoveFinalizer(aciScope.ACIMachine, infrav1.MachineFinalizer)
		}
	}()

	return reconcile.Result{}, nil
}

func newAzureMachinePoolMachineReconciler(scope *scope.ContainerInstanceMachineScope) azure.Reconciler {
	return &azureContainerInstanceMachineReconciler{
		Scope:      scope,
		aciService: aci.New(scope),
	}
}

// Reconcile will reconcile the state of the Machine Pool Machine with the state of the Azure VMSS VM
func (r *azureContainerInstanceMachineReconciler) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureContainerInstanceMachineReconciler.Reconcile")
	defer span.End()

	if r.Scope.VMState() == "" {
		r.Scope.SetVMState(infrav1.VMStateCreating)
		if err := r.Scope.PatchObject(ctx); err != nil {
			return errors.Wrap(err, "failed patching on start of delete")
		}
	}

	if err := r.aciService.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile container group")
	}

	return nil
}

// Delete will attempt to drain and delete the Azure VMSS VM
func (r *azureContainerInstanceMachineReconciler) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureMachinePoolMachineReconciler.Delete")
	defer span.End()

	r.Scope.SetVMState(infrav1.VMStateDeleting)
	if err := r.Scope.PatchObject(ctx); err != nil {
		return errors.Wrap(err, "failed patching on start of delete")
	}

	err := r.aciService.Delete(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to delete container group")
	}

	return nil
}
