package azure_provider

import (
	"fmt"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-01-01/network"
	azureconfigv1 "github.com/platform9/azure-actuator/azureproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (azure *AzureClient) GetIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	//Parse in configurations
	/*
		var machineConfig azureconfigv1.AzureMachineProviderConfig
		err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig, &machineConfig)
		if err != nil {
			return "", err
		}
	*/
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err := azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		return "", err
	}

	publicIPAddressClient := network.NewPublicIPAddressesClient(azure.SubscriptionID)
	publicIPAddressClient.Authorizer = azure.Authorizer

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
