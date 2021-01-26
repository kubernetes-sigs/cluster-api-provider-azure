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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	infracontroller "sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureManagedMachinePoolReconciler reconciles a AzureManagedMachinePool object
type AzureManagedMachinePoolReconciler struct {
	client.Client
	Log                                  logr.Logger
	Recorder                             record.EventRecorder
	ReconcileTimeout                     time.Duration
	createAzureManagedMachinePoolService azureManagedMachinePoolServiceCreator
}

type azureManagedMachinePoolServiceCreator func(managedControlPlaneScope *scope.ManagedControlPlaneScope) *azureManagedMachinePoolService

// NewAzureManagedMachinePoolReconciler returns a new AzureManagedMachinePoolReconciler instance
func NewAzureManagedMachinePoolReconciler(client client.Client, log logr.Logger, recorder record.EventRecorder, reconcileTimeout time.Duration) *AzureManagedMachinePoolReconciler {
	ampr := &AzureManagedMachinePoolReconciler{
		Client:           client,
		Log:              log,
		Recorder:         recorder,
		ReconcileTimeout: reconcileTimeout,
	}

	ampr.createAzureManagedMachinePoolService = newAzureManagedMachinePoolService

	return ampr
}

// SetupWithManager initializes this controller with a manager.
func (r *AzureManagedMachinePoolReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	log := r.Log.WithValues("controller", "AzureManagedMachinePool")
	azManagedMachinePool := &infrav1exp.AzureManagedMachinePool{}
	// create mapper to transform incoming AzureManagedClusters into AzureManagedMachinePool requests
	azureManagedClusterMapper, err := AzureManagedClusterToAzureManagedMachinePoolsMapper(r.Client, mgr.GetScheme(), log)
	if err != nil {
		return errors.Wrapf(err, "failed to create AzureManagedCluster to AzureManagedMachinePools mapper")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(azManagedMachinePool).
		WithEventFilter(predicates.ResourceNotPaused(log)). // don't queue reconcile if resource is paused
		// watch for changes in CAPI MachinePool resources
		Watches(
			&source.Kind{Type: &clusterv1exp.MachinePool{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureManagedMachinePool")),
			},
		).
		// watch for changes in AzureManagedClusters
		Watches(
			&source.Kind{Type: &infrav1exp.AzureManagedCluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: azureManagedClusterMapper,
			},
		).
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: util.ClusterToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureManagedMachinePool")),
		},
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=exp.cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch

// Reconcile idempotently gets, creates, and updates a machine pool.
func (r *AzureManagedMachinePoolReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "azureManagedMachinePool", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureManagedMachinePoolReconciler.Reconcile",
		trace.WithAttributes(
			label.String("namespace", req.Namespace),
			label.String("name", req.Name),
			label.String("kind", "AzureManagedMachinePool"),
		))
	defer span.End()

	// Fetch the AzureManagedMachinePool instance
	infraPool := &infrav1exp.AzureManagedMachinePool{}
	err := r.Get(ctx, req.NamespacedName, infraPool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the owning MachinePool.
	ownerPool, err := infracontroller.GetOwnerMachinePool(ctx, r.Client, infraPool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if ownerPool == nil {
		log.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	// Fetch the Cluster.
	ownerCluster, err := util.GetOwnerCluster(ctx, r.Client, ownerPool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if ownerCluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("ownerCluster", ownerCluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(ownerCluster, infraPool) {
		log.Info("AzureManagedMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	// Fetch the corresponding control plane which has all the interesting data.
	controlPlane := &infrav1exp.AzureManagedControlPlane{}
	controlPlaneName := client.ObjectKey{
		Namespace: ownerCluster.Spec.ControlPlaneRef.Namespace,
		Name:      ownerCluster.Spec.ControlPlaneRef.Name,
	}
	if err := r.Client.Get(ctx, controlPlaneName, controlPlane); err != nil {
		return reconcile.Result{}, err
	}

	// Create the scope.
	mcpScope, err := scope.NewManagedControlPlaneScope(scope.ManagedControlPlaneScopeParams{
		Client:           r.Client,
		Logger:           log,
		ControlPlane:     controlPlane,
		Cluster:          ownerCluster,
		MachinePool:      ownerPool,
		InfraMachinePool: infraPool,
		PatchTarget:      infraPool,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always patch when exiting so we can persist changes to finalizers and status
	defer func() {
		if err := mcpScope.PatchObject(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !infraPool.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, mcpScope)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, mcpScope)
}

func (r *AzureManagedMachinePoolReconciler) reconcileNormal(ctx context.Context, scope *scope.ManagedControlPlaneScope) (reconcile.Result, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureManagedMachinePoolReconciler.reconcileNormal")
	defer span.End()

	scope.Logger.Info("Reconciling AzureManagedMachinePool")

	// If the AzureManagedMachinePool doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(scope.InfraMachinePool, infrav1.ClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := scope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.createAzureManagedMachinePoolService(scope).Reconcile(ctx, scope); err != nil {
		if IsAgentPoolVMSSNotFoundError(err) {
			// if the underlying VMSS is not yet created, requeue for 30s in the future
			return reconcile.Result{
				RequeueAfter: 30 * time.Second,
			}, nil
		}
		return reconcile.Result{}, errors.Wrapf(err, "error creating AzureManagedMachinePool %s/%s", scope.InfraMachinePool.Namespace, scope.InfraMachinePool.Name)
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	scope.InfraMachinePool.Status.Ready = true

	return reconcile.Result{}, nil
}

func (r *AzureManagedMachinePoolReconciler) reconcileDelete(ctx context.Context, scope *scope.ManagedControlPlaneScope) (reconcile.Result, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureManagedMachinePoolReconciler.reconcileDelete")
	defer span.End()

	scope.Logger.Info("Reconciling AzureManagedMachinePool delete")

	if err := r.createAzureManagedMachinePoolService(scope).Delete(ctx, scope); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting AzureManagedMachinePool %s/%s", scope.InfraMachinePool.Namespace, scope.InfraMachinePool.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(scope.InfraMachinePool, infrav1.ClusterFinalizer)

	if err := scope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
