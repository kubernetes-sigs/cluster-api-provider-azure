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

package vnetpeerings

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
	peerings armnetwork.VirtualNetworkPeeringsClient
}

// NewClient creates an AzureClient from an Authorizer.
func NewClient(auth azure.Authorizer) *AzureClient {
	c, err := newPeeringsClient(auth.SubscriptionID())
	if err != nil {
		panic(err) // TODO: Handle this properly!
	}
	return &AzureClient{c}
}

// newPeeringsClient creates a new virtual network peerings client from subscription ID.
func newPeeringsClient(subscriptionID string) (armnetwork.VirtualNetworkPeeringsClient, error) {
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return armnetwork.VirtualNetworkPeeringsClient{}, errors.Wrap(err, "failed to create credential")
	}
	client, err := armnetwork.NewVirtualNetworkPeeringsClient(subscriptionID, credential, &arm.ClientOptions{})
	if err != nil {
		return armnetwork.VirtualNetworkPeeringsClient{}, errors.Wrap(err, "cannot create new virtual network peerings client")
	}
	return *client, nil
}

// Get returns a virtual network peering by the specified resource group, virtual network, and peering name.
func (ac *AzureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "vnetpeerings.AzureClient.Get")
	defer done()

	opts := &armnetwork.VirtualNetworkPeeringsClientGetOptions{}
	resp, err := ac.peerings.Get(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), opts)
	if err != nil {
		return nil, err
	}
	return resp.VirtualNetworkPeering, nil
}

// CreateOrUpdateAsync creates or updates a virtual network peering asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string, parameters interface{}) (result interface{}, poller *runtime.Poller[armnetwork.VirtualNetworkPeeringsClientCreateOrUpdateResponse], err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "vnetpeerings.AzureClient.CreateOrUpdateAsync")
	defer done()

	var peering armnetwork.VirtualNetworkPeering
	if parameters != nil {
		p, ok := parameters.(armnetwork.VirtualNetworkPeering)
		if !ok {
			return nil, nil, errors.Errorf("%T is not an armnetwork.VirtualNetworkPeering", parameters)
		}
		peering = p
	}

	opts := &armnetwork.VirtualNetworkPeeringsClientBeginCreateOrUpdateOptions{ResumeToken: resumeToken}
	log.Info("CreateOrUpdateAsync: sending request", "resumeToken", resumeToken)
	poller, err = ac.peerings.BeginCreateOrUpdate(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), peering, opts)
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

// DeleteAsync deletes a virtual network peering asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string) (poller *runtime.Poller[armnetwork.VirtualNetworkPeeringsClientDeleteResponse], err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "vnetpeerings.AzureClient.Delete")
	defer done()

	opts := &armnetwork.VirtualNetworkPeeringsClientBeginDeleteOptions{ResumeToken: resumeToken}
	log.Info("DeleteAsync: sending request", "resumeToken", resumeToken)
	poller, err = ac.peerings.BeginDelete(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), opts)
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
func (ac *AzureClient) IsDone(ctx context.Context, poller interface{}) (isDone bool, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "vnetpeerings.AzureClient.IsDone")
	defer done()

	switch t := poller.(type) {
	case *runtime.Poller[armnetwork.VirtualNetworkPeeringsClientCreateOrUpdateResponse]:
		c, _ := poller.(*runtime.Poller[armnetwork.VirtualNetworkPeeringsClientCreateOrUpdateResponse])
		return c.Done(), nil
	case *runtime.Poller[armnetwork.VirtualNetworkPeeringsClientDeleteResponse]:
		d, _ := poller.(*runtime.Poller[armnetwork.VirtualNetworkPeeringsClientDeleteResponse])
		return d.Done(), nil
	default:
		return false, errors.Errorf("unexpected poller type %T", t)
	}
}

// Result fetches the result of a long-running operation future.
func (ac *AzureClient) Result(ctx context.Context, poller interface{}) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "vnetpeerings.AzureClient.Result")
	defer done()

	switch t := poller.(type) {
	case *runtime.Poller[armnetwork.VirtualNetworkPeeringsClientCreateOrUpdateResponse]:
		c, _ := poller.(*runtime.Poller[armnetwork.VirtualNetworkPeeringsClientCreateOrUpdateResponse])
		return c.Result(ctx)
	case *runtime.Poller[armnetwork.VirtualNetworkPeeringsClientDeleteResponse]:
		d, _ := poller.(*runtime.Poller[armnetwork.VirtualNetworkPeeringsClientDeleteResponse])
		return d.Result(ctx)
	default:
		return false, errors.Errorf("unexpected poller type %T", t)
	}
}
