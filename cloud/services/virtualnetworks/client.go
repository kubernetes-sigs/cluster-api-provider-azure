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

package virtualnetworks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string, string) (network.VirtualNetwork, error)
	CreateOrUpdate(context.Context, string, string, network.VirtualNetwork) error
	Delete(context.Context, string, string) error
	CheckIPAddressAvailability(context.Context, string, string, string) (network.IPAddressAvailabilityResult, error)
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	virtualnetworks network.VirtualNetworksClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(subscriptionID string, authorizer autorest.Authorizer) *AzureClient {
	c := newVirtualNetworksClient(subscriptionID, authorizer)
	return &AzureClient{c}
}

// newVirtualNetworksClient creates a new vnet client from subscription ID.
func newVirtualNetworksClient(subscriptionID string, authorizer autorest.Authorizer) network.VirtualNetworksClient {
	vnetsClient := network.NewVirtualNetworksClient(subscriptionID)
	vnetsClient.Authorizer = authorizer
	vnetsClient.AddToUserAgent(azure.UserAgent())
	return vnetsClient
}

// Get gets the specified virtual network by resource group.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, vnetName string) (network.VirtualNetwork, error) {
	return ac.virtualnetworks.Get(ctx, resourceGroupName, vnetName, "")
}

// CreateOrUpdate creates or updates a virtual network in the specified resource group.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, vnetName string, vn network.VirtualNetwork) error {
	future, err := ac.virtualnetworks.CreateOrUpdate(ctx, resourceGroupName, vnetName, vn)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.virtualnetworks.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.virtualnetworks)
	return err
}

// Delete deletes the specified virtual network.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, vnetName string) error {
	future, err := ac.virtualnetworks.Delete(ctx, resourceGroupName, vnetName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.virtualnetworks.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.virtualnetworks)
	return err
}

// CheckIPAddressAvailability checks whether a private IP address is available for use.
func (ac *AzureClient) CheckIPAddressAvailability(ctx context.Context, resourceGroupName, vnetName, ip string) (network.IPAddressAvailabilityResult, error) {
	return ac.virtualnetworks.CheckIPAddressAvailability(ctx, resourceGroupName, vnetName, ip)
}
