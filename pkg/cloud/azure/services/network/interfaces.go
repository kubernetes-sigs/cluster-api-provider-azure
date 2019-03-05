/*
Copyright 2018 The Kubernetes Authors.

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

package network

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// GetNetworkInterface returns a network interface, if it exists.
func (s *Service) GetNetworkInterface(resourceGroup, networkInterfaceName string) (network.Interface, error) {
	klog.V(2).Infof("Attempting to find existing network interface %q", networkInterfaceName)
	nic, err := s.scope.Interfaces.Get(s.scope.Context, resourceGroup, networkInterfaceName, "")

	if err != nil {
		return nic, err
	}

	klog.V(2).Info("Successfully found network interface")
	return nic, nil
}

// CreateOrUpdateNetworkInterface creates a new network interface or updates an
// existing one, using the supplied parameters.
func (s *Service) CreateOrUpdateNetworkInterface(resourceGroup, networkInterfaceName string, params network.Interface) (nic network.Interface, err error) {
	klog.V(2).Infof("Attempting to update existing network interface %q", networkInterfaceName)
	future, err := s.scope.Interfaces.CreateOrUpdate(s.scope.Context, resourceGroup, networkInterfaceName, params)

	err = future.WaitForCompletionRef(s.scope.Context, s.scope.Interfaces.Client)
	if err != nil {
		return nic, errors.Wrapf(err, "cannot get network interface create or update future response")
	}

	klog.V(2).Info("Successfully create/update network interface")
	return future.Result(s.scope.Interfaces)
}

// DeleteNetworkInterface deletes the NIC resource.
func (s *Service) DeleteNetworkInterface(resourceGroup, networkInterfaceName string) (network.InterfacesDeleteFuture, error) {
	return s.scope.Interfaces.Delete(s.scope.Context, resourceGroup, networkInterfaceName)
}

// WaitForNetworkInterfacesDeleteFuture waits for the DeleteNetworkInterface operation to complete.
func (s *Service) WaitForNetworkInterfacesDeleteFuture(future network.InterfacesDeleteFuture) error {
	return future.Future.WaitForCompletionRef(s.scope.Context, s.scope.Interfaces.Client)
}

// CreateDefaultVMNetworkInterface
func (s *Service) CreateDefaultVMNetworkInterface(resourceGroup string, machine *clusterv1.Machine) (nic network.Interface, err error) {
	return s.CreateOrUpdateNetworkInterface(resourceGroup, s.GetNetworkInterfaceName(machine), s.getDefaultVMNetworkInterfaceConfig())
}

// ReconcileNICBackendPool attaches a backend address pool ID to the supplied NIC.
func (s *Service) ReconcileNICBackendPool(networkInterfaceName, backendPoolID string) error {
	klog.V(2).Info("Attempting to attach NIC to load balancer backend pool")
	nic, err := s.GetNetworkInterface(s.scope.ClusterConfig.ResourceGroup, networkInterfaceName)
	if err != nil {
		return errors.Wrapf(err, "Failed to get NIC %q", networkInterfaceName)
	}

	ipConfigs := (*nic.IPConfigurations)
	if len(ipConfigs) > 0 {
		ipConfig := ipConfigs[0]

		if ipConfig.LoadBalancerBackendAddressPools != nil {
			backendPool := (*ipConfig.LoadBalancerBackendAddressPools)[0]
			if *backendPool.ID != backendPoolID {
				klog.V(2).Infof("Could not attach NIC to load balancer backend pool (%q). NIC is already attached to %q.", backendPoolID, *backendPool.ID)
				return nil
			}

			klog.V(2).Infof("No action required. NIC is already attached to %q.", backendPoolID)
			return nil
		}

		backendPools := []network.BackendAddressPool{
			{
				ID: &backendPoolID,
			},
		}

		// TODO: Remove debug once reconcile logic is improved.
		klog.V(2).Info("ReconcileNICBackendPool: Never checked the state of existing IP configuration")
		klog.V(2).Info("Setting NIC backend pool")
		(*nic.InterfacePropertiesFormat.IPConfigurations)[0].InterfaceIPConfigurationPropertiesFormat.LoadBalancerBackendAddressPools = &backendPools

		_, err = s.CreateOrUpdateNetworkInterface(s.scope.ClusterConfig.ResourceGroup, networkInterfaceName, nic)
		if err != nil {
			return errors.Wrapf(err, "Failed to update NIC %q", networkInterfaceName)
		}
	}

	return nil
}

// ReconcileNICPublicIP attaches a backend address pool ID to the supplied NIC.
func (s *Service) ReconcileNICPublicIP(networkInterfaceName string, publicIP network.PublicIPAddress) error {
	klog.V(2).Info("Attempting to attach public IP to NIC")
	nic, err := s.GetNetworkInterface(s.scope.ClusterConfig.ResourceGroup, networkInterfaceName)
	if err != nil {
		return errors.Wrapf(err, "Failed to get NIC %q", networkInterfaceName)
	}

	ipConfigs := (*nic.IPConfigurations)
	if len(ipConfigs) > 0 {
		ipConfig := ipConfigs[0]
		pip := ipConfig.PublicIPAddress

		if pip != nil {
			pipID := *ipConfig.PublicIPAddress.ID
			if pipID != *publicIP.ID {
				klog.V(2).Infof("Could not associate NIC to public IP (%q). NIC is already associated with %q.", *publicIP.ID, pipID)
				return nil
			}

			klog.V(2).Infof("No action required. NIC is already attached to %q.", *publicIP.ID)
			return nil
		}

		// TODO: Remove debug once reconcile logic is improved.
		klog.V(2).Info("ReconcileNICPublicIP: Never checked the state of existing IP configuration")
		klog.V(2).Info("Setting NIC public IP")
		(*nic.InterfacePropertiesFormat.IPConfigurations)[0].InterfaceIPConfigurationPropertiesFormat.PublicIPAddress = &publicIP

		_, err = s.CreateOrUpdateNetworkInterface(s.scope.ClusterConfig.ResourceGroup, networkInterfaceName, nic)
		if err != nil {
			return errors.Wrapf(err, "Failed to update NIC %q", networkInterfaceName)
		}
	}

	return nil
}

func (s *Service) getDefaultVMNetworkInterfaceConfig() network.Interface {
	return network.Interface{
		Location: to.StringPtr(s.scope.Location()),
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			IPConfigurations: &[]network.InterfaceIPConfiguration{
				{
					Name: to.StringPtr("ipconfig1"),
					InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &network.Subnet{
							// TODO: Need a method to pull the specific role (controlplane, node) subnet ID. This only works because we're only creating one subnet currently.
							ID: to.StringPtr(s.scope.ClusterStatus.Network.Subnets[0].ID),
						},
					},
				},
			},
		},
	}
}

// GetNetworkInterfaceName returns the nic resource name of the machine.
func (s *Service) GetNetworkInterfaceName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s-nic", machine.Name)
}
