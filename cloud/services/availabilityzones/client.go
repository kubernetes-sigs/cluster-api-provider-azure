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

package availabilityzones

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	ListComplete(context.Context, string) (compute.ResourceSkusResultIterator, error)
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	resourceSkus compute.ResourceSkusClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(subscriptionID string, authorizer autorest.Authorizer) *AzureClient {
	c := newResourceSkusClient(subscriptionID, authorizer)
	return &AzureClient{c}
}

// getResourceSkusClient creates a new availability zones client from subscription ID.
func newResourceSkusClient(subscriptionID string, authorizer autorest.Authorizer) compute.ResourceSkusClient {
	skusClient := compute.NewResourceSkusClient(subscriptionID)
	skusClient.Authorizer = authorizer
	skusClient.AddToUserAgent(azure.UserAgent)
	return skusClient
}

// ListComplete enumerates all values, automatically crossing page boundaries as required.
func (ac *AzureClient) ListComplete(ctx context.Context, filter string) (compute.ResourceSkusResultIterator, error) {
	return ac.resourceSkus.ListComplete(ctx, filter)
}
