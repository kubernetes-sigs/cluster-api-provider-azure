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
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-11-01/network"
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	ListInstances(context.Context, string, string) ([]compute.VirtualMachineScaleSetVM, error)
	Get(context.Context, string, string) (compute.VirtualMachineScaleSet, error)
	CreateOrUpdate(context.Context, string, string, compute.VirtualMachineScaleSet) error
	Delete(context.Context, string, string) error
	GetPublicIPAddress(context.Context, string, string) (network.PublicIPAddress, error)
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	scalesetvms compute.VirtualMachineScaleSetVMsClient
	scalesets   compute.VirtualMachineScaleSetsClient
	publicIPs   network.PublicIPAddressesClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VMSS client from subscription ID.
func NewClient(subscriptionID string, authorizer autorest.Authorizer) *AzureClient {
	return &AzureClient{
		scalesetvms: newVirtualMachineScaleSetVMsClient(subscriptionID, authorizer),
		scalesets:   newVirtualMachineScaleSetsClient(subscriptionID, authorizer),
		publicIPs:   newPublicIPsClient(subscriptionID, authorizer),
	}
}

// newVirtualMachineScaleSetVMsClient creates a new vmss VM client from subscription ID.
func newVirtualMachineScaleSetVMsClient(subscriptionID string, authorizer autorest.Authorizer) compute.VirtualMachineScaleSetVMsClient {
	c := compute.NewVirtualMachineScaleSetVMsClient(subscriptionID)
	c.Authorizer = authorizer
	_ = c.AddToUserAgent(azure.UserAgent) // intentionally ignore error as it doesn't matter
	return c
}

// newVirtualMachineScaleSetsClient creates a new vmss client from subscription ID.
func newVirtualMachineScaleSetsClient(subscriptionID string, authorizer autorest.Authorizer) compute.VirtualMachineScaleSetsClient {
	c := compute.NewVirtualMachineScaleSetsClient(subscriptionID)
	c.Authorizer = authorizer
	_ = c.AddToUserAgent(azure.UserAgent) // intentionally ignore error as it doesn't matter
	return c
}

// newPublicIPsClient creates a new publicIPs client from subscription ID.
func newPublicIPsClient(subscriptionID string, authorizer autorest.Authorizer) network.PublicIPAddressesClient {
	c := network.NewPublicIPAddressesClient(subscriptionID)
	c.Authorizer = authorizer
	_ = c.AddToUserAgent(azure.UserAgent) // intentionally ignore error as it doesn't matter
	return c
}

// Get retrieves information about the model view of a virtual machine scale set.
func (ac *AzureClient) ListInstances(ctx context.Context, resourceGroupName, vmssName string) ([]compute.VirtualMachineScaleSetVM, error) {
	itr, err := ac.scalesetvms.ListComplete(ctx, resourceGroupName, vmssName, "", "", "")
	if err != nil {
		return nil, err
	}

	var instances []compute.VirtualMachineScaleSetVM
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
	return ac.scalesets.Get(ctx, resourceGroupName, vmssName)
}

// CreateOrUpdate the operation to create or update a virtual machine scale set.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, vmssName string, vmss compute.VirtualMachineScaleSet) error {
	future, err := ac.scalesets.CreateOrUpdate(ctx, resourceGroupName, vmssName, vmss)
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

// Delete the operation to delete a virtual machine scale set.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, vmssName string) error {
	future, err := ac.scalesets.Delete(ctx, resourceGroupName, vmssName)
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

func (ac *AzureClient) GetPublicIPAddress(ctx context.Context, resourceGroupName, publicIPName string) (network.PublicIPAddress, error) {
	return ac.publicIPs.Get(ctx, resourceGroupName, publicIPName, "true")
}
