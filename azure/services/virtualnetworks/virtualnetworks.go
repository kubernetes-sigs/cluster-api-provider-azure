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

package virtualnetworks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "virtualnetworks"

// VNetScope defines the scope interface for a virtual network service.
type VNetScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	Vnet() *infrav1.VnetSpec
	VNetSpec() azure.ResourceSpecGetter
	ClusterName() string
	IsVnetManaged() bool
	UpdateSubnetCIDRs(string, []string)
}

// Service provides operations on Azure resources.
type Service struct {
	Scope VNetScope
	async.Reconciler
	async.Getter
}

// New creates a new service.
func New(scope VNetScope) *Service {
	client := newClient(scope)
	return &Service{
		Scope:      scope,
		Getter:     client,
		Reconciler: async.New(scope, client, client),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	vnetSpec := s.Scope.VNetSpec()
	if vnetSpec == nil {
		return nil
	}

	result, err := s.CreateResource(ctx, vnetSpec, serviceName)
	if err == nil && result != nil {
		existingVnet, ok := result.(network.VirtualNetwork)
		if !ok {
			return errors.Errorf("%T is not a network.VirtualNetwork", result)
		}
		vnet := s.Scope.Vnet()
		vnet.ID = to.String(existingVnet.ID)
		vnet.Tags = converters.MapToTags(existingVnet.Tags)

		var prefixes []string
		if existingVnet.VirtualNetworkPropertiesFormat != nil && existingVnet.VirtualNetworkPropertiesFormat.AddressSpace != nil {
			prefixes = to.StringSlice(existingVnet.VirtualNetworkPropertiesFormat.AddressSpace.AddressPrefixes)
		}
		vnet.CIDRBlocks = prefixes

		// Update the subnet CIDRs if they already exist.
		// This makes sure the subnet CIDRs are up to date and there are no validation errors when updating the VNet.
		// Subnets that are not part of this cluster spec are silently ignored.
		if existingVnet.Subnets != nil {
			for _, subnet := range *existingVnet.Subnets {
				s.Scope.UpdateSubnetCIDRs(to.String(subnet.Name), converters.GetSubnetAddresses(subnet))
			}
		}
	}

	if s.Scope.IsVnetManaged() {
		s.Scope.UpdatePutStatus(infrav1.VNetReadyCondition, serviceName, err)
	}

	return err
}

// Delete deletes the virtual network if it is managed by capz.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	vnetSpec := s.Scope.VNetSpec()
	if vnetSpec == nil {
		return nil
	}

	// Check that the vnet is not BYO.
	managed, err := s.IsManaged(ctx)
	if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted or doesn't exist, cleanup status and return.
			s.Scope.DeleteLongRunningOperationState(vnetSpec.ResourceName(), serviceName)
			s.Scope.UpdateDeleteStatus(infrav1.VNetReadyCondition, serviceName, nil)
			return nil
		}
		return errors.Wrap(err, "could not get VNet management state")
	}
	if !managed {
		log.Info("Skipping VNet deletion in custom vnet mode")
		return nil
	}

	err = s.DeleteResource(ctx, vnetSpec, serviceName)
	s.Scope.UpdateDeleteStatus(infrav1.VNetReadyCondition, serviceName, err)
	return err
}

// IsManaged returns true if the virtual network has an owned tag with the cluster name as value,
// meaning that the vnet's lifecycle is managed.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.Service.IsManaged")
	defer done()

	spec := s.Scope.VNetSpec()
	if spec == nil {
		return false, errors.New("cannot get vnet to check if it is managed: spec is nil")
	}

	vnetIface, err := s.Get(ctx, spec)
	if err != nil {
		return false, err
	}
	vnet, ok := vnetIface.(network.VirtualNetwork)
	if !ok {
		return false, errors.Errorf("%T is not a network.VirtualNetwork", vnetIface)
	}
	tags := converters.MapToTags(vnet.Tags)
	return tags.HasOwned(s.Scope.ClusterName()), nil
}
