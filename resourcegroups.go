package azure_provider

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	azureconfigv1 "github.com/platform9/azure-provider/azureproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type GroupsClientWrapper struct {
	client resources.GroupsClient
	mock   *GroupsClientMock
}

type GroupsClientMock struct{}

func getGroupsClient(SubscriptionID string) *GroupsClientWrapper {
	if SubscriptionID == "test" {
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

func (azure *AzureClient) createOrUpdateGroup(cluster *clusterv1.Cluster) (*resources.Group, error) {
	//Parse in provider configs
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err := azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		return nil, err
	}
	groupsClient := getGroupsClient(azure.SubscriptionID)
	groupsClient.SetAuthorizer(azure.Authorizer)
	group, err := groupsClient.CreateOrUpdate(
		azure.ctx,
		clusterConfig.ResourceGroup,
		resources.Group{
			Location: to.StringPtr(clusterConfig.Location)})
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (azure *AzureClient) checkResourceGroupExists(cluster *clusterv1.Cluster) (bool, error) {
	//Parse in provider configs
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err := azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		return false, err
	}
	groupsClient := getGroupsClient(azure.SubscriptionID)
	groupsClient.SetAuthorizer(azure.Authorizer)
	response, err := groupsClient.CheckExistence(azure.ctx, clusterConfig.ResourceGroup)
	if err != nil {
		return false, err
	}
	return response.StatusCode != 404, nil
}
