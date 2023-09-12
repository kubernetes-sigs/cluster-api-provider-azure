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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/asyncpoller"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	Get(context.Context, azure.ResourceSpecGetter) (interface{}, error)
	CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string, parameters interface{}) (result interface{}, poller *runtime.Poller[armresources.ResourceGroupsClientCreateOrUpdateResponse], err error)
	DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string) (poller *runtime.Poller[armresources.ResourceGroupsClientDeleteResponse], err error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	groups *armresources.ResourceGroupsClient
}

var _ client = (*azureClient)(nil)

// newClient creates a resource groups client from an authorizer.
func newClient(auth azure.Authorizer) (*azureClient, error) {
	opts, err := azure.ARMClientOptions(auth.CloudEnvironment())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create resourcegroups client options")
	}
	factory, err := armresources.NewClientFactory(auth.SubscriptionID(), auth.Token(), opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create armresources client factory")
	}
	return &azureClient{factory.NewResourceGroupsClient()}, nil
}

// Get gets a resource group.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.AzureClient.Get")
	defer done()

	resp, err := ac.groups.Get(ctx, spec.ResourceGroupName(), nil)
	if err != nil {
		return nil, err
	}
	return resp.ResourceGroup, nil
}

// CreateOrUpdateAsync creates or updates a resource group.
// Creating a resource group is not a long running operation, so we don't ever return a poller.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string, parameters interface{}) (result interface{}, poller *runtime.Poller[armresources.ResourceGroupsClientCreateOrUpdateResponse], err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.AzureClient.CreateOrUpdate")
	defer done()

	group, ok := parameters.(armresources.ResourceGroup)
	if !ok && parameters != nil {
		return nil, nil, errors.Errorf("%T is not an armresources.ResourceGroup", parameters)
	}

	resp, err := ac.groups.CreateOrUpdate(ctx, spec.ResourceName(), group, nil)
	return resp.ResourceGroup, nil, err
}

// DeleteAsync deletes a resource group asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Poller which can be used to track the ongoing
// progress of the operation.
//
// NOTE: When you delete a resource group, all of its resources are also deleted.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string) (poller *runtime.Poller[armresources.ResourceGroupsClientDeleteResponse], err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.AzureClient.Delete")
	defer done()

	opts := &armresources.ResourceGroupsClientBeginDeleteOptions{ResumeToken: resumeToken}
	poller, err = ac.groups.BeginDelete(ctx, spec.ResourceGroupName(), opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	pollOpts := &runtime.PollUntilDoneOptions{Frequency: asyncpoller.DefaultPollerFrequency}
	_, err = poller.PollUntilDone(ctx, pollOpts)
	if err != nil {
		// if an error occurs, return the Poller.
		// this means the long-running operation didn't finish in the specified timeout.
		return poller, err
	}

	// if the operation completed, return a nil poller.
	return nil, err
}
