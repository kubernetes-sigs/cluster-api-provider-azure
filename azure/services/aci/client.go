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

package aci

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2019-12-01/containerinstance"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-06-01/network"
	"github.com/Azure/go-autorest/autorest"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	// Client wraps go-sdk
	Client interface {
		Get(ctx context.Context, resourceGroup, name string) (containerinstance.ContainerGroup, error)
		CreateOrUpdate(ctx context.Context, resourceGroup, name string, params containerinstance.ContainerGroup) (containerinstance.ContainerGroup, error)
		CreateOrUpdateNetworkProfile(ctx context.Context, resourceGroup, name string, params network.Profile) (network.Profile, error)
		Delete(ctx context.Context, resourceGroup, name string) error
		DeleteNetworkProfile(ctx context.Context, resourceGroup, name string) error
	}

	// azureClient contains the Azure go-sdk Client
	azureClient struct {
		aciGroupsClient       containerinstance.ContainerGroupsClient
		networkProfilesClient network.ProfilesClient
	}
)

var _ Client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newContainerGroupClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	np := newNetworkProfilesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{
		aciGroupsClient:       c,
		networkProfilesClient: np,
	}
}

// newContainerGroupClient creates a new container group client from subscription ID.
func newContainerGroupClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) containerinstance.ContainerGroupsClient {
	aciGroupsClient := containerinstance.NewContainerGroupsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&aciGroupsClient.Client, authorizer)
	aciGroupsClient.RetryAttempts = 1
	return aciGroupsClient
}

// newNetworkProfilesClient creates a new network profiles client from subscription ID.
func newNetworkProfilesClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.ProfilesClient {
	npClient := network.NewProfilesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&npClient.Client, authorizer)
	npClient.RetryAttempts = 1
	return npClient
}

func (c *azureClient) CreateOrUpdateNetworkProfile(ctx context.Context, resourceGroup, name string, params network.Profile) (network.Profile, error) {
	ctx, span := tele.Tracer().Start(ctx, "aci.azureClient.CreateOrUpdateNetworkProfile")
	defer span.End()

	return c.networkProfilesClient.CreateOrUpdate(ctx, resourceGroup, name, params)
}

func (c *azureClient) DeleteNetworkProfile(ctx context.Context, resourceGroup, name string) error {
	ctx, span := tele.Tracer().Start(ctx, "aci.azureClient.DeleteNetworkProfile")
	defer span.End()

	future, err :=  c.networkProfilesClient.Delete(ctx, resourceGroup, name)
	if err != nil {
		return err
	}

	err = future.WaitForCompletionRef(ctx, c.aciGroupsClient.Client)
	if err != nil {
		return err
	}

	_, err = future.Result(c.networkProfilesClient)
	return err
}

// CreateOrUpdate a container group in Azure
func (c *azureClient) CreateOrUpdate(ctx context.Context, resourceGroup, name string, params containerinstance.ContainerGroup) (containerinstance.ContainerGroup, error) {
	ctx, span := tele.Tracer().Start(ctx, "aci.azureClient.CreateOrUpdate")
	defer span.End()

	future, err := c.aciGroupsClient.CreateOrUpdate(ctx, resourceGroup, name, params)
	if err != nil {
		return containerinstance.ContainerGroup{}, err
	}

	err = future.WaitForCompletionRef(ctx, c.aciGroupsClient.Client)
	if err != nil {
		return containerinstance.ContainerGroup{}, err
	}

	return future.Result(c.aciGroupsClient)
}

// Get a container group from Azure
func (c *azureClient) Get(ctx context.Context, resourceGroup, name string) (containerinstance.ContainerGroup, error) {
	ctx, span := tele.Tracer().Start(ctx, "aci.azureClient.Get")
	defer span.End()

	return c.aciGroupsClient.Get(ctx, resourceGroup, name)
}

// Delete a container group in Azure
func (c *azureClient) Delete(ctx context.Context, resourceGroup, name string) error {
	ctx, span := tele.Tracer().Start(ctx, "aci.azureClient.Delete")
	defer span.End()

	future, err := c.aciGroupsClient.Delete(ctx, resourceGroup, name)
	if err != nil {
		return err
	}

	err = future.WaitForCompletionRef(ctx, c.aciGroupsClient.Client)
	if err != nil {
		return err
	}

	_, err = future.Result(c.aciGroupsClient)
	return err
}
