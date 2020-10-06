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

	//"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/azure-sdk-for-go/profiles/2018-03-01/network/mgmt/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Reconcile gets/creates/updates a network interface.
func (s *Service) Reconcile(ctx context.Context) error {
	log := klogr.New()
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

			//nicConfig.Subnet = &network.Subnet{ID: to.StringPtr(azure.SubnetID(s.Scope.SubscriptionID(), nicSpec.VNetResourceGroup, nicSpec.VNetName, nicSpec.SubnetName))}
			nicConfig.Subnet = &network.Subnet{ID: to.StringPtr(azure.SubnetID(s.Scope.SubscriptionID(), nicSpec.VNetResourceGroup, nicSpec.VNetName, nicSpec.SubnetName))}

			//commenting out to test api server IP, uncomment this later on. LoadBalancer should return IP address
			nicConfig.PrivateIPAllocationMethod = network.Dynamic
			if nicSpec.StaticIPAddress != "" {
				log.Info("I am using the IP address from nicSpec")
				log.Info(nicSpec.StaticIPAddress)
				nicConfig.PrivateIPAllocationMethod = network.Static
				nicConfig.PrivateIPAddress = to.StringPtr(nicSpec.StaticIPAddress)
			}

			backendAddressPools := []network.BackendAddressPool{}
			if nicSpec.PublicLBName != "" {
				log.Info(nicSpec.PublicLBName)
				if nicSpec.PublicLBAddressPoolName != "" {
					backendAddressPools = append(backendAddressPools,
						network.BackendAddressPool{
							ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicSpec.PublicLBName, nicSpec.PublicLBAddressPoolName)),
						})
				}
				log.Info("AddressPoolID using publicLBName and publicLBAddress")
				log.Info(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicSpec.PublicLBName, nicSpec.PublicLBAddressPoolName))
				if nicSpec.PublicLBNATRuleName != "" {
					log.Info(nicSpec.PublicLBNATRuleName)
					/*nicConfig.LoadBalancerInboundNatRules = &[]network.InboundNatRule{
						{
							ID: to.StringPtr(azure.NATRuleID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicSpec.PublicLBName, nicSpec.PublicLBNATRuleName)),
						},
					}*/
				}
			}
			if nicSpec.InternalLBName != "" && nicSpec.InternalLBAddressPoolName != "" {
				log.Info(nicSpec.InternalLBName)
				log.Info(nicSpec.InternalLBAddressPoolName)
				// only control planes have an attached internal LB
				backendAddressPools = append(backendAddressPools,
					network.BackendAddressPool{
						ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicSpec.InternalLBName, nicSpec.InternalLBAddressPoolName)),
					})
			}
			log.Info("AddressPoolID using InternalLBName and InternalLBAddress")
			//nicConfig.LoadBalancerBackendAddressPools = &backendAddressPools

			if nicSpec.PublicIPName != "" {
				log.Info("Inside publicIPName not equal to null")
				log.Info(nicSpec.PublicIPName)
				nicConfig.PublicIPAddress = &network.PublicIPAddress{
					ID: to.StringPtr(azure.PublicIPID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicSpec.PublicIPName)),
				}
			}

			/*if nicSpec.AcceleratedNetworking == nil {
				// set accelerated networking to the capability of the VMSize
				sku, err := s.ResourceSKUCache.Get(ctx, nicSpec.VMSize, resourceskus.VirtualMachines)
				if err != nil {
					return errors.Wrapf(err, "failed to get find vm sku %s in compute api", nicSpec.VMSize)
				}

				accelNet := sku.HasCapability(resourceskus.AcceleratedNetworking)
				nicSpec.AcceleratedNetworking = &accelNet
			}*/

			err = s.Client.CreateOrUpdate(ctx,
				s.Scope.ResourceGroup(),
				nicSpec.Name,
				network.Interface{
					Location: to.StringPtr(s.Scope.Location()),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name:                                     to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: nicConfig,
							},
						},
						//EnableAcceleratedNetworking: nicSpec.AcceleratedNetworking,
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
