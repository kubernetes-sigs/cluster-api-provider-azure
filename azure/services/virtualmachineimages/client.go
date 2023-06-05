/*
Copyright 2022 The Kubernetes Authors.

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

package virtualmachineimages

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client is an interface for listing VM images.
type Client interface {
	List(ctx context.Context, location, publisher, offer, sku string) (compute.ListVirtualMachineImageResource, error)
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	images compute.VirtualMachineImagesClient
}

var _ Client = (*AzureClient)(nil)

// NewClient creates a new VM images client from auth info.
func NewClient(auth azure.Authorizer) *AzureClient {
	return &AzureClient{
		images: newVirtualMachineImagesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
	}
}

// newVirtualMachineImagesClient creates a new VM images client from subscription ID, base URI and authorizer.
func newVirtualMachineImagesClient(subscriptionID, baseURI string, authorizer autorest.Authorizer) compute.VirtualMachineImagesClient {
	c := compute.NewVirtualMachineImagesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&c.Client, authorizer)
	return c
}

// List returns a VM image list resource.
func (ac *AzureClient) List(ctx context.Context, location, publisher, offer, sku string) (compute.ListVirtualMachineImageResource, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachineimages.AzureClient.List")
	defer done()

	// See https://docs.microsoft.com/en-us/odata/concepts/queryoptions-overview for how to use these query options.
	expand, orderby := "", ""
	var top *int32
	return ac.images.List(ctx, location, publisher, offer, sku, expand, top, orderby)
}
