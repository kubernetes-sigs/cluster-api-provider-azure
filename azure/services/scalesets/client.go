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
	"encoding/base64"
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
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	List(context.Context, string) ([]compute.VirtualMachineScaleSet, error)
	ListInstances(context.Context, string, string) ([]compute.VirtualMachineScaleSetVM, error)
	Get(context.Context, string, string) (compute.VirtualMachineScaleSet, error)
	CreateOrUpdateAsync(context.Context, string, string, compute.VirtualMachineScaleSet) (*infrav1.Future, error)
	UpdateAsync(context.Context, string, string, compute.VirtualMachineScaleSetUpdate) (*infrav1.Future, error)
	GetResultIfDone(ctx context.Context, future *infrav1.Future) (compute.VirtualMachineScaleSet, error)
	UpdateInstances(context.Context, string, string, []string) error
	DeleteAsync(context.Context, string, string) (*infrav1.Future, error)
}

type (
	// AzureClient contains the Azure go-sdk Client.
	AzureClient struct {
		scalesetvms compute.VirtualMachineScaleSetVMsClient
		scalesets   compute.VirtualMachineScaleSetsClient
	}

	genericScaleSetFuture interface {
		DoneWithContext(ctx context.Context, sender autorest.Sender) (done bool, err error)
		Result(client compute.VirtualMachineScaleSetsClient) (vmss compute.VirtualMachineScaleSet, err error)
	}

	genericScaleSetFutureImpl struct {
		azureautorest.FutureAPI
		result func(client compute.VirtualMachineScaleSetsClient) (vmss compute.VirtualMachineScaleSet, err error)
	}

	deleteResultAdapter struct {
		compute.VirtualMachineScaleSetsDeleteFuture
	}
)

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
func (ac *AzureClient) ListInstances(ctx context.Context, resourceGroupName, vmssName string) ([]compute.VirtualMachineScaleSetVM, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.ListInstances")
	defer done()

	itr, err := ac.scalesetvms.ListComplete(ctx, resourceGroupName, vmssName, "", "", "")
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
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, vmssName string) (compute.VirtualMachineScaleSet, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.Get")
	defer done()

	return ac.scalesets.Get(ctx, resourceGroupName, vmssName, "")
}

// CreateOrUpdateAsync the operation to create or update a virtual machine scale set without waiting for the operation
// to complete.
func (ac *AzureClient) CreateOrUpdateAsync(ctx context.Context, resourceGroupName, vmssName string, vmss compute.VirtualMachineScaleSet) (*infrav1.Future, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.CreateOrUpdateAsync")
	defer done()

	future, err := ac.scalesets.CreateOrUpdate(ctx, resourceGroupName, vmssName, vmss)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.scalesets.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return converters.SDKToFuture(&future, infrav1.PutFuture, serviceName, vmssName, resourceGroupName)
	}

	// todo: this returns the result VMSS, we should use it
	_, err = future.Result(ac.scalesets)

	// if the operation completed, return a nil future.
	return nil, err
}

// UpdateAsync update a VM scale set without waiting for the result of the operation. UpdateAsync sends a PATCH
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
//
// Parameters:
//
//	resourceGroupName - the name of the resource group.
//	vmssName - the name of the VM scale set to create or update. parameters - the scale set object.
func (ac *AzureClient) UpdateAsync(ctx context.Context, resourceGroupName, vmssName string, parameters compute.VirtualMachineScaleSetUpdate) (*infrav1.Future, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.UpdateAsync")
	defer done()

	future, err := ac.scalesets.Update(ctx, resourceGroupName, vmssName, parameters)
	if err != nil {
		return nil, errors.Wrapf(err, "failed updating vmss named %q", vmssName)
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.scalesets.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return converters.SDKToFuture(&future, infrav1.PatchFuture, serviceName, vmssName, resourceGroupName)
	}
	// todo: this returns the result VMSS, we should use it
	_, err = future.Result(ac.scalesets)

	// if the operation completed, return a nil future.
	return nil, err
}

