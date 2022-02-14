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

package inboundnatrules

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	List(context.Context, string, string) (result []network.InboundNatRule, err error)
	Get(context.Context, azure.ResourceSpecGetter) (result interface{}, err error)
	CreateOrUpdateAsync(context.Context, azure.ResourceSpecGetter, interface{}) (result interface{}, future azureautorest.FutureAPI, err error)
	DeleteAsync(context.Context, azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error)
	IsDone(context.Context, azureautorest.FutureAPI) (isDone bool, err error)
	Result(context.Context, azureautorest.FutureAPI, string) (result interface{}, err error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	inboundnatrules network.InboundNatRulesClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new inbound NAT rules client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	inboundNatRulesClient := newInboundNatRulesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{
		inboundnatrules: inboundNatRulesClient,
	}
}

// newInboundNatClient creates a new inbound NAT rules client from subscription ID.
func newInboundNatRulesClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.InboundNatRulesClient {
	inboundNatRulesClient := network.NewInboundNatRulesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&inboundNatRulesClient.Client, authorizer)
	return inboundNatRulesClient
}

// Get gets the specified inbound NAT rules.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "inboundnatrules.azureClient.Get")
	defer done()

	return ac.inboundnatrules.Get(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), "")
}

// List returns all inbound NAT rules on a load balancer.
func (ac *azureClient) List(ctx context.Context, resourceGroupName, lbName string) (result []network.InboundNatRule, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "inboundnatrules.azureClient.List")
	defer done()

	iter, err := ac.inboundnatrules.ListComplete(ctx, resourceGroupName, lbName)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("could not list inbound NAT rules for load balancer %s", lbName))
	}

	var natRules []network.InboundNatRule
	for iter.NotDone() {
		natRules = append(natRules, iter.Value())
		if err := iter.NextWithContext(ctx); err != nil {
			return natRules, errors.Wrap(err, "could not iterate inbound NAT rules")
		}
	}

	return natRules, nil
}

// CreateOrUpdateAsync creates or updates an inbound NAT rule asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "inboundnatrules.azureClient.CreateOrUpdateAsync")
	defer done()

	natRule, ok := parameters.(network.InboundNatRule)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a network.InboundNatRule", parameters)
	}

	createFuture, err := ac.inboundnatrules.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), natRule)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = createFuture.WaitForCompletionRef(ctx, ac.inboundnatrules.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &createFuture, err
	}

	result, err = createFuture.Result(ac.inboundnatrules)
	// if the operation completed, return a nil future
	return result, nil, err
}

// DeleteAsync deletes an inbound NAT rule asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "inboundnatrules.azureClient.DeleteAsync")
	defer done()

	deleteFuture, err := ac.inboundnatrules.Delete(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = deleteFuture.WaitForCompletionRef(ctx, ac.inboundnatrules.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &deleteFuture, err
	}
	_, err = deleteFuture.Result(ac.inboundnatrules)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "inboundnatrules.azureClient.IsDone")
	defer done()

	isDone, err = future.DoneWithContext(ctx, ac.inboundnatrules)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return isDone, nil
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "inboundnatrules.azureClient.Result")
	defer done()

	if future == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}

	switch futureType {
	case infrav1.PutFuture:
		// Marshal and Unmarshal the future to put it into the correct future type so we can access the Result function.
		// Unfortunately the FutureAPI can't be casted directly to InboundNatRulesCreateOrUpdateFuture because it is a azureautorest.Future, which doesn't implement the Result function. See PR #1686 for discussion on alternatives.
		// It was converted back to a generic azureautorest.Future from the CAPZ infrav1.Future type stored in Status: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/azure/converters/futures.go#L49.
		var createFuture *network.InboundNatRulesCreateOrUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &createFuture); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return createFuture.Result(ac.inboundnatrules)

	case infrav1.DeleteFuture:
		// Delete does not return a result inbound NAT rule
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}
}
