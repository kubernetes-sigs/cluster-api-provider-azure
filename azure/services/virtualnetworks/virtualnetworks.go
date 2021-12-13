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

const serviceName = "virtualnetwork"

// VNetScope defines the scope interface for a virtual network service.
type VNetScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	Vnet() *infrav1.VnetSpec
	VNetSpec() azure.ResourceSpecGetter
	ClusterName() string
	GetVnetManagedCache() *bool
	SetVnetManagedCache(bool)
}

// Service provides operations on Azure resources.
type Service struct {
	Scope VNetScope
	async.Reconciler
	Client
}

// New creates a new service.
func New(scope VNetScope) *Service {
	client := NewClient(scope)
	return &Service{
		Scope:      scope,
		Client:     client,
		Reconciler: async.New(scope, client, client),
	}
}

func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	vnetSpec := s.Scope.VNetSpec()

	result, err := s.CreateResource(ctx, vnetSpec, serviceName)
	s.Scope.UpdatePutStatus(infrav1.VNetReadyCondition, serviceName, err)
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
	}
	return err
}

// Delete deletes the virtual network if it is managed by us.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	vnetSpec := s.Scope.VNetSpec()

	// Check that the vnet is not BYO.
	managed, err := s.IsManaged(ctx, vnetSpec)
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
// meaning that the vnet's lifecycle is managed, and caches the result in scope so that other services that depend on the vnet can check if it is managed.
func (s *Service) IsManaged(ctx context.Context, spec azure.ResourceSpecGetter) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.Service.IsManaged")
	defer done()

	if s.Scope.GetVnetManagedCache() != nil {
		return *s.Scope.GetVnetManagedCache(), nil
	}

	vnetIface, err := s.Client.Get(ctx, spec)
	if err != nil {
		if azure.ResourceNotFound(err) {
			// if the vnet was already deleted, attempt to get previous management state from the spec.
			if s.Scope.Vnet().ID != "" {
				return s.Scope.Vnet().Tags.HasOwned(s.Scope.ClusterName()), nil
			}
		}
		return false, err
	}
	vnet, ok := vnetIface.(network.VirtualNetwork)
	if !ok {
		return false, errors.Errorf("%T is not a network.VirtualNetwork", vnetIface)
	}
	tags := converters.MapToTags(vnet.Tags)
	managed := tags.HasOwned(s.Scope.ClusterName())
	s.Scope.SetVnetManagedCache(managed)
	return managed, nil
}
