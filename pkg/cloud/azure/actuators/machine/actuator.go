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

	clusterv1 "github.com/openshift/cluster-api/pkg/apis/cluster/v1alpha1"
	machinev1 "github.com/openshift/cluster-api/pkg/apis/machine/v1beta1"
	client "github.com/openshift/cluster-api/pkg/client/clientset_generated/clientset/typed/machine/v1beta1"
	controllerError "github.com/openshift/cluster-api/pkg/controller/error"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/deployer"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"
)

//+kubebuilder:rbac:groups=azureprovider.k8s.io,resources=azuremachineproviderconfigs;azuremachineproviderstatuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=machines;machines/status;machinedeployments;machinedeployments/status;machinesets;machinesets/status;machineclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes;events,verbs=get;list;watch;create;update;patch;delete

// Actuator is responsible for performing machine reconciliation.
type Actuator struct {
	*deployer.Deployer

	client     client.MachineV1beta1Interface
	coreClient controllerclient.Client
}

// ActuatorParams holds parameter information for Actuator.
type ActuatorParams struct {
	Client     client.MachineV1beta1Interface
	CoreClient controllerclient.Client
}

// NewActuator returns an actuator.
func NewActuator(params ActuatorParams) *Actuator {
	return &Actuator{
		Deployer:   deployer.New(deployer.Params{ScopeGetter: actuators.DefaultScopeGetter}),
		client:     params.Client,
		coreClient: params.CoreClient,
	}
}

// Create creates a machine and is invoked by the machine controller.
func (a *Actuator) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *machinev1.Machine) error {
	klog.Infof("Creating machine %v", machine.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{
		Machine:    machine,
		Cluster:    nil,
		Client:     a.client,
		CoreClient: a.coreClient,
	})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}
	defer scope.Close()

	err = NewReconciler(scope).Create(context.Background())
	if err != nil {
		klog.Errorf("failed to reconcile machine %s: %v", machine.Name, err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	return nil
}

// Delete deletes a machine and is invoked by the Machine Controller.
func (a *Actuator) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *machinev1.Machine) error {
	klog.Infof("Deleting machine %v", machine.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{
		Machine:    machine,
		Cluster:    nil,
		Client:     a.client,
		CoreClient: a.coreClient,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to create scope")
	}

	defer scope.Close()

	err = NewReconciler(scope).Delete(context.Background())
	if err != nil {
		klog.Errorf("failed to delete machine %s: %v", machine.Name, err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	return nil
}

// Update updates a machine and is invoked by the Machine Controller.
// If the Update attempts to mutate any immutable state, the method will error
// and no updates will be performed.
func (a *Actuator) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *machinev1.Machine) error {
	klog.Infof("Updating machine %v", machine.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{
		Machine:    machine,
		Cluster:    nil,
		Client:     a.client,
		CoreClient: a.coreClient,
	})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	err = NewReconciler(scope).Update(context.Background())
	if err != nil {
		klog.Errorf("failed to update machine %s: %v", machine.Name, err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	return nil
}

// Exists test for the existence of a machine and is invoked by the Machine Controller
func (a *Actuator) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *machinev1.Machine) (bool, error) {
	klog.Infof("Checking if machine %v exists", machine.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{
		Machine:    machine,
		Cluster:    nil,
		Client:     a.client,
		CoreClient: a.coreClient,
	})
	if err != nil {
		return false, errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	isExists, err := NewReconciler(scope).Exists(context.Background())
	if err != nil {
		klog.Errorf("failed to check machine %s exists: %v", machine.Name, err)
	}

	return isExists, err
}
