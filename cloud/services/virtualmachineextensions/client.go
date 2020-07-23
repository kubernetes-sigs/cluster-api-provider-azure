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

package virtualmachineextensions

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string, string, string) (compute.VirtualMachineExtension, error)
	CreateOrUpdate(context.Context, string, string, string, compute.VirtualMachineExtension) error
	Delete(context.Context, string, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	vmextensions compute.VirtualMachineExtensionsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(subscriptionID string, auth azure.Authorizer) *AzureClient {
	c := newVirtualMachineExtensionsClient(subscriptionID, auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newVirtualMachineExtensionsClient creates a new VM extension client from subscription ID.
func newVirtualMachineExtensionsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.VirtualMachineExtensionsClient {
	vmExtClient := compute.NewVirtualMachineExtensionsClientWithBaseURI(baseURI, subscriptionID)
	vmExtClient.Authorizer = authorizer
	vmExtClient.AddToUserAgent(azure.UserAgent())
	return vmExtClient
}

// Get the operation to get the extension.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, vmName, extName string) (compute.VirtualMachineExtension, error) {
	return ac.vmextensions.Get(ctx, resourceGroupName, vmName, extName, "")
}

// CreateOrUpdate the operation to create or update the extension.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, vmName, extName string, ext compute.VirtualMachineExtension) error {
	future, err := ac.vmextensions.CreateOrUpdate(ctx, resourceGroupName, vmName, extName, ext)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.vmextensions.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.vmextensions)
	return err
}

// Delete the operation to delete the extension.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, vmName, extName string) error {
	future, err := ac.vmextensions.Delete(ctx, resourceGroupName, vmName, extName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.vmextensions.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.vmextensions)
	return err
}
