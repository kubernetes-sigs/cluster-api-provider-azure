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

package disks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk
type client interface {
	Delete(context.Context, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type azureClient struct {
	disks compute.DisksClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newDisksClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newDisksClient creates a new disks client from subscription ID.
func newDisksClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.DisksClient {
	disksClient := compute.NewDisksClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&disksClient.Client, authorizer)
	return disksClient
}

// Delete removes the disk client
func (ac *azureClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	ctx, span := tele.Tracer().Start(ctx, "disks.AzureClient.Delete")
	defer span.End()

	future, err := ac.disks.Delete(ctx, resourceGroupName, name)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.disks.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.disks)
	return err
}
