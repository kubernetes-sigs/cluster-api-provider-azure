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
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk
type client interface {
	CreateOrUpdateZone(context.Context, string, string, privatedns.PrivateZone) error
	DeleteZone(context.Context, string, string) error
	CreateOrUpdateLink(context.Context, string, string, string, privatedns.VirtualNetworkLink) error
	DeleteLink(context.Context, string, string, string) error
	CreateOrUpdateRecordSet(context.Context, string, string, privatedns.RecordType, string, privatedns.RecordSet) error
	DeleteRecordSet(context.Context, string, string, privatedns.RecordType, string) error
}

// AzureClient contains the Azure go-sdk Client
type azureClient struct {
	privatezones privatedns.PrivateZonesClient
	vnetlinks    privatedns.VirtualNetworkLinksClient
	recordsets   privatedns.RecordSetsClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newPrivateZonesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	v := newVirtualNetworkLinksClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	r := newRecordSetsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c, v, r}
}

// newPrivateZonesClient creates a new private zones client from subscription ID.
func newPrivateZonesClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) privatedns.PrivateZonesClient {
	zonesClient := privatedns.NewPrivateZonesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&zonesClient.Client, authorizer)
	return zonesClient
}

// newVirtualNetworkLinksClient creates a new virtual networks link client from subscription ID.
func newVirtualNetworkLinksClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) privatedns.VirtualNetworkLinksClient {
	linksClient := privatedns.NewVirtualNetworkLinksClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&linksClient.Client, authorizer)
	return linksClient
}

// newRecordSetsClient creates a new record sets client from subscription ID.
func newRecordSetsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) privatedns.RecordSetsClient {
	recordsClient := privatedns.NewRecordSetsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&recordsClient.Client, authorizer)
	return recordsClient
}

// CreateOrUpdateZone creates or updates a private zone.
func (ac *azureClient) CreateOrUpdateZone(ctx context.Context, resourceGroupName string, zoneName string, zone privatedns.PrivateZone) error {
	ctx, span := tele.Tracer().Start(ctx, "privatedns.AzureClient.CreateOrUpdateZone")
	defer span.End()

	future, err := ac.privatezones.CreateOrUpdate(ctx, resourceGroupName, zoneName, zone, "", "")
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.privatezones.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.privatezones)
	return err
}

// DeleteZone deletes the private zone.
func (ac *azureClient) DeleteZone(ctx context.Context, resourceGroupName, name string) error {
	ctx, span := tele.Tracer().Start(ctx, "privatedns.AzureClient.DeleteZone")
	defer span.End()

	future, err := ac.privatezones.Delete(ctx, resourceGroupName, name, "")
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.privatezones.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.privatezones)
	return err
}

// CreateOrUpdateLink creates or updates a virtual network link to the specified Private DNS zone.
func (ac *azureClient) CreateOrUpdateLink(ctx context.Context, resourceGroupName, privateZoneName, name string, link privatedns.VirtualNetworkLink) error {
	ctx, span := tele.Tracer().Start(ctx, "privatedns.AzureClient.CreateOrUpdateLink")
	defer span.End()

	future, err := ac.vnetlinks.CreateOrUpdate(ctx, resourceGroupName, privateZoneName, name, link, "", "")
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.vnetlinks.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.vnetlinks)
	return err
}

// DeleteLink deletes a virtual network link to the specified Private DNS zone.
func (ac *azureClient) DeleteLink(ctx context.Context, resourceGroupName, privateZoneName, name string) error {
	ctx, span := tele.Tracer().Start(ctx, "privatedns.AzureClient.DeleteLink")
	defer span.End()

	future, err := ac.vnetlinks.Delete(ctx, resourceGroupName, privateZoneName, name, "")
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.vnetlinks.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.vnetlinks)
	return err
}

// CreateOrUpdateRecordSet creates or updates a record set within the specified Private DNS zone.
func (ac *azureClient) CreateOrUpdateRecordSet(ctx context.Context, resourceGroupName string, privateZoneName string, recordType privatedns.RecordType, name string, set privatedns.RecordSet) error {
	ctx, span := tele.Tracer().Start(ctx, "privatedns.AzureClient.CreateOrUpdateRecordSet")
	defer span.End()

	_, err := ac.recordsets.CreateOrUpdate(ctx, resourceGroupName, privateZoneName, recordType, name, set, "", "")
	return err
}

// DeleteRecordSet deletes a record set within the specified Private DNS zone.
func (ac *azureClient) DeleteRecordSet(ctx context.Context, resourceGroupName string, privateZoneName string, recordType privatedns.RecordType, name string) error {
	ctx, span := tele.Tracer().Start(ctx, "privatedns.AzureClient.DeleteRecordSet")
	defer span.End()

	_, err := ac.recordsets.Delete(ctx, resourceGroupName, privateZoneName, recordType, name, "")
	return err
}
