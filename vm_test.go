package azureactuator

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
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
	defer deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	deployment, err := azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create deployment: %v", err)
	}
	deploymentsClient := resources.NewDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.Authorizer = azure.Authorizer
	_, err = deploymentsClient.Get(azure.ctx, clusterConfig.ResourceGroup, *deployment.Name)
	if err != nil {
		t.Fatalf("unable to get created deployment: %v", err)
	}
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
	defer deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	deployment, err := azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create deployment: %v", err)
	}
	deploymentsClient := resources.NewDeploymentsClient(azure.SubscriptionID)
	deploymentsClient.Authorizer = azure.Authorizer
	_, err = deploymentsClient.Get(azure.ctx, clusterConfig.ResourceGroup, *deployment.Name)
	if err != nil {
		t.Fatalf("unable to get created deployment: %v", err)
	}

	_, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create/update deployment after deployment has been created already: %v", err)
	}
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
	defer deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	_, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create deployment: %v", err)
	}
	//Try to grab the vm we just created
	vm, err := azure.vmIfExists(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to check if vm exists: %v", err)
	}
	if vm == nil {
		t.Fatalf("unable to find existing vm")
	}
	//Ensure we get nothing when trying to get the vm after deleting it
	vm, err = azure.vmIfExists(cluster, machines[0])
	if vm != nil {
		t.Fatalf("got vm that should have been deleted")
	}
}
