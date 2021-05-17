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

	//"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/2018-03-01/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest"
	azure "github.com/niachary/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string, string) (compute.VirtualMachine, error)
	CreateOrUpdate(context.Context, string, string, compute.VirtualMachine) error
	Delete(context.Context, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	virtualmachines compute.VirtualMachinesClient
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
	vmClient.Authorizer = authorizer
	vmClient.AddToUserAgent(azure.UserAgent())
	return vmClient
}

// Get retrieves information about the model view or the instance view of a virtual machine.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, vmName string) (compute.VirtualMachine, error) {
	return ac.virtualmachines.Get(ctx, resourceGroupName, vmName, "")
}

// CreateOrUpdate the operation to create or update a virtual machine.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, vmName string, vm compute.VirtualMachine) error {
	future, err := ac.virtualmachines.CreateOrUpdate(ctx, resourceGroupName, vmName, vm)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.virtualmachines.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.virtualmachines)
	return err
}

// Delete the operation to delete a virtual machine.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, vmName string) error {
	future, err := ac.virtualmachines.Delete(ctx, resourceGroupName, vmName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.virtualmachines.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.virtualmachines)
	return err
}
