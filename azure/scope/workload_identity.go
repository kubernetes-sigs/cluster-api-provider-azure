/*
Copyright 2023 The Kubernetes Authors.

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
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/pkg/errors"
)

/*

Azure Workload Identity (AZWI) requires deploying AZWI mutating admission webhook
for self managed clusters e.g. Kind.

The webhook injects the following environment variables to the pod that
uses a label `azure.workload.identity/use=true`
|-----------------------------------------------------------------------------------|
|AZURE_AUTHORITY_HOST       | The Azure Active Directory (AAD) endpoint.            |
|AZURE_CLIENT_ID            | The client ID of the Azure AD             |
|                           | application or user-assigned managed identity.        |
|AZURE_TENANT_ID            | The tenant ID of the Azure subscription.              |
|AZURE_FEDERATED_TOKEN_FILE | The path of the projected service account token file. |
|-----------------------------------------------------------------------------------|

In addition to the service account, it also projects a signed service account token to the
workload's volume(in this case the capz pod). The volume name is `azure-identity-token`
which is mounted at path `/var/run/secrets/azure/tokens/azure-identity-token` to the pod.

*/

const (
	// AzureFedratedTokenFileEnvKey is the env key for AZURE_FEDERATED_TOKEN_FILE.
	AzureFedratedTokenFileEnvKey = "AZURE_FEDERATED_TOKEN_FILE"
	// AzureClientIDEnvKey is the env key for AZURE_CLIENT_ID.
	AzureClientIDEnvKey = "AZURE_CLIENT_ID"
	// AzureTenantIDEnvKey is the env key for AZURE_TENANT_ID.
	AzureTenantIDEnvKey = "AZURE_TENANT_ID"
)

type workloadIdentityCredential struct {
	assertion string
	file      string
	cred      *azidentity.ClientAssertionCredential
	lastRead  time.Time
}

// WorkloadIdentityCredentialOptions contains the configurable options for azwi.
type WorkloadIdentityCredentialOptions struct {
	azcore.ClientOptions
	ClientID      string
	TenantID      string
	TokenFilePath string
}

// NewWorkloadIdentityCredentialOptions returns an empty instance of WorkloadIdentityCredentialOptions.
func NewWorkloadIdentityCredentialOptions() *WorkloadIdentityCredentialOptions {
	return &WorkloadIdentityCredentialOptions{}
}

// WithClientID sets client ID to WorkloadIdentityCredentialOptions.
func (w *WorkloadIdentityCredentialOptions) WithClientID(clientID string) *WorkloadIdentityCredentialOptions {
	w.ClientID = clientID
	return w
}

// WithTenantID sets tenant ID to WorkloadIdentityCredentialOptions.
func (w *WorkloadIdentityCredentialOptions) WithTenantID(tenantID string) *WorkloadIdentityCredentialOptions {
	w.TenantID = tenantID
	return w
}

// GetProjectedTokenPath return projected token file path from the env variable.
func GetProjectedTokenPath() (string, error) {
	tokenPath := os.Getenv(AzureFedratedTokenFileEnvKey)
	if strings.TrimSpace(tokenPath) == "" {
		return "", errors.New("projected token path not injected")
	}
	return tokenPath, nil
}

// WithDefaults sets token file path. It also sets the client tenant ID from injected env in
// case empty values are passed.
func (w *WorkloadIdentityCredentialOptions) WithDefaults() (*WorkloadIdentityCredentialOptions, error) {
	tokenFilePath, err := GetProjectedTokenPath()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get token file path for identity")
	}
	w.TokenFilePath = tokenFilePath

	// Fallback to using client ID from env variable if not set.
	if strings.TrimSpace(w.ClientID) == "" {
		w.ClientID = os.Getenv(AzureClientIDEnvKey)
		if strings.TrimSpace(w.ClientID) == "" {
			return nil, errors.New("empty client ID")
		}
	}

	// // Fallback to using tenant ID from env variable.
	if strings.TrimSpace(w.TenantID) == "" {
		w.TenantID = os.Getenv(AzureTenantIDEnvKey)
		if strings.TrimSpace(w.TenantID) == "" {
			return nil, errors.New("empty tenant ID")
		}
	}
	return w, nil
}

// NewWorkloadIdentityCredential returns a workload identity credential.
func NewWorkloadIdentityCredential(options *WorkloadIdentityCredentialOptions) (*workloadIdentityCredential, error) {
	w := &workloadIdentityCredential{file: options.TokenFilePath}
	cred, err := azidentity.NewClientAssertionCredential(options.TenantID, options.ClientID, w.getAssertion, &azidentity.ClientAssertionCredentialOptions{ClientOptions: options.ClientOptions})
	if err != nil {
		return nil, err
	}
	w.cred = cred
	return w, nil
}

// GetToken returns the token for azwi.
func (w *workloadIdentityCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return w.cred.GetToken(ctx, opts)
}

func (w *workloadIdentityCredential) getAssertion(context.Context) (string, error) {
	if now := time.Now(); w.lastRead.Add(5 * time.Minute).Before(now) {
		content, err := os.ReadFile(w.file)
		if err != nil {
			return "", err
		}
		w.assertion = string(content)
		w.lastRead = now
	}
	return w.assertion, nil
}
