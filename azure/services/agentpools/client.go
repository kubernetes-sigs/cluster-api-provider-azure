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

package agentpools

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	agentpools containerservice.AgentPoolsClient
	Reader     services.ServiceLimiter
	Writer     services.ServiceLimiter
	Deleter    services.ServiceLimiter
}

// newClient creates a new agent pools client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newAgentPoolsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	reader, writer, deleter := services.NewRateLimiters(&services.RateLimitConfig{
		AzureServiceRateLimit:             services.AzureServiceRateLimitEnabled,
		AzureServiceRateLimitQPS:          services.AzureServiceRateLimitQPS,
		AzureServiceRateLimitBucket:       services.AzureServiceRateLimitBucket,
		AzureServiceRateLimitQPSWrite:     services.AzureServiceRateLimitQPS,
		AzureServiceRateLimitBucketWrite:  services.AzureServiceRateLimitBucket,
		AzureServiceRateLimitQPSDelete:    services.AzureServiceRateLimitQPS,
		AzureServiceRateLimitBucketDelete: services.AzureServiceRateLimitBucket,
	})
	return &azureClient{
		agentpools: c,
		Reader: services.ServiceLimiter{
			RateLimiter: reader,
		},
		Writer: services.ServiceLimiter{
			RateLimiter: writer,
		},
		Deleter: services.ServiceLimiter{
			RateLimiter: deleter,
		},
	}
}

// newAgentPoolsClient creates a new agent pool client from subscription ID.
func newAgentPoolsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) containerservice.AgentPoolsClient {
	agentPoolsClient := containerservice.NewAgentPoolsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&agentPoolsClient.Client, authorizer)
	return agentPoolsClient
}

// Get gets an agent pool.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "agentpools.azureClient.Get")
	defer done()

	// Verify that we aren't currently rate limited or under active exponential backoff
	if err := ac.Reader.TryRequest(log); err != nil {
		return nil, err
	}

	ret, err := ac.agentpools.Get(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName())
	if err != nil {
		ac.Reader.StoreRetryAfter(log, ret.Response.Response, err, services.DefaultBackoffWaitTimeRead)
		return nil, err
	}
	return ret, nil
}

// CreateOrUpdateAsync creates or updates an agent pool asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "agentpools.azureClient.CreateOrUpdate")
	defer done()

	// Verify that we aren't currently rate limited or under active exponential backoff
	if err := ac.Writer.TryRequest(log); err != nil {
		return nil, nil, err
	}

	agentPool, ok := parameters.(containerservice.AgentPool)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a containerservice.AgentPool", parameters)
	}

	preparer, err := ac.agentpools.CreateOrUpdatePreparer(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), agentPool)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to prepare operation")
	}

	headerSpec, ok := spec.(azure.ResourceSpecGetterWithHeaders)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a azure.ResourceSpecGetterWithHeaders", spec)
	}
	for key, element := range headerSpec.CustomHeaders() {
		preparer.Header.Add(key, element)
	}

	createFuture, err := ac.agentpools.CreateOrUpdateSender(preparer)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to begin operation")
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = createFuture.WaitForCompletionRef(ctx, ac.agentpools.Client)
	if err != nil {
		resp := createFuture.Response()
		ac.Writer.StoreRetryAfter(log, resp, err, services.DefaultBackoffWaitTimeWrite)
		resp.Body.Close()
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &createFuture, err
	}
	result, err = createFuture.Result(ac.agentpools)
	// if the operation completed, return a nil future
	return result, nil, err
}

// DeleteAsync deletes an agent pool asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "agentpools.azureClient.Delete")
	defer done()

	// Verify that we aren't currently rate limited or under active exponential backoff
	if err := ac.Deleter.TryRequest(log); err != nil {
		return nil, err
	}

	deleteFuture, err := ac.agentpools.Delete(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = deleteFuture.WaitForCompletionRef(ctx, ac.agentpools.Client)
	if err != nil {
		resp := deleteFuture.Response()
		ac.Deleter.StoreRetryAfter(log, resp, err, services.DefaultBackoffWaitTimeDelete)
		resp.Body.Close()
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &deleteFuture, err
	}
	_, err = deleteFuture.Result(ac.agentpools)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "agentpools.azureClient.IsDone")
	defer done()

	isDone, err = future.DoneWithContext(ctx, ac.agentpools)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return isDone, nil
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "agentpools.azureClient.Result")
	defer done()

	if future == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}

	switch futureType {
	case infrav1.PutFuture:
		// Marshal and Unmarshal the future to put it into the correct future type so we can access the Result function.
		// Unfortunately the FutureAPI can't be casted directly to AgentPoolsCreateOrUpdateFuture because it is a azureautorest.Future, which doesn't implement the Result function. See PR #1686 for discussion on alternatives.
		// It was converted back to a generic azureautorest.Future from the CAPZ infrav1.Future type stored in Status: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/azure/converters/futures.go#L49.
		var createFuture *containerservice.AgentPoolsCreateOrUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &createFuture); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return createFuture.Result(ac.agentpools)

	case infrav1.DeleteFuture:
		// Delete does not return a result agentPool.
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}
}
