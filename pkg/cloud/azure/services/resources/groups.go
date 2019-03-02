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

package resources

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog"
)

// ReconcileResourceGroup reconciles the resource group of the given cluster.
func (s *Service) ReconcileResourceGroup() (err error) {
	klog.V(2).Info("Reconciling resource group")

	// TODO: Refactor
	// Reconcile resource group
	_, err = s.CreateOrUpdateGroup(s.scope.ClusterConfig.ResourceGroup, s.scope.ClusterConfig.Location)
	if err != nil {
		return fmt.Errorf("failed to create or update resource group: %v", err)
	}

	klog.V(2).Info("Reconciling resource group completed successfully")
	return nil
}

// DeleteResourceGroup deletes the network of the given cluster.
func (s *Service) DeleteResourceGroup() (err error) {
	klog.V(2).Info("Deleting resource group")

	// TODO: Refactor
	resp, err := s.CheckGroupExistence(s.scope.ClusterConfig.ResourceGroup)
	if err != nil {
		return fmt.Errorf("error checking for resource group existence: %v", err)
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("resource group %v does not exist", s.scope.ClusterConfig.ResourceGroup)
	}

	groupsDeleteFuture, err := s.DeleteGroup(s.scope.ClusterConfig.ResourceGroup)
	if err != nil {
		return fmt.Errorf("error deleting resource group: %v", err)
	}
	err = s.WaitForGroupsDeleteFuture(groupsDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for resource group deletion: %v", err)
	}

	klog.V(2).Info("Deleting resource group completed successfully")
	return nil
}

// CreateOrUpdateGroup creates or updates an azure resource group.
func (s *Service) CreateOrUpdateGroup(resourceGroupName string, location string) (resources.Group, error) {
	return s.scope.Groups.CreateOrUpdate(s.scope.Context, resourceGroupName, resources.Group{Location: to.StringPtr(location)})
}

// DeleteGroup deletes an azure resource group.
func (s *Service) DeleteGroup(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
	return s.scope.Groups.Delete(s.scope.Context, resourceGroupName)
}

// CheckGroupExistence checks if the resource group exists or not.
func (s *Service) CheckGroupExistence(resourceGroupName string) (autorest.Response, error) {
	return s.scope.Groups.CheckExistence(s.scope.Context, resourceGroupName)
}

// WaitForGroupsDeleteFuture returns when the DeleteGroup operation completes.
func (s *Service) WaitForGroupsDeleteFuture(future resources.GroupsDeleteFuture) error {
	return future.WaitForCompletionRef(s.scope.Context, s.scope.Groups.Client)
}
