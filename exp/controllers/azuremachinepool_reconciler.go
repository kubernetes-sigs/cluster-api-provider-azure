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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/roleassignments"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureMachinePoolService is the group of services called by the AzureMachinePool controller.
type azureMachinePoolService struct {
	scope    *scope.MachinePoolScope
	skuCache *resourceskus.Cache
	services []azure.ServiceReconciler
}

// newAzureMachinePoolService populates all the services based on input scope.
func newAzureMachinePoolService(machinePoolScope *scope.MachinePoolScope) (*azureMachinePoolService, error) {
	cache, err := resourceskus.GetCache(machinePoolScope, machinePoolScope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a NewCache")
	}

	return &azureMachinePoolService{
		scope: machinePoolScope,
		services: []azure.ServiceReconciler{
			scalesets.New(machinePoolScope, cache),
			roleassignments.New(machinePoolScope),
		},
		skuCache: cache,
	}, nil
}

func (s *azureMachinePoolService) Snapshot(subscriptionID string, cred *azidentity.DefaultAzureCredential, snapshotName string, resourceGroup string, location string, ctx context.Context, osDisk *string) error {
	snapshotFactory, err := armcompute.NewSnapshotsClient(subscriptionID, cred, nil)

	if err != nil {
		return errors.Wrapf(err, "Failed to create snapshot client")
	}

	_, err = snapshotFactory.BeginCreateOrUpdate(ctx, resourceGroup, snapshotName, armcompute.Snapshot{ // step 3
		Location: to.Ptr(location),
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceURI:    osDisk,
			},
		},
	}, nil)

	if err != nil {
		return errors.Wrapf(err, "Failed to create snapshot")
	}

	return nil
}

// Reconcile reconciles all the services in pre determined order.
func (s *azureMachinePoolService) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.azureMachinePoolService.Reconcile")
	defer done()

	// Ensure that the deprecated networking field values have been migrated to the new NetworkInterfaces field.
	s.scope.AzureMachinePool.SetNetworkInterfacesDefaults()

	if err := s.scope.SetSubnetName(); err != nil {
		return errors.Wrap(err, "failed defaulting subnet name")
	}

	for _, service := range s.services {
		if err := service.Reconcile(ctx); err != nil {
			return errors.Wrapf(err, "failed to reconcile AzureMachinePool service %s", service.Name())
		}
	}

	return nil
}

// Delete reconciles all the services in pre determined order.
func (s *azureMachinePoolService) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.azureMachinePoolService.Delete")
	defer done()

	// Delete services in reverse order of creation.
	for i := len(s.services) - 1; i >= 0; i-- {
		if err := s.services[i].Delete(ctx); err != nil {
			return errors.Wrapf(err, "failed to delete AzureMachinePool service %s", s.services[i].Name())
		}
	}

	return nil
}
