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
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// PublicIPScope defines the scope interface for a public IP service.
type PublicIPScope interface {
	logr.Logger
	azure.ClusterDescriber
	PublicIPSpecs() []azure.PublicIPSpec
}

// Service provides operations on Azure resources.
type Service struct {
	Scope PublicIPScope
	Client
}

// New creates a new service.
func New(scope PublicIPScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope),
	}
}

// Reconcile gets/creates/updates a public ip.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "publicips.Service.Reconcile")
	defer span.End()

	for _, ip := range s.Scope.PublicIPSpecs() {
		s.Scope.V(2).Info("creating public IP", "public ip", ip.Name)

		// only set DNS properties if there is a DNS name specified
		addressVersion := network.IPv4
		if ip.IsIPv6 {
			addressVersion = network.IPv6
		}

		// only set DNS properties if there is a DNS name specified
		var dnsSettings *network.PublicIPAddressDNSSettings
		if ip.DNSName != "" {
			dnsSettings = &network.PublicIPAddressDNSSettings{
				DomainNameLabel: to.StringPtr(strings.Split(ip.DNSName, ".")[0]),
				Fqdn:            to.StringPtr(ip.DNSName),
			}
		}

		err := s.Client.CreateOrUpdate(
			ctx,
			s.Scope.ResourceGroup(),
			ip.Name,
			network.PublicIPAddress{
				Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
					ClusterName: s.Scope.ClusterName(),
					Lifecycle:   infrav1.ResourceLifecycleOwned,
					Name:        to.StringPtr(ip.Name),
					Additional:  s.Scope.AdditionalTags(),
				})),
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
	ctx, span := tele.Tracer().Start(ctx, "publicips.Service.Delete")
	defer span.End()

	for _, ip := range s.Scope.PublicIPSpecs() {
		managed, err := s.isIPManaged(ctx, ip.Name)
		if err != nil && !azure.ResourceNotFound(err) {
			return errors.Wrap(err, "could not get public IP management state")
		}

		if !managed {
			s.Scope.V(2).Info("Skipping IP deletion for unmanaged public IP", "public ip", ip.Name)
			continue
		}

		s.Scope.V(2).Info("deleting public IP", "public ip", ip.Name)
		err = s.Client.Delete(ctx, s.Scope.ResourceGroup(), ip.Name)
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

// isIPManaged returns true if the IP has an owned tag with the cluster name as value,
// meaning that the IP's lifecycle is managed.
func (s *Service) isIPManaged(ctx context.Context, ipName string) (bool, error) {
	ip, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), ipName)
	if err != nil {
		return false, err
	}
	tags := converters.MapToTags(ip.Tags)
	return tags.HasOwned(s.Scope.ClusterName()), nil
}
