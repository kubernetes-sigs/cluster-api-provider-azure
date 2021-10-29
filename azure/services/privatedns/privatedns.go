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

package privatedns

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Scope defines the scope interface for a private dns service.
type Scope interface {
	azure.ClusterDescriber
	PrivateDNSSpec() *azure.PrivateDNSSpec
}

// Service provides operations on Azure resources.
type Service struct {
	Scope Scope
	client
}

// New creates a new private dns service.
func New(scope Scope) *Service {
	return &Service{
		Scope:  scope,
		client: newClient(scope),
	}
}

// Reconcile creates or updates the private zone, links it to the vnet, and creates DNS records.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "privatedns.Service.Reconcile")
	defer done()

	zoneSpec := s.Scope.PrivateDNSSpec()
	if zoneSpec != nil {
		// Create the private DNS zone.
		log.V(2).Info("creating private DNS zone", "private dns zone", zoneSpec.ZoneName)
		err := s.client.CreateOrUpdateZone(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName, privatedns.PrivateZone{Location: to.StringPtr(azure.Global)})
		if err != nil {
			return errors.Wrapf(err, "failed to create private DNS zone %s", zoneSpec.ZoneName)
		}
		log.V(2).Info("successfully created private DNS zone", "private dns zone", zoneSpec.ZoneName)

		for _, linkSpec := range zoneSpec.Links {
			// Link each virtual network.
			log.V(2).Info("creating a virtual network link", "virtual network", linkSpec.VNetName, "private dns zone", zoneSpec.ZoneName)
			link := privatedns.VirtualNetworkLink{
				VirtualNetworkLinkProperties: &privatedns.VirtualNetworkLinkProperties{
					VirtualNetwork: &privatedns.SubResource{
						ID: to.StringPtr(azure.VNetID(s.Scope.SubscriptionID(), linkSpec.VNetResourceGroup, linkSpec.VNetName)),
					},
					RegistrationEnabled: to.BoolPtr(false),
				},
				Location: to.StringPtr(azure.Global),
			}
			err = s.client.CreateOrUpdateLink(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName, linkSpec.LinkName, link)
			if err != nil {
				return errors.Wrapf(err, "failed to create virtual network link %s", linkSpec.LinkName)
			}
			log.V(2).Info("successfully created virtual network link", "virtual network", linkSpec.VNetName, "private dns zone", zoneSpec.ZoneName)
		}

		// Create the record(s).
		for _, record := range zoneSpec.Records {
			log.V(2).Info("creating record set", "private dns zone", zoneSpec.ZoneName, "record", record.Hostname)
			set := privatedns.RecordSet{
				RecordSetProperties: &privatedns.RecordSetProperties{
					TTL: to.Int64Ptr(300),
				},
			}
			recordType := converters.GetRecordType(record.IP)
			if recordType == privatedns.A {
				set.RecordSetProperties.ARecords = &[]privatedns.ARecord{{
					Ipv4Address: &record.IP,
				}}
			} else if recordType == privatedns.AAAA {
				set.RecordSetProperties.AaaaRecords = &[]privatedns.AaaaRecord{{
					Ipv6Address: &record.IP,
				}}
			}
			err := s.client.CreateOrUpdateRecordSet(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName, recordType, record.Hostname, set)
			if err != nil {
				return errors.Wrapf(err, "failed to create record %s in private DNS zone %s", record.Hostname, zoneSpec.ZoneName)
			}
			log.V(2).Info("successfully created record set", "private dns zone", zoneSpec.ZoneName, "record", record.Hostname)
		}
	}
	return nil
}

// Delete deletes the private zone.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "privatedns.Service.Delete")
	defer done()

	zoneSpec := s.Scope.PrivateDNSSpec()
	if zoneSpec != nil {
		for _, linkSpec := range zoneSpec.Links {
			// Remove each virtual network link.
			log.V(2).Info("removing virtual network link", "virtual network", linkSpec.VNetName, "private dns zone", zoneSpec.ZoneName)
			err := s.client.DeleteLink(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName, linkSpec.LinkName)
			if err != nil && !azure.ResourceNotFound(err) {
				return errors.Wrapf(err, "failed to delete virtual network link %s with zone %s in resource group %s", linkSpec.VNetName, zoneSpec.ZoneName, s.Scope.ResourceGroup())
			}
		}

		// Delete the private DNS zone, which also deletes all records.
		log.V(2).Info("deleting private dns zone", "private dns zone", zoneSpec.ZoneName)
		err := s.client.DeleteZone(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		if err != nil && !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete private dns zone %s in resource group %s", zoneSpec.ZoneName, s.Scope.ResourceGroup())
		}
		log.V(2).Info("successfully deleted private dns zone", "private dns zone", zoneSpec.ZoneName)
	}
	return nil
}
