package azure_provider

import (
	"errors"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	azureconfigv1 "github.com/platform9/azure-provider/azureproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

//Assumes resource group has already been created and has the name found in clusterConfig.ResourceGroup
func (azure *AzureClient) createOrUpdateDeployment(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (*resources.DeploymentExtended, error) {
	//Parse in provider configs
	var machineConfig azureconfigv1.AzureMachineProviderConfig
	err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig, &machineConfig)
	if err != nil {
		return nil, err
	}
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		return nil, err
	}
	//Parse the ARM template
	template, err := readJSON(templateFile)
	if err != nil {
		return nil, err
	}
	//Convert machine provider config to ARM parameters
	params, err := azure.convertMachineToDeploymentParams(machine)
	if err != nil {
		return nil, err
	}
	(*params)["vm_password"] = map[string]string{
		"value": azure.VMPassword,
	}
	deploymentName := machine.ObjectMeta.Name
	deploymentsClient := resources.NewDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.Authorizer = azure.Authorizer
	res, err := deploymentsClient.Validate(azure.ctx, clusterConfig.ResourceGroup, deploymentName, resources.Deployment{
		Properties: &resources.DeploymentProperties{
			Template:   template,
			Parameters: params,
			Mode:       resources.Incremental,
		},
	})
	if res.Error != nil {
		fmt.Printf("%+v\n", *(*res.Error.Details)[0].Message)
		return nil, errors.New(*res.Error.Message)
	}
	if err != nil {
		return nil, err
	}
	deploymentFuture, err := deploymentsClient.CreateOrUpdate(
		azure.ctx,
		clusterConfig.ResourceGroup,
		deploymentName,
		resources.Deployment{
			Properties: &resources.DeploymentProperties{
				Template:   template,
				Parameters: params,
				Mode:       resources.Incremental,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	err = deploymentFuture.Future.WaitForCompletion(azure.ctx, deploymentsClient.BaseClient.Client)
	if err != nil {
		return nil, err
	}
	deployment, err := deploymentFuture.Result(deploymentsClient)

	// Work around possible bugs or late-stage failures
	if deployment.Name == nil || err != nil {
		deployment, _ = deploymentsClient.Get(azure.ctx, clusterConfig.ResourceGroup, deploymentName)
	}
	return &deployment, nil
}

func (azure *AzureClient) vmIfExists(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (*resources.DeploymentExtended, error) {
	//Parse in provider configs
	var machineConfig azureconfigv1.AzureMachineProviderConfig
	err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig, &machineConfig)
	if err != nil {
		return nil, err
	}
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		return nil, err
	}
	deploymentName := machine.ObjectMeta.Name
	deploymentsClient := resources.NewDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.Authorizer = azure.Authorizer
	deployment, err := deploymentsClient.Get(azure.ctx, clusterConfig.ResourceGroup, deploymentName)
	if err != nil {
		return nil, err
	}
	if deployment.StatusCode != 200 {
		return nil, nil
	}
	return &deployment, nil
}

func (azure *AzureClient) getLogin(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, string, error) {
	//Parse in provider configs
	var machineConfig azureconfigv1.AzureMachineProviderConfig
	err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig, &machineConfig)
	if err != nil {
		return "", "", err
	}
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		return "", "", err
	}
	params, err := readJSON(parametersFile)
	if err != nil {
		log.Fatalf("Unable to read parameters. Get login information with `az network public-ip list -g %s", clusterConfig.ResourceGroup)
	}

	addressClient := network.NewPublicIPAddressesClient(azure.SubscriptionID)
	addressClient.Authorizer = azure.Authorizer
	ipName := (*params)["publicIPAddresses_Quickstart_ip_name"].(map[string]interface{})
	ipAddress, err := addressClient.Get(azure.ctx, clusterConfig.ResourceGroup, ipName["value"].(string), "")
	if err != nil {
		log.Fatalf("Unable to get IP information. Try using `az network public-ip list -g %s", clusterConfig.ResourceGroup)
	}

	vmUser := (*params)["vm_user"].(map[string]interface{})

	log.Printf("Log in with ssh: %s@%s, password: %s",
		vmUser["value"].(string),
		*ipAddress.PublicIPAddressPropertiesFormat.IPAddress,
		azure.VMPassword)
	return fmt.Sprintf("%s@%s", vmUser["value"].(string), *ipAddress.PublicIPAddressPropertiesFormat.IPAddress), azure.VMPassword, nil
}

func (azure *AzureClient) deleteVM(deployment *resources.DeploymentExtended, resourceGroupName string) error {
	deploymentsClient := resources.NewDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.Authorizer = azure.Authorizer
	deploymentDeleteFuture, err := deploymentsClient.Delete(azure.ctx, resourceGroupName, *deployment.Name)
	if err != nil {
		return err
	}
	deploymentDeleteFuture.Future.WaitForCompletion(azure.ctx, deploymentsClient.BaseClient.Client)
	return nil
}

func getVMName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("ClusterAPIVM-%s", machine.ObjectMeta.Name)
}

func getOSDiskName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s_OSDisk", getVMName(machine))
}
