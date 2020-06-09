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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/record"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AzureMachineReconciler reconciles a AzureMachine object
type AzureMachineReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
}

func (r *AzureMachineReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.AzureMachine{}).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("AzureMachine")),
			},
		).
		Watches(
			&source.Kind{Type: &infrav1.AzureCluster{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.AzureClusterToAzureMachines)},
		).
		Complete(r)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *AzureMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	logger := r.Log.WithValues("namespace", req.Namespace, "azureMachine", req.Name)

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
		logger.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	azureCluster := &infrav1.AzureCluster{}

	azureClusterName := client.ObjectKey{
		Namespace: azureMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, azureClusterName, azureCluster); err != nil {
		logger.Info("AzureCluster is not available yet")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("AzureCluster", azureCluster.Name)

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:       r.Client,
		Logger:       logger,
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Logger:       logger,
		Client:       r.Client,
		Cluster:      cluster,
		Machine:      machine,
		AzureCluster: azureCluster,
		AzureMachine: azureMachine,
	})
	if err != nil {
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

// findVM queries the Azure APIs and retrieves the VM if it exists, returns nil otherwise.
func (r *AzureMachineReconciler) findVM(ctx context.Context, scope *scope.MachineScope, ams *azureMachineService) (*infrav1.VM, error) {
	var vm *infrav1.VM

	// If the ProviderID is populated, describe the VM using its name and resource group name.
	vm, err := ams.VMIfExists(ctx, scope.GetVMID())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query AzureMachine VM")
	}

	return vm, nil
}

func (r *AzureMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
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

	if !machineScope.Cluster.Status.InfrastructureReady {
		machineScope.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		machineScope.Info("Bootstrap data secret reference is not yet available")
		return reconcile.Result{}, nil
	}

	// Check that the image is valid
	// NOTE: this validation logic is also in the validating webhook
	if machineScope.AzureMachine.Spec.Image != nil {
		if errs := infrav1.ValidateImage(machineScope.AzureMachine.Spec.Image, field.NewPath("image")); len(errs) > 0 {
			agg := kerrors.NewAggregate(errs.ToAggregate().Errors())
			machineScope.Info("Invalid image: %s", agg.Error())
			r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "InvalidImage", "Invalid image: %s", agg.Error())
			return reconcile.Result{}, nil
		}
	}

	if machineScope.AzureMachine.Spec.AvailabilityZone.ID != nil {
		message := "AvailavilityZone is deprecated, use FailureDomain instead"
		machineScope.Info(message)
		r.Recorder.Eventf(machineScope.AzureCluster, corev1.EventTypeWarning, "DeprecatedField", message)

		// Set FailureDomain if its not set
		if machineScope.AzureMachine.Spec.FailureDomain == nil {
			machineScope.V(2).Info("Failure domain not set, setting with value from AvailabilityZone.ID")
			machineScope.AzureMachine.Spec.FailureDomain = machineScope.AzureMachine.Spec.AvailabilityZone.ID
		}
	}

	ams := newAzureMachineService(machineScope, clusterScope)

	// Get or create the virtual machine.
	vm, err := r.getOrCreate(ctx, machineScope, ams)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Make sure Spec.ProviderID is always set.
	machineScope.SetProviderID(fmt.Sprintf("azure:///%s", vm.ID))

	machineScope.SetAnnotation("cluster-api-provider-azure", "true")

	machineScope.SetAddresses(vm.Addresses)

	// Proceed to reconcile the AzureMachine state.
	machineScope.SetVMState(vm.State)

	switch vm.State {
	case infrav1.VMStateSucceeded:
		machineScope.V(2).Info("VM is running", "id", *machineScope.GetVMID())
		machineScope.SetReady()
	case infrav1.VMStateCreating:
		machineScope.V(2).Info("VM is creating", "id", *machineScope.GetVMID())
		machineScope.SetNotReady()
	case infrav1.VMStateUpdating:
		machineScope.V(2).Info("VM is updating", "id", *machineScope.GetVMID())
		machineScope.SetNotReady()
	case infrav1.VMStateDeleting:
		machineScope.Info("Unexpected VM deletion", "state", vm.State, "instance-id", *machineScope.GetVMID())
		r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "UnexpectedVMDeletion", "Unexpected Azure VM deletion")
		machineScope.SetNotReady()
	case infrav1.VMStateFailed:
		machineScope.SetNotReady()
		machineScope.Error(errors.New("Failed to create or update VM"), "VM is in failed state", "id", *machineScope.GetVMID())
		r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "FailedVMState", "Azure VM is in failed state")
		machineScope.SetFailureReason(capierrors.UpdateMachineError)
		machineScope.SetFailureMessage(errors.Errorf("Azure VM state is %s", vm.State))
	default:
		machineScope.SetNotReady()
		machineScope.Info("VM state is undefined", "state", vm.State, "instance-id", *machineScope.GetVMID())
		r.Recorder.Eventf(machineScope.AzureMachine, corev1.EventTypeWarning, "UnhandledVMState", "Azure VM state is undefined")
		machineScope.SetFailureReason(capierrors.UpdateMachineError)
		machineScope.SetFailureMessage(errors.Errorf("Azure VM state %q is undefined", vm.State))
	}

	// Ensure that the tags are correct.
	err = r.reconcileTags(ctx, machineScope, clusterScope, machineScope.AdditionalTags())
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to ensure tags: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *AzureMachineReconciler) getOrCreate(ctx context.Context, scope *scope.MachineScope, ams *azureMachineService) (*infrav1.VM, error) {
	vm, err := r.findVM(ctx, scope, ams)
	if err != nil {
		return nil, err
	}

	if vm == nil {
		// Create a new AzureMachine VM if we couldn't find a running VM.
		vm, err = ams.Create(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create AzureMachine VM")
		}
	}

	return vm, nil
}

func (r *AzureMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (_ reconcile.Result, reterr error) {
	machineScope.Info("Handling deleted AzureMachine")

	if err := newAzureMachineService(machineScope, clusterScope).Delete(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting AzureCluster %s/%s", clusterScope.Namespace(), clusterScope.Name())
	}

	defer func() {
		if reterr == nil {
			// VM is deleted so remove the finalizer.
			controllerutil.RemoveFinalizer(machineScope.AzureMachine, infrav1.MachineFinalizer)
		}
	}()

	return reconcile.Result{}, nil
}

// AzureClusterToAzureMachines is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// of AzureMachines.
func (r *AzureMachineReconciler) AzureClusterToAzureMachines(o handler.MapObject) []ctrl.Request {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultMappingTimeout)
	defer cancel()

	result := []ctrl.Request{}
	c, ok := o.Object.(*infrav1.AzureCluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a AzureCluster but got a %T", o.Object), "failed to get AzureMachine for AzureCluster")
		return nil
	}
	log := r.Log.WithValues("AzureCluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return result
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return result
	}

	labels := map[string]string{clusterv1.ClusterLabelName: cluster.Name}
	machineList := &clusterv1.MachineList{}
	if err := r.List(ctx, machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list Machines")
		return nil
	}
	for _, m := range machineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}
