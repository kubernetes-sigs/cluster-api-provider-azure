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
)

// CreateOrUpdatePublicIPAddress retrieves the Public IP address resource.
func (s *Service) CreateOrUpdatePublicIPAddress(resourceGroup string, IPName string) (pip network.PublicIPAddress, err error) {
	future, err := s.scope.PublicIPAddresses.CreateOrUpdate(
		s.scope.Context,
		resourceGroup,
		IPName,
		network.PublicIPAddress{
			Name:                            to.StringPtr(IPName),
			Location:                        to.StringPtr(s.scope.Location()),
			Sku:                             s.getDefaultPublicIPSKU(),
			PublicIPAddressPropertiesFormat: s.getDefaultPublicIPProperties(IPName),
		},
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
func (s *Service) DeletePublicIPAddress(resourceGroup string, IPName string) (network.PublicIPAddressesDeleteFuture, error) {
	return s.scope.PublicIPAddresses.Delete(s.scope.Context, resourceGroup, IPName)
}

// WaitForPublicIPAddressDeleteFuture waits for the DeletePublicIPAddress operation to complete.
func (s *Service) WaitForPublicIPAddressDeleteFuture(future network.PublicIPAddressesDeleteFuture) error {
	return future.Future.WaitForCompletionRef(s.scope.Context, s.scope.PublicIPAddresses.Client)
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
