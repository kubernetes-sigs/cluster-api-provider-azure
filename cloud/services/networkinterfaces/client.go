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

package networkinterfaces

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string, string) (network.Interface, error)
	CreateOrUpdate(context.Context, string, string, network.Interface) error
	Delete(context.Context, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	interfaces network.InterfacesClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(settings azure.ClientSettings, authorizer autorest.Authorizer) *AzureClient {
	c := newInterfacesClient(settings, authorizer)
	return &AzureClient{c}
}

// newInterfacesClient creates a new network interfaces client from subscription ID.
func newInterfacesClient(settings azure.ClientSettings, authorizer autorest.Authorizer) network.InterfacesClient {
	nicClient := network.NewInterfacesClientWithBaseURI(settings.BaseURI, settings.SubscriptionID)
	nicClient.Authorizer = authorizer
	nicClient.AddToUserAgent(azure.UserAgent())
	return nicClient
}

// Get gets information about the specified network interface.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, nicName string) (network.Interface, error) {
	return ac.interfaces.Get(ctx, resourceGroupName, nicName, "")
}

// CreateOrUpdate creates or updates a network interface.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, nicName string, nic network.Interface) error {
	future, err := ac.interfaces.CreateOrUpdate(ctx, resourceGroupName, nicName, nic)
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
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, nicName string) error {
	future, err := ac.interfaces.Delete(ctx, resourceGroupName, nicName)
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
