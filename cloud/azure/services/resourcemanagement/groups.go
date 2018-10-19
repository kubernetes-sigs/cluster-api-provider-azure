package resourcemanagement

import (
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
)

func (s *Service) CreateOrUpdateGroup(resourceGroupName string, location string) (resources.Group, error) {
	return s.GroupsClient.CreateOrUpdate(s.ctx, resourceGroupName, resources.Group{Location: to.StringPtr(location)})
}

func (s *Service) DeleteGroup(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
	return s.GroupsClient.Delete(s.ctx, resourceGroupName)
}

func (s *Service) CheckGroupExistence(resourceGroupName string) (autorest.Response, error) {
	return s.GroupsClient.CheckExistence(s.ctx, resourceGroupName)
}

func (s *Service) WaitForGroupsDeleteFuture(future resources.GroupsDeleteFuture) error {
	return future.WaitForCompletionRef(s.ctx, s.GroupsClient.Client)
}
