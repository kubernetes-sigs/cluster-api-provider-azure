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

package tags

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-10-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	GetAtScope(context.Context, string) (resources.TagsResource, error)
	UpdateAtScope(context.Context, string, resources.TagsPatchResource) (resources.TagsResource, error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	tags resources.TagsClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new tags client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newTagsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newTagsClient creates a new tags client from subscription ID.
func newTagsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) resources.TagsClient {
	tagsClient := resources.NewTagsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&tagsClient.Client, authorizer)
	return tagsClient
}

// GetAtScope sends the get at scope request.
func (ac *azureClient) GetAtScope(ctx context.Context, scope string) (resources.TagsResource, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "tags.AzureClient.GetAtScope")
	defer done()

	return ac.tags.GetAtScope(ctx, scope)
}

// UpdateAtScope this operation allows replacing, merging or selectively deleting tags on the specified resource or
// subscription.
func (ac *azureClient) UpdateAtScope(ctx context.Context, scope string, parameters resources.TagsPatchResource) (resources.TagsResource, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "tags.AzureClient.UpdateAtScope")
	defer done()

	return ac.tags.UpdateAtScope(ctx, scope, parameters)
}
