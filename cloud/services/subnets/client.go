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

package subnets

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string, string, string) (network.Subnet, error)
	CreateOrUpdate(context.Context, string, string, string, network.Subnet) error
	Delete(context.Context, string, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	subnets network.SubnetsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new subnets client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newSubnetsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newSubnetsClient creates a new subnets client from subscription ID.
func newSubnetsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.SubnetsClient {
	subnetsClient := network.NewSubnetsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&subnetsClient.Client, authorizer)
	return subnetsClient
}

// Get gets the specified subnet by virtual network and resource group.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, vnetName, snName string) (network.Subnet, error) {
	ctx, span := tele.Tracer().Start(ctx, "subnets.AzureClient.Get")
	defer span.End()

	return ac.subnets.Get(ctx, resourceGroupName, vnetName, snName, "")
}

// CreateOrUpdate creates or updates a subnet in the specified virtual network.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, vnetName, snName string, sn network.Subnet) error {
	ctx, span := tele.Tracer().Start(ctx, "subnets.AzureClient.CreateOrUpdate")
	defer span.End()

	future, err := ac.subnets.CreateOrUpdate(ctx, resourceGroupName, vnetName, snName, sn)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.subnets.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.subnets)
	return err
}

// Delete deletes the specified subnet.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, vnetName, snName string) error {
	ctx, span := tele.Tracer().Start(ctx, "subnets.AzureClient.Delete")
	defer span.End()

	future, err := ac.subnets.Delete(ctx, resourceGroupName, vnetName, snName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.subnets.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.subnets)
	return err
}
