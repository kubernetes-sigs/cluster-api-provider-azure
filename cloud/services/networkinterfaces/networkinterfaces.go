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
	"github.com/pkg/errors"
	"k8s.io/klog"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Spec specification for routetable
type Spec struct {
	Name                     string
	SubnetName               string
	VnetName                 string
	StaticIPAddress          string
	PublicLoadBalancerName   string
	InternalLoadBalancerName string
	PublicIPName             string
	NatRule                  int
}

// Get provides information about a network interface.
func (s *Service) Get(ctx context.Context, spec interface{}) (interface{}, error) {
	nicSpec, ok := spec.(*Spec)
	if !ok {
		return network.Interface{}, errors.New("invalid network interface specification")
	}
	nic, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), nicSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "network interface %s not found", nicSpec.Name)
	}

	return nic, err
}

// Reconcile gets/creates/updates a network interface.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	nicSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid network interface specification")
	}

	nicConfig := &network.InterfaceIPConfigurationPropertiesFormat{}

	subnet, err := s.SubnetsClient.Get(ctx, s.Scope.Vnet().ResourceGroup, nicSpec.VnetName, nicSpec.SubnetName)
	if err != nil {
		return errors.Wrap(err, "failed to get subNets")
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
			return errors.Wrap(lberr, "failed to get publicLB")
		}

		backendAddressPools = append(backendAddressPools,
			network.BackendAddressPool{
				ID: (*lb.BackendAddressPools)[0].ID,
			})

		nicConfig.LoadBalancerInboundNatRules = &[]network.InboundNatRule{
			{
				ID: (*lb.InboundNatRules)[nicSpec.NatRule].ID,
			},
		}
	}
	if nicSpec.InternalLoadBalancerName != "" {
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
			},
		})

	if err != nil {
		return errors.Wrapf(err, "failed to create network interface %s in resource group %s", nicSpec.Name, s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("successfully created network interface %s", nicSpec.Name)
	return nil
}

// Delete deletes the network interface with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	nicSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid network interface specification")
	}
	klog.V(2).Infof("deleting nic %s", nicSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), nicSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete network interface %s in resource group %s", nicSpec.Name, s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("successfully deleted nic %s", nicSpec.Name)
	return nil
}
