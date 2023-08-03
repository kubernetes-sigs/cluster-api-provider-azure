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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	Get(context.Context, azure.ResourceSpecGetter) (interface{}, error)
	DeleteAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
	IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	scalesetvms compute.VirtualMachineScaleSetVMsClient
}

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
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (interface{}, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesetvms.azureClient.Get")
	defer done()

	return ac.scalesetvms.Get(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), "")
}

// CreateOrUpdateAsync is a dummy implementation to fulfill the async.Reconciler interface.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "scalesets.AzureClient.CreateOrUpdateAsync")
	defer done()

	return nil, nil, nil
}

// DeleteAsync deletes a virtual machine scale set instance asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesetvms.AzureClient.DeleteAsync")
	defer done()

	deleteFuture, err := ac.scalesetvms.Delete(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), ptr.To(false))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = deleteFuture.WaitForCompletionRef(ctx, ac.scalesetvms.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &deleteFuture, err
	}
	_, err = deleteFuture.Result(ac.scalesetvms)
	// if the operation completed, return a nil future.
	return nil, err
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	return nil, nil
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesetvms.AzureClient.IsDone")
	defer done()

	return future.DoneWithContext(ctx, ac.scalesetvms)
}
