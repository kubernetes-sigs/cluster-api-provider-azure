/*
Copyright 2019 The Kubernetes Authors.

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

package securitygroups

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk
type client interface {
	Get(context.Context, string, string) (network.SecurityGroup, error)
	CreateOrUpdate(context.Context, string, string, network.SecurityGroup) error
	Delete(context.Context, string, string) error
}

// azureClient contains the Azure go-sdk Client
type azureClient struct {
	securitygroups network.SecurityGroupsClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newSecurityGroupsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newSecurityGroupsClient creates a new security groups client from subscription ID.
func newSecurityGroupsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.SecurityGroupsClient {
	securityGroupsClient := network.NewSecurityGroupsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&securityGroupsClient.Client, authorizer)
	return securityGroupsClient
}

// Get gets the specified network security group.
func (ac *azureClient) Get(ctx context.Context, resourceGroupName, sgName string) (network.SecurityGroup, error) {
	ctx, span := tele.Tracer().Start(ctx, "securitygroups.AzureClient.Get")
	defer span.End()

	return ac.securitygroups.Get(ctx, resourceGroupName, sgName, "")
}

// CreateOrUpdate creates or updates a network security group in the specified resource group.
func (ac *azureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, sgName string, sg network.SecurityGroup) error {
	ctx, span := tele.Tracer().Start(ctx, "securitygroups.AzureClient.CreateOrUpdate")
	defer span.End()

	var etag string
	if sg.Etag != nil {
		etag = *sg.Etag
	}
	req, err := ac.securitygroups.CreateOrUpdatePreparer(ctx, resourceGroupName, sgName, sg)
	if err != nil {
		err = autorest.NewErrorWithError(err, "network.SecurityGroupsClient", "CreateOrUpdate", nil, "Failure preparing request")
		return err
	}
	if etag != "" {
		req.Header.Add("If-Match", etag)
	}

	future, err := ac.securitygroups.CreateOrUpdateSender(req)
	if err != nil {
		err = autorest.NewErrorWithError(err, "network.SecurityGroupsClient", "CreateOrUpdate", future.Response(), "Failure sending request")
		return err
	}

	err = future.WaitForCompletionRef(ctx, ac.securitygroups.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.securitygroups)
	return err
}

// Delete deletes the specified network security group.
func (ac *azureClient) Delete(ctx context.Context, resourceGroupName, sgName string) error {
	ctx, span := tele.Tracer().Start(ctx, "securitygroups.AzureClient.Delete")
	defer span.End()

	future, err := ac.securitygroups.Delete(ctx, resourceGroupName, sgName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.securitygroups.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.securitygroups)
	return err
}
