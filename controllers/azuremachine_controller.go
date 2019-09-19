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
	"path"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	gcompute "google.golang.org/api/compute/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/compute"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AzureMachineReconciler reconciles a AzureMachine object
type AzureMachineReconciler struct {
	client.Client
	Log logr.Logger
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

func (r *AzureMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.TODO()
	logger := r.Log.
		WithName(controllerName).
		WithName(fmt.Sprintf("namespace=%s", req.Namespace)).
		WithName(fmt.Sprintf("azureMachine=%s", req.Name))

	// Fetch the AzureMachine instance.
	azureMachine := &infrav1.AzureMachine{}
	err := r.Get(ctx, req.NamespacedName, azureMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	logger = logger.WithName(azureMachine.APIVersion)

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, azureMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		logger.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	logger = logger.WithName(fmt.Sprintf("machine=%s", machine.Name))

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithName(fmt.Sprintf("cluster=%s", cluster.Name))

	azureCluster := &infrav1.AzureCluster{}

	azureClusterName := client.ObjectKey{
		Namespace: azureMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, azureClusterName, azureCluster); err != nil {
		logger.Info("AzureCluster is not available yet")
		return reconcile.Result{}, nil
	}

	logger = logger.WithName(fmt.Sprintf("azureCluster=%s", azureCluster.Name))

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:     r.Client,
		Logger:     logger,
		Cluster:    cluster,
		AzureCluster: azureCluster,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Logger:     logger,
		Client:     r.Client,
		Cluster:    cluster,
		Machine:    machine,
		AzureCluster: azureCluster,
		AzureMachine: azureMachine,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any AzureMachine changes.
	defer func() {
		if err := machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !azureMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machineScope, clusterScope)
	}

	// Handle non-deleted machines
	return r.reconcile(ctx, machineScope, clusterScope)
}

func (r *AzureMachineReconciler) reconcile(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	machineScope.Info("Reconciling AzureMachine")
	// If the AzureMachine is in an error state, return early.
	if machineScope.AzureMachine.Status.ErrorReason != nil || machineScope.AzureMachine.Status.ErrorMessage != nil {
		machineScope.Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the AzureMachine doesn't have our finalizer, add it.
	if !util.Contains(machineScope.AzureMachine.Finalizers, infrav1.MachineFinalizer) {
		machineScope.AzureMachine.Finalizers = append(machineScope.AzureMachine.Finalizers, infrav1.MachineFinalizer)
	}

	if !machineScope.Cluster.Status.InfrastructureReady {
		machineScope.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.Data == nil {
		machineScope.Info("Bootstrap data is not yet available")
		return reconcile.Result{}, nil
	}

	computeSvc := compute.NewService(clusterScope)

	// Get or create the instance.
	instance, err := r.getOrCreate(machineScope, computeSvc)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Set an error message if we couldn't find the instance.
	if instance == nil {
		machineScope.SetErrorReason(capierrors.UpdateMachineError)
		machineScope.SetErrorMessage(errors.New("EC2 instance cannot be found"))
		return reconcile.Result{}, nil
	}

	// TODO(ncdc): move this validation logic into a validating webhook
	if errs := r.validateUpdate(&machineScope.AzureMachine.Spec, instance); len(errs) > 0 {
		agg := kerrors.NewAggregate(errs)
		record.Warnf(machineScope.AzureMachine, "InvalidUpdate", "Invalid update: %s", agg.Error())
		machineScope.Error(err, "Invalid update")
		return reconcile.Result{}, nil
	}

	// Make sure Spec.ProviderID is always set.
	machineScope.SetProviderID(fmt.Sprintf("gce://%s/%s/%s", clusterScope.Project(), machineScope.Zone(), instance.Name))

	// Proceed to reconcile the AzureMachine state.
	machineScope.SetInstanceStatus(infrav1.InstanceStatus(instance.Status))

	// TODO(vincepri): Remove this annotation when clusterctl is no longer relevant.
	machineScope.SetAnnotation("cluster-api-provider-azure", "true")

	switch infrav1.InstanceStatus(instance.Status) {
	case infrav1.InstanceStatusRunning:
		machineScope.Info("Machine instance is running", "instance-id", *machineScope.GetInstanceID())
		machineScope.SetReady()
	case infrav1.InstanceStatusProvisioning, infrav1.InstanceStatusStaging:
		machineScope.Info("Machine instance is pending", "instance-id", *machineScope.GetInstanceID())
	default:
		machineScope.SetErrorReason(capierrors.UpdateMachineError)
		machineScope.SetErrorMessage(errors.Errorf("EC2 instance state %q is unexpected", instance.Status))
	}

	if err := r.reconcileLBAttachment(machineScope, clusterScope, instance); err != nil {
		return reconcile.Result{}, errors.Errorf("failed to reconcile LB attachment: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *AzureMachineReconciler) reconcileDelete(machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (_ reconcile.Result, reterr error) {
	machineScope.Info("Handling deleted AzureMachine")

	computeSvc := compute.NewService(clusterScope)

	instance, err := r.findInstance(machineScope, computeSvc)
	if err != nil {
		return reconcile.Result{}, err
	}

	defer func() {
		if reterr == nil {
			machineScope.AzureMachine.Finalizers = util.Filter(machineScope.AzureMachine.Finalizers, infrav1.MachineFinalizer)
		}
	}()

	if instance == nil {
		// The machine was never created or was deleted by some other entity
		machineScope.V(3).Info("Unable to locate instance by ID or tags")
		return reconcile.Result{}, nil
	}

	// Check the instance state. If it's already shutting down or terminated,
	// do nothing. Otherwise attempt to delete it.
	switch infrav1.InstanceStatus(instance.Status) {
	case infrav1.InstanceStatusTerminated:
		machineScope.Info("Instance is shutting down or already terminated")
	default:
		machineScope.Info("Terminating instance")
		if err := computeSvc.TerminateInstanceAndWait(machineScope); err != nil {
			record.Warnf(machineScope.AzureMachine, "FailedTerminate", "Failed to terminate instance %q: %v", instance.Name, err)
			return reconcile.Result{}, errors.Errorf("failed to terminate instance: %+v", err)
		}
		record.Eventf(machineScope.AzureMachine, "SuccessfulTerminate", "Terminated instance %q", instance.Name)
	}

	return reconcile.Result{}, nil
}

// findInstance queries the EC2 apis and retrieves the instance if it exists, returns nil otherwise.
func (r *AzureMachineReconciler) findInstance(scope *scope.MachineScope, computeSvc *compute.Service) (*gcompute.Instance, error) {
	instance, err := computeSvc.InstanceIfExists(scope)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query AzureMachine instance")
	}
	return instance, nil
}

func (r *AzureMachineReconciler) getOrCreate(scope *scope.MachineScope, computeSvc *compute.Service) (*gcompute.Instance, error) {
	instance, err := r.findInstance(scope, computeSvc)
	if err != nil {
		return nil, err
	}

	if instance == nil {
		// Create a new AzureMachine instance if we couldn't find a running instance.
		instance, err = computeSvc.CreateInstance(scope)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create AzureMachine instance")
		}
	}

	return instance, nil
}

func (r *AzureMachineReconciler) reconcileLBAttachment(machineScope *scope.MachineScope, clusterScope *scope.ClusterScope, i *gcompute.Instance) error {
	if !machineScope.IsControlPlane() {
		return nil
	}
	computeSvc := compute.NewService(clusterScope)
	groupName := fmt.Sprintf("%s-%s-%s", clusterScope.Name(), infrav1.APIServerRoleTagValue, machineScope.Zone())

	// Get the instance group, or create if necessary.
	group, err := computeSvc.GetOrCreateInstanceGroup(machineScope.Zone(), groupName)
	if err != nil {
		return err
	}

	// Make sure the instance is registered.
	if err := computeSvc.EnsureInstanceGroupMember(machineScope.Zone(), group.Name, i); err != nil {
		return err
	}

	// Update the backend service.
	return computeSvc.UpdateBackendServices()
}

// validateUpdate checks that no immutable fields have been updated and
// returns a slice of errors representing attempts to change immutable state.
func (r *AzureMachineReconciler) validateUpdate(spec *infrav1.AzureMachineSpec, i *gcompute.Instance) (errs []error) {
	// Instance Type
	if spec.InstanceType != path.Base(i.MachineType) {
		errs = append(errs, errors.Errorf("instance type cannot be mutated from %q to %q", i.MachineType, spec.InstanceType))
	}

	// Root Device Size
	if len(i.Disks) > 0 && spec.RootDeviceSize > 0 && spec.RootDeviceSize != i.Disks[0].InitializeParams.DiskSizeGb {
		errs = append(errs, errors.Errorf("Root volume size cannot be mutated from %v to %v",
			i.Disks[0].InitializeParams.DiskSizeGb, spec.RootDeviceSize))
	}

	// TODO(vincepri): Validate other fields.
	return errs
}

// AzureClusterToAzureMachine is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of AzureMachines.
func (r *AzureMachineReconciler) AzureClusterToAzureMachines(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}

	c, ok := o.Object.(*infrav1.AzureCluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a AzureCluster but got a %T", o.Object), "failed to get AzureMachine for AzureCluster")
		return nil
	}
	log := r.Log.WithValues("AzureCluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return result
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return result
	}

	labels := map[string]string{clusterv1.MachineClusterLabelName: cluster.Name}
	machineList := &clusterv1.MachineList{}
	if err := r.List(context.TODO(), machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
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
