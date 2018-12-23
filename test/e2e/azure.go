package e2e

import (
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/platform9/cluster-api-provider-azure/pkg/cloud/azure/services"
	"github.com/platform9/cluster-api-provider-azure/pkg/cloud/azure/services/compute"
	"github.com/platform9/cluster-api-provider-azure/pkg/cloud/azure/services/network"
	"github.com/platform9/cluster-api-provider-azure/pkg/cloud/azure/services/resourcemanagement"
)

func NewAzureServicesClient(subscriptionID string) (*services.AzureClients, error) {
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, err
	}

	azureComputeClient := compute.NewService(subscriptionID)
	azureComputeClient.SetAuthorizer(authorizer)
	azureResourceManagementClient := resourcemanagement.NewService(subscriptionID)
	azureResourceManagementClient.SetAuthorizer(authorizer)
	azureNetworkClient := network.NewService(subscriptionID)
	azureNetworkClient.SetAuthorizer(authorizer)
	return &services.AzureClients{
		Compute:            azureComputeClient,
		Resourcemanagement: azureResourceManagementClient,
		Network:            azureNetworkClient,
	}, nil
}
