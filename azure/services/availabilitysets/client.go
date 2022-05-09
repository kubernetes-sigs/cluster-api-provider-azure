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

package availabilitysets

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	availabilitySets compute.AvailabilitySetsClient
}

// NewClient creates a new Resource SKUs Client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	return &AzureClient{
		availabilitySets: newAvailabilitySetsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
	}
}

// newAvailabilitySetsClient creates a new AvailabilitySets Client from subscription ID.
func newAvailabilitySetsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.AvailabilitySetsClient {
	asClient := compute.NewAvailabilitySetsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&asClient.Client, authorizer)
	return asClient
}

// Get gets an availability set.
func (ac *AzureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "availabilitysets.AzureClient.Get")
	defer done()

	return ac.availabilitySets.Get(ctx, spec.ResourceGroupName(), spec.ResourceName())
}

// CreateOrUpdateAsync creates or updates a availability set asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "availabilitySets.AzureClient.CreateOrUpdateAsync")
	defer done()

	availabilitySet, ok := parameters.(compute.AvailabilitySet)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a compute.AvailabilitySet", parameters)
	}

	result, err = ac.availabilitySets.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), availabilitySet)
	return result, nil, err
}

// DeleteAsync deletes a availability set asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "availabilitysets.AzureClient.Delete")
	defer done()

	_, err = ac.availabilitySets.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName())

	if err != nil {
		return nil, err
	}

	return nil, nil
}

// Result fetches the result of a long-running operation future.
func (ac *AzureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	// Result is a no-op for resource groups as only Delete operations return a future.
	return nil, nil
}

// IsDone returns true if the long-running operation has completed.
func (ac *AzureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "availabilitysets.AzureClient.IsDone")
	defer done()

	isDone, err = future.DoneWithContext(ctx, ac.availabilitySets)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return isDone, nil
}
