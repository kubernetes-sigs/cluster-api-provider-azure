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

package availabilitysets

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk
type Client interface {
	Get(ctx context.Context, resourceGroup, availabilitySetsName string) (compute.AvailabilitySet, error)
	CreateOrUpdate(ctx context.Context, resourceGroup, availabilitySetsName string, params compute.AvailabilitySet) (compute.AvailabilitySet, error)
	Delete(ctx context.Context, resourceGroup, availabilitySetsName string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	availabilitySets compute.AvailabilitySetsClient
}

var _ Client = (*AzureClient)(nil)

// NewClient creates a new Resource SKUs Client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	return &AzureClient{
		availabilitySets: newAvailabilitySetsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
	}
}

// newAvailabilitySetsClient creates a new AvailabilitySets Client from subscription ID.
func newAvailabilitySetsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.AvailabilitySetsClient {
	asClient := compute.NewAvailabilitySetsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&asClient.Client, authorizer)
	return asClient
}

// Get gets an availability set
func (a *AzureClient) Get(ctx context.Context, resourceGroup, availabilitySetsName string) (compute.AvailabilitySet, error) {
	ctx, span := tele.Tracer().Start(ctx, "availabilitysets.AzureClient.Get")
	defer span.End()

	return a.availabilitySets.Get(ctx, resourceGroup, availabilitySetsName)
}

// CreateOrUpdate creates or updates an availability set
func (a *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroup string, availabilitySetsName string,
	params compute.AvailabilitySet) (compute.AvailabilitySet, error) {
	ctx, span := tele.Tracer().Start(ctx, "availabilitysets.AzureClient.CreateOrUpdate")
	defer span.End()

	return a.availabilitySets.CreateOrUpdate(ctx, resourceGroup, availabilitySetsName, params)
}

// Delete deletes an availability set
func (a *AzureClient) Delete(ctx context.Context, resourceGroup, availabilitySetsName string) error {
	ctx, span := tele.Tracer().Start(ctx, "availabilitysets.AzureClient.Delete")
	defer span.End()
	_, err := a.availabilitySets.Delete(ctx, resourceGroup, availabilitySetsName)
	return err
}
