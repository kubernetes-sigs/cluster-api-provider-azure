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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	infracontroller "sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/coalescing"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AzureManagedControlPlaneReconciler reconciles an AzureManagedControlPlane object.
type AzureManagedControlPlaneReconciler struct {
	client.Client
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
	WatchFilterValue string
}

// SetupWithManager initializes this controller with a manager.
func (amcpr *AzureManagedControlPlaneReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options infracontroller.Options) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AzureManagedControlPlaneReconciler.SetupWithManager",
		tele.KVP("controller", "AzureManagedControlPlane"),
	)
	defer done()

	var r reconcile.Reconciler = amcpr
	if options.Cache != nil {
		r = coalescing.NewReconciler(amcpr, options.Cache, log)
	}

	azManagedControlPlane := &infrav1exp.AzureManagedControlPlane{}
	// create mapper to transform incoming AzureManagedClusters into AzureManagedControlPlane requests
	azureManagedClusterMapper, err := AzureManagedClusterToAzureManagedControlPlaneMapper(ctx, amcpr.Client, log)
	if err != nil {
		return errors.Wrap(err, "failed to create AzureManagedCluster to AzureManagedControlPlane mapper")
	}

	// map requests for machine pools corresponding to AzureManagedControlPlane's defaultPool back to the corresponding AzureManagedControlPlane.
	azureManagedMachinePoolMapper := MachinePoolToAzureManagedControlPlaneMapFunc(ctx, amcpr.Client, infrav1exp.GroupVersion.WithKind("AzureManagedControlPlane"), log)

	// map requests for Cluster corresponding to AzureManagedControlPlane back to the corresponding AzureManagedControlPlane.
	clusterMapper := ClusterToAzureManagedControlPlaneMapper(log)

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options.Options).
		For(azManagedControlPlane).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, amcpr.WatchFilterValue)).
		// watch AzureManagedCluster resources
		Watches(
			&source.Kind{Type: &infrav1exp.AzureManagedCluster{}},
			handler.EnqueueRequestsFromMapFunc(azureManagedClusterMapper),
		).
		// watch MachinePool resources
		Watches(
			&source.Kind{Type: &clusterv1exp.MachinePool{}},
			handler.EnqueueRequestsFromMapFunc(azureManagedMachinePoolMapper),
		).
		// Add a watch on clusterv1.Cluster object for unpause notifications.
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(clusterMapper),
		).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureManagedControlPlane"))),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
		predicates.ResourceNotPausedAndHasFilterLabel(log, amcpr.WatchFilterValue),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile idempotently gets, creates, and updates a managed control plane.
func (amcpr *AzureManagedControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(amcpr.ReconcileTimeout))
	defer cancel()

	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AzureManagedControlPlaneReconciler.Reconcile",
		tele.KVP("namespace", req.Namespace),
		tele.KVP("name", req.Name),
		tele.KVP("kind", "AzureManagedControlPlane"),
	)
	defer done()

	// Fetch the AzureManagedControlPlane instance
	azureControlPlane := &infrav1exp.AzureManagedControlPlane{}
	err := amcpr.Get(ctx, req.NamespacedName, azureControlPlane)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, amcpr.Client, azureControlPlane.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, azureControlPlane) {
		log.Info("AzureManagedControlPlane or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	// check if the control plane's namespace is allowed for this identity and update owner references for the identity.
	if azureControlPlane.Spec.IdentityRef != nil {
		identity, err := infracontroller.GetClusterIdentityFromRef(ctx, amcpr.Client, azureControlPlane.Namespace, azureControlPlane.Spec.IdentityRef)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !scope.IsClusterNamespaceAllowed(ctx, amcpr.Client, identity.Spec.AllowedNamespaces, azureControlPlane.Namespace) {
			return reconcile.Result{}, errors.New("AzureClusterIdentity list of allowed namespaces doesn't include current azure managed control plane namespace")
		}
	} else {
		warningMessage := ("You're using deprecated functionality: ")
		warningMessage += ("Using Azure credentials from the manager environment is deprecated and will be removed in future releases. ")
		warningMessage += ("Please specify an AzureClusterIdentity for the AzureManagedControlPlane instead, see: https://capz.sigs.k8s.io/topics/multitenancy.html ")
		log.Info(fmt.Sprintf("WARNING, %s", warningMessage))
		amcpr.Recorder.Eventf(azureControlPlane, corev1.EventTypeWarning, "AzureClusterIdentity", warningMessage)
	}

	// Create the scope.
	mcpScope, err := scope.NewManagedControlPlaneScope(ctx, scope.ManagedControlPlaneScopeParams{
		Client:       amcpr.Client,
		Cluster:      cluster,
		ControlPlane: azureControlPlane,
		PatchTarget:  azureControlPlane,
	})
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create scope")
	}

	// Always patch when exiting so we can persist changes to finalizers and status
	defer func() {
		if err := mcpScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !azureControlPlane.DeletionTimestamp.IsZero() {
		return amcpr.reconcileDelete(ctx, mcpScope)
	}
	// Handle non-deleted clusters
	return amcpr.reconcileNormal(ctx, mcpScope)
}

func (amcpr *AzureManagedControlPlaneReconciler) reconcileNormal(ctx context.Context, scope *scope.ManagedControlPlaneScope) (reconcile.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AzureManagedControlPlaneReconciler.reconcileNormal")
	defer done()

	log.Info("Reconciling AzureManagedControlPlane")

	// If the AzureManagedControlPlane doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(scope.ControlPlane, infrav1.ClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := scope.PatchObject(ctx); err != nil {
		amcpr.Recorder.Eventf(scope.ControlPlane, corev1.EventTypeWarning, "AzureManagedControlPlane unavailable", "failed to patch resource: %s", err)
		return reconcile.Result{}, err
	}

	if err := newAzureManagedControlPlaneReconciler(scope).Reconcile(ctx); err != nil {
		// Handle transient and terminal errors
		log := log.WithValues("name", scope.ControlPlane.Name, "namespace", scope.ControlPlane.Namespace)
		var reconcileError azure.ReconcileError
		if errors.As(err, &reconcileError) {
			if reconcileError.IsTerminal() {
				log.Error(err, "failed to reconcile AzureManagedControlPlane")
				return reconcile.Result{}, nil
			}

			if reconcileError.IsTransient() {
				log.V(4).Info("requeuing due to transient transient failure", "error", err)
				return reconcile.Result{RequeueAfter: reconcileError.RequeueAfter()}, nil
			}

			return reconcile.Result{}, errors.Wrap(err, "failed to reconcile AzureManagedControlPlane")
		}

		return reconcile.Result{}, errors.Wrapf(err, "error creating AzureManagedControlPlane %s/%s", scope.ControlPlane.Namespace, scope.ControlPlane.Name)
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	scope.ControlPlane.Status.Ready = true
	scope.ControlPlane.Status.Initialized = true
	amcpr.Recorder.Event(scope.ControlPlane, corev1.EventTypeNormal, "AzureManagedControlPlane available", "successfully reconciled")
	return reconcile.Result{}, nil
}

func (amcpr *AzureManagedControlPlaneReconciler) reconcileDelete(ctx context.Context, scope *scope.ManagedControlPlaneScope) (reconcile.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AzureManagedControlPlaneReconciler.reconcileDelete")
	defer done()

	log.Info("Reconciling AzureManagedControlPlane delete")

	if err := newAzureManagedControlPlaneReconciler(scope).Delete(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting AzureManagedControlPlane %s/%s", scope.ControlPlane.Namespace, scope.ControlPlane.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(scope.ControlPlane, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}
