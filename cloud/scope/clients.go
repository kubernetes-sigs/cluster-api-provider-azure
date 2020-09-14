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
	"fmt"
	"os"
	"strings"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"k8s.io/klog/klogr"
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
	Authorizer                 autorest.Authorizer
	environment                string
	ResourceManagerEndpoint    string
	ResourceManagerVMDNSSuffix string
	subscriptionID             string
	tenantID                   string
	clientID                   string
	clientSecret               string
}

// CloudEnvironment returns the Azure environment the controller runs in.
func (c *AzureClients) CloudEnvironment() string {
	return c.environment
}

// SubscriptionID returns the Azure subscription id from the controller environment
func (c *AzureClients) SubscriptionID() string {
	return c.subscriptionID
}

// TenantID returns the Azure tenant id the controller runs in.
func (c *AzureClients) TenantID() string {
	return c.tenantID
}

// ClientID returns the Azure client id from the controller environment
func (c *AzureClients) ClientID() string {
	return c.clientID
}

// ClientSecret returns the Azure client secret from the controller environment
func (c *AzureClients) ClientSecret() string {
	return c.clientSecret
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

	c.subscriptionID = subscriptionID
	c.tenantID = strings.TrimSuffix(settings.Values[auth.TenantID], "\n")
	c.clientID = strings.TrimSuffix(settings.Values[auth.ClientID], "\n")
	c.clientSecret = strings.TrimSuffix(settings.Values[auth.ClientSecret], "\n")

	c.environment = settings.Values[auth.EnvironmentName]
	if c.environment == "" {
		c.environment = azure.PublicCloud.Name
	}

	log := klogr.New()
	log.Info("Adding a log line to clients.go")
	log.Info("Checking if we can read the environment from armEndpoint")
	armEndpoint := os.Getenv("ARM_ENDPOINT")
	log.Info(armEndpoint)
	subsID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	log.Info(subsID)
	armEndpoint = "https://management.dbe-1dkphq2.microsoftdatabox.com"
	env1, err1 := azure.EnvironmentFromURL(armEndpoint)
	if err1 != nil {
		log.Info(err1.Error())
	}
	log.Info(env1.Name)
	log.Info(env1.ResourceManagerEndpoint)
	log.Info(env1.ActiveDirectoryEndpoint)
	log.Info("Done extracting endpoint from environment")

	c.ResourceManagerEndpoint = settings.Environment.ResourceManagerEndpoint
	c.ResourceManagerVMDNSSuffix = GetAzureDNSZoneForEnvironment(settings.Environment.Name)
	settings.Values[auth.SubscriptionID] = subscriptionID
	settings.Values[auth.TenantID] = c.tenantID

	c.Authorizer, err = settings.GetAuthorizer()
	return err
}

func (c *AzureClients) setDBECredentials(subscriptionID string) error {
	log := klogr.New()
	c.subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
	c.tenantID = os.Getenv("AZURE_TENANT_ID")
	c.clientID = os.Getenv("AZURE_CLIENT_ID")
	c.clientSecret = os.Getenv("AZURE_CLIENT_SECRET")
	armEndpoint := os.Getenv("ARM_ENDPOINT")
	env, err := azure.EnvironmentFromURL(armEndpoint)
	if err != nil {
		log.Info(err.Error())
		return err
	}
	//c.environment = env.Name
	//env Name gives HybridEnvironment
	c.environment = "AzureStackCloud"
	c.ResourceManagerEndpoint = env.ResourceManagerEndpoint
	c.ResourceManagerVMDNSSuffix = ""

	log.Info(c.subscriptionID)
	log.Info(c.tenantID)
	log.Info(c.clientID)
	log.Info(c.clientSecret)
	log.Info(armEndpoint)
	log.Info(c.ResourceManagerEndpoint)
	log.Info("Printing Resource Manager VMDNS Suffix")
	log.Info(c.ResourceManagerVMDNSSuffix)

	token, err := GetResourceManagementTokenHybrid(armEndpoint, c.tenantID, c.clientID, c.clientSecret)
	if err != nil {
		log.Info(err.Error())
		return err
	}
	c.Authorizer = autorest.NewBearerAuthorizer(token)
	/*groupsClient := azurestack.NewGroupsClientWithBaseURI(armEndpoint, c.subscriptionID)
	groupsClient.Authorizer = autorest.NewBearerAuthorizer(token)
	cntx := context.Background()
	location := "dbelocal"
	rg, errRg := groupsClient.CreateOrUpdate(cntx, "msk8srg", azurestack.Group{Location: &location})
	if errRg != nil {
		log.Info(errRg.Error())
	}
	log.Info("Printing rg name")
	log.Info(*rg.Name)*/
	return err
}

// Gets the token for authorizer on DBE
func GetResourceManagementTokenHybrid(armEndpoint, tenantID, clientID, clientSecret string) (adal.OAuthTokenProvider, error) {
	var token adal.OAuthTokenProvider
	log := klogr.New()
	environment, err := azure.EnvironmentFromURL(armEndpoint)
	if err != nil {
		log.Info(err.Error())
		return nil, err
	}
	tokenAudience := environment.TokenAudience
	log.Info(tokenAudience)
	activeDirectoryEndpoint := environment.ActiveDirectoryEndpoint
	activeDirectoryEndpoint = strings.TrimRight(activeDirectoryEndpoint, "/adfs") + "/"
	log.Info(activeDirectoryEndpoint)
	oauthConfig, err := adal.NewOAuthConfig(activeDirectoryEndpoint, tenantID)
	token, err = adal.NewServicePrincipalToken(
		*oauthConfig,
		clientID,
		clientSecret,
		tokenAudience)

	return token, err
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
