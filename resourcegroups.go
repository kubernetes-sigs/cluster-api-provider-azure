package azure_provider

import (
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest/to"
	azureconfigv1 "github.com/platform9/azure-provider/azureproviderconfig/v1alpha1"
	"github.com/platform9/azure-provider/wrappers"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (azure *AzureClient) createOrUpdateGroup(cluster *clusterv1.Cluster) (*resources.Group, error) {
	//Parse in provider configs
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err := azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		return nil, err
	}
	groupsClient := wrappers.GetGroupsClient(azure.SubscriptionID)
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
	groupsClient := wrappers.GetGroupsClient(azure.SubscriptionID)
	groupsClient.SetAuthorizer(azure.Authorizer)
	response, err := groupsClient.CheckExistence(azure.ctx, clusterConfig.ResourceGroup)
	if err != nil {
		return false, err
	}
	return response.StatusCode != 404, nil
}
