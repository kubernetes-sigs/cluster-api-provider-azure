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

package utils

import (
	"context"
	"log"

	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/auth"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	azureAuth "github.com/Azure/go-autorest/autorest/azure/auth"
)

// GetGroup gets the resource group if exist
func GetGroup(ctx context.Context, creds auth.Creds, resourceName string) (resources.Group, error) {
	groupClient, err := getGroupsClient(creds)
	if err != nil {
		return resources.Group{}, err
	}
	return groupClient.Get(ctx, resourceName)
}

// CleanupE2EResources deletes the resource group created during the testing
func CleanupE2EResources(ctx context.Context, creds auth.Creds, resourceName string) error {
	groupClient, err := getGroupsClient(creds)
	if err != nil {
		return err
	}

	deleteGroupFuture, err := groupClient.Delete(ctx, resourceName)
	if err != nil {
		log.Printf("failed to delete group %s: %s\n", resourceName, err.Error())
	}

	autorestClient := autorest.Client{
		PollingDelay:    autorest.DefaultPollingDelay,
		PollingDuration: autorest.DefaultPollingDuration,
		RetryAttempts:   autorest.DefaultRetryAttempts,
		RetryDuration:   autorest.DefaultRetryDuration,
		Authorizer:      groupClient.Authorizer,
	}

	log.Println("waiting for the group deletion")

	err = deleteGroupFuture.WaitForCompletionRef(context.Background(), autorestClient)
	if err != nil {
		log.Printf("failed to wait for deletion %s\n", err.Error())
		return err
	}

	log.Printf("group %s deleted\n", resourceName)
	return nil
}

func getGroupsClient(creds auth.Creds) (resources.GroupsClient, error) {
	groupsClient := resources.NewGroupsClient(creds.SubscriptionID)
	a, err := getAuthorizer(creds)
	if err != nil {
		return resources.GroupsClient{}, err
	}
	groupsClient.Authorizer = a
	return groupsClient, nil
}

func getAuthorizer(creds auth.Creds) (autorest.Authorizer, error) {
	credsConfig := azureAuth.NewClientCredentialsConfig(creds.ClientID, creds.ClientSecret, creds.TenantID)
	return credsConfig.Authorizer()
}
