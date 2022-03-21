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

package secrets

import (
	"context"
	"fmt"

	secretskeyvault "github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	secretsClient     secretskeyvault.BaseClient
	keyvaultDNSSuffix string
}

// newClient creates a new azure go-sdk client for secrets.
func newClient(auth azure.Authorizer) *azureClient {
	secretsClient, keyvaultDNSSuffix := newSecretsClient(auth.Authorizer())
	return &azureClient{secretsClient: secretsClient, keyvaultDNSSuffix: keyvaultDNSSuffix}
}

// newSecretsClient creates a new key vault client.
func newSecretsClient(authorizer autorest.Authorizer) (secretskeyvault.BaseClient, string) {
	secretsClient := secretskeyvault.New()
	azure.SetAutoRestClientDefaults(&secretsClient.Client, authorizer)

	// override to use keyvault management endpoint
	settings, _ := auth.GetSettingsFromEnvironment()
	settings.Values[auth.Resource] = fmt.Sprintf("%s%s", "https://", settings.Environment.KeyVaultDNSSuffix)
	keyvaultAuthorizer, _ := settings.GetAuthorizer()
	secretsClient.Authorizer = keyvaultAuthorizer

	return secretsClient, settings.Environment.KeyVaultDNSSuffix
}

// CreateOrUpdateAsync creates a secret in azure keyvault.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "secrets.AzureClient.CreateOrUpdateAsync")
	defer done()

	secretParameters, ok := parameters.(secretskeyvault.SecretSetParameters)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a keyvault.SecretSetParameters", secretParameters)
	}

	vaultURL := fmt.Sprintf("https://%s.%s/", spec.OwnerResourceName(), ac.keyvaultDNSSuffix)
	result, err = ac.secretsClient.SetSecret(ctx, vaultURL, spec.ResourceName(), secretParameters)
	if err != nil {
		return nil, nil, err
	}

	return result, nil, nil
}

// IsDone returns true if the long-running operation has completed.
// no-op for secrets.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error) {
	return true, nil
}

// Result fetches the result of a long-running operation future.
// no-op for secrets.
func (ac *azureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	return nil, nil
}

// Get gets the specified secret from azure keyvault.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "secrets.AzureClient.Get")
	defer done()

	vaultURL := fmt.Sprintf("https://%s.%s/", spec.OwnerResourceName(), ac.keyvaultDNSSuffix)
	return ac.secretsClient.GetSecret(ctx, vaultURL, spec.ResourceName(), "")
}

// DeleteAsync deletes the specified azure resource.
// no-op for secrets.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	return nil, nil
}
