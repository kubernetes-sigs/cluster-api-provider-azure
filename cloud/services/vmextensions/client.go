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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk
type client interface {
	CreateOrUpdate(context.Context, string, string, string, compute.VirtualMachineExtension) error
	Delete(context.Context, string, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type azureClient struct {
	vmextensions compute.VirtualMachineExtensionsClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newVirtualMachineExtensionsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newVirtualMachineExtensionsClient creates a new vm extension client from subscription ID.
func newVirtualMachineExtensionsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.VirtualMachineExtensionsClient {
	vmextensionsClient := compute.NewVirtualMachineExtensionsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&vmextensionsClient.Client, authorizer)
	return vmextensionsClient
}

// CreateOrUpdate creates or updates the virtual machine extension
func (ac *azureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, vmName, name string, parameters compute.VirtualMachineExtension) error {
	ctx, span := tele.Tracer().Start(ctx, "vmextensions.AzureClient.CreateOrUpdate")
	defer span.End()

	future, err := ac.vmextensions.CreateOrUpdate(ctx, resourceGroupName, vmName, name, parameters)
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

// Delete removes the virtual machine extension.
func (ac *azureClient) Delete(ctx context.Context, resourceGroupName, vmName, name string) error {
	ctx, span := tele.Tracer().Start(ctx, "vmextensions.AzureClient.Delete")
	defer span.End()

	future, err := ac.vmextensions.Delete(ctx, resourceGroupName, vmName, name)
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
