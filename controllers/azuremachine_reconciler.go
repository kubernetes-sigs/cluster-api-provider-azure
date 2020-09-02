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
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/tags"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/inboundnatrules"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/roleassignments"

	"github.com/pkg/errors"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/disks"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines"
)

// azureMachineService is the group of services called by the AzureMachine controller.
type azureMachineService struct {
	networkInterfacesSvc azure.Service
	inboundNatRulesSvc   azure.Service
	virtualMachinesSvc   azure.Service
	roleAssignmentsSvc   azure.Service
	disksSvc             azure.Service
	publicIPsSvc         azure.Service
	tagsSvc              azure.Service
	skuCache             *resourceskus.Cache
}

// newAzureMachineService populates all the services based on input scope.
func newAzureMachineService(machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) *azureMachineService {
	cache := resourceskus.NewCache(clusterScope, clusterScope.Location())

	return &azureMachineService{
		inboundNatRulesSvc:   inboundnatrules.NewService(machineScope),
		networkInterfacesSvc: networkinterfaces.NewService(machineScope, cache),
		virtualMachinesSvc:   virtualmachines.NewService(machineScope, cache),
		roleAssignmentsSvc:   roleassignments.NewService(machineScope),
		disksSvc:             disks.NewService(machineScope),
		publicIPsSvc:         publicips.NewService(machineScope),
		tagsSvc:              tags.NewService(machineScope),
		skuCache:             cache,
	}
}

// Reconcile reconciles all the services in pre determined order
func (s *azureMachineService) Reconcile(ctx context.Context) error {
	if err := s.publicIPsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to create public IP")
	}

	if err := s.inboundNatRulesSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to create inbound NAT rule")
	}

	if err := s.networkInterfacesSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to create network interface")
	}

	if err := s.virtualMachinesSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to create virtual machine")
	}

	if err := s.roleAssignmentsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "unable to create role assignment")
	}

	if err := s.tagsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "unable to update tags")
	}

	return nil
}

// Delete deletes all the services in pre determined order
func (s *azureMachineService) Delete(ctx context.Context) error {
	if err := s.virtualMachinesSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete machine")
	}

	if err := s.networkInterfacesSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete network interface")
	}

	if err := s.inboundNatRulesSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete inbound NAT rule")
	}

	if err := s.publicIPsSvc.Delete(ctx); err != nil {
		return errors.Wrap(err, "failed to delete public IPs")
	}

	if err := s.disksSvc.Delete(ctx); err != nil {
		return errors.Wrap(err, "failed to delete OS disk")
	}

	return nil
}

// Delete deletes the VM and its disk so it can be replaced.
func (s *azureMachineService) DeleteVM(ctx context.Context) error {
	if err := s.virtualMachinesSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete machine")
	}
	if err := s.disksSvc.Delete(ctx); err != nil {
		return errors.Wrap(err, "failed to delete OS disk")
	}
	return nil
}
