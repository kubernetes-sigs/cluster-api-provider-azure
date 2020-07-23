/*
Copyright 2018 The Kubernetes Authors.

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

package scope

import (
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

const (
	// ChinaCloud is the cloud environment operated in China
	ChinaCloud = "AzureChinaCloud"
	// GermanCloud is the cloud environment operated in Germany
	GermanCloud = "AzureGermanCloud"
	// PublicCloud is the default public Azure cloud environment
	PublicCloud = "AzurePublicCloud"
	// USGovernmentCloud is the cloud environment for the US Government
	USGovernmentCloud = "AzureUSGovernmentCloud"
)

var _ azure.Authorizer = new(AzureClients)

// AzureClients contains all the Azure clients used by the scopes.
type AzureClients struct {
	auth.EnvironmentSettings
	subscriptionID             string
	ResourceManagerEndpoint    string
	ResourceManagerVMDNSSuffix string
	authorizer                 autorest.Authorizer
}

// NewAzureClients discovers and initializes
func NewAzureClients(subscriptionID string) (*AzureClients, error) {
	c := new(AzureClients)
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return nil, err
	}
	if subscriptionID != "" {
		settings.Values[auth.SubscriptionID] = subscriptionID
	}

	c.subscriptionID = settings.Values[auth.SubscriptionID]
	c.ResourceManagerEndpoint = settings.Environment.ResourceManagerEndpoint
	c.ResourceManagerVMDNSSuffix = settings.Environment.ResourceManagerVMDNSSuffix
	c.EnvironmentSettings = settings
	c.authorizer, err = settings.GetAuthorizer()
	if err != nil {
		return nil, err
	}
	return c, err
}

// BaseURI returns the Azure ResourceManagerEndpoint.
func (c *AzureClients) BaseURI() string {
	return c.Environment.ResourceManagerEndpoint
}

// Authorizer returns the Azure client Authorizer.
func (c *AzureClients) Authorizer() autorest.Authorizer {
	return c.authorizer
}
