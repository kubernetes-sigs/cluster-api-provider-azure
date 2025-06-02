/*
Copyright 2025 The Kubernetes Authors.

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

// Package controllers provides a way to reconcile ARO resources.
package controllers

import (
	"context"
	errorsCore "errors"
	"fmt"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/controllers"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav2exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/coalescing"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const (
	// AROControlPlaneFinalizer allows the controller to clean up resources on delete.
	AROControlPlaneFinalizer = "arocontrolplane.controlplane.cluster.x-k8s.io"

	// AROControlPlaneForceDeleteAnnotation annotation can be set to force the deletion of AROControlPlane bypassing any deletion validations/errors.
	AROControlPlaneForceDeleteAnnotation = "controlplane.cluster.x-k8s.io/arocontrolplane-force-delete"

	// ExternalAuthProviderLastAppliedAnnotation annotation tracks the last applied external auth configuration to inform if an update is required.
	ExternalAuthProviderLastAppliedAnnotation = "controlplane.cluster.x-k8s.io/arocontrolplane-last-applied-external-auth-provider"
)

var errInvalidClusterKind = errors.New("AROControlPlane cannot be used without AROCluster")

// ErrNoAROClusterDefined is returned when no AROCluster is defined in the AROControlPlane spec.
var ErrNoAROClusterDefined = fmt.Errorf("no %s AROCluster defined in AROControlPlane spec.resources", infrav2exp.GroupVersion.Group)

// AROControlPlaneReconciler reconciles a AROControlPlane object.
type AROControlPlaneReconciler struct {
	client.Client
	WatchFilterValue                string
	CredentialCache                 azure.CredentialCache
	Timeouts                        reconciler.Timeouts
	getNewAROControlPlaneReconciler func(scope *scope.AROControlPlaneScope) (*aroControlPlaneService, error)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AROControlPlaneReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controllers.Options) error {
	_, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROControlPlaneReconciler.SetupWithManager",
		tele.KVP("controller", cplane.AROControlPlaneKind),
	)
	defer done()

	r.getNewAROControlPlaneReconciler = newAROControlPlaneService

	var reconciler reconcile.Reconciler = r
	if options.Cache != nil {
		reconciler = coalescing.NewReconciler(r, options.Cache, log)
	}

	_, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options.Options).
		For(&cplane.AROControlPlane{}, builder.WithPredicates(
			predicate.And(
				predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue),
				predicate.Or(
					predicate.GenerationChangedPredicate{},
					predicate.AnnotationChangedPredicate{},
					predicate.LabelChangedPredicate{},
				),
			),
		)).
		Watches(&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToAROControlPlane),
			builder.WithPredicates(
				predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue),
				predicates.ClusterPausedTransitions(mgr.GetScheme(), log),
			),
		).
		// User errors that CAPZ passes through agentPoolProfiles on create must be fixed in the
		// AROMachinePool, so trigger a reconciliation to consume those fixes.
		Watches(
			&infrav2exp.AROMachinePool{},
			handler.EnqueueRequestsFromMapFunc(r.aroMachinePoolToAROControlPlane),
		).
		// Watch for changes to ASO HcpOpenShiftCluster resources
		// Only reconcile on spec changes (generation changed), not status updates
		Watches(
			&asoredhatopenshiftv1.HcpOpenShiftCluster{},
			handler.EnqueueRequestsFromMapFunc(r.hcpClusterToAROControlPlane),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&corev1.Secret{}).
		Build(reconciler)
	if err != nil {
		return fmt.Errorf("failed setting up the AROControlPlane controller manager: %w", err)
	}

	return nil
}

func clusterToAROControlPlane(_ context.Context, o client.Object) []ctrl.Request {
	cluster := o.(*clusterv1.Cluster)
	controlPlaneRef := cluster.Spec.ControlPlaneRef
	if controlPlaneRef.IsDefined() &&
		controlPlaneRef.APIGroup == infrav2exp.GroupVersion.Group &&
		controlPlaneRef.Kind == cplane.AROControlPlaneKind {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: cluster.Namespace, Name: controlPlaneRef.Name}}}
	}
	return nil
}

func (r *AROControlPlaneReconciler) aroMachinePoolToAROControlPlane(ctx context.Context, o client.Object) []ctrl.Request {
	aroMachinePool := o.(*infrav2exp.AROMachinePool)
	clusterName := aroMachinePool.Labels[clusterv1.ClusterNameLabel]
	if clusterName == "" {
		return nil
	}
	cluster, err := util.GetClusterByName(ctx, r.Client, aroMachinePool.Namespace, clusterName)
	if client.IgnoreNotFound(err) != nil || cluster == nil {
		return nil
	}
	return clusterToAROControlPlane(ctx, cluster)
}

// hcpClusterToAROControlPlane maps ASO HcpOpenShiftCluster changes to the owning AROControlPlane.
func (r *AROControlPlaneReconciler) hcpClusterToAROControlPlane(_ context.Context, o client.Object) []ctrl.Request {
	hcpCluster := o.(*asoredhatopenshiftv1.HcpOpenShiftCluster)

	// Find the owning AROControlPlane from owner references
	for _, ref := range hcpCluster.OwnerReferences {
		if ref.APIVersion == cplane.GroupVersion.Identifier() && ref.Kind == cplane.AROControlPlaneKind {
			return []ctrl.Request{{
				NamespacedName: client.ObjectKey{
					Namespace: hcpCluster.Namespace,
					Name:      ref.Name,
				},
			}}
		}
	}

	return nil
}

//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes/finalizers,verbs=update
//+kubebuilder:rbac:groups=redhatopenshift.azure.com,resources=hcpopenshiftclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=redhatopenshift.azure.com,resources=hcpopenshiftclusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=redhatopenshift.azure.com,resources=hcpopenshiftclustersexternalauths,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=redhatopenshift.azure.com,resources=hcpopenshiftclustersexternalauths/status,verbs=get;list;watch

// Reconcile will reconcile AROControlPlane resources.
func (r *AROControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, r.Timeouts.DefaultedLoopTimeout())
	defer cancel()

	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROControlPlaneReconciler.Reconcile",
		tele.KVP("namespace", req.Namespace),
		tele.KVP("name", req.Name),
		tele.KVP("kind", cplane.AROControlPlaneKind),
	)
	defer done()

	log = log.WithValues("namespace", req.Namespace, "AROControlPlane", req.Name)

	// Get the control plane instance
	aroControlPlane := &cplane.AROControlPlane{}
	err := r.Get(ctx, req.NamespacedName, aroControlPlane)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Create a patch helper for the AROControlPlane to ensure status fields are persisted
	patchHelper, err := patch.NewHelper(aroControlPlane, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patch helper: %w", err)
	}
	defer func() {
		patchErr := patchHelper.Patch(ctx, aroControlPlane)
		if patchErr != nil && resultErr == nil {
			resultErr = patchErr
			result = ctrl.Result{}
		}
	}()

	// Initialize the Initialization status early so it's available even if reconciliation fails early
	if aroControlPlane.Status.Initialization == nil {
		aroControlPlane.Status.Initialization = &cplane.AROControlPlaneInitializationStatus{ControlPlaneInitialized: false}
	}

	// Get the cluster
	cluster, err := util.GetOwnerCluster(ctx, r.Client, aroControlPlane.ObjectMeta)
	if err != nil {
		log.Error(err, "Failed to retrieve owner Cluster from the API Server")
		return ctrl.Result{}, err
	}

	if cluster != nil {
		_ = log.WithValues("cluster", cluster.Name)
	}

	// Create the scope.
	aroScope, err := scope.NewAROControlPlaneScope(ctx, scope.AROControlPlaneScopeParams{
		Client:          r.Client,
		Cluster:         cluster,
		ControlPlane:    aroControlPlane,
		Timeouts:        r.Timeouts,
		CredentialCache: r.CredentialCache,
	})

	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	// Always close the scope
	defer func() {
		if err := aroScope.Close(ctx); err != nil {
			resultErr = errorsCore.Join(resultErr, err)
			result = ctrl.Result{}
		}
	}()

	if cluster != nil && cluster.Spec.Paused != nil && *cluster.Spec.Paused ||
		annotations.HasPaused(aroControlPlane) {
		return r.reconcilePaused(ctx, aroScope)
	}

	if !aroControlPlane.GetDeletionTimestamp().IsZero() {
		// Handle deletion reconciliation loop.
		return r.reconcileDelete(ctx, aroScope)
	}

	return r.reconcileNormal(ctx, aroScope, cluster)
}

func (r *AROControlPlaneReconciler) reconcileNormal(ctx context.Context, scope *scope.AROControlPlaneScope, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROControlPlaneReconciler.reconcileNormal",
	)
	defer done()

	log.Info("Reconciling AROControlPlane")

	if cluster == nil {
		log.V(4).Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}
	infraRef := cluster.Spec.InfrastructureRef
	if !infraRef.IsDefined() ||
		infraRef.APIGroup != infrav2exp.GroupVersion.Group ||
		infraRef.Kind != infrav2exp.AROClusterKind {
		return ctrl.Result{}, reconcile.TerminalError(errInvalidClusterKind)
	}

	aroControlPlane := scope.ControlPlane
	// Register our finalizer immediately to avoid orphaning Azure resources on delete
	needsPatch := controllerutil.AddFinalizer(aroControlPlane, cplane.AROControlPlaneFinalizer)
	// Register the block-move annotation immediately to avoid moving un-paused ASO resources
	needsPatch = controllers.AddBlockMoveAnnotation(aroControlPlane) || needsPatch
	if needsPatch {
		return ctrl.Result{Requeue: true}, nil
	}

	if aroControlPlane.Spec.AroClusterName == "" {
		return ctrl.Result{}, reconcile.TerminalError(ErrNoAROClusterDefined)
	}

	svc, err := r.getNewAROControlPlaneReconciler(scope)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create aroControlPlane service")
	}
	if err := svc.Reconcile(ctx); err != nil {
		// Handle transient and terminal errors
		log := log.WithValues("name", scope.ControlPlane.Name, "namespace", scope.ControlPlane.Namespace)
		var reconcileError azure.ReconcileError
		if errors.As(err, &reconcileError) {
			if reconcileError.IsTerminal() {
				log.Error(err, "failed to reconcile AROControlPlane")
				return reconcile.Result{}, nil
			}

			if reconcileError.IsTransient() {
				log.V(4).Info("requeuing due to transient failure", "error", err)
				return reconcile.Result{RequeueAfter: reconcileError.RequeueAfter()}, nil
			}

			return reconcile.Result{}, errors.Wrap(err, "failed to reconcile AROControlPlane")
		}

		return reconcile.Result{}, errors.Wrapf(err, "error creating AROControlPlane %s/%s", scope.ControlPlane.Namespace, scope.ControlPlane.Name)
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	scope.ControlPlane.Status.Ready = (aroControlPlane.Status.APIURL != "")
	// Always initialize the Initialization status if not set
	if scope.ControlPlane.Status.Initialization == nil {
		scope.ControlPlane.Status.Initialization = &cplane.AROControlPlaneInitializationStatus{}
	}
	scope.ControlPlane.Status.Initialization.ControlPlaneInitialized = scope.ControlPlane.Status.Ready

	return ctrl.Result{}, nil
}

func (r *AROControlPlaneReconciler) reconcilePaused(ctx context.Context, scope *scope.AROControlPlaneScope) (ctrl.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AROControlPlaneReconciler.reconcilePaused")
	defer done()

	log.Info("Reconciling AROControlPlane pause")

	svc, err := r.getNewAROControlPlaneReconciler(scope)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create aroControlPlane service")
	}
	if err := svc.Pause(ctx); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to pause control plane services")
	}
	controllers.RemoveBlockMoveAnnotation(scope.ControlPlane)

	return reconcile.Result{}, nil
}

// +kubebuilder:rbac:groups=managedidentity.azure.com,resources=userassignedidentities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=managedidentity.azure.com,resources=userassignedidentities/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=authorization.azure.com,resources=roleassignments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authorization.azure.com,resources=roleassignments/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=keyvault.azure.com,resources=vaults,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.azure.com,resources=networksecuritygroups,verbs=get;list;watch;create;update;patch;delete

func (r *AROControlPlaneReconciler) reconcileDelete(ctx context.Context, scope *scope.AROControlPlaneScope) (ctrl.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROControlPlaneReconciler.reconcileDelete",
	)
	defer done()

	log.Info("Reconciling AROControlPlane delete")

	svc, err := r.getNewAROControlPlaneReconciler(scope)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create aroControlPlane service")
	}
	if err := svc.Delete(ctx); err != nil {
		// Handle transient errors
		var reconcileError azure.ReconcileError
		if errors.As(err, &reconcileError) && reconcileError.IsTransient() {
			if azure.IsOperationNotDoneError(reconcileError) {
				log.V(2).Info(fmt.Sprintf("AROControlPlane delete not done: %s", reconcileError.Error()))
			} else {
				log.V(2).Info("transient failure to delete AROControlPlane, retrying")
			}
			return reconcile.Result{RequeueAfter: reconcileError.RequeueAfter()}, nil
		}
		return reconcile.Result{}, errors.Wrapf(err, "error deleting AROControlPlane %s/%s", scope.ControlPlane.Namespace, scope.ControlPlane.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(scope.ControlPlane, cplane.AROControlPlaneFinalizer)

	if scope.ControlPlane.Spec.IdentityRef != nil {
		err := controllers.RemoveClusterIdentityFinalizer(ctx, r.Client, scope.ControlPlane, scope.ControlPlane.Spec.IdentityRef, infrav1.ManagedClusterFinalizer)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}
