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

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk
type client interface {
	Get(context.Context, string) (resources.Group, error)
	CreateOrUpdate(context.Context, string, resources.Group) (resources.Group, error)
	Delete(context.Context, string) error
}

// azureClient contains the Azure go-sdk Client
type azureClient struct {
	groups resources.GroupsClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newGroupsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{
		groups: c,
	}
}

// newGroupsClient creates a new groups client from subscription ID.
func newGroupsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) resources.GroupsClient {
	groupsClient := resources.NewGroupsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&groupsClient.Client, authorizer)
	return groupsClient
}

// Get gets a resource group.
func (ac *azureClient) Get(ctx context.Context, name string) (resources.Group, error) {
	ctx, span := tele.Tracer().Start(ctx, "groups.AzureClient.Get")
	defer span.End()

	return ac.groups.Get(ctx, name)
}

// CreateOrUpdate creates or updates a resource group.
func (ac *azureClient) CreateOrUpdate(ctx context.Context, name string, group resources.Group) (resources.Group, error) {
	ctx, span := tele.Tracer().Start(ctx, "groups.AzureClient.CreateOrUpdate")
	defer span.End()

	return ac.groups.CreateOrUpdate(ctx, name, group)
}

// Delete deletes a resource group. When you delete a resource group, all of its resources are also deleted.
func (ac *azureClient) Delete(ctx context.Context, name string) error {
	ctx, span := tele.Tracer().Start(ctx, "groups.AzureClient.Delete")
	defer span.End()

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
