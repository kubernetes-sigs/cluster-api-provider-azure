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
	Get(context.Context, string, string, string) (network.VirtualNetworkPeering, error)
	CreateOrUpdateAsync(context.Context, azure.ResourceSpecGetter) (interface{}, azureautorest.FutureAPI, error)
	DeleteAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
	IsDone(context.Context, azureautorest.FutureAPI) (bool, error)
	Result(context.Context, azureautorest.FutureAPI, string) (interface{}, error)
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	peerings network.VirtualNetworkPeeringsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new virtual network peerings client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newPeeringsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newPeeringsClient creates a new virtual network peerings client from subscription ID.
func newPeeringsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.VirtualNetworkPeeringsClient {
	peeringsClient := network.NewVirtualNetworkPeeringsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&peeringsClient.Client, authorizer)
	return peeringsClient
}

// Get gets the specified virtual network peering by the peering name, virtual network, and resource group.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, vnetName, peeringName string) (network.VirtualNetworkPeering, error) {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.AzureClient.Get")
	defer span.End()

	return ac.peerings.Get(ctx, resourceGroupName, vnetName, peeringName)
}

// CreateOrUpdateAsync creates or updates a virtual network peering asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter) (interface{}, azureautorest.FutureAPI, error) {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.AzureClient.CreateOrUpdateAsync")
	defer span.End()

	var existingPeering interface{}

	if existing, err := ac.Get(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName()); err != nil && !azure.ResourceNotFound(err) {
		return nil, nil, errors.Wrapf(err, "failed to get virtual network peering %s for %s in %s", spec.ResourceName(), spec.OwnerResourceName(), spec.ResourceGroupName())
	} else if err == nil {
		existingPeering = existing
	}

	params, err := spec.Parameters(existingPeering)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get desired parameters for virtual network peering %s", spec.ResourceName())
	}

	peering, ok := params.(network.VirtualNetworkPeering)
	if !ok {
		if params == nil {
			// nothing to do here.
			return existingPeering, nil, nil
		}
		return nil, nil, errors.Errorf("%T is not a network.VirtualNetworkPeering", params)
	}

	future, err := ac.peerings.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), peering, network.SyncRemoteAddressSpaceTrue)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.peerings.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &future, err
	}

	result, err := future.Result(ac.peerings)
	// if the operation completed, return a nil future
	return result, nil, err
}

// DeleteAsync deletes a virtual network peering asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.AzureClient.Delete")
	defer span.End()

	future, err := ac.peerings.Delete(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.peerings.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &future, err
	}
	_, err = future.Result(ac.peerings)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *AzureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.AzureClient.IsDone")
	defer span.End()

	done, err := future.DoneWithContext(ctx, ac.peerings)
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
	var result func(client network.VirtualNetworkPeeringsClient) (peering network.VirtualNetworkPeering, err error)

	switch futureType {
	case infrav1.PutFuture:
		var future *network.VirtualNetworkPeeringsCreateOrUpdateFuture
		jsonData, err := futureData.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &future); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		result = (*future).Result

	case infrav1.DeleteFuture:
		// Delete does not return a result virtual network peering
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}

	return result(ac.peerings)
}
