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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/pkg/errors"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/agentpools"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/scalesets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// azureManagedMachinePoolReconciler are list of services required by cluster controller
type azureManagedMachinePoolReconciler struct {
	kubeclient    client.Client
	agentPoolsSvc azure.OldService
	scaleSetsSvc  NodeLister
}

// NodeLister is a service interface for returning generic lists.
type NodeLister interface {
	ListInstances(context.Context, string, string) ([]compute.VirtualMachineScaleSetVM, error)
	List(context.Context, string) ([]compute.VirtualMachineScaleSet, error)
}

// newAzureManagedMachinePoolReconciler populates all the services based on input scope
func newAzureManagedMachinePoolReconciler(scope *scope.ManagedControlPlaneScope) *azureManagedMachinePoolReconciler {
	return &azureManagedMachinePoolReconciler{
		kubeclient:    scope.Client,
		agentPoolsSvc: agentpools.NewService(scope),
		scaleSetsSvc:  scalesets.NewService(scope),
	}
}

// Reconcile reconciles all the services in pre determined order
func (r *azureManagedMachinePoolReconciler) Reconcile(ctx context.Context, scope *scope.ManagedControlPlaneScope) error {
	scope.Logger.Info("reconciling machine pool")
	agentPoolSpec := &agentpools.Spec{
		Name:          scope.InfraMachinePool.Name,
		ResourceGroup: scope.ControlPlane.Spec.ResourceGroup,
		Cluster:       scope.ControlPlane.Name,
		SKU:           scope.InfraMachinePool.Spec.SKU,
		Replicas:      1,
		Version:       scope.MachinePool.Spec.Template.Spec.Version,
	}

	if scope.InfraMachinePool.Spec.OSDiskSizeGB != nil {
		agentPoolSpec.OSDiskSizeGB = *scope.InfraMachinePool.Spec.OSDiskSizeGB
	}

	if scope.MachinePool.Spec.Replicas != nil {
		agentPoolSpec.Replicas = *scope.MachinePool.Spec.Replicas
	}

	if err := r.agentPoolsSvc.Reconcile(ctx, agentPoolSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile machine pool %s", scope.InfraMachinePool.Name)
	}

	nodeResourceGroup := fmt.Sprintf("MC_%s_%s_%s", scope.ControlPlane.Spec.ResourceGroup, scope.ControlPlane.Name, scope.ControlPlane.Spec.Location)
	vmss, err := r.scaleSetsSvc.List(ctx, nodeResourceGroup)
	if err != nil {
		return errors.Wrapf(err, "failed to list vmss in resource group %s", nodeResourceGroup)
	}

	var match *compute.VirtualMachineScaleSet
	for _, ss := range vmss {
		ss := ss
		if ss.Tags["poolName"] != nil && *ss.Tags["poolName"] == scope.InfraMachinePool.Name {
			match = &ss
			break
		}
	}

	if match == nil {
		return errors.New("failed to find vm scale set matching pool")
	}

	instances, err := r.scaleSetsSvc.ListInstances(ctx, nodeResourceGroup, *match.Name)
	if err != nil {
		return errors.Wrapf(err, "failed to reconcile machine pool %s", scope.InfraMachinePool.Name)
	}

	var providerIDs = make([]string, len(instances))
	for _, vm := range instances {
		vm := vm
		providerIDs = append(providerIDs, fmt.Sprintf("azure://%s", *vm.ID))
	}

	scope.InfraMachinePool.Spec.ProviderIDList = providerIDs
	scope.InfraMachinePool.Status.Replicas = int32(len(providerIDs))
	scope.InfraMachinePool.Status.Ready = true

	scope.Logger.Info("reconciled machine pool successfully")
	return nil
}

// Delete reconciles all the services in pre determined order
func (r *azureManagedMachinePoolReconciler) Delete(ctx context.Context, scope *scope.ManagedControlPlaneScope) error {
	agentPoolSpec := &agentpools.Spec{
		Name:          scope.InfraMachinePool.Name,
		ResourceGroup: scope.ControlPlane.Spec.ResourceGroup,
		Cluster:       scope.ControlPlane.Name,
		SKU:           scope.InfraMachinePool.Spec.SKU,
	}

	if err := r.agentPoolsSvc.Delete(ctx, agentPoolSpec); err != nil {
		return errors.Wrapf(err, "failed to delete machine pool %s", scope.InfraMachinePool.Name)
	}

	return nil
}
