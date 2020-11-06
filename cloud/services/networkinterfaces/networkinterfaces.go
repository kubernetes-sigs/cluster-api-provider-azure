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

package networkinterfaces

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// NICScope defines the scope interface for a network interfaces service.
type NICScope interface {
	logr.Logger
	azure.ClusterDescriber
	NICSpecs() []azure.NICSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope NICScope
	Client
	resourceSKUCache *resourceskus.Cache
}

// New creates a new service.
func New(scope NICScope, skuCache *resourceskus.Cache) *Service {
	return &Service{
		Scope:            scope,
		Client:           NewClient(scope),
		resourceSKUCache: skuCache,
	}
}

// Reconcile gets/creates/updates a network interface.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "networkinterfaces.Service.Reconcile")
	defer span.End()

	for _, nicSpec := range s.Scope.NICSpecs() {

		_, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), nicSpec.Name)
		switch {
		case err != nil && !azure.ResourceNotFound(err):
			return errors.Wrapf(err, "failed to fetch network interface %s", nicSpec.Name)
		case err == nil:
			// network interface already exists, do nothing
			continue
		default:
			nicConfig := &network.InterfaceIPConfigurationPropertiesFormat{}

			subnet := &network.Subnet{
				ID: to.StringPtr(azure.SubnetID(s.Scope.SubscriptionID(), nicSpec.VNetResourceGroup, nicSpec.VNetName, nicSpec.SubnetName)),
			}
			nicConfig.Subnet = subnet

			nicConfig.PrivateIPAllocationMethod = network.Dynamic
			if nicSpec.StaticIPAddress != "" {
				nicConfig.PrivateIPAllocationMethod = network.Static
				nicConfig.PrivateIPAddress = to.StringPtr(nicSpec.StaticIPAddress)
			}

			backendAddressPools := []network.BackendAddressPool{}
			if nicSpec.PublicLBName != "" {
				if nicSpec.PublicLBAddressPoolName != "" {
					backendAddressPools = append(backendAddressPools,
						network.BackendAddressPool{
							ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicSpec.PublicLBName, nicSpec.PublicLBAddressPoolName)),
						})
				}
				if nicSpec.PublicLBNATRuleName != "" {
					nicConfig.LoadBalancerInboundNatRules = &[]network.InboundNatRule{
						{
							ID: to.StringPtr(azure.NATRuleID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicSpec.PublicLBName, nicSpec.PublicLBNATRuleName)),
						},
					}
				}
			}
			if nicSpec.InternalLBName != "" && nicSpec.InternalLBAddressPoolName != "" {
				backendAddressPools = append(backendAddressPools,
					network.BackendAddressPool{
						ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicSpec.InternalLBName, nicSpec.InternalLBAddressPoolName)),
					})
			}
			nicConfig.LoadBalancerBackendAddressPools = &backendAddressPools

			if nicSpec.PublicIPName != "" {
				nicConfig.PublicIPAddress = &network.PublicIPAddress{
					ID: to.StringPtr(azure.PublicIPID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicSpec.PublicIPName)),
				}
			}

			if nicSpec.AcceleratedNetworking == nil {
				// set accelerated networking to the capability of the VMSize
				sku, err := s.resourceSKUCache.Get(ctx, nicSpec.VMSize, resourceskus.VirtualMachines)
				if err != nil {
					return errors.Wrapf(err, "failed to get find vm sku %s in compute api", nicSpec.VMSize)
				}

				accelNet := sku.HasCapability(resourceskus.AcceleratedNetworking)
				nicSpec.AcceleratedNetworking = &accelNet
			}

			ipConfigurations := []network.InterfaceIPConfiguration{
				{
					Name:                                     to.StringPtr("pipConfig"),
					InterfaceIPConfigurationPropertiesFormat: nicConfig,
				},
			}

			if nicSpec.IPv6Enabled {
				ipv6Config := network.InterfaceIPConfiguration{
					Name: to.StringPtr("ipConfigv6"),
					InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAddressVersion: "IPv6",
						Primary:                 to.BoolPtr(false),
						Subnet:                  &network.Subnet{ID: subnet.ID},
					},
				}

				ipConfigurations = append(ipConfigurations, ipv6Config)
			}

			err = s.Client.CreateOrUpdate(ctx,
				s.Scope.ResourceGroup(),
				nicSpec.Name,
				network.Interface{
					Location: to.StringPtr(s.Scope.Location()),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: nicSpec.AcceleratedNetworking,
						IPConfigurations:            &ipConfigurations,
						EnableIPForwarding:          to.BoolPtr(nicSpec.EnableIPForwarding),
					},
				})

			if err != nil {
				return errors.Wrapf(err, "failed to create network interface %s in resource group %s", nicSpec.Name, s.Scope.ResourceGroup())
			}
			s.Scope.V(2).Info("successfully created network interface", "network interface", nicSpec.Name)
		}
	}
	return nil
}

// Delete deletes the network interface with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "networkinterfaces.Service.Delete")
	defer span.End()

	for _, nicSpec := range s.Scope.NICSpecs() {
		s.Scope.V(2).Info("deleting network interface %s", "network interface", nicSpec.Name)
		err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), nicSpec.Name)
		if err != nil && !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete network interface %s in resource group %s", nicSpec.Name, s.Scope.ResourceGroup())
		}
		s.Scope.V(2).Info("successfully deleted NIC", "network interface", nicSpec.Name)
	}
	return nil
}
