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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/controllers"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infra "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/mutators"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

var errInvalidControlPlaneKind = errors.New("AROCluster cannot be used without AROControlPlane")

// AROClusterReconciler reconciles a AROCluster object.
type AROClusterReconciler struct {
	client.Client
	WatchFilterValue string

	newResourceReconciler func(*infra.AROCluster, []*unstructured.Unstructured) *controllers.ResourceReconciler
}

// SetupWithManager sets up the controller with the Manager.
func (r *AROClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROClusterReconciler.SetupWithManager",
		tele.KVP("controller", infra.AROClusterKind),
	)
	defer done()

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infra.AROCluster{}).
		WithEventFilter(predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue)).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(mgr.GetScheme(), log)).
		// Watch clusters for pause/unpause notifications
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(
				util.ClusterToInfrastructureMapFunc(ctx, infra.GroupVersion.WithKind(infra.AROClusterKind), mgr.GetClient(), &infra.AROCluster{}),
			),
			builder.WithPredicates(
				predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue),
				predicates.ClusterPausedTransitions(mgr.GetScheme(), log),
			),
		).
		Watches(
			&cplane.AROControlPlane{},
			handler.EnqueueRequestsFromMapFunc(aroControlPlaneToAroClusterMap(r.Client, log)),
			builder.WithPredicates(
				predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue),
				predicate.Funcs{
					CreateFunc: func(ev event.CreateEvent) bool {
						controlPlane := ev.Object.(*cplane.AROControlPlane)
						return controlPlane.Status.APIURL != ""
					},
					UpdateFunc: func(ev event.UpdateEvent) bool {
						oldControlPlane := ev.ObjectOld.(*cplane.AROControlPlane)
						newControlPlane := ev.ObjectNew.(*cplane.AROControlPlane)
						return (oldControlPlane.Status.APIURL != newControlPlane.Status.APIURL) ||
							(oldControlPlane.Status.Ready != newControlPlane.Status.Ready)
					},
				},
			),
		).
		Build(r)
	if err != nil {
		return err
	}

	externalTracker := &external.ObjectTracker{
		Cache:           mgr.GetCache(),
		Controller:      c,
		Scheme:          mgr.GetScheme(),
		PredicateLogger: &log,
	}

	r.newResourceReconciler = func(aroCluster *infra.AROCluster, resources []*unstructured.Unstructured) *controllers.ResourceReconciler {
		return controllers.NewResourceReconciler(r.Client, resources, aroCluster, controllers.WithWatcher(externalTracker))
	}

	return nil
}

func aroControlPlaneToAroClusterMap(c client.Client, log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		aroControlPlane, ok := o.(*cplane.AROControlPlane)
		if !ok {
			log.Error(fmt.Errorf("expected a AROControlPlane, got %T instead", o), "failed to map AROControlPlane")
			return nil
		}

		log := log.WithValues("objectMapper", "arocpToaroc", "AROcontrolplane", klog.KRef(aroControlPlane.Namespace, aroControlPlane.Name))

		if !aroControlPlane.ObjectMeta.DeletionTimestamp.IsZero() {
			log.Info("AROControlPlane has a deletion timestamp, skipping mapping")
			return nil
		}

		if aroControlPlane.Status.APIURL == "" {
			log.V(4).Info("AROControlPlane has no control plane endpoint, skipping mapping")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, c, aroControlPlane.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to get owning cluster")
			return nil
		}
		if cluster == nil {
			log.Info("no owning cluster, skipping mapping")
			return nil
		}

		aroClusterRef := cluster.Spec.InfrastructureRef
		if !aroClusterRef.IsDefined() ||
			aroClusterRef.APIGroup != infra.GroupVersion.Group ||
			aroClusterRef.Kind != infra.AROClusterKind {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: cluster.Namespace,
					Name:      aroClusterRef.Name,
				},
			},
		}
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aroclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aroclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aroclusters/finalizers,verbs=update

