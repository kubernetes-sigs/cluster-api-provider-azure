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
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type (
	Client interface {
		Get(context.Context, string, string) (compute.VirtualMachine, error)
		CreateOrUpdateAsync(context.Context, azure.ResourceSpecGetter) (interface{}, azureautorest.FutureAPI, error)
		DeleteAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
		IsDone(context.Context, azureautorest.FutureAPI) (bool, error)
		Result(context.Context, azureautorest.FutureAPI, string) (interface{}, error)
	}

	// AzureClient contains the Azure go-sdk Client.
	AzureClient struct {
		virtualmachines compute.VirtualMachinesClient
	}
)

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
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, vmName string) (compute.VirtualMachine, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.Get")
	defer done()

	return ac.virtualmachines.Get(ctx, resourceGroupName, vmName, "")
}

// CreateOrUpdateAsync creates or updates a virtual machine asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter) (interface{}, azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.CreateOrUpdate")
	defer done()

	vm, err := ac.vmParams(ctx, spec)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get desired parameters for virtual machine %s", spec.ResourceName())
	} else if vm == nil {
		// nothing to do here.
		return nil, nil, nil
	}

	future, err := ac.virtualmachines.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), *vm)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.virtualmachines.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &future, err
	}
	result, err := future.Result(ac.virtualmachines)
	// if the operation completed, return a nil future
	return result, nil, err
}

// DeleteAsync deletes a virtual machine asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.AzureClient.Delete")
	defer done()

	// TODO: pass variable to force the deletion or not
	// now we are not forcing.
	future, err := ac.virtualmachines.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName(), to.BoolPtr(false))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.virtualmachines.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &future, err
	}
	_, err = future.Result(ac.virtualmachines)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *AzureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, span := tele.Tracer().Start(ctx, "virtualmachines.AzureClient.IsDone")
	defer span.End()

	done, err := future.DoneWithContext(ctx, ac.virtualmachines)
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
	var result func(client compute.VirtualMachinesClient) (VM compute.VirtualMachine, err error)

	switch futureType {
	case infrav1.PatchFuture:
		var future *compute.VirtualMachinesUpdateFuture
		jsonData, err := futureData.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &future); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		result = (*future).Result

	case infrav1.PutFuture:
		var future *compute.VirtualMachinesCreateOrUpdateFuture
		jsonData, err := futureData.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &future); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		result = (*future).Result

	case infrav1.DeleteFuture:
		// Delete does not return a result VM.
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}

	return result(ac.virtualmachines)
}

// vmParams creates a VirtualMachine object from the given spec.
func (ac *AzureClient) vmParams(ctx context.Context, spec azure.ResourceSpecGetter) (*compute.VirtualMachine, error) {
	ctx, span := tele.Tracer().Start(ctx, "virtualmachines.AzureClient.vmParams")
	defer span.End()

	var params interface{}
	var existing interface{}

	if existingVM, err := ac.Get(ctx, spec.ResourceGroupName(), spec.ResourceName()); err != nil && !azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "failed to get virtual machine %s in %s", spec.ResourceName(), spec.ResourceGroupName())
	} else if err == nil {
		// virtual machine already exists
		existing = existingVM
	}

	params, err := spec.Parameters(existing)
	if err != nil {
		return nil, err
	}

	vm, ok := params.(compute.VirtualMachine)
	if !ok {
		if params == nil {
			// nothing to do here.
			return nil, nil
		}
		return nil, errors.Errorf("%T is not a compute.VirtualMachine", params)
	}

	return &vm, nil
}
