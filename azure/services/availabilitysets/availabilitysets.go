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

package availabilitysets

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "availabilitysets"

// AvailabilitySetScope defines the scope interface for a availability sets service.
type AvailabilitySetScope interface {
	azure.ClusterDescriber
	azure.AsyncStatusUpdater
	AvailabilitySetSpec(context.Context) azure.ResourceSpecGetter
}

// Service provides operations on Azure resources.
type Service struct {
	Scope AvailabilitySetScope
	async.Getter
	async.Reconciler
	resourceSKUCache *resourceskus.Cache
}

// New creates a new availability sets service.
func New(scope AvailabilitySetScope, skuCache *resourceskus.Cache) *Service {
	client := NewClient(scope)
	return &Service{
		Scope:            scope,
		Getter:           client,
		resourceSKUCache: skuCache,
		Reconciler:       async.New(scope, client, client),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile creates or updates availability sets.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "availabilitysets.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	var err error
	if setSpec := s.Scope.AvailabilitySetSpec(ctx); setSpec != nil {
		_, err = s.CreateResource(ctx, setSpec, serviceName)
	} else {
		log.V(2).Info("skip creation when no availability set spec is found")
		return nil
	}

	s.Scope.UpdatePutStatus(infrav1.AvailabilitySetReadyCondition, serviceName, err)
	return err
}

// Delete deletes availability sets.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "availabilitysets.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	var resultingErr error
	setSpec := s.Scope.AvailabilitySetSpec(ctx)
	if setSpec == nil {
		log.V(2).Info("skip deletion when no availability set spec is found")
		return nil
	}

	existingSet, err := s.Get(ctx, setSpec)
	if err != nil {
		if !azure.ResourceNotFound(err) {
			resultingErr = errors.Wrapf(err, "failed to get availability set %s in resource group %s", setSpec.ResourceName(), setSpec.ResourceGroupName())
		}
	} else {
		availabilitySet, ok := existingSet.(compute.AvailabilitySet)
		if !ok {
			resultingErr = errors.Errorf("%T is not a compute.AvailabilitySet", existingSet)
		} else {
			// only delete when the availability set does not have any vms
			if availabilitySet.AvailabilitySetProperties != nil && availabilitySet.VirtualMachines != nil && len(*availabilitySet.VirtualMachines) > 0 {
				log.V(2).Info("skip deleting availability set with VMs", "availability set", setSpec.ResourceName())
			} else {
				resultingErr = s.DeleteResource(ctx, setSpec, serviceName)
			}
		}
	}

	s.Scope.UpdateDeleteStatus(infrav1.AvailabilitySetReadyCondition, serviceName, resultingErr)
	return resultingErr
}

// IsManaged returns always returns true as CAPZ does not support BYO availability set.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}
