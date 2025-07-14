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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	control2exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav2exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AROControlPlaneReconciler reconciles a AroControlPlane object.
type AROControlPlaneReconciler struct {
	client.Client
	WatchFilterValue string
}

// SetupWithManager sets up the controller with the Manager.
func (r *AROControlPlaneReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	_, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROControlPlaneReconciler.SetupWithManager",
		tele.KVP("controller", control2exp.AROControlPlaneKind),
	)
	defer done()

	_, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&control2exp.AROControlPlane{}).
		WithEventFilter(predicates.ResourceHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue)).
		Owns(&corev1.Secret{}).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating new controller manager: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes/finalizers,verbs=update

// Reconcile reconciles an AROControlPlane.
func (r *AROControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, resultErr error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx,
		"controllers.AROControlPlaneReconciler.Reconcile",
		tele.KVP("namespace", req.Namespace),
		tele.KVP("name", req.Name),
		tele.KVP("kind", control2exp.AROControlPlaneKind),
	)
	defer done()

	log = log.WithValues("namespace", req.Namespace, "azureControlPlane", req.Name)

	aroControlPlane := &control2exp.AROControlPlane{}
	err := r.Get(ctx, req.NamespacedName, aroControlPlane)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = fmt.Errorf("not implemented")
	log.Error(err, fmt.Sprintf("Reconciling %s", infrav2exp.AROMachinePoolKind))
	return ctrl.Result{}, err
}
