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
	for _, publicIPSpec := range s.Scope.PublicIPSpecs() {
		s.Scope.V(2).Info("creating public IP", "public-ip", publicIPSpec.Name)

		ipSpec := network.PublicIPAddress{
			Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
			Name:     to.StringPtr(publicIPSpec.Name),
			Location: to.StringPtr(s.Scope.Location()),
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				PublicIPAddressVersion:   network.IPv4,
				PublicIPAllocationMethod: network.Static,
				DNSSettings: &network.PublicIPAddressDNSSettings{
					DomainNameLabel: to.StringPtr(strings.ToLower(publicIPSpec.Name)),
					Fqdn:            to.StringPtr(publicIPSpec.DNSName),
				},
			},
		}

		if err := s.Client.CreateOrUpdate(
			ctx,
			s.Scope.ResourceGroup(),
			publicIPSpec.Name,
			ipSpec,
		); err != nil {
			return errors.Wrap(err, "cannot create public IP")
		}

		s.Scope.V(2).Info("successfully created public IP", "public ip", publicIPSpec.Name)
	}

	return nil
}

// Delete deletes the public IP with the provided scope.
func (s *Service) Delete(ctx context.Context) error {
	for _, publicIPSpec := range s.Scope.PublicIPSpecs() {
		s.Scope.V(2).Info("deleting public IP", "public ip", publicIPSpec.Name)

		err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), publicIPSpec.Name)
		if err == nil {
			s.Scope.V(2).Info("deleted public IP", "public ip", publicIPSpec.Name)
			continue
		}

		if azure.ResourceNotFound(err) {
			s.Scope.V(2).Info("tried to delete public IP which didn't exist", "public ip", publicIPSpec.Name)
			continue
		}

		return errors.Wrapf(err, "failed to delete public IP %s in resource group %s", publicIPSpec.Name, s.Scope.ResourceGroup())
	}

	return nil
}
