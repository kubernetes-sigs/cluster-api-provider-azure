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

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/availabilitysets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/disks"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/inboundnatrules"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/roleassignments"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/tags"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/vmextensions"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
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
	vmExtensionsSvc      azure.Service
	availabilitySetsSvc  azure.Service
	skuCache             *resourceskus.Cache
}

var _ azure.Service = (*azureMachineService)(nil)

// newAzureMachineService populates all the services based on input scope.
func newAzureMachineService(machineScope *scope.MachineScope) (*azureMachineService, error) {
	cache, err := resourceskus.GetCache(machineScope, machineScope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}

	return &azureMachineService{
		inboundNatRulesSvc:   inboundnatrules.New(machineScope),
		networkInterfacesSvc: networkinterfaces.New(machineScope, cache),
		virtualMachinesSvc:   virtualmachines.New(machineScope, cache),
		roleAssignmentsSvc:   roleassignments.New(machineScope),
		disksSvc:             disks.New(machineScope),
		publicIPsSvc:         publicips.New(machineScope),
		tagsSvc:              tags.New(machineScope),
		vmExtensionsSvc:      vmextensions.New(machineScope),
		availabilitySetsSvc:  availabilitysets.New(machineScope, cache),
		skuCache:             cache,
	}, nil
}

// Reconcile reconciles all the services in pre determined order
func (s *azureMachineService) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureMachineService.Reconcile")
	defer span.End()

	if err := s.publicIPsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to create public IP")
	}

	if err := s.inboundNatRulesSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to create inbound NAT rule")
	}

	if err := s.networkInterfacesSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to create network interface")
	}

	if err := s.availabilitySetsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to create availability set")
	}

	if err := s.virtualMachinesSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to create virtual machine")
	}

	if err := s.roleAssignmentsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "unable to create role assignment")
	}

	if err := s.vmExtensionsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "unable to create vm extension")
	}

	if err := s.tagsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "unable to update tags")
	}

	return nil
}

// Delete deletes all the services in pre determined order
func (s *azureMachineService) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureMachineService.Delete")
	defer span.End()

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

	if err := s.availabilitySetsSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete availability set")
	}

	return nil
}
