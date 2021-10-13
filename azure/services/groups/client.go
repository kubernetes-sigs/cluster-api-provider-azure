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
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	Get(context.Context, string) (resources.Group, error)
	CreateOrUpdateAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
	DeleteAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
	IsDone(context.Context, azureautorest.FutureAPI) (bool, error)
}

// azureClient contains the Azure go-sdk Client.
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
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.AzureClient.Get")
	defer done()

	return ac.groups.Get(ctx, name)
}

// CreateOrUpdateAsync creates or updates a resource group.
// Creating a resource group is not a long running operation, so we don't ever return a future.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.AzureClient.CreateOrUpdate")
	defer done()

	group, err := ac.resourceGroupParams(ctx, spec)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get desired parameters for group %s", spec.ResourceName())
	} else if group == nil {
		// nothing to do here
		return nil, nil
	}

	_, err = ac.groups.CreateOrUpdate(ctx, spec.ResourceName(), *group)
	return nil, err
}

// DeleteAsync deletes a resource group asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
//
// NOTE: When you delete a resource group, all of its resources are also deleted.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.AzureClient.Delete")
	defer done()

	future, err := ac.groups.Delete(ctx, spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.groups.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &future, err
	}
	_, err = future.Result(ac.groups)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.AzureClient.IsDone")
	defer done()

	isDone, err := future.DoneWithContext(ctx, ac.groups)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return isDone, nil
}

// resourceGroupParams returns the desired resource group parameters from the given spec.
func (ac *azureClient) resourceGroupParams(ctx context.Context, spec azure.ResourceSpecGetter) (*resources.Group, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.AzureClient.resourceGroupParams")
	defer done()

	var params interface{}

	existingRG, err := ac.Get(ctx, spec.ResourceName())
	if azure.ResourceNotFound(err) {
		// rg doesn't exist, create it from scratch.
		params, err = spec.Parameters(nil)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to get RG %s", spec.ResourceName())
	} else {
		// rg already exists
		params, err = spec.Parameters(existingRG)
		if err != nil {
			return nil, err
		}
	}

	rg, ok := params.(resources.Group)
	if !ok {
		if params == nil {
			// nothing to do here.
			return nil, nil
		}
		return nil, errors.Errorf("%T is not a resources.Group", params)
	}

	return &rg, nil
}
