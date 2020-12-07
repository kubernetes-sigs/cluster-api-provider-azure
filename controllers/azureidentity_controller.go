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

	aadpodv1 "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/util/identity"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AzureIdentityReconciler reconciles azure identity objects
type AzureIdentityReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
}

// SetupWithManager initializes this controller with a manager
func (r *AzureIdentityReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	log := r.Log.WithValues("controller", "AzureIdentity")
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.AzureCluster{}).
		WithEventFilter(predicates.ResourceNotPaused(log)). // don't queue reconcile if resource is paused
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: util.ClusterToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("AzureCluster")),
		},
		predicates.ClusterUnpaused(log),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=aadpodidentity.k8s.io,resources=azureidentities;azureidentities/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=aadpodidentity.k8s.io,resources=azureidentitybindings;azureidentitybindings/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

// Reconcile reconciles the Azure identity.
func (r *AzureIdentityReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "azureIdentity", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureIdentityReconciler.Reconcile",
		trace.WithAttributes(
			label.String("namespace", req.Namespace),
			label.String("name", req.Name),
			label.String("kind", "AzureCluster"),
		))
	defer span.End()

	// Fetch the AzureCluster instance
	azureCluster := &infrav1.AzureCluster{}
	err := r.Get(ctx, req.NamespacedName, azureCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Recorder.Eventf(azureCluster, corev1.EventTypeNormal, "AzureClusterObjectNotFound", err.Error())
			log.Info("object was not found")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithValues("azurecluster", azureCluster.Name)

	// get all the bindings
	var bindings aadpodv1.AzureIdentityBindingList
	if err := r.List(ctx, &bindings, client.InNamespace(infrav1.ControllerNamespace)); err != nil {
		return ctrl.Result{}, err
	}

	bindingsToDelete := []aadpodv1.AzureIdentityBinding{}
	for _, b := range bindings.Items {
		log = log.WithValues("azureidentitybinding", b.Name)

		binding := b
		clusterName := binding.ObjectMeta.Labels[clusterv1.ClusterLabelName]
		clusterNamespace := binding.ObjectMeta.Labels[infrav1.ClusterLabelNamespace]

		key := client.ObjectKey{Name: clusterName, Namespace: clusterNamespace}
		azCluster := &infrav1.AzureCluster{}
		if err := r.Get(ctx, key, azCluster); err != nil {
			if apierrors.IsNotFound(err) {
				bindingsToDelete = append(bindingsToDelete, b)
				continue
			} else {
				return ctrl.Result{}, errors.Wrapf(err, "failed to get AzureCluster")
			}

		}
		expectedIdentityName := identity.GetAzureIdentityName(azCluster.Name, azCluster.Namespace, azCluster.Spec.IdentityRef.Name)
		if binding.Spec.AzureIdentity != expectedIdentityName {
			bindingsToDelete = append(bindingsToDelete, b)
		}
	}

	// delete bindings and identites no longer used by a cluster
	for _, bindingToDelete := range bindingsToDelete {
		binding := bindingToDelete
		identityName := binding.Spec.AzureIdentity
		if err := r.Client.Delete(ctx, &binding); err != nil {
			r.Recorder.Eventf(azureCluster, corev1.EventTypeWarning, "Error reconciling AzureIdentity for AzureCluster", err.Error())
			log.Error(err, "failed to delete AzureIdentityBinding")
			return ctrl.Result{}, err
		}
		azureIdentity := &aadpodv1.AzureIdentity{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: identityName, Namespace: infrav1.ControllerNamespace}, azureIdentity); err != nil {
			log.Error(err, "failed to fetch AzureIdentity")
			return ctrl.Result{}, err
		}
		if err := r.Client.Delete(ctx, azureIdentity); err != nil {
			r.Recorder.Eventf(azureCluster, corev1.EventTypeWarning, "Error reconciling AzureIdentity for AzureCluster", err.Error())
			log.Error(err, "failed to delete AzureIdentity")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
