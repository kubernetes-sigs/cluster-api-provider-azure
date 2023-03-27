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

package identities

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/go-autorest/autorest"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	Get(ctx context.Context, resourceGroupName, name string) (msi.Identity, error)
	GetClientID(ctx context.Context, providerID string) (string, error)
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	userAssignedIdentities msi.UserAssignedIdentitiesClient
}

// NewClient creates a new MSI client from auth info.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newUserAssignedIdentitiesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newUserAssignedIdentitiesClient creates a new MSI client from subscription ID, base URI, and authorizer.
func newUserAssignedIdentitiesClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) msi.UserAssignedIdentitiesClient {
	userAssignedIdentitiesClient := msi.NewUserAssignedIdentitiesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&userAssignedIdentitiesClient.Client, authorizer)
	return userAssignedIdentitiesClient
}

// Get returns a managed service identity.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, name string) (msi.Identity, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "identities.AzureClient.Get")
	defer done()

	return ac.userAssignedIdentities.Get(ctx, resourceGroupName, name)
}

// GetClientID returns the client ID of a managed service identity, given its full URL identifier.
func (ac *AzureClient) GetClientID(ctx context.Context, providerID string) (string, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "identities.GetClientID")
	defer done()

	parsed, err := arm.ParseResourceID(providerID)
	if err != nil {
		return "", err
	}
	ident, err := ac.Get(ctx, parsed.ResourceGroupName, parsed.Name)
	if err != nil {
		return "", err
	}
	return ident.ClientID.String(), nil
}
