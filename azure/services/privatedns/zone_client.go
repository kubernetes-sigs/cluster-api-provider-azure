/*
Copyright 2022 The Kubernetes Authors.

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

package privatedns

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureZonesClient contains the Azure go-sdk Client for private dns zone.
type azureZonesClient struct {
	privatezones privatedns.PrivateZonesClient
}

// newPrivateZonesClient creates a new private zones client from subscription ID.
func newPrivateZonesClient(auth azure.Authorizer) *azureZonesClient {
	c := privatedns.NewPrivateZonesClientWithBaseURI(auth.BaseURI(), auth.SubscriptionID())
	azure.SetAutoRestClientDefaults(&c.Client, auth.Authorizer())
	return &azureZonesClient{
		privatezones: c,
	}
}

// CreateOrUpdateAsync creates or updates a private dns zone asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (azc *azureZonesClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatedns.azureZonesClient.CreateOrUpdateAsync")
	defer done()

	zone, ok := parameters.(privatedns.PrivateZone)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a privatedns.PrivateZone", parameters)
	}

	createFuture, err := azc.privatezones.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), zone, "", "")
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = createFuture.WaitForCompletionRef(ctx, azc.privatezones.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &createFuture, err
	}
	result, err = createFuture.Result(azc.privatezones)
	// if the operation completed, return a nil future
	return result, nil, err
}

// Get gets the specified private dns zone.
func (azc *azureZonesClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatedns.azureZonesClient.Get")
	defer done()
	zone, err := azc.privatezones.Get(ctx, spec.ResourceGroupName(), spec.ResourceName())
	if err != nil {
		return privatedns.PrivateZone{}, err
	}
	return zone, nil
}

// DeleteAsync deletes a private dns zone asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (azc *azureZonesClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatedns.azureZonesClient.DeleteAsync")
	defer done()

	deleteFuture, err := azc.privatezones.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName(), "")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = deleteFuture.WaitForCompletionRef(ctx, azc.privatezones.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &deleteFuture, err
	}
	_, err = deleteFuture.Result(azc.privatezones)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (azc *azureZonesClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatedns.azureZonesClient.IsDone")
	defer done()

	isDone, err = future.DoneWithContext(ctx, azc.privatezones)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return isDone, nil
}

// Result fetches the result of a long-running operation future.
func (azc *azureZonesClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "privatedns.azureZonesClient.Result")
	defer done()

	if future == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}

	switch futureType {
	case infrav1.PutFuture:
		// Marshal and Unmarshal the future to put it into the correct future type so we can access the Result function.
		// Unfortunately the FutureAPI can't be casted directly to PrivateZonesCreateOrUpdateFuture because it is a azureautorest.Future, which doesn't implement the Result function. See PR #1686 for discussion on alternatives.
		// It was converted back to a generic azureautorest.Future from the CAPZ infrav1.Future type stored in Status: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/azure/converters/futures.go#L49.
		var createFuture *privatedns.PrivateZonesCreateOrUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &createFuture); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return createFuture.Result(azc.privatezones)

	case infrav1.DeleteFuture:
		// Delete does not return a result private dns zone.
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}
}
