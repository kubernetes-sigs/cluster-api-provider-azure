package machine

import (
	"fmt"

	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/wrappers"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// Return the ip address of an existing machine based on the cluster and machine spec passed.
func (azure *AzureClient) GetIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return "", err
	}

	publicIPAddressClient := wrappers.GetPublicIPAddressesClient(azure.SubscriptionID)
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
