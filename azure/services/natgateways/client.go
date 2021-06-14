/*
Copyright 2021 The Kubernetes Authors.

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

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	Get(context.Context, string, string) (network.NatGateway, error)
	CreateOrUpdate(context.Context, string, string, network.NatGateway) error
	Delete(context.Context, string, string) error
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	natgateways network.NatGatewaysClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := netNatGatewaysClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// netNatGatewaysClient creates a new nat gateways client from subscription ID.
func netNatGatewaysClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.NatGatewaysClient {
	natGatewaysClient := network.NewNatGatewaysClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&natGatewaysClient.Client, authorizer)
	return natGatewaysClient
}

// Get gets the specified nat gateway.
func (ac *azureClient) Get(ctx context.Context, resourceGroupName, natGatewayName string) (network.NatGateway, error) {
	ctx, span := tele.Tracer().Start(ctx, "natgateways.AzureClient.Get")
	defer span.End()

	return ac.natgateways.Get(ctx, resourceGroupName, natGatewayName, "")
}

// CreateOrUpdate create or updates a nat gateway in a specified resource group.
func (ac *azureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, natGatewayName string, natGateway network.NatGateway) error {
	ctx, span := tele.Tracer().Start(ctx, "natgateways.AzureClient.CreateOrUpdate")
	defer span.End()

	future, err := ac.natgateways.CreateOrUpdate(ctx, resourceGroupName, natGatewayName, natGateway)
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
func (ac *azureClient) Delete(ctx context.Context, resourceGroupName, natGatewayName string) error {
	ctx, span := tele.Tracer().Start(ctx, "natgateways.AzureClient.Delete")
	defer span.End()

	future, err := ac.natgateways.Delete(ctx, resourceGroupName, natGatewayName)
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
