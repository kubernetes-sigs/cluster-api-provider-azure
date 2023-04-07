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

package natgateways

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

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	natgateways armnetwork.NatGatewaysClient
}

// newClient creates a new azureClient from an Authorizer.
func newClient(auth azure.Authorizer) *azureClient {
	c, err := newNatGatewaysClient(auth.SubscriptionID())
	if err != nil {
		panic(err) // TODO: Handle this properly!
	}
	return &azureClient{c}
}

// newNatGatewaysClient creates a new nat gateways client from subscription ID.
func newNatGatewaysClient(subscriptionID string) (armnetwork.NatGatewaysClient, error) {
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return armnetwork.NatGatewaysClient{}, errors.Wrap(err, "failed to create credential")
	}
	client, err := armnetwork.NewNatGatewaysClient(subscriptionID, credential, &arm.ClientOptions{})
	if err != nil {
		return armnetwork.NatGatewaysClient{}, errors.Wrap(err, "cannot create new Resource SKUs client")
	}
	return *client, nil
}

// Get gets the specified nat gateway.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "natgateways.azureClient.Get")
	defer done()

	resp, err := ac.natgateways.Get(ctx, spec.ResourceGroupName(), spec.ResourceName(), &armnetwork.NatGatewaysClientGetOptions{})
	if err != nil {
		return nil, err
	}
	return resp.NatGateway, nil
}

// CreateOrUpdateAsync creates or updates a Nat Gateway asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string, parameters interface{}) (result interface{}, poller *runtime.Poller[armnetwork.NatGatewaysClientCreateOrUpdateResponse], err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "natgateways.azureClient.CreateOrUpdateAsync")
	defer done()

	var natGateway armnetwork.NatGateway
	if parameters != nil {
		ngw, ok := parameters.(armnetwork.NatGateway)
		if !ok {
			return nil, nil, errors.Errorf("%T is not an armnetwork.NatGateway", parameters)
		}
		natGateway = ngw
	}

	opts := &armnetwork.NatGatewaysClientBeginCreateOrUpdateOptions{ResumeToken: resumeToken}
	log.Info("CreateOrUpdateAsync: sending request", "resumeToken", resumeToken)
	poller, err = ac.natgateways.BeginCreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), natGateway, opts)
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

// DeleteAsync deletes a Nat Gateway asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string) (poller *runtime.Poller[armnetwork.NatGatewaysClientDeleteResponse], err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "natgateways.azureClient.DeleteAsync")
	defer done()

	opts := &armnetwork.NatGatewaysClientBeginDeleteOptions{ResumeToken: resumeToken}
	log.Info("DeleteAsync: sending request", "resumeToken", resumeToken)
	poller, err = ac.natgateways.BeginDelete(ctx, spec.ResourceGroupName(), spec.ResourceName(), opts)
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

	// if the operation completed, return a nil poller.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, poller interface{}) (isDone bool, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "natgateways.azureClient.IsDone")
	defer done()

	switch t := poller.(type) {
	case *runtime.Poller[armnetwork.NatGatewaysClientCreateOrUpdateResponse]:
		c, _ := poller.(*runtime.Poller[armnetwork.NatGatewaysClientCreateOrUpdateResponse])
		return c.Done(), nil
	case *runtime.Poller[armnetwork.NatGatewaysClientDeleteResponse]:
		d, _ := poller.(*runtime.Poller[armnetwork.NatGatewaysClientDeleteResponse])
		return d.Done(), nil
	default:
		return false, errors.Errorf("unexpected poller type %T", t)
	}
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, poller interface{}) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "natgateways.azureClient.Result")
	defer done()

	switch t := poller.(type) {
	case *runtime.Poller[armnetwork.NatGatewaysClientCreateOrUpdateResponse]:
		c, _ := poller.(*runtime.Poller[armnetwork.NatGatewaysClientCreateOrUpdateResponse])
		return c.Result(ctx)
	case *runtime.Poller[armnetwork.NatGatewaysClientDeleteResponse]:
		d, _ := poller.(*runtime.Poller[armnetwork.NatGatewaysClientDeleteResponse])
		return d.Result(ctx)
	default:
		return false, errors.Errorf("unexpected poller type %T", t)
	}
}
