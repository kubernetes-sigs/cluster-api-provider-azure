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

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	infracontroller "sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureManagedControlPlaneReconciler reconciles an AzureManagedControlPlane object.
type AzureManagedControlPlaneReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
	WatchFilterValue string
}

// SetupWithManager initializes this controller with a manager.
func (r *AzureManagedControlPlaneReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := r.Log.WithValues("controller", "AzureManagedControlPlane")
	azManagedControlPlane := &infrav1exp.AzureManagedControlPlane{}
	// create mapper to transform incoming AzureManagedClusters into AzureManagedControlPlane requests
	azureManagedClusterMapper, err := AzureManagedClusterToAzureManagedControlPlaneMapper(ctx, r.Client, log)
	if err != nil {
		return errors.Wrap(err, "failed to create AzureManagedCluster to AzureManagedControlPlane mapper")
	}

	// map requests for machine pools corresponding to AzureManagedControlPlane's defaultPool back to the corresponding AzureManagedControlPlane.
	azureManagedMachinePoolMapper := MachinePoolToAzureManagedControlPlaneMapFunc(ctx, r.Client, infrav1exp.GroupVersion.WithKind("AzureManagedControlPlane"), log)

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(azManagedControlPlane).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
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
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureManagedControlPlane"))),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile idempotently gets, creates, and updates a managed control plane.
func (r *AzureManagedControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "azureManagedControlPlane", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureManagedControlPlaneReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("namespace", req.Namespace),
			attribute.String("name", req.Name),
			attribute.String("kind", "AzureManagedControlPlane"),
		))
	defer span.End()

	// Fetch the AzureManagedControlPlane instance
	azureControlPlane := &infrav1exp.AzureManagedControlPlane{}
	err := r.Get(ctx, req.NamespacedName, azureControlPlane)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, azureControlPlane.ObjectMeta)
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
		identity, err := infracontroller.GetClusterIdentityFromRef(ctx, r.Client, azureControlPlane.Namespace, azureControlPlane.Spec.IdentityRef)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !scope.IsClusterNamespaceAllowed(ctx, r.Client, identity.Spec.AllowedNamespaces, azureControlPlane.Namespace) {
			return reconcile.Result{}, errors.New("AzureClusterIdentity list of allowed namespaces doesn't include current azure managed control plane namespace")
		}
	} else {
		warningMessage := ("You're using deprecated functionality: ")
		warningMessage += ("Using Azure credentials from the manager environment is deprecated and will be removed in future releases. ")
		warningMessage += ("Please specify an AzureClusterIdentity for the AzureManagedControlPlane instead, see: https://capz.sigs.k8s.io/topics/multitenancy.html ")
		log.Info(fmt.Sprintf("WARNING, %s", warningMessage))
		r.Recorder.Eventf(azureControlPlane, corev1.EventTypeWarning, "AzureClusterIdentity", warningMessage)
	}

	// Create the scope.
	mcpScope, err := scope.NewManagedControlPlaneScope(ctx, scope.ManagedControlPlaneScopeParams{
		Client:       r.Client,
		Logger:       log,
		Cluster:      cluster,
		ControlPlane: azureControlPlane,
		PatchTarget:  azureControlPlane,
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
	if !azureControlPlane.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, mcpScope)
	}
	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, mcpScope)
}

func (r *AzureManagedControlPlaneReconciler) reconcileNormal(ctx context.Context, scope *scope.ManagedControlPlaneScope) (reconcile.Result, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureManagedControlPlaneReconciler.reconcileNormal")
	defer span.End()

	scope.Logger.Info("Reconciling AzureManagedControlPlane")

	// If the AzureManagedControlPlane doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(scope.ControlPlane, infrav1.ClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := scope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if err := newAzureManagedControlPlaneReconciler(scope).Reconcile(ctx, scope); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error creating AzureManagedControlPlane %s/%s", scope.ControlPlane.Namespace, scope.ControlPlane.Name)
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	scope.ControlPlane.Status.Ready = true
	scope.ControlPlane.Status.Initialized = true

	return reconcile.Result{}, nil
}

func (r *AzureManagedControlPlaneReconciler) reconcileDelete(ctx context.Context, scope *scope.ManagedControlPlaneScope) (reconcile.Result, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureManagedControlPlaneReconciler.reconcileDelete")
	defer span.End()

	scope.Logger.Info("Reconciling AzureManagedControlPlane delete")

	if err := newAzureManagedControlPlaneReconciler(scope).Delete(ctx, scope); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting AzureManagedControlPlane %s/%s", scope.ControlPlane.Namespace, scope.ControlPlane.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(scope.ControlPlane, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}
