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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/pkg/errors"

	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	Get(context.Context, string, string) (network.PublicIPAddress, error)
	CreateOrUpdateAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
	DeleteAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
	IsDone(context.Context, azureautorest.FutureAPI) (bool, error)
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	publicips network.PublicIPAddressesClient
}

var _ Client = &AzureClient{}

// NewClient creates a new public IP client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newPublicIPAddressesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newPublicIPAddressesClient creates a new public IP client from subscription ID.
func newPublicIPAddressesClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.PublicIPAddressesClient {
	publicIPsClient := network.NewPublicIPAddressesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&publicIPsClient.Client, authorizer)
	return publicIPsClient
}

// Get gets the specified public IP address in a specified resource group.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, ipName string) (network.PublicIPAddress, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.Get")
	defer done()

	return ac.publicips.Get(ctx, resourceGroupName, ipName, "")
}

// CreateOrUpdateAsync creates or updates a static public IP in the specified resource group asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.CreateOrUpdateAsync")
	defer done()

	ip, err := ac.publicIPParams(ctx, spec)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get desired parameters for group %s", spec.ResourceName())
	} else if ip == nil {
		// nothing to do here
		return nil, nil
	}

	future, err := ac.publicips.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), *ip)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.publicips.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &future, err
	}

	_, err = future.Result(ac.publicips)
	return nil, err
}

func (ac *AzureClient) publicIPParams(ctx context.Context, spec azure.ResourceSpecGetter) (*network.PublicIPAddress, error) {
	var params interface{}
	var existing interface{}

	if existingPublicIP, err := ac.Get(ctx, spec.ResourceGroupName(), spec.ResourceName()); err != nil && !azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "failed to get public IP %s in %s", spec.ResourceName(), spec.ResourceGroupName())
	} else if err == nil {
		// public IP already exists
		existing = existingPublicIP
	}

	params, err := spec.Parameters(existing)
	if err != nil {
		return nil, err
	}

	if params == nil {
		// nothing to do here.
		return nil, nil
	}

	ip, ok := params.(network.PublicIPAddress)
	if !ok {
		return nil, errors.Errorf("%T is not a network.PublicIPAddress", params)
	}

	return &ip, nil
}

// TODO(karuppiah7890): Delete this Delete method in favor of Delete Async
// TODO(karuppiah7890): Rename to DeleteAsync and make the delete logic async to delete public IPs asynchronously
// and not block till the operation is over.
// TODO(karuppiah7890): Check all usages and convert it into DeleteAsync and also recreate the mock client logic
// using the new name and signature. Write tests of course
// Delete deletes the specified public IP address.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, ipName string) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.Delete")
	defer done()

	future, err := ac.publicips.Delete(ctx, resourceGroupName, ipName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.publicips.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.publicips)
	return err
}

// Delete deletes the specified public IP asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.DeleteAsync")
	defer done()

	future, err := ac.publicips.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.publicips.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &future, err
	}

	_, err = future.Result(ac.publicips)
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *AzureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.AzureClient.IsDone")
	defer done()

	doneWithContext, err := future.DoneWithContext(ctx, ac.publicips)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return doneWithContext, nil
}
