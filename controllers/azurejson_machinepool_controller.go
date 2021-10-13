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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	expv1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureJSONMachinePoolReconciler reconciles Azure json secrets for AzureMachinePool objects.
type AzureJSONMachinePoolReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
	WatchFilterValue string
}

// SetupWithManager initializes this controller with a manager.
func (r *AzureJSONMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&expv1.AzureMachinePool{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Owns(&corev1.Secret{}).
		Complete(r)
}

// Reconcile reconciles the Azure json for AzureMachinePool objects.
func (r *AzureJSONMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	ctx, log, done := tele.StartSpanWithLogger(
		ctx,
		"controllers.AzureJSONMachinePoolReconciler.Reconcile",
		tele.KVP("namespace", req.Namespace),
		tele.KVP("name", req.Name),
		tele.KVP("kind", "AzureMachinePool"),
	)
	defer done()

	log = log.WithValues("namespace", req.Namespace, "azureMachinePool", req.Name)

	// Fetch the AzureMachine instance
	azureMachinePool := &expv1.AzureMachinePool{}
	err := r.Get(ctx, req.NamespacedName, azureMachinePool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("object was not found")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the CAPI MachinePool.
	machinePool, err := GetOwnerMachinePool(ctx, r.Client, azureMachinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machinePool == nil {
		log.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("machinePool", machinePool.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		log.Info("MachinePool is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	_, kind := infrav1.GroupVersion.WithKind("AzureCluster").ToAPIVersionAndKind()

	// only look at azure clusters
	if cluster.Spec.InfrastructureRef.Kind != kind {
		log.WithValues("kind", cluster.Spec.InfrastructureRef.Kind).Info("infra ref was not an AzureCluster")
		return ctrl.Result{}, nil
	}

	// fetch the corresponding azure cluster
	azureCluster := &infrav1.AzureCluster{}
	azureClusterName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	if err := r.Get(ctx, azureClusterName, azureCluster); err != nil {
		log.Error(err, "failed to fetch AzureCluster")
		return reconcile.Result{}, err
	}

	// Construct secret for this machine
	userAssignedIdentityIfExists := ""
	if len(azureMachinePool.Spec.UserAssignedIdentities) > 0 {
		userAssignedIdentityIfExists = azureMachinePool.Spec.UserAssignedIdentities[0].ProviderID
	}

	// Create the scope.
	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client:       r.Client,
		Logger:       log,
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	apiVersion, kind := infrav1.GroupVersion.WithKind("AzureMachinePool").ToAPIVersionAndKind()
	owner := metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       azureMachinePool.GetName(),
		UID:        azureMachinePool.GetUID(),
	}

	if azureMachinePool.Spec.Identity == infrav1.VMIdentityNone {
		log.Info(fmt.Sprintf("WARNING, %s", spIdentityWarning))
		r.Recorder.Eventf(azureMachinePool, corev1.EventTypeWarning, "VMIdentityNone", spIdentityWarning)
	}

	newSecret, err := GetCloudProviderSecret(
		clusterScope,
		azureMachinePool.Namespace,
		azureMachinePool.Name,
		owner,
		azureMachinePool.Spec.Identity,
		userAssignedIdentityIfExists,
	)

	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create cloud provider config")
	}

	if err := reconcileAzureSecret(ctx, log, r.Client, owner, newSecret, clusterScope.ClusterName()); err != nil {
		r.Recorder.Eventf(azureMachinePool, corev1.EventTypeWarning, "Error reconciling cloud provider secret for AzureMachinePool", err.Error())
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile azure secret")
	}

	return ctrl.Result{}, nil
}
