/*
Copyright 2020 The Kubernetes Authors.

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

package bastionhosts

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk
type client interface {
	Get(context.Context, string, string) (network.BastionHost, error)
	CreateOrUpdate(context.Context, string, string, network.BastionHost) error
	Delete(context.Context, string, string) error
}

// azureClient contains the Azure go-sdk Client
type azureClient struct {
	interfaces network.BastionHostsClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newBastionHostsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newBastionHostsClient creates a new bastion host client from subscription ID.
func newBastionHostsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.BastionHostsClient {
	bastionClient := network.NewBastionHostsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&bastionClient.Client, authorizer)
	return bastionClient
}

// Get gets information about the specified bastion host.
func (ac *azureClient) Get(ctx context.Context, resourceGroupName, bastionName string) (network.BastionHost, error) {
	ctx, span := tele.Tracer().Start(ctx, "bastionhosts.AzureClient.Get")
	defer span.End()

	return ac.interfaces.Get(ctx, resourceGroupName, bastionName)
}

// CreateOrUpdate creates or updates a bastion host.
func (ac *azureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, bastionName string, bastionHost network.BastionHost) error {
	ctx, span := tele.Tracer().Start(ctx, "bastionhosts.AzureClient.CreateOrUpdate")
	defer span.End()

	future, err := ac.interfaces.CreateOrUpdate(ctx, resourceGroupName, bastionName, bastionHost)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.interfaces.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.interfaces)
	return err
}

// Delete deletes the specified network interface.
func (ac *azureClient) Delete(ctx context.Context, resourceGroupName, bastionName string) error {
	ctx, span := tele.Tracer().Start(ctx, "bastionhosts.AzureClient.Delete")
	defer span.End()

	future, err := ac.interfaces.Delete(ctx, resourceGroupName, bastionName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.interfaces.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.interfaces)
	return err
}
