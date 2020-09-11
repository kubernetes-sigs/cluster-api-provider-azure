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

package groups

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/resources/mgmt/resources"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string) (resources.Group, error)
	CreateOrUpdate(context.Context, string, resources.Group) (resources.Group, error)
	Delete(context.Context, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	groups resources.GroupsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newGroupsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newGroupsClient creates a new groups client from subscription ID.
func newGroupsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) resources.GroupsClient {
	groupsClient := resources.NewGroupsClientWithBaseURI(baseURI, subscriptionID)
	groupsClient.Authorizer = authorizer
	groupsClient.AddToUserAgent(azure.UserAgent())
	return groupsClient
}

// Get gets a resource group.
func (ac *AzureClient) Get(ctx context.Context, name string) (resources.Group, error) {
	return ac.groups.Get(ctx, name)
}

// CreateOrUpdate creates or updates a resource group.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, name string, group resources.Group) (resources.Group, error) {
	return ac.groups.CreateOrUpdate(ctx, name, group)
}

// Delete deletes a resource group. When you delete a resource group, all of its resources are also deleted.
func (ac *AzureClient) Delete(ctx context.Context, name string) error {
	future, err := ac.groups.Delete(ctx, name)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.groups.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.groups)
	return err
}
