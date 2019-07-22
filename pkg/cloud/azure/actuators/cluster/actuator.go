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

package cluster

import (
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/deployer"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	controllerError "sigs.k8s.io/cluster-api/pkg/controller/error"
)

// TODO: Backfill logic
//const waitForControlPlaneMachineDuration = 15 * time.Second

//+kubebuilder:rbac:groups=azureprovider.k8s.io,resources=azureclusterproviderconfigs;azureclusterproviderstatuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=,resources=secrets,verbs=create;get;watch;list
//+kubebuilder:rbac:groups=,resources=configmaps,verbs=create;get;delete

// Actuator is responsible for performing cluster reconciliation
type Actuator struct {
	*deployer.Deployer

	coreClient corev1.CoreV1Interface
	client     client.ClusterV1alpha1Interface
	log        logr.Logger
}

// ActuatorParams holds parameter information for Actuator
type ActuatorParams struct {
	CoreClient     corev1.CoreV1Interface
	Client         client.ClusterV1alpha1Interface
	LoggingContext string
}

// NewActuator creates a new Actuator
func NewActuator(params ActuatorParams) *Actuator {
	return &Actuator{
		client:     params.Client,
		coreClient: params.CoreClient,
		log:        klogr.New().WithName(params.LoggingContext),
		Deployer:   deployer.New(deployer.Params{ScopeGetter: actuators.DefaultScopeGetter}),
	}
}

// Reconcile reconciles a cluster and is invoked by the Cluster Controller
func (a *Actuator) Reconcile(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling cluster %v", cluster.Name)

	scope, err := actuators.NewScope(actuators.ScopeParams{Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Wrap(err, "failed to create scope")
	}

	defer scope.Close()

	err = NewReconciler(scope).Reconcile()
	if err != nil {
		return errors.Wrap(err, "failed to reconcile cluster services")
	}

	return nil
}

// Delete deletes a cluster and is invoked by the Cluster Controller.
func (a *Actuator) Delete(cluster *clusterv1.Cluster) error {
	klog.Infof("Reconciling cluster %v", cluster.Name)

	scope, err := actuators.NewScope(actuators.ScopeParams{Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Wrap(err, "failed to create scope")
	}

	defer scope.Close()

	if err := NewReconciler(scope).Delete(); err != nil {
		klog.Errorf("Error deleting resource group: %v.", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: 5 * 1000 * 1000 * 1000,
		}
	}

	return nil
}
