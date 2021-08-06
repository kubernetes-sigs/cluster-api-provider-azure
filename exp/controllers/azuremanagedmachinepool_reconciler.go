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
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	"sigs.k8s.io/cluster-api/controllers/remote"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedMachinePoolScopeName is the sourceName, or more specifically the UserAgent, of client used in AMMP reconciliation.
	ManagedMachinePoolScopeName = "azuremanagedmachinepoolmachine-scope"
)

type (
	// azureManagedMachinePoolService contains the services required by the cluster controller.
	azureManagedMachinePoolService struct {
		kubeclient    client.Client
		agentPoolsSvc azure.OldService
		scaleSetsSvc  NodeLister
	}

	// AgentPoolVMSSNotFoundError represents a reconcile error when the VMSS for an agent pool can't be found.
	AgentPoolVMSSNotFoundError struct {
		NodeResourceGroup string
		PoolName          string
	}

	// NodeLister is a service interface for returning generic lists.
	NodeLister interface {
		ListInstances(context.Context, string, string) ([]compute.VirtualMachineScaleSetVM, error)
		List(context.Context, string) ([]compute.VirtualMachineScaleSet, error)
	}
)

var (
	notFoundErr = new(AgentPoolVMSSNotFoundError)
)

// NewAgentPoolVMSSNotFoundError creates a new AgentPoolVMSSNotFoundError.
func NewAgentPoolVMSSNotFoundError(nodeResourceGroup, poolName string) *AgentPoolVMSSNotFoundError {
	return &AgentPoolVMSSNotFoundError{
		NodeResourceGroup: nodeResourceGroup,
		PoolName:          poolName,
	}
}

func (a *AgentPoolVMSSNotFoundError) Error() string {
	msgFmt := "failed to find vm scale set in resource group %s matching pool named %s"
	return fmt.Sprintf(msgFmt, a.NodeResourceGroup, a.PoolName)
}

// Is returns true if the target error is an `AgentPoolVMSSNotFoundError`.
func (a *AgentPoolVMSSNotFoundError) Is(target error) bool {
	var err *AgentPoolVMSSNotFoundError
	ok := errors.As(target, &err)
	return ok
}

// newAzureManagedMachinePoolService populates all the services based on input scope.
func newAzureManagedMachinePoolService(scope *scope.ManagedControlPlaneScope) *azureManagedMachinePoolService {
	return &azureManagedMachinePoolService{
		kubeclient:    scope.Client,
		agentPoolsSvc: agentpools.NewService(scope),
		scaleSetsSvc:  scalesets.NewClient(scope),
	}
}

// Reconcile reconciles all the services in a predetermined order.
func (s *azureManagedMachinePoolService) Reconcile(ctx context.Context, scope *scope.ManagedControlPlaneScope) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedMachinePoolService.Reconcile")
	defer span.End()

	scope.Logger.Info("reconciling machine pool")

	var normalizedVersion *string
	if scope.MachinePool.Spec.Template.Spec.Version != nil {
		v := strings.TrimPrefix(*scope.MachinePool.Spec.Template.Spec.Version, "v")
		normalizedVersion = &v
	}

	replicas := int32(1)
	if scope.MachinePool.Spec.Replicas != nil {
		replicas = *scope.MachinePool.Spec.Replicas
	}

	agentPoolSpec := &agentpools.Spec{
		Name:          scope.InfraMachinePool.Name,
		ResourceGroup: scope.ControlPlane.Spec.ResourceGroupName,
		Cluster:       scope.ControlPlane.Name,
		SKU:           scope.InfraMachinePool.Spec.SKU,
		Replicas:      replicas,
		Version:       normalizedVersion,
		VnetSubnetID: azure.SubnetID(
			scope.ControlPlane.Spec.SubscriptionID,
			scope.ControlPlane.Spec.ResourceGroupName,
			scope.ControlPlane.Spec.VirtualNetwork.Name,
			scope.ControlPlane.Spec.VirtualNetwork.Subnet.Name,
		),
		Mode: scope.InfraMachinePool.Spec.Mode,
	}

	if scope.InfraMachinePool.Spec.OSDiskSizeGB != nil {
		agentPoolSpec.OSDiskSizeGB = *scope.InfraMachinePool.Spec.OSDiskSizeGB
	}

	scope.V(2).Info("Reconciling agentpool")
	if err := s.agentPoolsSvc.Reconcile(ctx, agentPoolSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile machine pool %s", scope.InfraMachinePool.Name)
	}

	clusterClient, err := remote.NewClusterClient(ctx, ManagedMachinePoolScopeName, s.kubeclient, types.NamespacedName{Namespace: scope.Cluster.Namespace, Name: scope.Cluster.Name})
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve kubeconfig for cluster")
	}

	matchingLabels := client.MatchingLabels(map[string]string{
		"agentpool": scope.InfraMachinePool.Name,
	})

	scope.V(2).Info(fmt.Sprintf("Listing node in agentpool '%s'", scope.InfraMachinePool.Name))
	var nodeList corev1.NodeList
	if err := clusterClient.List(ctx, &nodeList, matchingLabels); err != nil {
		return errors.Wrapf(err, "failed to list nodes to extract providerIDs")
	}

	var providerIDs = make([]string, len(nodeList.Items))
	for i := 0; i < len(nodeList.Items); i++ {
		providerIDs[i] = nodeList.Items[i].Spec.ProviderID
	}

	scope.InfraMachinePool.Spec.ProviderIDList = providerIDs
	scope.InfraMachinePool.Status.Replicas = int32(len(providerIDs))
	scope.InfraMachinePool.Status.Ready = int32(len(providerIDs)) == *scope.MachinePool.Spec.Replicas

	scope.V(2).Info("reconciled machine pool successfully")
	return nil
}

// Delete reconciles all the services in a predetermined order.
func (s *azureManagedMachinePoolService) Delete(ctx context.Context, scope *scope.ManagedControlPlaneScope) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedMachinePoolService.Delete")
	defer span.End()

	agentPoolSpec := &agentpools.Spec{
		Name:          scope.InfraMachinePool.Name,
		ResourceGroup: scope.ControlPlane.Spec.ResourceGroupName,
		Cluster:       scope.ControlPlane.Name,
	}

	if err := s.agentPoolsSvc.Delete(ctx, agentPoolSpec); err != nil {
		return errors.Wrapf(err, "failed to delete machine pool %s", scope.InfraMachinePool.Name)
	}

	return nil
}

// IsAgentPoolVMSSNotFoundError returns true if the error is an AgentPoolVMSSNotFoundError.
func IsAgentPoolVMSSNotFoundError(err error) bool {
	return errors.Is(err, notFoundErr)
}
