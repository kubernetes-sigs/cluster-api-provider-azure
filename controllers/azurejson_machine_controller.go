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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureJSONMachineReconciler reconciles azure json secrets for AzureMachine objects
type AzureJSONMachineReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
}

// SetupWithManager initializes this controller with a manager
func (r *AzureJSONMachineReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.AzureMachine{}).
		WithEventFilter(filterUnclonedMachinesPredicate{log: r.Log}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

type filterUnclonedMachinesPredicate struct {
	log logr.Logger
	predicate.Funcs
}

func (f filterUnclonedMachinesPredicate) Create(e event.CreateEvent) bool {
	return f.Generic(event.GenericEvent(e))
}

func (f filterUnclonedMachinesPredicate) Update(e event.UpdateEvent) bool {
	return f.Generic(event.GenericEvent{
		Meta:   e.MetaNew,
		Object: e.ObjectNew,
	})
}

// Generic implements default GenericEvent filter
func (f filterUnclonedMachinesPredicate) Generic(e event.GenericEvent) bool {
	if e.Object == nil {
		f.log.Error(nil, "Generic event has no old metadata", "event", e)
		return false
	}

	if e.Meta == nil {
		f.log.Error(nil, "Generic event has no new metadata", "event", e)
		return false
	}

	// when watching machines, we only care about machines users created one-off
	// outside of machinedeployments/machinesets and using AzureMachineTemplates. if a machine is part of a machineset
	// or machinedeployment, we already created a secret for the template. All machines
	// in the machinedeployment will share that one secret.
	gvk := infrav1.GroupVersion.WithKind("AzureMachineTemplate")
	isClonedFromTemplate := e.Meta.GetAnnotations()[clusterv1.TemplateClonedFromGroupKindAnnotation] == gvk.GroupKind().String()

	return !isClonedFromTemplate
}

// Reconcile reconciles the azure json for a specific machine not in a machine deployment
func (r *AzureJSONMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "azureMachine", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureJSONMachineReconciler.Reconcile",
		trace.WithAttributes(
			label.String("namespace", req.Namespace),
			label.String("name", req.Name),
			label.String("kind", "AzureMachine"),
		))
	defer span.End()

	// Fetch the AzureMachine instance
	azureMachine := &infrav1.AzureMachine{}
	err := r.Get(ctx, req.NamespacedName, azureMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("object was not found")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, azureMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, azureMachine) {
		log.Info("AzureMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

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

	apiVersion, kind := infrav1.GroupVersion.WithKind("AzureMachine").ToAPIVersionAndKind()
	owner := metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       azureMachine.GetName(),
		UID:        azureMachine.GetUID(),
	}

	// Construct secret for this machine
	userAssignedIdentityIfExists := ""
	if len(azureMachine.Spec.UserAssignedIdentities) > 0 {
		userAssignedIdentityIfExists = azureMachine.Spec.UserAssignedIdentities[0].ProviderID
	}

	newSecret, err := GetCloudProviderSecret(
		clusterScope,
		azureMachine.Namespace,
		azureMachine.Name,
		owner,
		azureMachine.Spec.Identity,
		userAssignedIdentityIfExists,
	)

	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create cloud provider config")
	}

	if err := reconcileAzureSecret(ctx, log, r.Client, owner, newSecret, clusterScope.ClusterName()); err != nil {
		r.Recorder.Eventf(azureMachine, corev1.EventTypeWarning, "Error reconciling cloud provider secret for AzureMachine", err.Error())
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile azure secret")
	}

	return ctrl.Result{}, nil
}
