package wrappers

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"net/http"
)

type GroupsClientWrapper struct {
	Client resources.GroupsClient
	mock   *GroupsClientMock
}

type GroupsClientMock struct{}

var mockGroups = make([]string, 0)

func GetGroupsClient(SubscriptionID string) *GroupsClientWrapper {
	if SubscriptionID == MockSubscriptionID {
		return &GroupsClientWrapper{
			mock: &GroupsClientMock{},
		}
	}
	return &GroupsClientWrapper{
		Client: resources.NewGroupsClient(SubscriptionID),
	}
}

func (wrapper *GroupsClientWrapper) SetAuthorizer(Authorizer autorest.Authorizer) {
	if wrapper.mock == nil {
		wrapper.Client.BaseClient.Client.Authorizer = Authorizer
	}
}

func (wrapper *GroupsClientWrapper) CreateOrUpdate(ctx context.Context, rgName string, rg resources.Group) (resources.Group, error) {
	if wrapper.mock == nil {
		return wrapper.Client.CreateOrUpdate(ctx, rgName, rg)
	}
	if !contains(mockGroups, rgName) {
		mockGroups = append(mockGroups, rgName)
	}
	str := rgName
	return resources.Group{Name: &str}, nil
}

func (wrapper *GroupsClientWrapper) Delete(ctx context.Context, rgName string) (*GroupsDeleteFutureWrapper, error) {
	if wrapper.mock == nil {
		future, err := wrapper.Client.Delete(ctx, rgName)
		return &GroupsDeleteFutureWrapper{mock: false, future: future}, err
	}
	if contains(mockGroups, rgName) {
		mockGroups = remove(mockGroups, rgName)
	}
	return &GroupsDeleteFutureWrapper{mock: true}, nil
}

func (wrapper *GroupsClientWrapper) CheckExistence(ctx context.Context, rgName string) (autorest.Response, error) {
	if wrapper.mock == nil {
		return wrapper.Client.CheckExistence(ctx, rgName)
	}
	if contains(mockGroups, rgName) {
		return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
	}
	return autorest.Response{Response: &http.Response{StatusCode: 404}}, nil

}
