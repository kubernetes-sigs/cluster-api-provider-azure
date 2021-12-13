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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"

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
		// Skip the reconciliation of private DNS zone which is not managed by capz.
		isManaged, err := s.isPrivateDNSManaged(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName)
		if err != nil && !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "could not get private DNS zone state of %s in resource group %s", zoneSpec.ZoneName, s.Scope.ResourceGroup())
		}
		// If resource is not found, it means it should be created and hence setting isVnetLinkManaged to true
		// will allow the reconciliation to continue
		if err != nil && azure.ResourceNotFound(err) {
			isManaged = true
		}
		if !isManaged {
			log.V(1).Info("Skipping reconciliation of unmanaged private DNS zone", "private DNS", zoneSpec.ZoneName)
			log.V(1).Info("Tag the DNS manually from azure to manage it with capz."+
				"Please see https://capz.sigs.k8s.io/topics/custom-dns.html#manage-dns-via-capz-tool", "private DNS", zoneSpec.ZoneName)
			return nil
		}
		// Create the private DNS zone.
		log.V(2).Info("creating private DNS zone", "private dns zone", zoneSpec.ZoneName)
		pDNS := privatedns.PrivateZone{
			Location: to.StringPtr(azure.Global),
			Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
				ClusterName: s.Scope.ClusterName(),
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Additional:  s.Scope.AdditionalTags(),
			})),
		}
		err = s.client.CreateOrUpdateZone(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName, pDNS)
		if err != nil {
			return errors.Wrapf(err, "failed to create private DNS zone %s", zoneSpec.ZoneName)
		}
		log.V(2).Info("successfully created private DNS zone", "private dns zone", zoneSpec.ZoneName)
		for _, linkSpec := range zoneSpec.Links {
			// If the virtual network link is not managed by capz, skip its reconciliation
			isVnetLinkManaged, err := s.isVnetLinkManaged(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName, linkSpec.LinkName)
			if err != nil && !azure.ResourceNotFound(err) {
				return errors.Wrapf(err, "could not get vnet link state of %s in resource group %s", zoneSpec.ZoneName, s.Scope.ResourceGroup())
			}
			// If resource is not found, it means it should be created and hence setting isVnetLinkManaged to true
			// will allow the reconciliation to continue
			if err != nil && azure.ResourceNotFound(err) {
				isVnetLinkManaged = true
			}
			if !isVnetLinkManaged {
				log.V(2).Info("Skipping vnet link reconciliation for unmanaged vnet link", "vnet link", linkSpec.LinkName, "private dns zone", zoneSpec.ZoneName)
				continue
			}
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
				Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
					ClusterName: s.Scope.ClusterName(),
					Lifecycle:   infrav1.ResourceLifecycleOwned,
					Additional:  s.Scope.AdditionalTags(),
				})),
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

// Delete deletes the private zone and vnet links.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "privatedns.Service.Delete")
	defer done()

	zoneSpec := s.Scope.PrivateDNSSpec()
	if zoneSpec != nil {
		for _, linkSpec := range zoneSpec.Links {
			// If the virtual network link is not managed by capz, skip its removal
			isVnetLinkManaged, err := s.isVnetLinkManaged(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName, linkSpec.LinkName)
			if err != nil && !azure.ResourceNotFound(err) {
				return errors.Wrapf(err, "could not get vnet link state of %s in resource group %s", zoneSpec.ZoneName, s.Scope.ResourceGroup())
			}
			if !isVnetLinkManaged {
				log.V(2).Info("Skipping vnet link deletion for unmanaged vnet link", "vnet link", linkSpec.LinkName, "private dns zone", zoneSpec.ZoneName)
				continue
			}
			log.V(2).Info("removing virtual network link", "virtual network", linkSpec.VNetName, "private dns zone", zoneSpec.ZoneName)
			err = s.client.DeleteLink(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName, linkSpec.LinkName)
			if err != nil && !azure.ResourceNotFound(err) {
				return errors.Wrapf(err, "failed to delete virtual network link %s with zone %s in resource group %s", linkSpec.VNetName, zoneSpec.ZoneName, s.Scope.ResourceGroup())
			}
		}
		// Skip the deletion of private DNS zone which is not managed by capz.
		isManaged, err := s.isPrivateDNSManaged(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName)
		if err != nil && !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "could not get private DNS zone state of %s in resource group %s", zoneSpec.ZoneName, s.Scope.ResourceGroup())
		}
		if !isManaged {
			log.V(1).Info("Skipping private DNS zone deletion for unmanaged private DNS zone", "private DNS", zoneSpec.ZoneName)
			return nil
		}
		// Delete the private DNS zone, which also deletes all records.
		log.V(2).Info("deleting private dns zone", "private dns zone", zoneSpec.ZoneName)
		err = s.client.DeleteZone(ctx, s.Scope.ResourceGroup(), zoneSpec.ZoneName)
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

// isPrivateDNSManaged returns true if the private DNS has an owned tag with the cluster name as value,
// meaning that the DNS lifecycle is managed.
func (s *Service) isPrivateDNSManaged(ctx context.Context, resourceGroup, zoneName string) (bool, error) {
	zone, err := s.client.GetZone(ctx, resourceGroup, zoneName)
	if err != nil {
		return false, err
	}
	tags := converters.MapToTags(zone.Tags)
	return tags.HasOwned(s.Scope.ClusterName()), nil
}

// isVnetLinkManaged returns true if the vnet link has an owned tag with the cluster name as value,
// meaning that the vnet link lifecycle is managed.
func (s *Service) isVnetLinkManaged(ctx context.Context, resourceGroupName, zoneName, vnetLinkName string) (bool, error) {
	zone, err := s.client.GetLink(ctx, resourceGroupName, zoneName, vnetLinkName)
	if err != nil {
		return false, err
	}
	tags := converters.MapToTags(zone.Tags)
	return tags.HasOwned(s.Scope.ClusterName()), nil
}
