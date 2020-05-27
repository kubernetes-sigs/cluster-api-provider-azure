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

// AzureClients contains all the Azure clients used by the scopes.
type AzureClients struct {
	SubscriptionID             string
	ResourceManagerEndpoint    string
	ResourceManagerVMDNSSuffix string
	Authorizer                 autorest.Authorizer
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
	c.ResourceManagerEndpoint = settings.Environment.ResourceManagerEndpoint
	c.ResourceManagerVMDNSSuffix = GetAzureDNSZoneForEnvironment(settings.Environment.Name)
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

// GetAzureDNSZoneForEnvironment returnes the DNSZone to be used with the
// cloud environment, the default is the public cloud
func GetAzureDNSZoneForEnvironment(environmentName string) string {
	// default is public cloud
	switch environmentName {
	case ChinaCloud:
		return "cloudapp.chinacloudapi.cn"
	case GermanCloud:
		return "cloudapp.microsoftazure.de"
	case PublicCloud:
		return "cloudapp.azure.com"
	case USGovernmentCloud:
		return "cloudapp.usgovcloudapi.net"
	default:
		return "cloudapp.azure.com"
	}
}
