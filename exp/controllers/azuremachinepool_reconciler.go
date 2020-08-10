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
	"github.com/pkg/errors"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/scalesets"
)

// azureMachinePoolService is the group of services called by the AzureMachinePool controller.
type azureMachinePoolService struct {
	virtualMachinesScaleSetSvc azure.Service
	skuCache                   *resourceskus.Cache
}

// newAzureMachinePoolService populates all the services based on input scope.
func newAzureMachinePoolService(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) *azureMachinePoolService {
	cache := resourceskus.NewCache(clusterScope, clusterScope.Location())
	return &azureMachinePoolService{
		virtualMachinesScaleSetSvc: scalesets.NewService(machinePoolScope, cache),
		skuCache:                   cache,
	}
}

// Reconcile reconciles all the services in pre determined order.
func (s *azureMachinePoolService) Reconcile(ctx context.Context) error {
	if err := s.virtualMachinesScaleSetSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to create scale set")
	}
	return nil
}

// Delete reconciles all the services in pre determined order.
func (s *azureMachinePoolService) Delete(ctx context.Context) error {
	if err := s.virtualMachinesScaleSetSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete scale set")
	}
	return nil
}
