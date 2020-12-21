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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-11-01/network"
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk
type Client interface {
	List(context.Context, string) ([]compute.VirtualMachineScaleSet, error)
	ListInstances(context.Context, string, string) ([]compute.VirtualMachineScaleSetVM, error)
	Get(context.Context, string, string) (compute.VirtualMachineScaleSet, error)
	CreateOrUpdate(context.Context, string, string, compute.VirtualMachineScaleSet) error
	Update(context.Context, string, string, compute.VirtualMachineScaleSetUpdate) error
	UpdateInstances(context.Context, string, string, []string) error
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
func NewClient(auth azure.Authorizer) *AzureClient {
	return &AzureClient{
		scalesetvms: newVirtualMachineScaleSetVMsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
		scalesets:   newVirtualMachineScaleSetsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
		publicIPs:   newPublicIPsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
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

// newPublicIPsClient creates a new publicIPs client from subscription ID.
func newPublicIPsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.PublicIPAddressesClient {
	c := network.NewPublicIPAddressesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&c.Client, authorizer)
	return c
}

// ListInstances retrieves information about the model views of a virtual machine scale set.
func (ac *AzureClient) ListInstances(ctx context.Context, resourceGroupName, vmssName string) ([]compute.VirtualMachineScaleSetVM, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.AzureClient.ListInstances")
	defer span.End()

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
	ctx, span := tele.Tracer().Start(ctx, "scalesets.AzureClient.List")
	defer span.End()

	itr, err := ac.scalesets.ListComplete(ctx, resourceGroupName)
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
	ctx, span := tele.Tracer().Start(ctx, "scalesets.AzureClient.Get")
	defer span.End()

	return ac.scalesets.Get(ctx, resourceGroupName, vmssName)
}

// CreateOrUpdate the operation to create or update a virtual machine scale set.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, vmssName string, vmss compute.VirtualMachineScaleSet) error {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.AzureClient.CreateOrUpdate")
	defer span.End()

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

// Update update a VM scale set.
// Parameters: resourceGroupName - the name of the resource group. VMScaleSetName - the name of the VM scale set to create or update. parameters - the scale set object.
func (ac *AzureClient) Update(ctx context.Context, resourceGroupName, vmssName string, parameters compute.VirtualMachineScaleSetUpdate) error {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.AzureClient.Update")
	defer span.End()

	future, err := ac.scalesets.Update(ctx, resourceGroupName, vmssName, parameters)
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

// UpdateInstances update instances of a VM scale set.
func (ac *AzureClient) UpdateInstances(ctx context.Context, resourceGroupName, vmssName string, instanceIDs []string) error {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.AzureClient.UpdateInstances")
	defer span.End()

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

// Delete the operation to delete a virtual machine scale set.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, vmssName string) error {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.AzureClient.Delete")
	defer span.End()

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

// GetPublicIPAddress gets the public IP address for the given public IP name.
func (ac *AzureClient) GetPublicIPAddress(ctx context.Context, resourceGroupName, publicIPName string) (network.PublicIPAddress, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.AzureClient.GetPublicIPAddress")
	defer span.End()

	return ac.publicIPs.Get(ctx, resourceGroupName, publicIPName, "true")
}
