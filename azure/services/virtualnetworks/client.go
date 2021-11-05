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

package virtualnetworks

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	Get(context.Context, string, string) (network.VirtualNetwork, error)
	CreateOrUpdateAsync(context.Context, azure.ResourceSpecGetter) (interface{}, azureautorest.FutureAPI, error)
	DeleteAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
	CheckIPAddressAvailability(context.Context, string, string, string) (network.IPAddressAvailabilityResult, error)
	IsDone(context.Context, azureautorest.FutureAPI) (bool, error)
	Result(context.Context, azureautorest.FutureAPI, string) (interface{}, error)
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	virtualnetworks network.VirtualNetworksClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newVirtualNetworksClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{
		virtualnetworks: c,
	}
}

// newVirtualNetworksClient creates a new vnet client from subscription ID.
func newVirtualNetworksClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.VirtualNetworksClient {
	vnetsClient := network.NewVirtualNetworksClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&vnetsClient.Client, authorizer)
	return vnetsClient
}

// Get gets the specified virtual network by resource group.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, vnetName string) (network.VirtualNetwork, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.AzureClient.Get")
	defer done()

	return ac.virtualnetworks.Get(ctx, resourceGroupName, vnetName, "")
}

// CreateOrUpdateAsync creates or updates a virtual network in the specified resource group asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter) (interface{}, azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.AzureClient.CreateOrUpdateAsync")
	defer done()

	var existingVnet interface{}

	if existing, err := ac.Get(ctx, spec.ResourceGroupName(), spec.ResourceName()); err != nil && !azure.ResourceNotFound(err) {
		return nil, nil, errors.Wrapf(err, "failed to get VNet %s in %s", spec.ResourceName(), spec.ResourceGroupName())
	} else if err == nil {
		existingVnet = existing
	}

	params, err := spec.Parameters(existingVnet)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get desired parameters for virtual network %s", spec.ResourceName())
	}

	vn, ok := params.(network.VirtualNetwork)
	if !ok {
		if params == nil {
			// nothing to do here.
			return existingVnet, nil, nil
		}
		return nil, nil, errors.Errorf("%T is not a network.VirtualNetwork", params)
	}

	future, err := ac.virtualnetworks.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), vn)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.virtualnetworks.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &future, err
	}
	result, err := future.Result(ac.virtualnetworks)
	// if the operation completed, return a nil future.
	return result, nil, err
}

// DeleteAsync deletes a virtual network asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.AzureClient.DeleteAsync")
	defer done()

	future, err := ac.virtualnetworks.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.virtualnetworks.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &future, err
	}
	_, err = future.Result(ac.virtualnetworks)
	// if the operation completed, return a nil future.
	return nil, err
}

// CheckIPAddressAvailability checks whether a private IP address is available for use.
func (ac *AzureClient) CheckIPAddressAvailability(ctx context.Context, resourceGroupName, vnetName, ip string) (network.IPAddressAvailabilityResult, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.AzureClient.CheckIPAddressAvailability")
	defer done()

	return ac.virtualnetworks.CheckIPAddressAvailability(ctx, resourceGroupName, vnetName, ip)
}

// IsDone returns true if the long-running operation has completed.
func (ac *AzureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, span := tele.Tracer().Start(ctx, "virtualnetworks.AzureClient.IsDone")
	defer span.End()

	done, err := future.DoneWithContext(ctx, ac.virtualnetworks)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return done, nil
}

// Result fetches the result of a long-running operation future.
func (ac *AzureClient) Result(ctx context.Context, futureData azureautorest.FutureAPI, futureType string) (interface{}, error) {
	if futureData == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}
	var result func(client network.VirtualNetworksClient) (vnet network.VirtualNetwork, err error)

	switch futureType {
	case infrav1.PutFuture:
		var future *network.VirtualNetworksCreateOrUpdateFuture
		jsonData, err := futureData.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &future); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		result = (*future).Result

	case infrav1.DeleteFuture:
		// Delete does not return a result vnet.
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}

	return result(ac.virtualnetworks)
}
