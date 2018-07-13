package azure_provider

import (
	"context"
	"fmt"

	"github.com/Azure/go-autorest/autorest"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-01-01/network"
	azureconfigv1 "github.com/platform9/azure-provider/azureproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type IPAddressClientWrapper struct {
	client network.PublicIPAddressesClient
	mock   *IPAddressClientMock
}

type IPAddressClientMock struct{}

func (wrapper *IPAddressClientWrapper) Get(ctx context.Context, rg string, IPName string, expand string) (network.PublicIPAddress, error) {
	if wrapper.mock == nil {
		return wrapper.client.Get(ctx, rg, IPName, expand)
	} else {
		ip := "1.1.1.1"
		return network.PublicIPAddress{PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{IPAddress: &ip}}, nil
	}
}

func getPublicIPAddressesClient(SubscriptionID string) *IPAddressClientWrapper {
	if SubscriptionID == "test" {
		return &IPAddressClientWrapper{
			mock: &IPAddressClientMock{},
		}
	} else {
		return &IPAddressClientWrapper{
			client: network.NewPublicIPAddressesClient(SubscriptionID),
		}
	}
}

func (wrapper *IPAddressClientWrapper) SetAuthorizer(Authorizer autorest.Authorizer) {
	if wrapper.mock == nil {
		wrapper.client.Authorizer = Authorizer
	}
}

func (azure *AzureClient) GetIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err := azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		return "", err
	}

	publicIPAddressClient := getPublicIPAddressesClient(azure.SubscriptionID)
	publicIPAddressClient.SetAuthorizer(azure.Authorizer)

	publicIP, err := publicIPAddressClient.Get(azure.ctx, clusterConfig.ResourceGroup, getPublicIPName(machine), "")
	if err != nil {
		return "", err
	}

	return *publicIP.IPAddress, nil
}

func getPublicIPName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("ClusterAPIIP-%s", machine.ObjectMeta.Name)
}

func getNetworkInterfaceName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("ClusterAPINIC-%s", getVMName(machine))
}
