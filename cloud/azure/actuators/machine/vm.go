package machine

import (
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/wrappers"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// Create a machine based on the cluster and machine spec passed.
// Assumes resource group has already been created and has the name found in clusterConfig.ResourceGroup
func (azure *AzureClient) createOrUpdateDeployment(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (*resources.DeploymentExtended, error) {
	// Parse in provider configs
	_, err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return nil, err
	}
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return nil, err
	}
	// Parse the ARM template
	template, err := readJSON(templateFile)
	if err != nil {
		return nil, err
	}
	// Convert machine provider config to ARM parameters
	params, err := azure.convertMachineToDeploymentParams(cluster, machine)
	if err != nil {
		return nil, err
	}
	deploymentName := machine.ObjectMeta.Name
	deploymentsClient := wrappers.GetDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.SetAuthorizer(azure.Authorizer)
	res, err := deploymentsClient.Validate(azure.ctx, clusterConfig.ResourceGroup, deploymentName, resources.Deployment{
		Properties: &resources.DeploymentProperties{
			Template:   template,
			Parameters: params,
			Mode:       resources.Incremental, // Do not delete and re-create matching resources that already exist
		},
	})
	if res.Error != nil {
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
	err = deploymentFuture.WaitForCompletion(azure.ctx, deploymentsClient.Client.BaseClient.Client)
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
	// Parse in provider configs
	_, err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return nil, err
	}
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return nil, err
	}
	deploymentName := machine.ObjectMeta.Name
	deploymentsClient := wrappers.GetDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.SetAuthorizer(azure.Authorizer)
	response, err := deploymentsClient.CheckExistence(azure.ctx, clusterConfig.ResourceGroup, deploymentName)
	if err != nil {
		return nil, err
	}
	exists := response.StatusCode != 404
	if !exists {
		return nil, nil
	}
	deployment, err := deploymentsClient.Get(azure.ctx, clusterConfig.ResourceGroup, deploymentName)
	if err != nil {
		return nil, err
	}
	if deployment.StatusCode != 200 {
		return nil, nil
	}
	return &deployment, nil
}

func (azure *AzureClient) deleteVM(deployment *resources.DeploymentExtended, resourceGroupName string) error {
	deploymentsClient := wrappers.GetDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.SetAuthorizer(azure.Authorizer)
	deploymentDeleteFuture, err := deploymentsClient.Delete(azure.ctx, resourceGroupName, *deployment.Name)
	if err != nil {
		return err
	}
	deploymentDeleteFuture.WaitForCompletion(azure.ctx, deploymentsClient.Client.BaseClient.Client)
	return nil
}

func getVMName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("ClusterAPIVM-%s", machine.ObjectMeta.Name)
}

func getOSDiskName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s_OSDisk", getVMName(machine))
}
