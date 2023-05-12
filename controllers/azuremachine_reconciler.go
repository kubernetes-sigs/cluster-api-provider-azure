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
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/availabilitysets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/disks"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/inboundnatrules"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/roleassignments"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/tags"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachines"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vmextensions"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureMachineService is the group of services called by the AzureMachine controller.
type azureMachineService struct {
	scope *scope.MachineScope
	// services is the list of services to be reconciled.
	// The order of the services is important as it determines the order in which the services are reconciled.
	services  []azure.ServiceReconciler
	skuCache  *resourceskus.Cache
	Reconcile func(context.Context) error
	Delete    func(context.Context) error
}

// newAzureMachineService populates all the services based on input scope.
func newAzureMachineService(machineScope *scope.MachineScope) (*azureMachineService, error) {
	cache, err := resourceskus.GetCache(machineScope, machineScope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}
	vmextensionsSvc, err := vmextensions.New(machineScope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vmextensions service")
	}
	ams := &azureMachineService{
		scope: machineScope,
		services: []azure.ServiceReconciler{
			publicips.New(machineScope),
			inboundnatrules.New(machineScope),
			networkinterfaces.New(machineScope, cache),
			availabilitysets.New(machineScope, cache),
			disks.New(machineScope),
			virtualmachines.New(machineScope),
			roleassignments.New(machineScope),
			vmextensionsSvc,
			tags.New(machineScope),
		},
		skuCache: cache,
	}
	ams.Reconcile = ams.reconcile
	ams.Delete = ams.delete

	return ams, nil
}

// reconcile reconciles all the services in a predetermined order.
func (s *azureMachineService) reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.azureMachineService.Reconcile")
	defer done()

	// Ensure that the deprecated networking field values have been migrated to the new NetworkInterfaces field.
	s.scope.AzureMachine.Spec.SetNetworkInterfacesDefaults()

	if err := s.scope.SetSubnetName(); err != nil {
		return errors.Wrap(err, "failed defaulting subnet name")
	}

	for _, service := range s.services {
		if err := service.Reconcile(ctx); err != nil {
			return errors.Wrapf(err, "failed to reconcile AzureMachine service %s", service.Name())
		}
	}

	return nil
}

// delete deletes all the services in a predetermined order.
func (s *azureMachineService) delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.azureMachineService.Delete")
	defer done()

	// Delete services in reverse order of creation.
	for i := len(s.services) - 1; i >= 0; i-- {
		if err := s.services[i].Delete(ctx); err != nil {
			return errors.Wrapf(err, "failed to delete AzureMachine service %s", s.services[i].Name())
		}
	}

	return nil
}
