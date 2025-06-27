/*
Copyright 2024 The Kubernetes Authors.

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

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	infracontroller "sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav2exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AROClusterReconciler reconciles a AROCluster object.
type AROClusterReconciler struct {
	client.Client
	WatchFilterValue string
}

// SetupWithManager sets up the controller with the Manager.
func (r *AROClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROClusterReconciler.SetupWithManager",
		tele.KVP("controller", infrav2exp.AROClusterKind),
	)
	defer done()

	_, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav2exp.AROCluster{}).
		WithEventFilter(predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue)).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(mgr.GetScheme(), log)).
		// Watch clusters for pause/unpause notifications
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(
				util.ClusterToInfrastructureMapFunc(ctx, infrav2exp.GroupVersion.WithKind(infrav2exp.AROClusterKind), mgr.GetClient(), &infrav2exp.AROCluster{}),
			),
			builder.WithPredicates(
				predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue),
				infracontroller.ClusterUpdatePauseChange(log),
			),
		).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating new controller manager: %w", err)
	}

	return nil
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
		tele.KVP("kind", infrav2exp.AROClusterKind),
	)
	defer done()

	log = log.WithValues("namespace", req.Namespace, "AROCluster", req.Name)

	// Fetch the AROCluster instance
	aroCluster := &infrav2exp.AROCluster{}
	err := r.Get(ctx, req.NamespacedName, aroCluster)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = fmt.Errorf("not implemented")
	log.Error(err, fmt.Sprintf("Reconciling %s", infrav2exp.AROClusterKind))
	return ctrl.Result{}, err
}
