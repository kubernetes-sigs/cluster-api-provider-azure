/*
Copyright 2023 The Kubernetes Authors.

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

package asyncpoller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Service is an implementation of the Reconciler interface. It handles asynchronous creation and deletion of resources.
type Service[C, D any] struct {
	Scope FutureScope
	Creator[C]
	Deleter[D]
}

// New creates a new async service.
func New[C, D any](scope FutureScope, createClient Creator[C], deleteClient Deleter[D]) *Service[C, D] {
	return &Service[C, D]{
		Scope:   scope,
		Creator: createClient,
		Deleter: deleteClient,
	}
}

// processOngoingOperation is a helper function that will process an ongoing operation to check if it is done.
// If it is not done, it will return a transient error.
func processOngoingOperation[T any](ctx context.Context, scope FutureScope, client FutureHandler, resourceName string, serviceName string, futureType string) (result interface{}, err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "asyncpoller.processOngoingOperation")
	defer done()

	future := scope.GetLongRunningOperationState(resourceName, serviceName, futureType)
	if future == nil {
		log.V(2).Info("no long-running operation found", "service", serviceName, "resource", resourceName)
		return nil, nil
	}
	poller, err := converters.FutureToPoller[T](*future)
	if err != nil {
		// Reset the future data to avoid getting stuck in a bad loop.
		// In theory, this should never happen, but if for some reason the future that is already stored in Status isn't properly formatted
		// and we don't reset it we would be stuck in an infinite loop trying to parse it.
		scope.DeleteLongRunningOperationState(resourceName, serviceName, futureType)
		return nil, errors.Wrap(err, "could not decode future data, resetting long-running operation state")
	}

	isDone, err := client.IsDone(ctx, poller)
	// Assume that if isDone is true, then we successfully checked that the
	// operation was complete even if err is non-nil. Assume the error in that
	// case is unrelated and will be captured in Result below.
	if !isDone {
		if err != nil {
			return nil, errors.Wrap(err, "failed checking if the operation was complete")
		}

		// Operation is still in progress, update conditions and requeue.
		log.V(2).Info("long-running operation is still ongoing", "service", serviceName, "resource", resourceName)
		return nil, azure.WithTransientError(azure.NewOperationNotDoneError(future), getRequeueAfterFromPoller(poller))
	}
	if err != nil {
		log.V(2).Error(err, "error checking long-running operation status after it finished")
	}

	// Once the operation is done, we can delete the long-running operation state.
	// If the operation failed, this will allow it to be retried during the next reconciliation.
	// If the resource is not found, we also reset the long-running operation state so we can attempt to create it again.
	// This can happen if the resource was deleted by another process before we could get the result.
	scope.DeleteLongRunningOperationState(resourceName, serviceName, futureType)

	// Resource has been created/deleted/updated.
	log.V(2).Info("long-running operation has completed", "service", serviceName, "resource", resourceName)
	return client.Result(ctx, &poller)
}

// CreateOrUpdateResource implements the logic for creating a new, or updating an existing, resource Asynchronously.
func (s *Service[C, D]) CreateOrUpdateResource(ctx context.Context, spec azure.ResourceSpecGetter, serviceName string) (result interface{}, err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "asyncpoller.Service.CreateOrUpdateResource")
	defer done()

	resourceName := spec.ResourceName()
	rgName := spec.ResourceGroupName()
	futureType := infrav1.PutFuture

	// Check if there is an ongoing long-running operation.
	future := s.Scope.GetLongRunningOperationState(resourceName, serviceName, futureType)
	if future != nil {
		return processOngoingOperation[C](ctx, s.Scope, s.Creator, resourceName, serviceName, futureType)
	}

	// Get the resource if it already exists, and use it to construct the desired resource parameters.
	var existingResource interface{}
	if existing, err := s.Creator.Get(ctx, spec); err != nil && !azure.ResourceNotFound(err) {
		errWrapped := errors.Wrapf(err, "failed to get existing resource %s/%s (service: %s)", rgName, resourceName, serviceName)
		return nil, azure.WithTransientError(errWrapped, getRetryAfterFromError(err))
	} else if err == nil {
		existingResource = existing
		log.V(2).Info("successfully got existing resource", "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
	}

	// Construct parameters using the resource spec and information from the existing resource, if there is one.
	parameters, err := spec.Parameters(ctx, existingResource)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get desired parameters for resource %s/%s (service: %s)", rgName, resourceName, serviceName)
	} else if parameters == nil {
		// Nothing to do, don't create or update the resource and return the existing resource.
		log.V(2).Info("resource up to date", "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
		return existingResource, nil
	}

	// Create or update the resource with the desired parameters.
	logMessageVerbPrefix := "creat"
	if existingResource != nil {
		logMessageVerbPrefix = "updat"
	}
	log.V(2).Info(fmt.Sprintf("%sing resource", logMessageVerbPrefix), "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
	result, poller, err := s.Creator.CreateOrUpdateAsync(ctx, spec, parameters)
	errWrapped := errors.Wrapf(err, fmt.Sprintf("failed to %se resource %s/%s (service: %s)", logMessageVerbPrefix, rgName, resourceName, serviceName))
	if poller != nil {
		future, err := converters.PollerToFuture(poller, infrav1.PutFuture, serviceName, resourceName, rgName)
		if err != nil {
			return nil, errWrapped
		}
		s.Scope.SetLongRunningOperationState(future)
		return nil, azure.WithTransientError(azure.NewOperationNotDoneError(future), getRequeueAfterFromPoller(poller))
	} else if err != nil {
		return nil, errWrapped
	}

	log.V(2).Info(fmt.Sprintf("successfully %sed resource", logMessageVerbPrefix), "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
	return result, nil
}

// DeleteResource implements the logic for deleting a resource Asynchronously.
func (s *Service[C, D]) DeleteResource(ctx context.Context, spec azure.ResourceSpecGetter, serviceName string) (err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "asyncpoller.Service.DeleteResource")
	defer done()

	resourceName := spec.ResourceName()
	rgName := spec.ResourceGroupName()
	futureType := infrav1.DeleteFuture

	// Check if there is an ongoing long-running operation.
	future := s.Scope.GetLongRunningOperationState(resourceName, serviceName, futureType)
	if future != nil {
		_, err := processOngoingOperation[D](ctx, s.Scope, s.Deleter, resourceName, serviceName, futureType)
		return err
	}

	// No long-running operation is active, so delete the resource.
	log.V(2).Info("deleting resource", "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
	poller, err := s.Deleter.DeleteAsync(ctx, spec)
	if poller != nil {
		future, err := converters.PollerToFuture(poller, infrav1.DeleteFuture, serviceName, resourceName, rgName)
		if err != nil {
			return errors.Wrapf(err, "failed to delete resource %s/%s (service: %s)", rgName, resourceName, serviceName)
		}
		s.Scope.SetLongRunningOperationState(future)
		return azure.WithTransientError(azure.NewOperationNotDoneError(future), getRequeueAfterFromPoller(poller))
	} else if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		return errors.Wrapf(err, "failed to delete resource %s/%s (service: %s)", rgName, resourceName, serviceName)
	}

	log.V(2).Info("successfully deleted resource", "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
	return nil
}

// getRequeueAfterFromPoller returns the max between the `RETRY-AFTER` header and the default requeue time.
// This ensures we respect the retry-after header if it is set and avoid retrying too often during an API throttling event.
func getRequeueAfterFromPoller[T any](poller *runtime.Poller[T]) time.Duration {
	// TODO: there doesn't seem to be a replacement for sdkFuture.GetPollingDelay() in the new poller.
	return reconciler.DefaultReconcilerRequeue
}

// getRetryAfterFromError returns the time.Duration from the http.Response in the autorest.DetailedError.
// If there is no Response object, or if there is no meaningful Retry-After header data, we return a default.
func getRetryAfterFromError(err error) time.Duration {
	// TODO: need to refactor autorest out of this codebase entirely.
	// In case we aren't able to introspect Retry-After from the error type, we'll return this default
	ret := reconciler.DefaultReconcilerRequeue
	var responseError azcore.ResponseError
	// if we have a strongly typed azcore.ResponseError then we can introspect the HTTP response data
	if errors.As(err, &responseError) && responseError.RawResponse != nil {
		// If we have Retry-After HTTP header data for any reason, prefer it
		if retryAfter := responseError.RawResponse.Header.Get("Retry-After"); retryAfter != "" {
			// This handles the case where Retry-After data is in the form of units of seconds
			if rai, err := strconv.Atoi(retryAfter); err == nil {
				ret = time.Duration(rai) * time.Second
				// This handles the case where Retry-After data is in the form of absolute time
			} else if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
				ret = time.Until(t)
			}
			// If we didn't find Retry-After HTTP header data but the response type is 429,
			// we'll have to come up with our sane default.
		} else if responseError.RawResponse.StatusCode == http.StatusTooManyRequests {
			ret = reconciler.DefaultHTTP429RetryAfter
		}
	}
	return ret
}
