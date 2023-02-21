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

package publicips

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	publicips armnetwork.PublicIPAddressesClient
}

// NewClient creates a new public IP client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c, err := newPublicIPAddressesClient(auth.SubscriptionID())
	if err != nil {
		panic(err) // TODO: Handle this correctly!
	}
	return &AzureClient{c}
}

// newPublicIPAddressesClient creates a new public IP addresses client from subscription ID.
func newPublicIPAddressesClient(subscriptionID string) (armnetwork.PublicIPAddressesClient, error) {
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return armnetwork.PublicIPAddressesClient{}, errors.Wrap(err, "failed to create credential")
	}
	client, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, credential, &arm.ClientOptions{})
	if err != nil {
		return armnetwork.PublicIPAddressesClient{}, errors.Wrap(err, "cannot create new Resource SKUs client")
	}
	return *client, nil
}

// Get gets the specified public IP address in a specified resource group.
func (ac *AzureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.Get")
	defer done()

	resp, err := ac.publicips.Get(ctx, spec.ResourceGroupName(), spec.ResourceName(), &armnetwork.PublicIPAddressesClientGetOptions{})
	if err != nil {
		return nil, err
	}
	return resp.PublicIPAddress, nil
}

// CreateOrUpdateAsync creates or updates a static or dynamic public IP address.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, poller *runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse], err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.CreateOrUpdateAsync")
	defer done()

	publicip, ok := parameters.(armnetwork.PublicIPAddress)
	if !ok {
		return nil, nil, errors.Errorf("%T is not an armnetwork.PublicIPAddress", parameters)
	}

	opts := &armnetwork.PublicIPAddressesClientBeginCreateOrUpdateOptions{}
	poller, err = ac.publicips.BeginCreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), publicip, opts)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	result, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
	if err != nil {
		// if an error occurs, return the poller.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, poller, err
	}

	// if the operation completed, return a nil poller
	return result, nil, err
}

// DeleteAsync deletes the specified public IP address asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (poller *runtime.Poller[armnetwork.PublicIPAddressesClientDeleteResponse], err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.DeleteAsync")
	defer done()

	poller, err = ac.publicips.BeginDelete(ctx, spec.ResourceGroupName(), spec.ResourceName(), &armnetwork.PublicIPAddressesClientBeginDeleteOptions{})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
	if err != nil {
		// if an error occurs, return the poller.
		// this means the long-running operation didn't finish in the specified timeout.
		return poller, err
	}
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *AzureClient) IsDone(ctx context.Context, poller interface{}) (isDone bool, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.IsDone")
	defer done()

	switch t := poller.(type) {
	case runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse]:
		c, _ := poller.(runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse])
		return c.Done(), nil
	case runtime.Poller[armnetwork.PublicIPAddressesClientDeleteResponse]:
		d, _ := poller.(runtime.Poller[armnetwork.PublicIPAddressesClientDeleteResponse])
		return d.Done(), nil
	default:
		return false, errors.Errorf("unexpected poller type %T", t)
	}
}

// Result fetches the result of a long-running operation.
func (ac *AzureClient) Result(ctx context.Context, poller interface{}) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.Result")
	defer done()

	switch t := poller.(type) {
	case runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse]:
		c, _ := poller.(runtime.Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse])
		return c.Result(ctx)
	case runtime.Poller[armnetwork.PublicIPAddressesClientDeleteResponse]:
		d, _ := poller.(runtime.Poller[armnetwork.PublicIPAddressesClientDeleteResponse])
		return d.Result(ctx)
	default:
		return false, errors.Errorf("unknown poller type %T", t)
	}
}
