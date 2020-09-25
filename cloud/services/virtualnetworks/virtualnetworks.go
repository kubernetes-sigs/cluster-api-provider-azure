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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
)

// getExisting provides information about an existing virtual network.
func (s *Service) getExisting(ctx context.Context, spec azure.VNetSpec) (*infrav1.VnetSpec, error) {
	vnet, err := s.Client.Get(ctx, spec.ResourceGroup, spec.Name)
	if err != nil {
		if azure.ResourceNotFound(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "failed to get VNet %s", spec.Name)
	}
	var prefixes []string
	if vnet.VirtualNetworkPropertiesFormat != nil && vnet.VirtualNetworkPropertiesFormat.AddressSpace != nil {
		prefixes = to.StringSlice(vnet.VirtualNetworkPropertiesFormat.AddressSpace.AddressPrefixes)
	}
	return &infrav1.VnetSpec{
		ResourceGroup: spec.ResourceGroup,
		ID:            to.String(vnet.ID),
		Name:          to.String(vnet.Name),
		CIDRBlocks:    prefixes,
		Tags:          converters.MapToTags(vnet.Tags),
	}, nil
}

// Reconcile gets/creates/updates a virtual network.
func (s *Service) Reconcile(ctx context.Context) error {
	// Following should be created upstream and provided as an input to NewService
	// A VNet has following dependencies
	//    * VNet Cidr
	//    * Control Plane Subnet Cidr
	//    * Node Subnet Cidr
	//    * Control Plane NSG
	//    * Node NSG
	//    * Node Route Table
	for _, vnetSpec := range s.Scope.VNetSpecs() {
		existingVnet, err := s.getExisting(ctx, vnetSpec)

		switch {
		case err != nil && !azure.ResourceNotFound(err):
			return errors.Wrapf(err, "failed to get VNet %s", vnetSpec.Name)

		case err == nil:
			// vnet already exists, cannot update since it's immutable
			if !existingVnet.IsManaged(s.Scope.ClusterName()) {
				s.Scope.V(2).Info("Working on custom VNet", "vnet-id", existingVnet.ID)
			}
			existingVnet.DeepCopyInto(s.Scope.Vnet())

		default:
			s.Scope.V(2).Info("creating VNet", "VNet", vnetSpec.Name)

			vnetProperties := network.VirtualNetwork{
				Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
					ClusterName: s.Scope.ClusterName(),
					Lifecycle:   infrav1.ResourceLifecycleOwned,
					Name:        to.StringPtr(vnetSpec.Name),
					Role:        to.StringPtr(infrav1.CommonRole),
					Additional:  s.Scope.AdditionalTags(),
				})),
				Location: to.StringPtr(s.Scope.Location()),
				VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
					AddressSpace: &network.AddressSpace{
						AddressPrefixes: &vnetSpec.CIDRs,
					},
				},
			}
			err = s.Client.CreateOrUpdate(ctx, vnetSpec.ResourceGroup, vnetSpec.Name, vnetProperties)
			if err != nil {
				return errors.Wrapf(err, "failed to create virtual network %s", vnetSpec.Name)
			}
			s.Scope.V(2).Info("successfully created VNet", "VNet", vnetSpec.Name)
		}
	}

	return nil
}

// Delete deletes the virtual network with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	for _, vnetSpec := range s.Scope.VNetSpecs() {
		if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
			s.Scope.V(4).Info("Skipping VNet deletion in custom vnet mode")
			continue
		}

		s.Scope.V(2).Info("deleting VNet", "VNet", vnetSpec.Name)
		err := s.Client.Delete(ctx, vnetSpec.ResourceGroup, vnetSpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete VNet %s in resource group %s", vnetSpec.Name, vnetSpec.ResourceGroup)
		}

		s.Scope.V(2).Info("successfully deleted VNet", "VNet", vnetSpec.Name)
	}
	return nil
}
