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
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
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
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureClusterReconciler reconciles an AzureCluster object.
type AzureClusterReconciler struct {
	client.Client
	Log                       logr.Logger
	Recorder                  record.EventRecorder
	ReconcileTimeout          time.Duration
	WatchFilterValue          string
	createAzureClusterService azureClusterServiceCreator
}

type azureClusterServiceCreator func(clusterScope *scope.ClusterScope) (*azureClusterService, error)

// NewAzureClusterReconciler returns a new AzureClusterReconciler instance.
func NewAzureClusterReconciler(client client.Client, log logr.Logger, recorder record.EventRecorder, reconcileTimeout time.Duration, watchFilterValue string) *AzureClusterReconciler {
	acr := &AzureClusterReconciler{
		Client:           client,
		Log:              log,
		Recorder:         recorder,
		ReconcileTimeout: reconcileTimeout,
		WatchFilterValue: watchFilterValue,
	}

	acr.createAzureClusterService = newAzureClusterService

	return acr
}

// SetupWithManager initializes this controller with a manager.
func (r *AzureClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := r.Log.WithValues("controller", "AzureCluster")
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.AzureCluster{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(ctrl.LoggerFrom(ctx))).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("AzureCluster"))),
		predicates.ClusterUnpaused(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azuremachinetemplates;azuremachinetemplates/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureclusteridentities;azureclusteridentities/status,verbs=get;list;watch;create;update;patch;delete

// Reconcile idempotently gets, creates, and updates a cluster.
func (r *AzureClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "azureCluster", req.Name)

	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureClusterReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("namespace", req.Namespace),
			attribute.String("name", req.Name),
			attribute.String("kind", "AzureCluster"),
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

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, azureCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		r.Recorder.Eventf(azureCluster, corev1.EventTypeNormal, "OwnerRefNotSet", "Cluster Controller has not yet set OwnerRef")
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, azureCluster) {
		r.Recorder.Eventf(azureCluster, corev1.EventTypeNormal, "ClusterPaused", "AzureCluster or linked Cluster is marked as paused. Won't reconcile")
		log.Info("AzureCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	if azureCluster.Spec.IdentityRef != nil {
		identity, err := GetClusterIdentityFromRef(ctx, r.Client, azureCluster.Namespace, azureCluster.Spec.IdentityRef)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !scope.IsClusterNamespaceAllowed(ctx, r.Client, identity.Spec.AllowedNamespaces, azureCluster.Namespace) {
			conditions.MarkFalse(azureCluster, infrav1.NetworkInfrastructureReadyCondition, infrav1.NamespaceNotAllowedByIdentity, clusterv1.ConditionSeverityError, "")
			return reconcile.Result{}, errors.New("AzureClusterIdentity list of allowed namespaces doesn't include current cluster namespace")
		}
		if identity.Namespace == azureCluster.Namespace {
			patchhelper, err := patch.NewHelper(identity, r.Client)
			if err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to init patch helper")
			}
			identity.ObjectMeta.OwnerReferences = azureCluster.GetOwnerReferences()
			if err := patchhelper.Patch(ctx, identity); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else {
		warningMessage := ("You're using deprecated functionality: ")
		warningMessage += ("Using Azure credentials from the manager environment is deprecated and will be removed in future releases. ")
		warningMessage += ("Please specify an AzureClusterIdentity for the AzureCluster instead, see: https://capz.sigs.k8s.io/topics/multitenancy.html ")
		log.Info(fmt.Sprintf("WARNING, %s", warningMessage))
		r.Recorder.Eventf(azureCluster, corev1.EventTypeWarning, "AzureClusterIdentity", warningMessage)
	}

	// Create the scope.
	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client:       r.Client,
		Logger:       log,
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	if err != nil {
		err = errors.Errorf("failed to create scope: %+v", err)
		r.Recorder.Eventf(azureCluster, corev1.EventTypeWarning, "CreateClusterScopeFailed", err.Error())
		return reconcile.Result{}, err
	}

	// Always close the scope when exiting this function so we can persist any AzureMachine changes.
	defer func() {
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
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureClusterReconciler.reconcileNormal")
	defer span.End()

	clusterScope.Info("Reconciling AzureCluster")
	azureCluster := clusterScope.AzureCluster

	// If the AzureCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(azureCluster, infrav1.ClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := clusterScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	acr, err := r.createAzureClusterService(clusterScope)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create a new AzureClusterReconciler")
	}

	if err := acr.Reconcile(ctx); err != nil {
		wrappedErr := errors.Wrap(err, "failed to reconcile cluster services")
		r.Recorder.Eventf(azureCluster, corev1.EventTypeWarning, "ClusterReconcilerNormalFailed", wrappedErr.Error())
		return reconcile.Result{}, wrappedErr
	}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull them
	azureCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: clusterScope.APIServerHost(),
		Port: clusterScope.APIServerPort(),
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	azureCluster.Status.Ready = true
	conditions.MarkTrue(azureCluster, infrav1.NetworkInfrastructureReadyCondition)

	return reconcile.Result{}, nil
}

func (r *AzureClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.AzureClusterReconciler.reconcileDelete")
	defer span.End()

	clusterScope.Info("Reconciling AzureCluster delete")

	azureCluster := clusterScope.AzureCluster
	conditions.MarkFalse(azureCluster, infrav1.NetworkInfrastructureReadyCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "")
	if err := clusterScope.PatchObject(ctx); err != nil {
		return reconcile.Result{}, err
	}

	acr, err := r.createAzureClusterService(clusterScope)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create a new AzureClusterReconciler")
	}

	if err := acr.Delete(ctx); err != nil {
		wrappedErr := errors.Wrapf(err, "error deleting AzureCluster %s/%s", azureCluster.Namespace, azureCluster.Name)
		r.Recorder.Eventf(azureCluster, corev1.EventTypeWarning, "ClusterReconcilerDeleteFailed", wrappedErr.Error())
		conditions.MarkFalse(azureCluster, infrav1.NetworkInfrastructureReadyCondition, clusterv1.DeletionFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
		return reconcile.Result{}, wrappedErr
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(clusterScope.AzureCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}
