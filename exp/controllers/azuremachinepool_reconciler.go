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

	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/vmssextensions"

	"github.com/pkg/errors"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/roleassignments"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/scalesets"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureMachinePoolService is the group of services called by the AzureMachinePool controller.
type azureMachinePoolService struct {
	virtualMachinesScaleSetSvc azure.Service
	skuCache                   *resourceskus.Cache
	roleAssignmentsSvc         azure.Service
	vmssExtensionSvc           azure.Service
}

var _ azure.Service = (*azureMachinePoolService)(nil)

// newAzureMachinePoolService populates all the services based on input scope.
func newAzureMachinePoolService(machinePoolScope *scope.MachinePoolScope) (*azureMachinePoolService, error) {
	cache, err := resourceskus.GetCache(machinePoolScope, machinePoolScope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a NewCache")
	}

	return &azureMachinePoolService{
		virtualMachinesScaleSetSvc: scalesets.NewService(machinePoolScope, cache),
		skuCache:                   cache,
		roleAssignmentsSvc:         roleassignments.New(machinePoolScope),
		vmssExtensionSvc:           vmssextensions.New(machinePoolScope),
	}, nil
}

// Reconcile reconciles all the services in pre determined order.
func (s *azureMachinePoolService) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureMachinePoolService.Reconcile")
	defer span.End()

	if err := s.virtualMachinesScaleSetSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to create scale set")
	}

	if err := s.roleAssignmentsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "unable to create role assignment")
	}

	if err := s.vmssExtensionSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "unable to create vmss extension")
	}

	return nil
}

// Delete reconciles all the services in pre determined order.
func (s *azureMachinePoolService) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureMachinePoolService.Delete")
	defer span.End()

	if err := s.virtualMachinesScaleSetSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete scale set")
	}
	return nil
}
