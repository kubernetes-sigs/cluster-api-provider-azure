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

package virtualmachines

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	// AzureClient contains the Azure go-sdk Client.
	AzureClient struct {
		virtualmachines compute.VirtualMachinesClient
	}

	// Client provides operations on Azure virtual machine resources.
	Client interface {
		Get(context.Context, azure.ResourceSpecGetter) (interface{}, error)
		GetByID(context.Context, string) (compute.VirtualMachine, error)
		CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error)
		DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error)
		IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error)
		Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error)
		GetResultIfDone(ctx context.Context, future *infrav1.Future) (compute.VirtualMachine, error)
	}
)

type genericVMFuture interface {
	DoneWithContext(ctx context.Context, sender autorest.Sender) (done bool, err error)
	Result(client compute.VirtualMachinesClient) (vm compute.VirtualMachine, err error)
}

type deleteFutureAdapter struct {
	compute.VirtualMachinesDeleteFuture
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newVirtualMachinesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newVirtualMachinesClient creates a new VM client from subscription ID.
func newVirtualMachinesClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.VirtualMachinesClient {
	vmClient := compute.NewVirtualMachinesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&vmClient.Client, authorizer)
	return vmClient
}

// Get retrieves information about the model view or the instance view of a virtual machine.
func (ac *AzureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.Get")
	defer done()

	return ac.virtualmachines.Get(ctx, spec.ResourceGroupName(), spec.ResourceName(), "")
}

// GetByID retrieves information about the model or instance view of a virtual machine.
func (ac *AzureClient) GetByID(ctx context.Context, resourceID string) (compute.VirtualMachine, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.GetByID")
	defer done()

	parsed, err := azureautorest.ParseResourceID(resourceID)
	if err != nil {
		return compute.VirtualMachine{}, errors.Wrap(err, fmt.Sprintf("failed parsing the VM resource id %q", resourceID))
	}

	log.V(4).Info("parsed VM resourceID", "parsed", parsed)

	return ac.virtualmachines.Get(ctx, parsed.ResourceGroup, parsed.ResourceName, "")
}

// CreateOrUpdateAsync creates or updates a virtual machine asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.CreateOrUpdate")
	defer done()

	vm, ok := parameters.(compute.VirtualMachine)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a compute.VirtualMachine", parameters)
	}

	createFuture, err := ac.virtualmachines.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), vm)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = createFuture.WaitForCompletionRef(ctx, ac.virtualmachines.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &createFuture, err
	}
	result, err = createFuture.Result(ac.virtualmachines)
	// if the operation completed, return a nil future
	return result, nil, err
}

// DeleteAsync deletes a virtual machine asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.Delete")
	defer done()

	forceDelete := pointer.Bool(true)
	deleteFuture, err := ac.virtualmachines.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName(), forceDelete)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = deleteFuture.WaitForCompletionRef(ctx, ac.virtualmachines.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &deleteFuture, err
	}
	_, err = deleteFuture.Result(ac.virtualmachines)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *AzureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.IsDone")
	defer done()

	return future.DoneWithContext(ctx, ac.virtualmachines)
}

// Result fetches the result of a long-running operation future.
func (ac *AzureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.Result")
	defer done()

	if future == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}

	switch futureType {
	case infrav1.PatchFuture:
		// Marshal and Unmarshal the future to put it into the correct future type so we can access the Result function.
		// Unfortunately the FutureAPI can't be casted directly to VirtualMachinesUpdateFuture because it is a azureautorest.Future, which doesn't implement the Result function. See PR #1686 for discussion on alternatives.
		// It was converted back to a generic azureautorest.Future from the CAPZ infrav1.Future type stored in Status: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/azure/converters/futures.go#L49.
		var updateFuture *compute.VirtualMachinesUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &updateFuture); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return updateFuture.Result(ac.virtualmachines)

	case infrav1.PutFuture:
		// Marshal and Unmarshal the future to put it into the correct future type so we can access the Result function.
		// Unfortunately the FutureAPI can't be casted directly to VirtualMachinesCreateOrUpdateFuture because it is a azureautorest.Future, which doesn't implement the Result function. See PR #1686 for discussion on alternatives.
		// It was converted back to a generic azureautorest.Future from the CAPZ infrav1.Future type stored in Status: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/azure/converters/futures.go#L49.
		var createFuture *compute.VirtualMachinesCreateOrUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &createFuture); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return createFuture.Result(ac.virtualmachines)

	case infrav1.DeleteFuture:
		// Delete does not return a result VM.
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}
}

// GetResultIfDone fetches the result of a long-running operation future if it is done.
func (ac *AzureClient) GetResultIfDone(ctx context.Context, future *infrav1.Future) (compute.VirtualMachine, error) {
	ctx, _, spanDone := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.GetResultIfDone")
	defer spanDone()

	var genericFuture genericVMFuture
	futureData, err := base64.URLEncoding.DecodeString(future.Data)
	if err != nil {
		return compute.VirtualMachine{}, errors.Wrapf(err, "failed to base64 decode future data")
	}

	switch future.Type {
	case infrav1.DeleteFuture:
		var future compute.VirtualMachinesDeleteFuture
		if err := json.Unmarshal(futureData, &future); err != nil {
			return compute.VirtualMachine{}, errors.Wrap(err, "failed to unmarshal future data")
		}

		genericFuture = &deleteFutureAdapter{
			VirtualMachinesDeleteFuture: future,
		}
	default:
		return compute.VirtualMachine{}, errors.Errorf("unknown future type %q", future.Type)
	}

	done, err := genericFuture.DoneWithContext(ctx, ac.virtualmachines)
	if err != nil {
		return compute.VirtualMachine{}, errors.Wrapf(err, "failed checking if the operation was complete")
	}

	if !done {
		return compute.VirtualMachine{}, azure.WithTransientError(azure.NewOperationNotDoneError(future), 15*time.Second)
	}

	vm, err := genericFuture.Result(ac.virtualmachines)
	if err != nil {
		return vm, errors.Wrapf(err, "failed fetching the result of operation for vm")
	}

	return vm, nil
}

// Result wraps result of a delete so it can be treated generically, when only the success or error is important.
func (da *deleteFutureAdapter) Result(client compute.VirtualMachinesClient) (compute.VirtualMachine, error) {
	_, err := da.VirtualMachinesDeleteFuture.Result(client)
	return compute.VirtualMachine{}, err
}
