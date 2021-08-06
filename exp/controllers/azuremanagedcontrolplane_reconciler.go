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

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/managedclusters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualnetworks"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureManagedControlPlaneService contains the services required by the cluster controller.
type azureManagedControlPlaneService struct {
	kubeclient         client.Client
	scope              managedclusters.ManagedClusterScope
	managedClustersSvc azure.Reconciler
	groupsSvc          azure.Reconciler
	vnetSvc            azure.Reconciler
	subnetsSvc         azure.Reconciler
}

// newAzureManagedControlPlaneReconciler populates all the services based on input scope.
func newAzureManagedControlPlaneReconciler(scope *scope.ManagedControlPlaneScope) *azureManagedControlPlaneService {
	return &azureManagedControlPlaneService{
		kubeclient:         scope.Client,
		scope:              scope,
		managedClustersSvc: managedclusters.New(scope),
		groupsSvc:          groups.New(scope),
		vnetSvc:            virtualnetworks.New(scope),
		subnetsSvc:         subnets.New(scope),
	}
}

// Reconcile reconciles all the services in a predetermined order.
func (r *azureManagedControlPlaneService) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedControlPlaneService.Reconcile")
	defer span.End()

	if err := r.groupsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile managed cluster resource group")
	}

	if err := r.vnetSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile virtual network")
	}

	if err := r.subnetsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile subnet")
	}

	// Send to Azure for create/update.
	if err := r.managedClustersSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile managed cluster")
	}

	if err := r.reconcileKubeconfig(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile kubeconfig secret")
	}

	return nil
}

// Delete reconciles all the services in a predetermined order.
func (r *azureManagedControlPlaneService) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedControlPlaneService.Delete")
	defer span.End()

	if err := r.managedClustersSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete managed cluster")
	}

	if err := r.vnetSvc.Delete(ctx); err != nil {
		return errors.Wrap(err, "failed to delete virtual network")
	}

	if err := r.groupsSvc.Delete(ctx); err != nil && !errors.Is(err, azure.ErrNotOwned) {
		return errors.Wrap(err, "failed to delete managed cluster resource group")
	}

	return nil
}

func (r *azureManagedControlPlaneService) reconcileKubeconfig(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedControlPlaneService.reconcileKubeconfig")
	defer span.End()

	kubeConfigData := r.scope.GetKubeConfigData()
	if kubeConfigData == nil {
		return nil
	}
	kubeConfigSecret := r.scope.MakeEmptyKubeConfigSecret()

	// Always update credentials in case of rotation
	if _, err := controllerutil.CreateOrUpdate(ctx, r.kubeclient, &kubeConfigSecret, func() error {
		kubeConfigSecret.Data = map[string][]byte{
			secret.KubeconfigDataName: kubeConfigData,
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to kubeconfig secret for cluster")
	}

	return nil
}
