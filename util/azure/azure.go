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

package azure

import (
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/jongio/azidext/go/azidext"
)

// AzureSystemNodeLabelPrefix is a standard node label prefix for Azure features, e.g., kubernetes.azure.com/scalesetpriority.
const AzureSystemNodeLabelPrefix = "kubernetes.azure.com"

// IsAzureSystemNodeLabelKey is a helper function that determines whether a node label key is an Azure "system" label.
func IsAzureSystemNodeLabelKey(labelKey string) bool {
	return strings.HasPrefix(labelKey, AzureSystemNodeLabelPrefix)
}

func getCloudConfig(environment azure.Environment) cloud.Configuration {
	var config cloud.Configuration
	switch environment.Name {
	case "AzureStackCloud":
		config = cloud.Configuration{
			ActiveDirectoryAuthorityHost: environment.ActiveDirectoryEndpoint,
			Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
				cloud.ResourceManager: {
					Audience: environment.TokenAudience,
					Endpoint: environment.ResourceManagerEndpoint,
				},
			},
		}
	case "AzureChinaCloud":
		config = cloud.AzureChina
	case "AzureUSGovernmentCloud":
		config = cloud.AzureGovernment
	default:
		config = cloud.AzurePublic
	}
	return config
}

// GetAuthorizer returns an autorest.Authorizer-compatible object from MSAL.
func GetAuthorizer(settings auth.EnvironmentSettings) (autorest.Authorizer, error) {
	// azidentity uses different envvars for certificate authentication:
	//  azidentity: AZURE_CLIENT_CERTIFICATE_{PATH,PASSWORD}
	//  autorest: AZURE_CERTIFICATE_{PATH,PASSWORD}
	// Let's set them according to the envvars used by autorest, in case they are present
	_, azidSet := os.LookupEnv("AZURE_CLIENT_CERTIFICATE_PATH")
	path, autorestSet := os.LookupEnv("AZURE_CERTIFICATE_PATH")
	if !azidSet && autorestSet {
		os.Setenv("AZURE_CLIENT_CERTIFICATE_PATH", path)
		os.Setenv("AZURE_CLIENT_CERTIFICATE_PASSWORD", os.Getenv("AZURE_CERTIFICATE_PASSWORD"))
	}

	options := azidentity.DefaultAzureCredentialOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: getCloudConfig(settings.Environment),
		},
	}
	cred, err := azidentity.NewDefaultAzureCredential(&options)
	if err != nil {
		return nil, err
	}

	// We must use TokenAudience for StackCloud, otherwise we get an
	// AADSTS500011 error from the API
	scope := settings.Environment.TokenAudience
	if !strings.HasSuffix(scope, "/.default") {
		scope += "/.default"
	}
	return azidext.NewTokenCredentialAdapter(cred, []string{scope}), nil
}
