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

package scalesetvms

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	Get(context.Context, string, string, string) (compute.VirtualMachineScaleSetVM, error)
	GetResultIfDone(ctx context.Context, future *infrav1.Future) (compute.VirtualMachineScaleSetVM, error)
	DeleteAsync(context.Context, string, string, string) (*infrav1.Future, error)
}

type (
	// azureClient contains the Azure go-sdk Client.
	azureClient struct {
		scalesetvms compute.VirtualMachineScaleSetVMsClient
	}

	genericScaleSetVMFuture interface {
		DoneWithContext(ctx context.Context, sender autorest.Sender) (done bool, err error)
		Result(client compute.VirtualMachineScaleSetVMsClient) (vmss compute.VirtualMachineScaleSetVM, err error)
	}

	deleteFutureAdapter struct {
		compute.VirtualMachineScaleSetVMsDeleteFuture
	}
)

const (
	// DeleteFuture is a future that was derived from a DELETE request to VMSS.
	DeleteFuture string = "DELETE"
)

var _ client = &azureClient{}

// newClient creates a new VMSS client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	return &azureClient{
		scalesetvms: newVirtualMachineScaleSetVMsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
	}
}

// newVirtualMachineScaleSetVMsClient creates a new vmss VM client from subscription ID.
func newVirtualMachineScaleSetVMsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.VirtualMachineScaleSetVMsClient {
	c := compute.NewVirtualMachineScaleSetVMsClientWithBaseURI(baseURI, subscriptionID)
	c.Authorizer = authorizer
	c.RetryAttempts = 1
	_ = c.AddToUserAgent(azure.UserAgent()) // intentionally ignore error as it doesn't matter
	return c
}

// Get retrieves the Virtual Machine Scale Set Virtual Machine.
func (ac *azureClient) Get(ctx context.Context, resourceGroupName, vmssName, instanceID string) (compute.VirtualMachineScaleSetVM, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesetvms.azureClient.Get")
	defer span.End()

	return ac.scalesetvms.Get(ctx, resourceGroupName, vmssName, instanceID, "")
}

// GetResultIfDone fetches the result of a long-running operation future if it is done.
func (ac *azureClient) GetResultIfDone(ctx context.Context, future *infrav1.Future) (compute.VirtualMachineScaleSetVM, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesetvms.azureClient.GetResultIfDone")
	defer span.End()

	var genericFuture genericScaleSetVMFuture
	futureData, err := base64.URLEncoding.DecodeString(future.FutureData)
	if err != nil {
		return compute.VirtualMachineScaleSetVM{}, errors.Wrapf(err, "failed to base64 decode future data")
	}

	switch future.Type {
	case DeleteFuture:
		var future compute.VirtualMachineScaleSetVMsDeleteFuture
		if err := json.Unmarshal(futureData, &future); err != nil {
			return compute.VirtualMachineScaleSetVM{}, errors.Wrap(err, "failed to unmarshal future data")
		}

		genericFuture = &deleteFutureAdapter{
			VirtualMachineScaleSetVMsDeleteFuture: future,
		}
	default:
		return compute.VirtualMachineScaleSetVM{}, errors.Errorf("unknown furture type %q", future.Type)
	}

	done, err := genericFuture.DoneWithContext(ctx, ac.scalesetvms)
	if err != nil {
		return compute.VirtualMachineScaleSetVM{}, errors.Wrapf(err, "failed checking if the operation was complete")
	}

	if !done {
		return compute.VirtualMachineScaleSetVM{}, azure.WithTransientError(azure.NewOperationNotDoneError(future), 15*time.Second)
	}

	vm, err := genericFuture.Result(ac.scalesetvms)
	if err != nil {
		return vm, errors.Wrapf(err, "failed fetching the result of operation for vmss")
	}

	return vm, nil
}

// DeleteAsync is the operation to delete a virtual machine scale set instance asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
//
// Parameters:
//   resourceGroupName - the name of the resource group.
//   vmssName - the name of the VM scale set to create or update. parameters - the scale set object.
//   instanceID - the ID of the VM scale set VM.
func (ac *azureClient) DeleteAsync(ctx context.Context, resourceGroupName, vmssName, instanceID string) (*infrav1.Future, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesetvms.azureClient.DeleteAsync")
	defer span.End()

	future, err := ac.scalesetvms.Delete(ctx, resourceGroupName, vmssName, instanceID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed deleting vmss named %q", vmssName)
	}

	jsonData, err := future.MarshalJSON()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal async future")
	}

	return &infrav1.Future{
		Type:          DeleteFuture,
		ResourceGroup: resourceGroupName,
		Name:          vmssName,
		FutureData:    base64.URLEncoding.EncodeToString(jsonData),
	}, nil
}

// Result wraps the delete result so that we can treat it generically. The only thing we care about is if the delete
// was successful. If it wasn't, an error will be returned.
func (da *deleteFutureAdapter) Result(client compute.VirtualMachineScaleSetVMsClient) (compute.VirtualMachineScaleSetVM, error) {
	_, err := da.VirtualMachineScaleSetVMsDeleteFuture.Result(client)
	return compute.VirtualMachineScaleSetVM{}, err
}
