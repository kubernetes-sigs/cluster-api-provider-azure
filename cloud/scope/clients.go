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
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

// AzureClients contains all the Azure clients used by the scopes.
type AzureClients struct {
	auth.EnvironmentSettings

	Authorizer                 autorest.Authorizer
	ResourceManagerEndpoint    string
	ResourceManagerVMDNSSuffix string
}

// CloudEnvironment returns the Azure environment the controller runs in.
func (c *AzureClients) CloudEnvironment() string {
	return c.Environment.Name
}

// TenantID returns the Azure tenant id the controller runs in.
func (c *AzureClients) TenantID() string {
	return c.Values[auth.TenantID]
}

// ClientID returns the Azure client id from the controller environment
func (c *AzureClients) ClientID() string {
	return c.Values[auth.ClientID]
}

// ClientSecret returns the Azure client secret from the controller environment
func (c *AzureClients) ClientSecret() string {
	return c.Values[auth.ClientSecret]
}

// SubscriptionID returns the Azure subscription id of the cluster,
// either specified or from the environment
func (c *AzureClients) SubscriptionID() string {
	return c.Values[auth.SubscriptionID]
}

// HashKey returns a base64 url encoded sha256 hash for the Auth scope (Azure TenantID + CloudEnv + SubscriptionID +
// ClientID).
func (c *AzureClients) HashKey() string {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(c.TenantID() + c.CloudEnvironment() + c.SubscriptionID() + c.ClientID()))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (c *AzureClients) setCredentials(subscriptionID string) error {
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return err
	}

	if subscriptionID == "" {
		subscriptionID = settings.GetSubscriptionID()
		if subscriptionID == "" {
			return fmt.Errorf("error creating azure services. subscriptionID is not set in cluster or AZURE_SUBSCRIPTION_ID env var")
		}
	}

	c.EnvironmentSettings = settings
	c.ResourceManagerEndpoint = settings.Environment.ResourceManagerEndpoint
	c.ResourceManagerVMDNSSuffix = settings.Environment.ResourceManagerVMDNSSuffix
	c.Values[auth.ClientID] = strings.TrimSuffix(c.Values[auth.ClientID], "\n")
	c.Values[auth.ClientSecret] = strings.TrimSuffix(c.Values[auth.ClientSecret], "\n")
	c.Values[auth.SubscriptionID] = strings.TrimSuffix(subscriptionID, "\n")
	c.Values[auth.TenantID] = strings.TrimSuffix(c.Values[auth.TenantID], "\n")

	c.Authorizer, err = c.GetAuthorizer()
	return err
}

func (c *AzureClients) setCredentialsWithProvider(ctx context.Context, subscriptionID string, credentialsProvider *AzureCredentialsProvider) error {
	if credentialsProvider == nil {
		return fmt.Errorf("credentials provider cannot have an empty value")
	}

	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return err
	}

	if subscriptionID == "" {
		subscriptionID = settings.GetSubscriptionID()
		if subscriptionID == "" {
			return fmt.Errorf("error creating azure services. subscriptionID is not set in cluster or AZURE_SUBSCRIPTION_ID env var")
		}
	}

	c.EnvironmentSettings = settings
	c.ResourceManagerEndpoint = settings.Environment.ResourceManagerEndpoint
	c.ResourceManagerVMDNSSuffix = settings.Environment.ResourceManagerVMDNSSuffix
	c.Values[auth.SubscriptionID] = strings.TrimSuffix(subscriptionID, "\n")

	c.Authorizer, err = credentialsProvider.GetAuthorizer(ctx, c.ResourceManagerEndpoint)
	return err
}
