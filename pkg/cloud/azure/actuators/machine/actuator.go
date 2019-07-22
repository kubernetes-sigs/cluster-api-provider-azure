/*
Copyright 2018 The Kubernetes Authors.

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

package machine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/deployer"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	controllerError "sigs.k8s.io/cluster-api/pkg/controller/error"
)

const (
	// TODO: Backfill logic
	//defaultTokenTTL                             = 10 * time.Minute
	waitForClusterInfrastructureReadyDuration = 15 * time.Second
	//waitForControlPlaneMachineExistenceDuration = 5 * time.Second
	//waitForControlPlaneReadyDuration            = 5 * time.Second
)

//+kubebuilder:rbac:groups=azureprovider.k8s.io,resources=azuremachineproviderconfigs;azuremachineproviderstatuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=machines;machines/status;machinedeployments;machinedeployments/status;machinesets;machinesets/status;machineclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes;events,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list

// Actuator is responsible for performing machine reconciliation.
type Actuator struct {
	*deployer.Deployer

	coreClient             corev1.CoreV1Interface
	clusterClient          client.ClusterV1alpha1Interface
	log                    logr.Logger
	controlPlaneInitLocker ControlPlaneInitLocker
}

// ActuatorParams holds parameter information for Actuator.
type ActuatorParams struct {
	CoreClient             corev1.CoreV1Interface
	ClusterClient          client.ClusterV1alpha1Interface
	LoggingContext         string
	ControlPlaneInitLocker ControlPlaneInitLocker
}

// NewActuator returns an actuator.
func NewActuator(params ActuatorParams) *Actuator {
	log := klogr.New().WithName(params.LoggingContext)

	locker := params.ControlPlaneInitLocker
	if locker == nil {
		locker = newControlPlaneInitLocker(log, params.CoreClient)
	}

	return &Actuator{
		Deployer:               deployer.New(deployer.Params{ScopeGetter: actuators.DefaultScopeGetter}),
		coreClient:             params.CoreClient,
		clusterClient:          params.ClusterClient,
		log:                    log,
		controlPlaneInitLocker: locker,
	}
}

// GetControlPlaneMachines retrieves all non-deleted control plane nodes from a MachineList
func GetControlPlaneMachines(machineList *clusterv1.MachineList) []*clusterv1.Machine {
	var cpm []*clusterv1.Machine
	for _, m := range machineList.Items {
		if m.DeletionTimestamp.IsZero() && m.Spec.Versions.ControlPlane != "" {
			cpm = append(cpm, m.DeepCopy())
		}
	}
	return cpm
}

// Create creates a machine and is invoked by the machine controller.
func (a *Actuator) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if cluster == nil {
		return errors.Errorf("missing cluster for machine %s/%s", machine.Namespace, machine.Name)
	}

	log := a.log.WithValues("machine-name", machine.Name, "namespace", machine.Namespace, "cluster-name", cluster.Name)
	log.Info("Processing machine creation")

	if cluster.Annotations[v1alpha1.AnnotationClusterInfrastructureReady] != v1alpha1.ValueReady {
		log.Info("Cluster infrastructure is not ready yet - requeuing machine")
		return &controllerError.RequeueAfterError{RequeueAfter: waitForClusterInfrastructureReadyDuration}
	}

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.clusterClient, Logger: log})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	err = NewReconciler(scope).Create(context.Background())
	if err != nil {
		klog.Errorf("failed to reconcile machine %s for cluster %s: %v", machine.Name, cluster.Name, err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	return nil
}

// Delete deletes a machine and is invoked by the Machine Controller.
func (a *Actuator) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if cluster == nil {
		return errors.Errorf("missing cluster for machine %s/%s", machine.Namespace, machine.Name)
	}
	a.log.Info("Deleting machine in cluster", "machine-name", machine.Name, "machine-namespace", machine.Namespace, "cluster-name", cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.clusterClient, Logger: a.log})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	err = NewReconciler(scope).Delete(context.Background())
	if err != nil {
		klog.Errorf("failed to delete machine %s for cluster %s: %v", machine.Name, cluster.Name, err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	return nil
}

// Update updates a machine and is invoked by the Machine Controller.
// If the Update attempts to mutate any immutable state, the method will error
// and no updates will be performed.
func (a *Actuator) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if cluster == nil {
		return errors.Errorf("missing cluster for machine %s/%s", machine.Namespace, machine.Name)
	}

	a.log.Info("Updating machine in cluster", "machine-name", machine.Name, "machine-namespace", machine.Namespace, "cluster-name", cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.clusterClient, Logger: a.log})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	err = NewReconciler(scope).Update(context.Background())
	if err != nil {
		klog.Errorf("failed to update machine %s for cluster %s: %v", machine.Name, cluster.Name, err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	return nil
}

// Exists test for the existence of a machine and is invoked by the Machine Controller
func (a *Actuator) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	if cluster == nil {
		return false, errors.Errorf("missing cluster for machine %s/%s", machine.Namespace, machine.Name)
	}

	a.log.Info("Checking if machine exists in cluster", "machine-name", machine.Name, "machine-namespace", machine.Namespace, "cluster-name", cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.clusterClient, Logger: a.log})
	if err != nil {
		return false, errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	isExists, err := NewReconciler(scope).Exists(context.Background())
	if err != nil {
		klog.Errorf("failed to check machine %s exists for cluster %s: %v", machine.Name, cluster.Name, err)
	}

	return isExists, err
}
