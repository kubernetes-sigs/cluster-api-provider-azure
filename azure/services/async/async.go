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

package async

import (
	"context"
	"time"

	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// processOngoingOperation is a helper function that will process an ongoing operation to check if it is done.
// If it is not done, it will return a transient error.
func processOngoingOperation(ctx context.Context, scope FutureScope, client FutureHandler, resourceName string, serviceName string) (interface{}, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "async.Service.processOngoingOperation")
	defer done()

	future := scope.GetLongRunningOperationState(resourceName, serviceName)
	if future == nil {
		scope.V(2).Info("no long running operation found", "service", serviceName, "resource", resourceName)
		return nil, nil
	}
	sdkFuture, err := converters.FutureToSDK(*future)
	if err != nil {
		// Reset the future data to avoid getting stuck in a bad loop.
		// In theory, this should never happen, but if for some reason the future that is already stored in Status isn't properly formatted
		// and we don't reset it we would be stuck in an infinite loop trying to parse it.
		scope.DeleteLongRunningOperationState(resourceName, serviceName)
		return nil, errors.Wrap(err, "could not decode future data, resetting long-running operation state")
	}
	isDone, err := client.IsDone(ctx, sdkFuture)
	if err != nil {
		return nil, errors.Wrap(err, "failed checking if the operation was complete")
	}

	if !isDone {
		// Operation is still in progress, update conditions and requeue.
		scope.V(2).Info("long running operation is still ongoing", "service", serviceName, "resource", resourceName)
		return nil, azure.WithTransientError(azure.NewOperationNotDoneError(future), retryAfter(sdkFuture))
	}

	// Resource has been created/deleted/updated.
	scope.V(2).Info("long running operation has completed", "service", serviceName, "resource", resourceName)
	result, err := client.Result(ctx, sdkFuture, future.Type)
	if err == nil {
		scope.DeleteLongRunningOperationState(resourceName, serviceName)
	}
	return result, err
}

// CreateResource implements the logic for creating a resource Asynchronously.
func CreateResource(ctx context.Context, scope FutureScope, client Creator, spec azure.ResourceSpecGetter, serviceName string) (interface{}, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "async.Service.CreateResource")
	defer done()

	resourceName := spec.ResourceName()
	rgName := spec.ResourceGroupName()

	// Check if there is an ongoing long running operation.
	future := scope.GetLongRunningOperationState(resourceName, serviceName)
	if future != nil {
		return processOngoingOperation(ctx, scope, client, resourceName, serviceName)
	}

	// No long running operation is active, so create the resource.
	scope.V(2).Info("creating resource", "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
	result, sdkFuture, err := client.CreateOrUpdateAsync(ctx, spec)
	if sdkFuture != nil {
		future, err := converters.SDKToFuture(sdkFuture, infrav1.PutFuture, serviceName, resourceName, rgName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create resource %s/%s (service: %s)", rgName, resourceName, serviceName)
		}
		scope.SetLongRunningOperationState(future)
		return nil, azure.WithTransientError(azure.NewOperationNotDoneError(future), retryAfter(sdkFuture))
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to create resource %s/%s (service: %s)", rgName, resourceName, serviceName)
	}

	scope.V(2).Info("successfully created resource", "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
	return result, nil
}

// DeleteResource implements the logic for deleting a resource Asynchronously.
func DeleteResource(ctx context.Context, scope FutureScope, client Deleter, spec azure.ResourceSpecGetter, serviceName string) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "async.Service.DeleteResource")
	defer done()

	resourceName := spec.ResourceName()
	rgName := spec.ResourceGroupName()

	// Check if there is an ongoing long running operation.
	future := scope.GetLongRunningOperationState(resourceName, serviceName)
	if future != nil {
		_, err := processOngoingOperation(ctx, scope, client, resourceName, serviceName)
		return err
	}

	// No long running operation is active, so delete the resource.
	scope.V(2).Info("deleting resource", "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
	sdkFuture, err := client.DeleteAsync(ctx, spec)
	if sdkFuture != nil {
		future, err := converters.SDKToFuture(sdkFuture, infrav1.DeleteFuture, serviceName, resourceName, rgName)
		if err != nil {
			return errors.Wrapf(err, "failed to delete resource %s/%s (service: %s)", rgName, resourceName, serviceName)
		}
		scope.SetLongRunningOperationState(future)
		return azure.WithTransientError(azure.NewOperationNotDoneError(future), retryAfter(sdkFuture))
	} else if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		return errors.Wrapf(err, "failed to delete resource %s/%s (service: %s)", rgName, resourceName, serviceName)
	}

	scope.V(2).Info("successfully deleted resource", "service", serviceName, "resource", resourceName, "resourceGroup", rgName)
	return nil
}

// retryAfter returns the max between the `RETRY-AFTER` header and the default requeue time.
// This ensures we respect the retry-after header if it is set and avoid retrying too often during an API throttling event.
func retryAfter(sdkFuture azureautorest.FutureAPI) time.Duration {
	retryAfter, _ := sdkFuture.GetPollingDelay()
	if retryAfter < reconciler.DefaultReconcilerRequeue {
		retryAfter = reconciler.DefaultReconcilerRequeue
	}
	return retryAfter
}
