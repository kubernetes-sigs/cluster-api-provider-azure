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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	Get(context.Context, string, string) (network.PublicIPAddress, error)
	CreateOrUpdate(context.Context, string, string, network.PublicIPAddress) error
	Delete(context.Context, string, string) error
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	publicips network.PublicIPAddressesClient
}

var _ Client = &AzureClient{}

// NewClient creates a new public IP client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newPublicIPAddressesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newPublicIPAddressesClient creates a new public IP client from subscription ID.
func newPublicIPAddressesClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.PublicIPAddressesClient {
	publicIPsClient := network.NewPublicIPAddressesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&publicIPsClient.Client, authorizer)
	return publicIPsClient
}

// Get gets the specified public IP address in a specified resource group.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, ipName string) (network.PublicIPAddress, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.Get")
	defer done()

	return ac.publicips.Get(ctx, resourceGroupName, ipName, "")
}

// CreateOrUpdate creates or updates a static or dynamic public IP address.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, ipName string, ip network.PublicIPAddress) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.CreateOrUpdate")
	defer done()

	future, err := ac.publicips.CreateOrUpdate(ctx, resourceGroupName, ipName, ip)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.publicips.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.publicips)
	return err
}

// Delete deletes the specified public IP address.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, ipName string) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.Delete")
	defer done()

	future, err := ac.publicips.Delete(ctx, resourceGroupName, ipName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.publicips.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.publicips)
	return err
}
