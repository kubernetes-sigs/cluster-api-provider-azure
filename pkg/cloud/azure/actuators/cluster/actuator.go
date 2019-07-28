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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators/machine"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/deployer"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	controllerError "sigs.k8s.io/cluster-api/pkg/controller/error"
	"sigs.k8s.io/cluster-api/pkg/controller/remote"
)

const waitForControlPlaneMachineDuration = 15 * time.Second

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
	log := a.log.WithValues("cluster-name", cluster.Name, "cluster-namespace", cluster.Namespace)
	log.Info("Reconciling Cluster")

	scope, err := actuators.NewScope(actuators.ScopeParams{Cluster: cluster, Client: a.client, Logger: a.log})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	err = NewReconciler(scope).Reconcile()
	if err != nil {
		return errors.Wrap(err, "failed to reconcile cluster services")
	}

	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string)
	}
	cluster.Annotations[v1alpha1.AnnotationClusterInfrastructureReady] = v1alpha1.ValueReady

	// Store KubeConfig for Cluster API NodeRef controller to use.
	kubeConfigSecretName := remote.KubeConfigSecretName(cluster.Name)
	secretClient := a.coreClient.Secrets(cluster.Namespace)
	if _, secretErr := secretClient.Get(kubeConfigSecretName, metav1.GetOptions{}); secretErr != nil && apierrors.IsNotFound(err) {
		kubeConfig, kubeconfigErr := a.Deployer.GetKubeConfig(cluster, nil)
		if kubeconfigErr != nil {
			return errors.Wrapf(kubeconfigErr, "failed to get kubeconfig for cluster %q", cluster.Name)
		}

		kubeConfigSecret := &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: kubeConfigSecretName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: cluster.APIVersion,
						Kind:       cluster.Kind,
						Name:       cluster.Name,
						UID:        cluster.UID,
					},
				},
			},
			StringData: map[string]string{
				"value": kubeConfig,
			},
		}

		if _, secretCreateErr := secretClient.Create(kubeConfigSecret); secretCreateErr != nil {
			return errors.Wrapf(secretCreateErr, "failed to create kubeconfig secret for cluster %q", cluster.Name)
		}
	} else if secretErr != nil {
		return errors.Wrapf(secretErr, "failed to get kubeconfig secret for cluster %q", cluster.Name)
	}

	// If the control plane is ready, try to delete the control plane configmap lock, if it exists, and return.
	if cluster.Annotations[v1alpha1.AnnotationControlPlaneReady] == v1alpha1.ValueReady {
		configMapName := actuators.ControlPlaneConfigMapName(cluster)
		log.Info("Checking for existence of control plane configmap lock", "configmap-name", configMapName)

		_, cmErr := a.coreClient.ConfigMaps(cluster.Namespace).Get(configMapName, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			// It doesn't exist - no-op
		case cmErr != nil:
			return errors.Wrapf(cmErr, "Error retrieving control plane configmap lock %q", configMapName)
		default:
			if cmErr := a.coreClient.ConfigMaps(cluster.Namespace).Delete(configMapName, nil); cmErr != nil {
				return errors.Wrapf(err, "Error deleting control plane configmap lock %q", configMapName)
			}
		}

		// Nothing more to reconcile - return early.
		return nil
	}

	log.Info("Cluster does not have ready annotation - checking for ready control plane machines")

	machines, err := a.client.Machines(cluster.Namespace).List(actuators.ListOptionsForCluster(cluster.Name))
	if err != nil {
		return errors.Wrapf(err, "failed to list machines for cluster %q", cluster.Name)
	}

	controlPlaneMachines := machine.GetControlPlaneMachines(machines)

	machineReady := false
	for _, machine := range controlPlaneMachines {
		if machine.Status.NodeRef != nil {
			machineReady = true
			break
		}
	}

	if !machineReady {
		log.Info("No control plane machines are ready - requeuing cluster")
		return &controllerError.RequeueAfterError{RequeueAfter: waitForControlPlaneMachineDuration}
	}

	log.Info("Setting cluster ready annotation")
	cluster.Annotations[v1alpha1.AnnotationControlPlaneReady] = v1alpha1.ValueReady

	klog.V(2).Infof("successfully reconciled cluster %s", cluster.Name)
	return nil
}

// Delete deletes a cluster and is invoked by the Cluster Controller.
func (a *Actuator) Delete(cluster *clusterv1.Cluster) error {
	a.log.Info("Deleting cluster", "cluster-name", cluster.Name, "cluster-namespace", cluster.Namespace)

	scope, err := actuators.NewScope(actuators.ScopeParams{
		Cluster: cluster,
		Client:  a.client,
		Logger:  a.log,
	})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	if err := NewReconciler(scope).Delete(); err != nil {
		klog.Errorf("Error deleting resource group: %v.", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: 5 * time.Second,
		}
	}

	return nil
}
