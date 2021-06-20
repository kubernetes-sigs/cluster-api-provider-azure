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

package vmssextensions

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type client interface {
	Get(context.Context, string, string, string) (compute.VirtualMachineScaleSetExtension, error)
}

// AzureClient contains the Azure go-sdk Client.
type azureClient struct {
	vmssextensions compute.VirtualMachineScaleSetExtensionsClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VMSS client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newVirtualMachineScaleSetExtensionsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newVirtualMachineScaleSetExtensionsClient creates a new vmss extension client from subscription ID.
func newVirtualMachineScaleSetExtensionsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.VirtualMachineScaleSetExtensionsClient {
	vmssextensionsClient := compute.NewVirtualMachineScaleSetExtensionsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&vmssextensionsClient.Client, authorizer)
	return vmssextensionsClient
}

// Get creates or updates the virtual machine scale set extension.
func (ac *azureClient) Get(ctx context.Context, resourceGroupName, vmssName, name string) (compute.VirtualMachineScaleSetExtension, error) {
	ctx, span := tele.Tracer().Start(ctx, "vmssextensions.AzureClient.Get")
	defer span.End()

	return ac.vmssextensions.Get(ctx, resourceGroupName, vmssName, name, "")
}
