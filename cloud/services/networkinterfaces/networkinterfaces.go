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
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Reconcile gets/creates/updates a network interface.
func (s *Service) Reconcile(ctx context.Context) error {
	for _, nicSpec := range s.Scope.NICSpecs() {

		nicConfig := &network.InterfaceIPConfigurationPropertiesFormat{}

		subnet, err := s.SubnetsClient.Get(ctx, nicSpec.VNetResourceGroup, nicSpec.VNetName, nicSpec.SubnetName)
		if err != nil {
			return errors.Wrap(err, "failed to get subnets")
		}
		nicConfig.Subnet = &network.Subnet{ID: subnet.ID}

		nicConfig.PrivateIPAllocationMethod = network.Dynamic
		if nicSpec.StaticIPAddress != "" {
			nicConfig.PrivateIPAllocationMethod = network.Static
			nicConfig.PrivateIPAddress = to.StringPtr(nicSpec.StaticIPAddress)
		}

		backendAddressPools := []network.BackendAddressPool{}
		if nicSpec.PublicLoadBalancerName != "" {
			lb, lberr := s.LoadBalancersClient.Get(ctx, s.Scope.ResourceGroup(), nicSpec.PublicLoadBalancerName)
			if lberr != nil {
				return errors.Wrap(lberr, "failed to get public LB")
			}
			backendAddressPools = append(backendAddressPools,
				network.BackendAddressPool{
					ID: (*lb.BackendAddressPools)[0].ID,
				})

			if nicSpec.MachineRole == infrav1.ControlPlane {
				nicConfig.LoadBalancerInboundNatRules = &[]network.InboundNatRule{
					{
						ID: to.StringPtr(fmt.Sprintf("%s/inboundNatRules/%s", to.String(lb.ID), nicSpec.MachineName)),
					},
				}
			}
		}
		if nicSpec.InternalLoadBalancerName != "" {
			// only control planes have an attached internal LB
			internalLB, ilberr := s.LoadBalancersClient.Get(ctx, s.Scope.ResourceGroup(), nicSpec.InternalLoadBalancerName)
			if ilberr != nil {
				return errors.Wrap(ilberr, "failed to get internalLB")
			}

			backendAddressPools = append(backendAddressPools,
				network.BackendAddressPool{
					ID: (*internalLB.BackendAddressPools)[0].ID,
				})
		}
		nicConfig.LoadBalancerBackendAddressPools = &backendAddressPools

		if nicSpec.PublicIPName != "" {
			publicIP, err := s.PublicIPsClient.Get(ctx, s.Scope.ResourceGroup(), nicSpec.PublicIPName)
			if err != nil {
				return errors.Wrap(err, "failed to get publicIP")
			}
			nicConfig.PublicIPAddress = &publicIP
		}

		if nicSpec.AcceleratedNetworking == nil {
			// set accelerated networking to the capability of the VMSize
			sku := nicSpec.VMSize
			accelNet, err := s.ResourceSkusClient.HasAcceleratedNetworking(ctx, sku)
			if err != nil {
				return errors.Wrap(err, "failed to get accelerated networking capability")
			}
			nicSpec.AcceleratedNetworking = to.BoolPtr(accelNet)
		}

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
					EnableAcceleratedNetworking: nicSpec.AcceleratedNetworking,
				},
			})

		if err != nil {
			return errors.Wrapf(err, "failed to create network interface %s in resource group %s", nicSpec.Name, s.Scope.ResourceGroup())
		}
		s.Scope.V(2).Info("successfully created network interface", "network interface", nicSpec.Name)
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
