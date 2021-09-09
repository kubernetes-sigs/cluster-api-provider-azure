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

package vnetpeerings

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	Get(context.Context, string, string, string) (network.VirtualNetworkPeering, error)
	CreateOrUpdate(context.Context, string, string, string, network.VirtualNetworkPeering) error
	Delete(context.Context, string, string, string) error
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	peerings network.VirtualNetworkPeeringsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new virtual network peerings client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newPeeringsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newPeeringsClient creates a new virtual network peerings client from subscription ID.
func newPeeringsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.VirtualNetworkPeeringsClient {
	peeringsClient := network.NewVirtualNetworkPeeringsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&peeringsClient.Client, authorizer)
	return peeringsClient
}

// Get gets the specified virtual network peering by the peering name, virtual network, and resource group.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, vnetName, peeringName string) (network.VirtualNetworkPeering, error) {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.AzureClient.Get")
	defer span.End()

	return ac.peerings.Get(ctx, resourceGroupName, vnetName, peeringName)
}

// CreateOrUpdate creates or updates a virtual network peering in the specified virtual network.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, vnetName, peeringName string, peering network.VirtualNetworkPeering) error {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.AzureClient.CreateOrUpdate")
	defer span.End()

	future, err := ac.peerings.CreateOrUpdate(ctx, resourceGroupName, vnetName, peeringName, peering, network.SyncRemoteAddressSpaceTrue)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.peerings.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.peerings)
	return err
}

// Delete deletes the specified virtual network peering.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, vnetName, peeringName string) error {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.AzureClient.Delete")
	defer span.End()

	future, err := ac.peerings.Delete(ctx, resourceGroupName, vnetName, peeringName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.peerings.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.peerings)
	return err
}
