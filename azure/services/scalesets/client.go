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

package scalesets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	Get(context.Context, azure.ResourceSpecGetter) (interface{}, error)
	List(context.Context, string) ([]compute.VirtualMachineScaleSet, error)
	ListInstances(context.Context, string, string) ([]compute.VirtualMachineScaleSetVM, error)
	UpdateInstances(context.Context, string, string, []string) error

	CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error)
	DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error)
	IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error)
	Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error)
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	scalesetvms compute.VirtualMachineScaleSetVMsClient
	scalesets   compute.VirtualMachineScaleSetsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VMSS client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	return &AzureClient{
		scalesetvms: newVirtualMachineScaleSetVMsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
		scalesets:   newVirtualMachineScaleSetsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
	}
}

// newVirtualMachineScaleSetVMsClient creates a new vmss VM client from subscription ID.
func newVirtualMachineScaleSetVMsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.VirtualMachineScaleSetVMsClient {
	c := compute.NewVirtualMachineScaleSetVMsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&c.Client, authorizer)
	return c
}

// newVirtualMachineScaleSetsClient creates a new vmss client from subscription ID.
func newVirtualMachineScaleSetsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.VirtualMachineScaleSetsClient {
	c := compute.NewVirtualMachineScaleSetsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&c.Client, authorizer)
	return c
}

// ListInstances retrieves information about the model views of a virtual machine scale set.
func (ac *AzureClient) ListInstances(ctx context.Context, resourceGroupName string, resourceName string) ([]compute.VirtualMachineScaleSetVM, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.ListInstances")
	defer done()

	itr, err := ac.scalesetvms.ListComplete(ctx, resourceGroupName, resourceName, "", "", "")
	if err != nil {
		return nil, err
	}

	var instances []compute.VirtualMachineScaleSetVM
	for ; itr.NotDone(); err = itr.NextWithContext(ctx) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate vm scale set vms [%w]", err)
		}
		vm := itr.Value()
		instances = append(instances, vm)
	}
	return instances, nil
}

// List returns all scale sets in a resource group.
func (ac *AzureClient) List(ctx context.Context, resourceGroupName string) ([]compute.VirtualMachineScaleSet, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.List")
	defer done()

	itr, err := ac.scalesets.ListComplete(ctx, resourceGroupName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list scalesets in the resource group")
	}

	var instances []compute.VirtualMachineScaleSet
	for ; itr.NotDone(); err = itr.NextWithContext(ctx) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate vm scale sets [%w]", err)
		}
		vm := itr.Value()
		instances = append(instances, vm)
	}
	return instances, nil
}

// Get retrieves information about the model view of a virtual machine scale set.
func (ac *AzureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (interface{}, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.Get")
	defer done()

	return ac.scalesets.Get(ctx, spec.ResourceGroupName(), spec.ResourceName(), "")
}

// CreateOrUpdateAsync creates or updates a virtual machine scale set asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.CreateOrUpdateAsync")
	defer done()

	scaleset, ok := parameters.(compute.VirtualMachineScaleSet)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a compute.VirtualMachineScaleSet", parameters)
	}

	createFuture, err := ac.scalesets.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), scaleset)
	if err != nil {
		fmt.Printf("Willie: error: %v\n", err)
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	err = createFuture.WaitForCompletionRef(ctx, ac.scalesets.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		fmt.Printf("Willie: error2: %v\n", err)
		return nil, &createFuture, err
	}

	result, err = createFuture.Result(ac.scalesets)
	fmt.Printf("Willie: result: %v\n", result)
	// if the operation completed, return a nil future
	return result, nil, err
}

// UpdateInstances update instances of a VM scale set.
func (ac *AzureClient) UpdateInstances(ctx context.Context, resourceGroupName, vmssName string, instanceIDs []string) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.UpdateInstances")
	defer done()

	params := compute.VirtualMachineScaleSetVMInstanceRequiredIDs{
		InstanceIds: &instanceIDs,
	}
	future, err := ac.scalesets.UpdateInstances(ctx, resourceGroupName, vmssName, params)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.scalesets.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.scalesets)
	return err
}

// DeleteAsync is the operation to delete a virtual machine scale set asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
//
// Parameters:
//
//	spec - The ResourceSpecGetter containing used for name and resource group of the virtual machine scale set.
func (ac *AzureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.DeleteAsync")
	defer done()

	deleteFuture, err := ac.scalesets.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName(), ptr.To(false))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = deleteFuture.WaitForCompletionRef(ctx, ac.scalesets.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &deleteFuture, err
	}
	_, err = deleteFuture.Result(ac.scalesets)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *AzureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.IsDone")
	defer done()

	return future.DoneWithContext(ctx, ac.scalesets)
}

// Result fetches the result of a long-running operation future.
func (ac *AzureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.Result")
	defer done()

	if future == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}

	switch futureType {
	case infrav1.PatchFuture:
		// Marshal and Unmarshal the future to put it into the correct future type so we can access the Result function.
		// Unfortunately the FutureAPI can't be casted directly to VirtualMachineScaleSetsUpdateFuture because it is a azureautorest.Future, which doesn't implement the Result function. See PR #1686 for discussion on alternatives.
		// It was converted back to a generic azureautorest.Future from the CAPZ infrav1.Future type stored in Status: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/azure/converters/futures.go#L49.
		var updateFuture *compute.VirtualMachineScaleSetsUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &updateFuture); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return updateFuture.Result(ac.scalesets)

	case infrav1.PutFuture:
		// Marshal and Unmarshal the future to put it into the correct future type so we can access the Result function.
		// Unfortunately the FutureAPI can't be casted directly to VirtualMachineScaleSetsCreateOrUpdateFuture because it is a azureautorest.Future, which doesn't implement the Result function. See PR #1686 for discussion on alternatives.
		// It was converted back to a generic azureautorest.Future from the CAPZ infrav1.Future type stored in Status: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/azure/converters/futures.go#L49.
		var createFuture *compute.VirtualMachineScaleSetsCreateOrUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &createFuture); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return createFuture.Result(ac.scalesets)

	case infrav1.DeleteFuture:
		// Delete does not return a result compute.VirtualMachineScaleSet
		return nil, nil
	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}
}
