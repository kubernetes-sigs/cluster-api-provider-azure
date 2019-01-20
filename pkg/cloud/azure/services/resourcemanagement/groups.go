/*
Copyright 2018 The Kubernetes Authors.
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

package resourcemanagement

import (
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
)

// CreateOrUpdateGroup creates or updates an azure resource group.
func (s *Service) CreateOrUpdateGroup(resourceGroupName string, location string) (resources.Group, error) {
	return s.GroupsClient.CreateOrUpdate(s.ctx, resourceGroupName, resources.Group{Location: to.StringPtr(location)})
}

// DeleteGroup deletes an azure resource group.
func (s *Service) DeleteGroup(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
	return s.GroupsClient.Delete(s.ctx, resourceGroupName)
}

// CheckGroupExistence checks oif the resource group exists or not.
func (s *Service) CheckGroupExistence(resourceGroupName string) (autorest.Response, error) {
	return s.GroupsClient.CheckExistence(s.ctx, resourceGroupName)
}

// WaitForGroupsDeleteFuture returns when the DeleteGroup operation completes.
func (s *Service) WaitForGroupsDeleteFuture(future resources.GroupsDeleteFuture) error {
	return future.WaitForCompletionRef(s.ctx, s.GroupsClient.Client)
}