// GetResultIfDone fetches the result of a long-running operation future if it is done.
func (ac *AzureClient) GetResultIfDone(ctx context.Context, future *infrav1.Future) (compute.VirtualMachineScaleSet, error) {
	var genericFuture genericScaleSetFuture
	futureData, err := base64.URLEncoding.DecodeString(future.Data)
	if err != nil {
		return compute.VirtualMachineScaleSet{}, errors.Wrap(err, "failed to base64 decode future data")
	}

	switch future.Type {
	case infrav1.PatchFuture:
		var future compute.VirtualMachineScaleSetsUpdateFuture
		if err := json.Unmarshal(futureData, &future); err != nil {
			return compute.VirtualMachineScaleSet{}, errors.Wrap(err, "failed to unmarshal future data")
		}

		genericFuture = &genericScaleSetFutureImpl{
			FutureAPI: &future,
			result:    future.Result,
		}
	case infrav1.PutFuture:
		var future compute.VirtualMachineScaleSetsCreateOrUpdateFuture
		if err := json.Unmarshal(futureData, &future); err != nil {
			return compute.VirtualMachineScaleSet{}, errors.Wrap(err, "failed to unmarshal future data")
		}

		genericFuture = &genericScaleSetFutureImpl{
			FutureAPI: &future,
			result:    future.Result,
		}
	case infrav1.DeleteFuture:
		var future compute.VirtualMachineScaleSetsDeleteFuture
		if err := json.Unmarshal(futureData, &future); err != nil {
			return compute.VirtualMachineScaleSet{}, errors.Wrap(err, "failed to unmarshal future data")
		}

		genericFuture = &deleteResultAdapter{
			VirtualMachineScaleSetsDeleteFuture: future,
		}
	default:
		return compute.VirtualMachineScaleSet{}, errors.Errorf("unknown future type %q", future.Type)
	}

	done, err := genericFuture.DoneWithContext(ctx, ac.scalesets)
	if err != nil {
		return compute.VirtualMachineScaleSet{}, errors.Wrap(err, "failed checking if the operation was complete")
	}

	if !done {
		return compute.VirtualMachineScaleSet{}, azure.WithTransientError(azure.NewOperationNotDoneError(future), 15*time.Second)
	}

	vmss, err := genericFuture.Result(ac.scalesets)
	if err != nil {
		return vmss, errors.Wrap(err, "failed fetching the result of operation for vmss")
	}

	return vmss, nil
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
//	resourceGroupName - the name of the resource group.
//	vmssName - the name of the VM scale set to create or update. parameters - the scale set object.
func (ac *AzureClient) DeleteAsync(ctx context.Context, resourceGroupName, vmssName string) (*infrav1.Future, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.DeleteAsync")
	defer done()

	future, err := ac.scalesets.Delete(ctx, resourceGroupName, vmssName, ptr.To(false))
	if err != nil {
		return nil, errors.Wrapf(err, "failed deleting vmss named %q", vmssName)
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.scalesets.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return converters.SDKToFuture(&future, infrav1.DeleteFuture, serviceName, vmssName, resourceGroupName)
	}
	_, err = future.Result(ac.scalesets)

	// if the operation completed, return a nil future.
	return nil, err
}

// Result wraps the delete result so that we can treat it generically. The only thing we care about is if the delete
// was successful. If it wasn't, an error will be returned.
func (da *deleteResultAdapter) Result(client compute.VirtualMachineScaleSetsClient) (compute.VirtualMachineScaleSet, error) {
	_, err := da.VirtualMachineScaleSetsDeleteFuture.Result(client)
	return compute.VirtualMachineScaleSet{}, err
}

// Result returns the Result so that we can treat it generically.
func (g *genericScaleSetFutureImpl) Result(client compute.VirtualMachineScaleSetsClient) (compute.VirtualMachineScaleSet, error) {
	return g.result(client)
}
