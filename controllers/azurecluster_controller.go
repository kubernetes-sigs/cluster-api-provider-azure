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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

// AzureClusterReconciler reconciles a AzureCluster object
type AzureClusterReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
}

func (r *AzureClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	log := r.Log.WithValues("controller", "AzureCluster")
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

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachinetemplates;azuremachinetemplates/status,verbs=get;list;watch

func (r *AzureClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "AzureCluster", req.Name)

	// Fetch the AzureCluster instance
	azureCluster := &infrav1.AzureCluster{}
	err := r.Get(ctx, req.NamespacedName, azureCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("object was not found")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, azureCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, azureCluster) {
		log.Info("AzureCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	// Create the scope.
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:       r.Client,
		Logger:       log,
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any AzureMachine changes.
	defer func() {
		conditions.SetSummary(azureCluster,
			conditions.WithConditions(
				infrav1.NetworkInfrastructureReadyCondition,
			),
			conditions.WithStepCounterIfOnly(
				infrav1.NetworkInfrastructureReadyCondition,
			),
		)

		if err := clusterScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !azureCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, clusterScope)
}

func (r *AzureClusterReconciler) reconcileNormal(ctx context.Context, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	clusterScope.Info("Reconciling AzureCluster")
	azureCluster := clusterScope.AzureCluster

	// If the AzureCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(azureCluster, infrav1.ClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := clusterScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	err := newAzureClusterReconciler(clusterScope).Reconcile(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to reconcile cluster services")
	}

	/*if azureCluster.Status.Network.APIServerIP.DNSName == "" {
		clusterScope.Info("Waiting for Load Balancer to exist")
		conditions.MarkFalse(azureCluster, infrav1.NetworkInfrastructureReadyCondition, infrav1.LoadBalancerProvisioningReason, clusterv1.ConditionSeverityWarning, err.Error())
		return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
	}*/
	log := klogr.New()
	/*log.Info(azureCluster.Status.Network.APIServerIP.DNSName)*/
	log.Info("Setting the control plane endpoint and Port")
	// Set APIEndpoints so the Cluster API Cluster Controller can pull them
	azureCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		//Host: azureCluster.Status.Network.APIServerIP.DNSName,
		//Host: "capz-cluster-e1da2561.dbelocal",
		Host: azureCluster.Spec.ControlPlaneEndpointIP,
		Port: clusterScope.APIServerPort(),
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	azureCluster.Status.Ready = true
	conditions.MarkTrue(azureCluster, infrav1.NetworkInfrastructureReadyCondition)

	return reconcile.Result{}, nil
}

func (r *AzureClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	clusterScope.Info("Reconciling AzureCluster delete")

	azureCluster := clusterScope.AzureCluster

	if err := newAzureClusterReconciler(clusterScope).Delete(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting AzureCluster %s/%s", azureCluster.Namespace, azureCluster.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(clusterScope.AzureCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}
