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

package scope

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGettingEnvironment(t *testing.T) {
	g := NewWithT(t)

	var tests = map[string]struct {
		azureEnv             string
		expectedEndpoint     string
		expectedDNSSuffix    string
		expectedError        bool
		expectedErrorMessage string
	}{
		"AZURE_ENVIRONMENT is empty": {
			azureEnv:          "",
			expectedEndpoint:  "https://management.azure.com/",
			expectedDNSSuffix: "cloudapp.azure.com",
			expectedError:     false,
		}, "AZURE_ENVIRONMENT is AzurePublicCloud": {
			azureEnv:          "AzurePublicCloud",
			expectedEndpoint:  "https://management.azure.com/",
			expectedDNSSuffix: "cloudapp.azure.com",
			expectedError:     false,
		}, "AZURE_ENVIRONMENT is AzureUSGovernmentCloud": {
			azureEnv:          "AzureUSGovernmentCloud",
			expectedEndpoint:  "https://management.usgovcloudapi.net/",
			expectedDNSSuffix: "cloudapp.usgovcloudapi.net",
			expectedError:     false,
		}, "AZURE_ENVIRONMENT is AzureChina": {
			azureEnv:          "AzureChinaCloud",
			expectedEndpoint:  "https://management.chinacloudapi.cn/",
			expectedDNSSuffix: "cloudapp.chinacloudapi.cn",
			expectedError:     false,
		}, "AZURE_ENVIRONMENT is AzureGermany": {
			azureEnv:          "AzureGermanCloud",
			expectedEndpoint:  "https://management.microsoftazure.de/",
			expectedDNSSuffix: "cloudapp.microsoftazure.de",
			expectedError:     false,
		}, "AZURE_ENVIRONMENT has an invalid value": {
			azureEnv:             "AzureInSpace",
			expectedEndpoint:     "",
			expectedDNSSuffix:    "",
			expectedError:        true,
			expectedErrorMessage: "There is no cloud environment matching the name \"AZUREINSPACE\"",
		}}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			c := AzureClients{}
			err := c.setCredentials("1234", test.azureEnv)
			if test.expectedError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.expectedErrorMessage))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(c.ResourceManagerEndpoint).To(Equal(test.expectedEndpoint))
				g.Expect(c.ResourceManagerVMDNSSuffix).To(Equal(test.expectedDNSSuffix))
			}
		})
	}
}
