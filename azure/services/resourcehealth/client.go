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

package resourcehealth

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/resourcehealth/mgmt/2020-05-01/resourcehealth"
	"github.com/Azure/go-autorest/autorest"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	GetByResource(context.Context, string) (resourcehealth.AvailabilityStatus, error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	availabilityStatuses resourcehealth.AvailabilityStatusesClient
}

// newClient creates a new resource health client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newResourceHealthClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newResourceHealthClient creates a new resource health client from subscription ID.
func newResourceHealthClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) resourcehealth.AvailabilityStatusesClient {
	healthClient := resourcehealth.NewAvailabilityStatusesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&healthClient.Client, authorizer)
	return healthClient
}

// GetByResource gets the availability status for the specified resource.
func (ac *azureClient) GetByResource(ctx context.Context, resourceURI string) (resourcehealth.AvailabilityStatus, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "resourcehealth.AzureClient.GetByResource")
	defer done()

	return ac.availabilityStatuses.GetByResource(ctx, resourceURI, "", "")
}
