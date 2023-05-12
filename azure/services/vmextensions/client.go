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

package vmextensions

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	vmextensions armcompute.VirtualMachineExtensionsClient
}

// newClient creates an azureClient from an Authorizer.
func newClient(auth azure.Authorizer) (*azureClient, error) {
	c, err := newVirtualMachineExtensionsClient(auth.SubscriptionID(), auth.CloudEnvironment())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new VM extensions client")
	}
	return &azureClient{c}, nil
}

// newVirtualMachineExtensionsClient creates a new VM extensions client from subscription ID and Azure cloud environment name.
func newVirtualMachineExtensionsClient(subscriptionID, azureEnvironment string) (armcompute.VirtualMachineExtensionsClient, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return armcompute.VirtualMachineExtensionsClient{}, errors.Wrap(err, "failed to create default Azure credential")
	}
	opts, err := azure.ARMClientOptions(azureEnvironment)
	if err != nil {
		return armcompute.VirtualMachineExtensionsClient{}, errors.Wrap(err, "failed to create ARM client options")
	}
	factory, err := armcompute.NewClientFactory(subscriptionID, cred, opts)
	if err != nil {
		return armcompute.VirtualMachineExtensionsClient{}, errors.Wrap(err, "failed to create ARM compute client factory")
	}
	return *factory.NewVirtualMachineExtensionsClient(), nil
}

// Get the specified virtual machine extension.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "vmextensions.AzureClient.Get")
	defer done()

	opts := &armcompute.VirtualMachineExtensionsClientGetOptions{}
	resp, err := ac.vmextensions.Get(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), opts)
	if err != nil {
		return nil, err
	}
	return resp.VirtualMachineExtension, nil
}

// CreateOrUpdateAsync creates or updates a VM extension asynchronously.
// It sends a PUT request to Azure and if accepted without error, the func will return a Poller which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string, parameters interface{}) (result interface{}, poller *runtime.Poller[armcompute.VirtualMachineExtensionsClientCreateOrUpdateResponse], err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "vmextensions.AzureClient.CreateOrUpdateAsync")
	defer done()

	var vmExtension armcompute.VirtualMachineExtension
	if parameters != nil {
		vme, ok := parameters.(armcompute.VirtualMachineExtension)
		if !ok {
			return nil, nil, errors.Errorf("%T is not an armcompute.VirtualMachineExtension", parameters)
		}
		vmExtension = vme
	}

	opts := &armcompute.VirtualMachineExtensionsClientBeginCreateOrUpdateOptions{ResumeToken: resumeToken}
	log.V(4).Info("sending request", "resumeToken", resumeToken)
	poller, err = ac.vmextensions.BeginCreateOrUpdate(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), vmExtension, opts)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	result, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
	if err != nil {
		// if an error occurs, return the poller.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, poller, err
	}

	// if the operation completed, return a nil poller
	return result, nil, err
}

// DeleteAsync deletes a VM extension asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Poller which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string) (poller *runtime.Poller[armcompute.VirtualMachineExtensionsClientDeleteResponse], err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "vmextensions.AzureClient.DeleteAsync")
	defer done()

	opts := &armcompute.VirtualMachineExtensionsClientBeginDeleteOptions{ResumeToken: resumeToken}
	log.V(4).Info("sending request", "resumeToken", resumeToken)
	poller, err = ac.vmextensions.BeginDelete(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), spec.ResourceName(), opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
	if err != nil {
		// if an error occurs, return the poller.
		// this means the long-running operation didn't finish in the specified timeout.
		return poller, err
	}

	// if the operation completed, return a nil poller.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, poller interface{}) (isDone bool, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "virtualnetworks.azureClient.IsDone")
	defer done()

	switch t := poller.(type) {
	case *runtime.Poller[armcompute.VirtualMachineExtensionsClientCreateOrUpdateResponse]:
		c, _ := poller.(*runtime.Poller[armcompute.VirtualMachineExtensionsClientCreateOrUpdateResponse])
		return c.Done(), nil
	case *runtime.Poller[armcompute.VirtualMachineExtensionsClientDeleteResponse]:
		d, _ := poller.(*runtime.Poller[armcompute.VirtualMachineExtensionsClientDeleteResponse])
		return d.Done(), nil
	default:
		return false, errors.Errorf("unexpected poller type %T", t)
	}
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, poller interface{}) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "vmextensions.azureClient.Result")
	defer done()

	switch t := poller.(type) {
	case *runtime.Poller[armcompute.VirtualMachineExtensionsClientCreateOrUpdateResponse]:
		c, _ := poller.(*runtime.Poller[armcompute.VirtualMachineExtensionsClientCreateOrUpdateResponse])
		return c.Result(ctx)
	case *runtime.Poller[armcompute.VirtualMachineExtensionsClientDeleteResponse]:
		d, _ := poller.(*runtime.Poller[armcompute.VirtualMachineExtensionsClientDeleteResponse])
		return d.Result(ctx)
	default:
		return false, errors.Errorf("unexpected poller type %T", t)
	}
}
