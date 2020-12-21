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

package routetables

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk
type client interface {
	Get(context.Context, string, string) (network.RouteTable, error)
	CreateOrUpdate(context.Context, string, string, network.RouteTable) error
	Delete(context.Context, string, string) error
}

// azureClient contains the Azure go-sdk Client
type azureClient struct {
	routetables network.RouteTablesClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newRouteTablesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newRouteTablesClient creates a new route tables client from subscription ID.
func newRouteTablesClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.RouteTablesClient {
	routeTablesClient := network.NewRouteTablesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&routeTablesClient.Client, authorizer)
	return routeTablesClient
}

// Get gets the specified route table.
func (ac *azureClient) Get(ctx context.Context, resourceGroupName, rtName string) (network.RouteTable, error) {
	ctx, span := tele.Tracer().Start(ctx, "routetables.AzureClient.Get")
	defer span.End()

	return ac.routetables.Get(ctx, resourceGroupName, rtName, "")
}

// CreateOrUpdate create or updates a route table in a specified resource group.
func (ac *azureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, rtName string, rt network.RouteTable) error {
	ctx, span := tele.Tracer().Start(ctx, "routetables.AzureClient.CreateOrUpdate")
	defer span.End()

	future, err := ac.routetables.CreateOrUpdate(ctx, resourceGroupName, rtName, rt)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.routetables.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.routetables)
	return err
}

// Delete deletes the specified route table.
func (ac *azureClient) Delete(ctx context.Context, resourceGroupName, rtName string) error {
	ctx, span := tele.Tracer().Start(ctx, "routetables.AzureClient.Delete")
	defer span.End()

	future, err := ac.routetables.Delete(ctx, resourceGroupName, rtName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.routetables.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.routetables)
	return err
}
