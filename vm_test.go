package azure_provider

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	azureconfigv1 "github.com/platform9/azure-provider/azureproviderconfig/v1alpha1"
)

func TestCreateOrUpdateDeployment(t *testing.T) {
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	clusterConfig := mockAzureClusterProviderConfig(t)
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create resource group: %v", err)
	}
	deployment, err := azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create deployment: %v", err)
	}
	deploymentsClient := resources.NewDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.Authorizer = azure.Authorizer
	_, err = deploymentsClient.Get(azure.ctx, clusterConfig.ResourceGroup, *deployment.Name)
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to get created deployment: %v", err)
	}
	deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
}

// Test attempting to create a deployment when it has already been created
func TestCreateOrUpdateDeploymentWExisting(t *testing.T) {
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	clusterConfig := mockAzureClusterProviderConfig(t)
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create resource group: %v", err)
	}
	deployment, err := azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create deployment: %v", err)
	}
	deploymentsClient := resources.NewDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.Authorizer = azure.Authorizer
	_, err = deploymentsClient.Get(azure.ctx, clusterConfig.ResourceGroup, *deployment.Name)
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to get created deployment: %v", err)
	}

	_, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create/update deployment after deployment has been created already: %v", err)
	}
	deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
}

func TestVMIfExists(t *testing.T) {
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	clusterConfig := mockAzureClusterProviderConfig(t)
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create resource group: %v", err)
	}
	_, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create deployment: %v", err)
	}
	//Try to grab the vm we just created
	vm, err := azure.vmIfExists(cluster, machines[0])
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to check if vm exists: %v", err)
	}
	if vm == nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to find existing vm")
	}
	deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	//Ensure we get nothing when trying to get the vm after deleting it
	vm, _ = azure.vmIfExists(cluster, machines[0])
	if vm != nil {
		t.Fatalf("got vm that should have been deleted")
	}
}

func TestDeleteSingleVM(t *testing.T) {
	//Set up
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	var machineConfig1 azureconfigv1.AzureMachineProviderConfig
	err = azure.decodeMachineProviderConfig(machines[0].Spec.ProviderConfig, &machineConfig1)
	if err != nil {
		t.Fatalf("unable to parse machine provider config 1: %v", err)
	}
	var machineConfig2 azureconfigv1.AzureMachineProviderConfig
	err = azure.decodeMachineProviderConfig(machines[1].Spec.ProviderConfig, &machineConfig2)
	if err != nil {
		t.Fatalf("unable to parse machine provider config 2: %v", err)
	}
	var clusterConfig azureconfigv1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		t.Fatalf("unable to parse cluster provider config: %v", err)
	}

	//Create resource groups and VMs
	defer deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	_, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create deployment 1: %v", err)
	}
	_, err = azure.createOrUpdateDeployment(cluster, machines[1])
	if err != nil {
		t.Fatalf("unable to create deployment 2: %v", err)
	}

	//Ensure we can get vm 1
	vm, err := azure.vmIfExists(cluster, machines[0])
	if vm == nil {
		t.Fatalf("unable to get created vm 1: %v", vm)
	}
	if err != nil {
		t.Fatalf("error while getting created vm 1: %v", err)
	}

	//Delete vm 1
	err = azure.deleteVM(vm, clusterConfig.ResourceGroup)
	if err != nil {
		t.Fatalf("unable to delete created vm 1: %v", err)
	}

	//See if we can get vm 1 (we shouldn't be able to)
	vm, _ = azure.vmIfExists(cluster, machines[0])
	if vm != nil {
		t.Fatalf("got vm that should have been deleted")
	}

	//See if we can get vm 2 (we should be able to)
	vm, err = azure.vmIfExists(cluster, machines[1])
	if vm == nil {
		t.Fatalf("unable to get vm that should NOT have been deleted")
	}
	if err != nil {
		t.Fatalf("error getting vm that should NOT have been deleted: %v", err)
	}
}
