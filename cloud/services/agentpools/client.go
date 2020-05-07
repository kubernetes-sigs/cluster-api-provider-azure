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

package agentpools

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string, string, string) (containerservice.AgentPool, error)
	CreateOrUpdate(context.Context, string, string, string, containerservice.AgentPool) error
	Delete(context.Context, string, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	agentpools containerservice.AgentPoolsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new agent pools client from subscription ID.
func NewClient(subscriptionID string, authorizer autorest.Authorizer) *AzureClient {
	c := newAgentPoolsClient(subscriptionID, authorizer)
	return &AzureClient{c}
}

// newAgentPoolsClient creates a new agent pool client from subscription ID.
func newAgentPoolsClient(subscriptionID string, authorizer autorest.Authorizer) containerservice.AgentPoolsClient {
	agentPoolsClient := containerservice.NewAgentPoolsClient(subscriptionID)
	agentPoolsClient.Authorizer = authorizer
	agentPoolsClient.AddToUserAgent(azure.UserAgent)
	return agentPoolsClient
}

// Get gets an agent pool.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, cluster, name string) (containerservice.AgentPool, error) {
	return ac.agentpools.Get(ctx, resourceGroupName, cluster, name)
}

// CreateOrUpdate creates or updates an agent pool.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, cluster, name string, properties containerservice.AgentPool) error {
	future, err := ac.agentpools.CreateOrUpdate(ctx, resourceGroupName, cluster, name, properties)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.agentpools.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.agentpools)
	return err
}

// Delete deletes an agent pool.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, cluster, name string) error {
	future, err := ac.agentpools.Delete(ctx, resourceGroupName, cluster, name)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.agentpools.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.agentpools)
	return err
}