// Reconcile reconciles an AROCluster.
func (r *AROClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, resultErr error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROClusterReconciler.Reconcile",
		tele.KVP("namespace", req.Namespace),
		tele.KVP("name", req.Name),
		tele.KVP("kind", infra.AROClusterKind),
	)
	defer done()

	_ = log.WithValues("namespace", req.Namespace, "AROCluster", req.Name)

	// Fetch the AROCluster instance
	aroCluster := &infra.AROCluster{}
	err := r.Get(ctx, req.NamespacedName, aroCluster)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchHelper, err := patch.NewHelper(aroCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patch helper: %w", err)
	}
	defer func() {
		err := patchHelper.Patch(ctx, aroCluster)
		if err != nil && resultErr == nil {
			resultErr = err
			result = ctrl.Result{}
		}
	}()

	aroCluster.Status.Ready = false

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, aroCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	if cluster != nil && cluster.Spec.Paused != nil && *cluster.Spec.Paused ||
		annotations.HasPaused(aroCluster) {
		return r.reconcilePaused(ctx, aroCluster)
	}

	if !aroCluster.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(ctx, aroCluster)
	}

	return r.reconcileNormal(ctx, aroCluster, cluster)
}

func matchesAROControlPlaneAPIGroup(apiGroup string) bool {
	return apiGroup == cplane.GroupVersion.Group
}

func (r *AROClusterReconciler) reconcileNormal(ctx context.Context, aroCluster *infra.AROCluster, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROClusterReconciler.reconcileNormal",
	)
	defer done()
	log.V(4).Info("reconciling normally")

	if cluster == nil {
		log.V(4).Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}
	controlPlaneRef := cluster.Spec.ControlPlaneRef
	if !controlPlaneRef.IsDefined() ||
		!matchesAROControlPlaneAPIGroup(controlPlaneRef.APIGroup) ||
		controlPlaneRef.Kind != cplane.AROControlPlaneKind {
		return ctrl.Result{}, reconcile.TerminalError(errInvalidControlPlaneKind)
	}

	needsPatch := controllerutil.AddFinalizer(aroCluster, infra.AROClusterFinalizer)
	needsPatch = controllers.AddBlockMoveAnnotation(aroCluster) || needsPatch
	if needsPatch {
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile ASO resources if specified
	if len(aroCluster.Spec.Resources) > 0 {
		resources, err := mutators.ToUnstructured(ctx, aroCluster.Spec.Resources)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to convert resources to unstructured: %w", err)
		}
		resourceReconciler := r.newResourceReconciler(aroCluster, resources)
		err = resourceReconciler.Reconcile(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile resources: %w", err)
		}

		// Update infrastructure ready condition based on resource status
		totalResources := len(aroCluster.Status.Resources)
		readyResources := 0
		var notReadyResources []string
		for _, status := range aroCluster.Status.Resources {
			if status.Ready {
				readyResources++
			} else {
				notReadyResources = append(notReadyResources, fmt.Sprintf("%s/%s", status.Resource.Kind, status.Resource.Name))
			}
		}

		if readyResources == totalResources && totalResources > 0 {
			conditions.Set(aroCluster, metav1.Condition{
				Type:    string(infra.ResourcesReadyCondition),
				Status:  metav1.ConditionTrue,
				Reason:  "InfrastructureReady",
				Message: fmt.Sprintf("All %d infrastructure resources are ready", totalResources),
			})
			log.V(4).Info("all resources are ready", "total", totalResources)
		} else {
			conditions.Set(aroCluster, metav1.Condition{
				Type:    string(infra.ResourcesReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  "ProvisioningInfrastructure",
				Message: fmt.Sprintf("Waiting for infrastructure resources: %d/%d ready", readyResources, totalResources),
			})
			log.V(4).Info("waiting for resources to be ready", "ready", readyResources, "total", totalResources, "notReady", notReadyResources)
			return ctrl.Result{}, nil
		}
	}

	aroControlPlane := &cplane.AROControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      controlPlaneRef.Name,
		},
	}
	err := r.Get(ctx, client.ObjectKeyFromObject(aroControlPlane), aroControlPlane)
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get AROControlPlane %s/%s: %w", aroControlPlane.Namespace, aroControlPlane.Name, err)
	}

	endpoint, err := r.getControlPlaneEndpoint(aroControlPlane.Status.APIURL)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get enpoint for url %s from AROControlPlane %s/%s: %w", aroControlPlane.Status.APIURL, aroControlPlane.Namespace, aroControlPlane.Name, err)
	}

	// For ARO HCP, infrastructure is provisioned when ResourcesReady is true
	// The control plane is externally managed and comes up separately
	resourcesReady := false
	for _, condition := range aroCluster.Status.Conditions {
		if condition.Type == string(infra.ResourcesReadyCondition) && condition.Status == metav1.ConditionTrue {
			resourcesReady = true
			break
		}
	}

	// Set the values from the managed control plane
	aroCluster.Spec.ControlPlaneEndpoint = endpoint

	// AROCluster.Status.Ready depends on AROControlPlane.Status.Ready
	// AROControlPlane already checks BOTH HcpClusterReady + kubeconfig, so we just use its Ready status
	aroCluster.Status.Ready = aroControlPlane.Status.Ready && !aroCluster.Spec.ControlPlaneEndpoint.IsZero()

	// Initialization.Provisioned follows Status.Ready to create proper dependency chain:
	// ResourcesReady → AROControlPlane.Ready → AROCluster.Ready → Initialization.Provisioned
	// This ensures CAPI's InfrastructureProvisioned only becomes true when everything is actually ready
	if resourcesReady && aroCluster.Status.Ready {
		aroCluster.Status.Initialization = &infra.AROClusterInitializationStatus{Provisioned: true}
		log.V(4).Info("Infrastructure marked as provisioned (resources and control plane ready)")
	} else if aroCluster.Status.Initialization == nil {
		aroCluster.Status.Initialization = &infra.AROClusterInitializationStatus{Provisioned: false}
	} else if !aroCluster.Status.Ready {
		// If control plane not ready, ensure provisioned stays false
		aroCluster.Status.Initialization.Provisioned = false
	}
	if aroCluster.Status.Ready {
		conditions.Set(aroCluster, metav1.Condition{
			Type:   string(infrav1.NetworkInfrastructureReadyCondition),
			Status: metav1.ConditionTrue,
			Reason: "Succeeded",
		})
	} else {
		if !aroCluster.Spec.ControlPlaneEndpoint.IsZero() {
			conditions.Set(aroCluster, metav1.Condition{
				Type:    string(infrav1.NetworkInfrastructureReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  "ExternallyManagedControlPlane",
				Message: "Waiting for the Control Plane port",
			})
		} else {
			conditions.Set(aroCluster, metav1.Condition{
				Type:    string(infrav1.NetworkInfrastructureReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  "ExternallyManagedControlPlane",
				Message: "Waiting for the Control Plane to get ready",
			})
		}
	}

	log.Info("Successfully reconciled AROCluster", "Ready", aroCluster.Status.Ready)

	return ctrl.Result{}, nil
}

func (r *AROClusterReconciler) reconcilePaused(ctx context.Context, aroCluster *infra.AROCluster) (ctrl.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.AROClusterReconciler.reconcilePaused")
	defer done()
	log.V(4).Info("reconciling pause")

	// Pause ASO resources if specified
	if len(aroCluster.Spec.Resources) > 0 {
		resources, err := mutators.ToUnstructured(ctx, aroCluster.Spec.Resources)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to convert resources to unstructured: %w", err)
		}
		resourceReconciler := r.newResourceReconciler(aroCluster, resources)
		err = resourceReconciler.Pause(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to pause resources: %w", err)
		}
	}

	controllers.RemoveBlockMoveAnnotation(aroCluster)

	return ctrl.Result{}, nil
}

