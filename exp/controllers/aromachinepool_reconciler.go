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
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/pkg/errors"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/hcpopenshiftnodepools"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachines"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	// aroMachinePoolService contains the services required by the cluster controller.
	aroMachinePoolService struct {
		scope              *scope.AROMachinePoolScope
		agentPoolsSvc      azure.Reconciler
		virtualMachinesSvc NodeLister
	}

	// AgentPoolVMSSNotFoundError represents a reconcile error when the VMSS for an agent pool can't be found.
	AgentPoolVMSSNotFoundError struct {
		NodeResourceGroup string
		PoolName          string
	}

	// NodeLister is a service interface for returning generic lists.
	NodeLister interface {
		List(context.Context, string) ([]armcompute.VirtualMachine, error)
	}
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

// newAROMachinePoolService populates all the services based on input scope.
func newAROMachinePoolService(scope *scope.AROMachinePoolScope, apiCallTimeout time.Duration) (*aroMachinePoolService, error) {
	virtualMachinesAuthorizer, err := virtualMachinesAuthorizer(scope)
	if err != nil {
		return nil, err
	}
	virtualMachinesClient, err := virtualmachines.NewClient(virtualMachinesAuthorizer, apiCallTimeout)
	if err != nil {
		return nil, err
	}
	nodePoolService, err := hcpopenshiftnodepools.New(scope)
	if err != nil {
		return nil, err
	}
	return &aroMachinePoolService{
		scope:              scope,
		agentPoolsSvc:      nodePoolService,
		virtualMachinesSvc: virtualMachinesClient,
	}, nil
}

// virtualMachinesAuthorizer takes a scope and determines if a regional authorizer is needed for scale sets
// see https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/1850 for context on region based authorizer.
func virtualMachinesAuthorizer(scope *scope.AROMachinePoolScope) (azure.Authorizer, error) {
	/* TODO: mveber - why/how
	if scope.ControlPlane.Spec.AzureEnvironment == azure.PublicCloudName {
		return azure.WithRegionalBaseURI(scope, scope.Location()) // public cloud supports regional end points
	}
	*/

	return scope, nil
}

// Reconcile reconciles all the services in a predetermined order.
func (s *aroMachinePoolService) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroMachinePoolService.Reconcile")
	defer done()

	log.Info("reconciling ARO machine pool")

	if s.scope.InfraMachinePool.Spec.Autoscaling != nil && !annotations.ReplicasManagedByExternalAutoscaler(s.scope.MachinePool) {
		// make sure cluster.x-k8s.io/replicas-managed-by annotation is set on CAPI MachinePool when autoscaling is enabled.
		annotations.AddAnnotations(s.scope.MachinePool, map[string]string{
			clusterv1beta1.ReplicasManagedByAnnotation: "aro",
		})
	}

	agentPoolName := s.scope.Name()

	if err := s.agentPoolsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile ARO machine pool %s", agentPoolName)
	}

	nodeResourceGroup := s.scope.NodeResourceGroup()
	vmss, err := s.virtualMachinesSvc.List(ctx, nodeResourceGroup)
	if err != nil {
		return errors.Wrapf(err, "failed to list vmss in resource group %s", nodeResourceGroup)
	}

	namePrefix := s.scope.ClusterName() + "-" + s.scope.InfraMachinePool.Spec.NodePoolName + "-"
	var providerIDs []string
	for _, vm := range vmss {
		if vm.Name == nil || !strings.HasPrefix(*vm.Name, namePrefix) {
			continue
		}
		if vm.ID == nil {
			continue
		}
		providerIDs = append(providerIDs, "azure://"+*vm.ID)
	}
	currentReplicas := int32(len(providerIDs))

	if annotations.ReplicasManagedByExternalAutoscaler(s.scope.MachinePool) {
		// Set MachinePool replicas to aro autoscaling replicas
		if *s.scope.MachinePool.Spec.Replicas != currentReplicas {
			log.Info("Setting MachinePool replicas to aro autoscaling replicas",
				"local", *s.scope.MachinePool.Spec.Replicas,
				"external", currentReplicas)
			s.scope.MachinePool.Spec.Replicas = &currentReplicas
		}
	}

	s.scope.SetAgentPoolProviderIDList(providerIDs)
	s.scope.SetAgentPoolReplicas(currentReplicas)
	s.scope.SetAgentPoolReady(true)

	log.Info("reconciled ARO machine pool successfully")
	return nil
}

// Pause pauses all components making up the machine pool.
func (s *aroMachinePoolService) Pause(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.aroMachinePoolService.Pause")
	defer done()

	pauser, ok := s.agentPoolsSvc.(azure.Pauser)
	if !ok {
		return nil
	}
	if err := pauser.Pause(ctx); err != nil {
		return errors.Wrapf(err, "failed to pause machine pool %s", s.scope.Name())
	}

	return nil
}

// Delete reconciles all the services in a predetermined order.
func (s *aroMachinePoolService) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.aroMachinePoolService.Delete")
	defer done()

	if err := s.agentPoolsSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete machine pool %s", s.scope.Name())
	}

	return nil
}
