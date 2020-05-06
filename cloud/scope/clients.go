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
	"os"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
)

// AzureClients contains all the Azure clients used by the scopes.
type AzureClients struct {
	SubscriptionID string
	Authorizer     autorest.Authorizer
}

func (c *AzureClients) setCredentials(subscriptionID string) error {
	subID, err := getSubscriptionID(subscriptionID)
	if err != nil {
		return err
	}
	c.SubscriptionID = subID
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return err
	}
	settings.Values[auth.SubscriptionID] = subscriptionID
	c.Authorizer, err = settings.GetAuthorizer()
	return err
}

func getSubscriptionID(subscriptionID string) (string, error) {
	if subscriptionID != "" {
		return subscriptionID, nil
	}
	subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return "", errors.New("error creating azure services. Environment variable AZURE_SUBSCRIPTION_ID is not set")
	}
	return subscriptionID, nil
}
