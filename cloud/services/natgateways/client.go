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

package natgateways

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string, string) (network.NatGateway, error)
	CreateOrUpdate(context.Context, string, string, network.NatGateway) error
	Delete(context.Context, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	natgateways network.NatGatewaysClient
}

var _ Client = &AzureClient{}

// NewClient creates a new Nat Gateways client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newNatGatewaysClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newNatGatewaysClient creates a new nat gateways client from subscription ID.
func newNatGatewaysClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.NatGatewaysClient {
	natGatewaysClient := network.NewNatGatewaysClientWithBaseURI(baseURI, subscriptionID)
	natGatewaysClient.Authorizer = authorizer
	natGatewaysClient.AddToUserAgent(azure.UserAgent())
	return natGatewaysClient
}

// Get gets the specified nat gateway.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, natGatewayName string) (network.NatGateway, error) {
	return ac.natgateways.Get(ctx, resourceGroupName, natGatewayName, "")
}

// CreateOrUpdate create or updates a nat gateway in a specified resource group.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, rtName string, natGateway network.NatGateway) error {
	future, err := ac.natgateways.CreateOrUpdate(ctx, resourceGroupName, rtName, natGateway)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.natgateways.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.natgateways)
	return err
}

// Delete deletes the specified nat gateway.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, natGateway string) error {
	future, err := ac.natgateways.Delete(ctx, resourceGroupName, natGateway)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.natgateways.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.natgateways)
	return err
}
