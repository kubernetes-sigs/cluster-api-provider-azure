/*
Copyright 2025 The Kubernetes Authors.

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

package keyvaults

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	Get(context.Context, azure.ResourceSpecGetter) (interface{}, error)
	CreateOrUpdate(context.Context, azure.ResourceSpecGetter, interface{}) (interface{}, error)
	Delete(context.Context, azure.ResourceSpecGetter) error
	GetKey(context.Context, string, string, string) (interface{}, error)
	CreateKey(context.Context, string, string, string, interface{}) (interface{}, error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	vaults *armkeyvault.VaultsClient
	keys   *armkeyvault.KeysClient
}

// newClient creates a new Key Vault client from an authorizer.
func newClient(auth azure.Authorizer, _ time.Duration) (*azureClient, error) {
	opts, err := azure.ARMClientOptions(auth.CloudEnvironment())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create keyvault client options")
	}
	vaultsClient, err := armkeyvault.NewVaultsClient(auth.SubscriptionID(), auth.Token(), opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create armkeyvault vaults client")
	}
	keysClient, err := armkeyvault.NewKeysClient(auth.SubscriptionID(), auth.Token(), opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create armkeyvault keys client")
	}
	return &azureClient{vaults: vaultsClient, keys: keysClient}, nil
}

// Get gets the specified key vault.
//
// ResourceGroupName is the name of the Resource Group to which the vault belongs.
// VaultName is the name of the vault.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "keyvault.azureClient.Get")
	defer done()

	resp, err := ac.vaults.Get(ctx, spec.ResourceGroupName(), spec.ResourceName(), nil)
	if err != nil {
		return nil, err
	}
	return resp.Vault, nil
}

// CreateOrUpdate creates or updates a key vault synchronously.
func (ac *azureClient) CreateOrUpdate(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "keyvault.azureClient.CreateOrUpdate")
	defer done()

	vault, ok := parameters.(armkeyvault.VaultCreateOrUpdateParameters)
	if !ok && parameters != nil {
		return nil, errors.Errorf("%T is not an armkeyvault.VaultCreateOrUpdateParameters", parameters)
	}

	// For Key Vault, we'll use BeginCreateOrUpdate and wait for completion
	// In practice, Key Vault creation is usually fast
	poller, err := ac.vaults.BeginCreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), vault, nil)
	if err != nil {
		return nil, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return resp.Vault, nil
}

// Delete deletes a key vault.
func (ac *azureClient) Delete(ctx context.Context, spec azure.ResourceSpecGetter) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "keyvault.azureClient.Delete")
	defer done()

	_, err := ac.vaults.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName(), nil)
	return err
}

// GetKey gets a key from a key vault.
func (ac *azureClient) GetKey(ctx context.Context, resourceGroup, vaultName, keyName string) (interface{}, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "keyvault.azureClient.GetKey")
	defer done()

	resp, err := ac.keys.Get(ctx, resourceGroup, vaultName, keyName, nil)
	if err != nil {
		return nil, err
	}
	return resp.Key, nil
}

// CreateKey creates a key in a key vault.
func (ac *azureClient) CreateKey(ctx context.Context, resourceGroup, vaultName, keyName string, parameters interface{}) (interface{}, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "keyvault.azureClient.CreateKey")
	defer done()

	keyParams, ok := parameters.(armkeyvault.KeyCreateParameters)
	if !ok && parameters != nil {
		return nil, errors.Errorf("%T is not an armkeyvault.KeyCreateParameters", parameters)
	}

	resp, err := ac.keys.CreateIfNotExist(ctx, resourceGroup, vaultName, keyName, keyParams, nil)
	if err != nil {
		return nil, err
	}
	return resp.Key, nil
}
