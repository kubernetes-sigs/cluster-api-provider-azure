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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	// azureManagedMachinePoolService contains the services required by the cluster controller.
	azureManagedMachinePoolService struct {
		scope         agentpools.ManagedMachinePoolScope
		agentPoolsSvc azure.Reconciler
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
		scope:         scope,
		agentPoolsSvc: agentpools.New(scope),
		scaleSetsSvc:  scalesets.NewClient(scope),
	}
}

// Reconcile reconciles all the services in a predetermined order.
func (s *azureManagedMachinePoolService) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.azureManagedMachinePoolService.Reconcile")
	defer done()

	s.scope.Info("reconciling machine pool")
	agentPoolName := s.scope.AgentPoolSpec().Name

	if err := s.agentPoolsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile machine pool %s", agentPoolName)
	}

	nodeResourceGroup := s.scope.NodeResourceGroup()
	vmss, err := s.scaleSetsSvc.List(ctx, nodeResourceGroup)
	if err != nil {
		return errors.Wrapf(err, "failed to list vmss in resource group %s", nodeResourceGroup)
	}

	var match *compute.VirtualMachineScaleSet
	for _, ss := range vmss {
		ss := ss
		if ss.Tags["poolName"] != nil && *ss.Tags["poolName"] == agentPoolName {
			match = &ss
			break
		}
	}

	if match == nil {
		return NewAgentPoolVMSSNotFoundError(nodeResourceGroup, agentPoolName)
	}

	instances, err := s.scaleSetsSvc.ListInstances(ctx, nodeResourceGroup, *match.Name)
	if err != nil {
		return errors.Wrapf(err, "failed to reconcile machine pool %s", agentPoolName)
	}

	var providerIDs = make([]string, len(instances))
	for i := 0; i < len(instances); i++ {
		providerIDs[i] = strings.ToLower(azure.ProviderIDPrefix + *instances[i].ID)
	}

	s.scope.SetAgentPoolProviderIDList(providerIDs)
	s.scope.SetAgentPoolReplicas(int32(len(providerIDs)))
	s.scope.SetAgentPoolReady(true)

	s.scope.Info("reconciled machine pool successfully")
	return nil
}

// Delete reconciles all the services in a predetermined order.
func (s *azureManagedMachinePoolService) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.azureManagedMachinePoolService.Delete")
	defer done()

	if err := s.agentPoolsSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete machine pool %s", s.scope.AgentPoolSpec().Name)
	}

	return nil
}

// IsAgentPoolVMSSNotFoundError returns true if the error is an AgentPoolVMSSNotFoundError.
func IsAgentPoolVMSSNotFoundError(err error) bool {
	return errors.Is(err, notFoundErr)
}
