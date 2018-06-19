package azure_provider

import (
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest/to"
	azureconfigv1 "github.com/platform9/azure-provider/azureproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (azure *AzureClient) createOrUpdateGroup(cluster *clusterv1.Cluster) (*resources.Group, error) {
	//Parse in provider configs
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err := azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		return nil, err
	}
	groupsClient := resources.NewGroupsClient(azure.SubscriptionID)
	groupsClient.Authorizer = azure.Authorizer
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