func (r *AROClusterReconciler) reconcileDelete(ctx context.Context, aroCluster *infra.AROCluster) (ctrl.Result, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROClusterReconciler.reconcileDelete",
	)
	defer done()
	log.V(4).Info("reconciling delete")

	// Delete ASO resources if specified
	if len(aroCluster.Spec.Resources) > 0 {
		resourceReconciler := r.newResourceReconciler(aroCluster, nil)
		err := resourceReconciler.Delete(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete resources: %w", err)
		}

		// Wait for all resources to be deleted before removing finalizer
		if len(aroCluster.Status.Resources) > 0 {
			log.V(4).Info("waiting for resources to be deleted", "remainingResources", len(aroCluster.Status.Resources))
			return ctrl.Result{}, nil
		}
	}

	controllerutil.RemoveFinalizer(aroCluster, infra.AROClusterFinalizer)
	return ctrl.Result{}, nil
}

func (r *AROClusterReconciler) getControlPlaneEndpoint(apiURL string) (clusterv1.APIEndpoint, error) {
	if apiURL == "" {
		return clusterv1.APIEndpoint{}, nil
	}
	u, err := url.ParseRequestURI(apiURL)
	if err != nil {
		return clusterv1.APIEndpoint{}, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return clusterv1.APIEndpoint{}, err
	}
	if port < 0 || port > 65535 {
		return clusterv1.APIEndpoint{}, fmt.Errorf("invalid port number: %d", port)
	}
	host := strings.Split(u.Host, ":")[0]
	return clusterv1.APIEndpoint{
		Host: host,
		Port: int32(port), //nolint:gosec // port range validated above
	}, nil
}
