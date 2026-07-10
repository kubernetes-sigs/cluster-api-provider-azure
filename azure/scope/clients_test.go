/*
Copyright 2024 The Kubernetes Authors.

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
	"path/filepath"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

// validUSSecEnvFile is a sample Azure environment file for the USSec (IL6)
// cloud. The endpoints use example.com placeholders rather than real IL6
// domains so the fixture is safe to commit.
const validUSSecEnvFile = `{
	"name": "AzureUSSecCloud",
	"resourceManagerEndpoint": "https://management.example.com/",
	"activeDirectoryEndpoint": "https://login.example.com/",
	"tokenAudience": "https://management.example.com/",
	"resourceManagerVMDNSSuffix": "cloudapp.example.com"
}`

// writeEnvFile writes contents to a temporary environment file and points
// AZURE_ENVIRONMENT_FILEPATH at it for the duration of the test.
func writeEnvFile(t *testing.T, g *WithT, contents string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "environment.json")
	g.Expect(os.WriteFile(path, []byte(contents), 0o600)).To(Succeed())
	t.Setenv("AZURE_ENVIRONMENT_FILEPATH", path)
}

func TestLoadCloudEnvironmentFromFile(t *testing.T) {
	tests := []struct {
		name        string
		contents    string
		setEnv      bool
		expectErr   bool
		expectedRM  string
		expectedAD  string
		expectedAud string
	}{
		{
			name:        "valid environment file loads all endpoints",
			contents:    validUSSecEnvFile,
			setEnv:      true,
			expectedRM:  "https://management.example.com/",
			expectedAD:  "https://login.example.com/",
			expectedAud: "https://management.example.com/",
		},
		{
			name:      "unset environment variable returns an error",
			setEnv:    false,
			expectErr: true,
		},
		{
			name:      "malformed JSON returns an error",
			contents:  `{not valid json`,
			setEnv:    true,
			expectErr: true,
		},
		{
			name: "missing resourceManagerEndpoint returns an error",
			contents: `{
				"name": "AzureUSSecCloud",
				"activeDirectoryEndpoint": "https://login.example.com/",
				"tokenAudience": "https://management.example.com/"
			}`,
			setEnv:    true,
			expectErr: true,
		},
		{
			name: "missing activeDirectoryEndpoint returns an error",
			contents: `{
				"name": "AzureUSSecCloud",
				"resourceManagerEndpoint": "https://management.example.com/",
				"tokenAudience": "https://management.example.com/"
			}`,
			setEnv:    true,
			expectErr: true,
		},
		{
			name: "missing tokenAudience returns an error",
			contents: `{
				"name": "AzureUSSecCloud",
				"resourceManagerEndpoint": "https://management.example.com/",
				"activeDirectoryEndpoint": "https://login.example.com/"
			}`,
			setEnv:    true,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			if tt.setEnv {
				writeEnvFile(t, g, tt.contents)
			} else {
				t.Setenv("AZURE_ENVIRONMENT_FILEPATH", "")
			}
			env, err := loadCloudEnvironmentFromFile()
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(env.ResourceManagerEndpoint).To(Equal(tt.expectedRM))
			g.Expect(env.ActiveDirectoryEndpoint).To(Equal(tt.expectedAD))
			g.Expect(env.TokenAudience).To(Equal(tt.expectedAud))
		})
	}
}

func TestLoadCloudEnvironmentFromFileMissingFile(t *testing.T) {
	g := NewWithT(t)
	t.Setenv("AZURE_ENVIRONMENT_FILEPATH", filepath.Join(t.TempDir(), "does-not-exist.json"))
	_, err := loadCloudEnvironmentFromFile()
	g.Expect(err).To(HaveOccurred())
}

func TestGetSettingsFromEnvironmentUSSec(t *testing.T) {
	g := NewWithT(t)
	writeEnvFile(t, g, validUSSecEnvFile)

	c := &AzureClients{}
	g.Expect(c.getSettingsFromEnvironment(azure.USSecCloudName)).To(Succeed())

	g.Expect(c.CloudEnvironment()).To(Equal(azure.USSecCloudName))
	g.Expect(c.ResourceManagerEndpoint).To(Equal("https://management.example.com/"))
	g.Expect(c.ResourceManagerVMDNSSuffix).To(Equal("cloudapp.example.com"))

	cfg := c.CloudConfiguration()
	g.Expect(cfg.ActiveDirectoryAuthorityHost).To(Equal("https://login.example.com/"))
	g.Expect(cfg.Services[cloud.ResourceManager].Endpoint).To(Equal("https://management.example.com/"))
	g.Expect(cfg.Services[cloud.ResourceManager].Audience).To(Equal("https://management.example.com/"))
}

func TestGetSettingsFromEnvironmentUSSecMissingFile(t *testing.T) {
	g := NewWithT(t)
	t.Setenv("AZURE_ENVIRONMENT_FILEPATH", "")

	c := &AzureClients{}
	g.Expect(c.getSettingsFromEnvironment(azure.USSecCloudName)).NotTo(Succeed())
}
