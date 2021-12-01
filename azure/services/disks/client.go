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

package disks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	DeleteAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
	Result(context.Context, azureautorest.FutureAPI, string) (interface{}, error)
	IsDone(context.Context, azureautorest.FutureAPI) (bool, error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	disks compute.DisksClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM Client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := NewDisksClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// NewDisksClient creates a new disks Client from subscription ID.
func NewDisksClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.DisksClient {
	disksClient := compute.NewDisksClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&disksClient.Client, authorizer)
	return disksClient
}

// DeleteAsync deletes a route table asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "disks.Service.DeleteAsync")
	defer done()

	future, err := ac.disks.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.disks.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &future, err
	}
	_, err = future.Result(ac.disks)
	// if the operation completed, return a nil future.
	return nil, err
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, futureData azureautorest.FutureAPI, futureType string) (interface{}, error) {
	return nil, nil
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "natgateways.Service.IsDone")
	defer done()

	isDone, err := future.DoneWithContext(ctx, ac.disks)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return isDone, nil
}
