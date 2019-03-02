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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// CreateOrUpdatePublicIPAddress retrieves the Public IP address resource.
func (s *Service) CreateOrUpdatePublicIPAddress(resourceGroup, IPName, zone string) (pip network.PublicIPAddress, err error) {
	publicIP := network.PublicIPAddress{
		Name:                            to.StringPtr(IPName),
		Location:                        to.StringPtr(s.scope.Location()),
		Sku:                             s.getDefaultPublicIPSKU(),
		PublicIPAddressPropertiesFormat: s.getDefaultPublicIPProperties(IPName),
	}

	// TODO: Need logic for choosing public IP zone
	if zone != "" {
		pipZones := []string{zone}
		publicIP.Zones = &pipZones
	}

	future, err := s.scope.PublicIPAddresses.CreateOrUpdate(
		s.scope.Context,
		resourceGroup,
		IPName,
		publicIP,
	)

	if err != nil {
		return pip, err
	}

	err = future.WaitForCompletionRef(s.scope.Context, s.scope.PublicIPAddresses.Client)
	if err != nil {
		return pip, fmt.Errorf("cannot get public ip address create or update future response: %v", err)
	}

	return future.Result(s.scope.PublicIPAddresses)
}

// DeletePublicIPAddress deletes the Public IP address resource.
func (s *Service) DeletePublicIPAddress(resourceGroup string, IPName string) (err error) {
	future, err := s.scope.PublicIPAddresses.Delete(s.scope.Context, resourceGroup, IPName)

	if err != nil {
		return err
	}

	err = future.WaitForCompletionRef(s.scope.Context, s.scope.PublicIPAddresses.Client)
	if err != nil {
		return fmt.Errorf("cannot get public IP delete future response: %v", err)
	}

	klog.V(2).Info("Successfully deleted public IP")
	return nil
}

// GetPublicIPName returns the public IP resource name of the machine.
func (s *Service) GetPublicIPName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s-pip", machine.Name)
}

// GetDefaultPublicIPZone returns the public IP resource name of the machine.
func (s *Service) GetDefaultPublicIPZone() string {
	return "3"
}

func (s *Service) getDefaultPublicIPSKU() *network.PublicIPAddressSku {
	return &network.PublicIPAddressSku{
		Name: network.PublicIPAddressSkuNameStandard,
	}
}

func (s *Service) getDefaultPublicIPProperties(IPName string) *network.PublicIPAddressPropertiesFormat {
	return &network.PublicIPAddressPropertiesFormat{
		PublicIPAddressVersion:   network.IPv4,
		PublicIPAllocationMethod: network.Static,
		DNSSettings: &network.PublicIPAddressDNSSettings{
			DomainNameLabel: &IPName,
		},
	}
}
