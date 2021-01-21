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
	"strconv"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AvailabilitySetScope defines the scope interface for a availability sets service.
type AvailabilitySetScope interface {
	logr.Logger
	azure.ClusterDescriber
	AvailabilitySet() (string, bool)
}

// Service provides operations on azure resources
type Service struct {
	Scope AvailabilitySetScope
	Client
	resourceSKUCache *resourceskus.Cache
}

// New creates a new availability sets service.
func New(scope AvailabilitySetScope, skuCache *resourceskus.Cache) *Service {
	return &Service{
		Scope:            scope,
		Client:           NewClient(scope),
		resourceSKUCache: skuCache,
	}
}

// Reconcile creates or updates availability sets.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "availabilitysets.Service.Reconcile")
	defer span.End()

	availabilitySetName, ok := s.Scope.AvailabilitySet()
	if !ok {
		return nil
	}

	asSku, err := s.resourceSKUCache.Get(ctx, string(compute.Aligned), resourceskus.AvailabilitySets)
	if err != nil {
		return errors.Wrapf(err, "failed to get availability sets sku")
	}

	faultDomainCountStr, ok := asSku.GetCapability(resourceskus.MaximumPlatformFaultDomainCount)
	if !ok {
		return errors.Errorf("cannot find capability %s sku %s", resourceskus.MaximumPlatformFaultDomainCount, *asSku.Name)
	}

	faultDomainCount, err := strconv.ParseUint(faultDomainCountStr, 10, 32)
	if err != nil {
		return errors.Wrapf(err, "failed to determine max fault domain count")
	}

	s.Scope.V(2).Info("creating availability set", "availability set", availabilitySetName)

	asParams := compute.AvailabilitySet{
		Sku: &compute.Sku{
			Name: to.StringPtr(string(compute.Aligned)),
		},
		AvailabilitySetProperties: &compute.AvailabilitySetProperties{
			PlatformFaultDomainCount: to.Int32Ptr(int32(faultDomainCount)),
		},
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.Scope.ClusterName(),
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        to.StringPtr(availabilitySetName),
			Role:        to.StringPtr(infrav1.CommonRole),
			Additional:  s.Scope.AdditionalTags(),
		})),
		Location: to.StringPtr(s.Scope.Location()),
	}

	_, err = s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), availabilitySetName, asParams)
	if err != nil {
		return errors.Wrapf(err, "failed to create availability set %s", availabilitySetName)
	}

	s.Scope.V(2).Info("successfully created availability set", "availability set", availabilitySetName)

	return nil
}

// Delete deletes availability sets.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "availabilitysets.Service.Delete")
	defer span.End()

	availabilitySetName, ok := s.Scope.AvailabilitySet()
	if !ok {
		return nil
	}

	as, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), availabilitySetName)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "failed to get availability set %s in resource group %s", availabilitySetName, s.Scope.ResourceGroup())
	}

	// only delete when the availability set does not have any vms
	if as.AvailabilitySetProperties != nil && as.VirtualMachines != nil && len(*as.VirtualMachines) > 0 {
		return nil
	}

	s.Scope.V(2).Info("deleting availability set", "availability set", availabilitySetName)
	err = s.Client.Delete(ctx, s.Scope.ResourceGroup(), availabilitySetName)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "failed to delete availability set %s in resource group %s", availabilitySetName, s.Scope.ResourceGroup())
	}

	s.Scope.V(2).Info("successfully delete availability set", "availability set", availabilitySetName)

	return nil
}
