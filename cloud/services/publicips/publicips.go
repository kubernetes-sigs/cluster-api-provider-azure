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

package publicips

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Reconcile gets/creates/updates a public ip.
func (s *Service) Reconcile(ctx context.Context) error {
	for _, ip := range s.Scope.PublicIPSpecs() {
		s.Scope.V(2).Info("creating public IP", "public ip", ip.Name)

		// only set DNS properties if there is a DNS name specified
		addressVersion := network.IPv4
		if ip.IsIPv6 {
			addressVersion = network.IPv6
		}

		var dnsSettings *network.PublicIPAddressDNSSettings
		if ip.DNSName != "" {
			dnsSettings = &network.PublicIPAddressDNSSettings{
				DomainNameLabel: to.StringPtr(strings.ToLower(ip.Name)),
				Fqdn:            to.StringPtr(ip.DNSName),
			}
		}

		err := s.Client.CreateOrUpdate(
			ctx,
			s.Scope.ResourceGroup(),
			ip.Name,
			network.PublicIPAddress{
				Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
				Name:     to.StringPtr(ip.Name),
				Location: to.StringPtr(s.Scope.Location()),
				PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
					PublicIPAddressVersion:   addressVersion,
					PublicIPAllocationMethod: network.Static,
					DNSSettings:              dnsSettings,
				},
			},
		)

		if err != nil {
			return errors.Wrap(err, "cannot create public IP")
		}

		s.Scope.V(2).Info("successfully created public IP", "public ip", ip.Name)
	}

	return nil
}

// Delete deletes the public IP with the provided scope.
func (s *Service) Delete(ctx context.Context) error {
	for _, ip := range s.Scope.PublicIPSpecs() {
		s.Scope.V(2).Info("deleting public IP", "public ip", ip.Name)
		err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), ip.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete public IP %s in resource group %s", ip.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("deleted public IP", "public ip", ip.Name)
	}
	return nil
}
