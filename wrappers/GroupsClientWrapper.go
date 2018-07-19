package wrappers

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"net/http"
)

type GroupsClientWrapper struct {
	client resources.GroupsClient
	mock   *GroupsClientMock
}

type GroupsClientMock struct{}

func GetGroupsClient(SubscriptionID string) *GroupsClientWrapper {
	if SubscriptionID == MockSubscriptionID {
		return &GroupsClientWrapper{
			mock: &GroupsClientMock{},
		}
	}
	return &GroupsClientWrapper{
		client: resources.NewGroupsClient(SubscriptionID),
	}
}

func (wrapper *GroupsClientWrapper) SetAuthorizer(Authorizer autorest.Authorizer) {
	if wrapper.mock == nil {
		wrapper.client.BaseClient.Client.Authorizer = Authorizer
	}
}

func (wrapper *GroupsClientWrapper) CreateOrUpdate(ctx context.Context, rgName string, rg resources.Group) (resources.Group, error) {
	if wrapper.mock == nil {
		return wrapper.client.CreateOrUpdate(ctx, rgName, rg)
	}
	return resources.Group{}, nil
}

func (wrapper *GroupsClientWrapper) CheckExistence(ctx context.Context, rgName string) (autorest.Response, error) {
	if wrapper.mock == nil {
		return wrapper.client.CheckExistence(ctx, rgName)
	}
	return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil

}
