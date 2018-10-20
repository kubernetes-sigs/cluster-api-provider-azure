package machine

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/wrappers"
)

// Test creating a deployment with no existing deployments
func TestCreateOrUpdateDeployment(t *testing.T) {
	rg := "ClusterAPI-test-CI-create-update"
	clusterConfigFile := "testconfigs/cluster-ci-create-update.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	clusterConfig := mockAzureClusterProviderConfig(t, rg)
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create resource group: %v", err)
	}
	clusterActuator, err := createClusterActuator()
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = clusterActuator.Reconcile(cluster)
	if err != nil {
		t.Fatalf("failed to reconcile cluster: %v", err)
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

// Test creating a deployment with no existing deployments
func TestCreateOrUpdateDeploymentUnit(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-create-update.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create mock azure client: %v", err)
	}
	deployment, err := azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create deployment: %v", err)
	}
	if deployment == nil {
		t.Fatal("did not return created deployment: deployment == nil")
	}
	if *deployment.Name != wrappers.MockDeploymentName {
		t.Fatalf("returned deployment name does not match expected: %v != %v", *deployment.Name, wrappers.MockDeploymentName)
	}
}

// Test attempting to create a deployment when it has already been created. Tests idempotence.
func TestCreateOrUpdateDeploymentWExisting(t *testing.T) {
	rg := "ClusterAPI-test-CI-create-update-existing"
	clusterConfigFile := "testconfigs/cluster-ci-create-update-existing.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	clusterConfig := mockAzureClusterProviderConfig(t, rg)
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create resource group: %v", err)
	}
	clusterActuator, err := createClusterActuator()
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = clusterActuator.Reconcile(cluster)
	if err != nil {
		t.Fatalf("failed to reconcile cluster: %v", err)
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

// Test attempting to create a deployment when it has already been created. Tests idempotence.
func TestCreateOrUpdateDeploymentWExistingUnit(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-create-update-existing.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create mock azure client: %v", err)
	}
	deployment, err := azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create deployment: %v", err)
	}
	if deployment == nil {
		t.Fatal("did not return created deployment: deployment == nil")
	}
	deployment, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create deployment when it has already been created: %v", err)
	}
	if deployment == nil {
		t.Fatal("did not return created deployment when it had already been created: deployment == nil")
	}
	if *deployment.Name != wrappers.MockDeploymentName {
		t.Fatalf("returned deployment name does not match expected: %v != %v", *deployment.Name, wrappers.MockDeploymentName)
	}
}

// Test ability to see if a VM already exists
func TestVMIfExists(t *testing.T) {
	rg := "ClusterAPI-test-CI-vm-exists"
	clusterConfigFile := "testconfigs/cluster-ci-vm-exists.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	clusterConfig := mockAzureClusterProviderConfig(t, rg)
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create resource group: %v", err)
	}
	clusterActuator, err := createClusterActuator()
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = clusterActuator.Reconcile(cluster)
	if err != nil {
		t.Fatalf("failed to reconcile cluster: %v", err)
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

// Test ability to see if a VM already exists
func TestVMIfExistsUnit(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-vm-exists.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	vm, err := azure.vmIfExists(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to check if vm exists: %v", err)
	}
	if vm == nil {
		t.Fatalf("unable to find existing vm")
	}
	if *vm.Name != wrappers.MockDeploymentName {
		t.Fatalf("returned deployment name does not match expected: %v != %v", *vm.Name, wrappers.MockDeploymentName)
	}
}

// Test ability to delete a single VM with multiple VMs existing
func TestDeleteSingleVM(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-delete-single.yaml"
	//Set up
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = azure.decodeMachineProviderConfig(machines[0].Spec.ProviderConfig)
	if err != nil {
		t.Fatalf("unable to parse machine provider config 1: %v", err)
	}
	_, err = azure.decodeMachineProviderConfig(machines[1].Spec.ProviderConfig)
	if err != nil {
		t.Fatalf("unable to parse machine provider config 2: %v", err)
	}
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		t.Fatalf("unable to parse cluster provider config: %v", err)
	}

	//Create resource groups and VMs
	defer deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	clusterActuator, err := createClusterActuator()
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = clusterActuator.Reconcile(cluster)
	if err != nil {
		t.Fatalf("failed to reconcile cluster: %v", err)
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

func TestDeleteSingleVMUnit(t *testing.T) {
	rg := "ClusterAPI-test-CI-delete-single"
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	deploymentName := wrappers.MockDeploymentName
	vm := &resources.DeploymentExtended{Name: &deploymentName}
	err = azure.deleteVM(vm, rg)
	if err != nil {
		t.Fatalf("unable to delete created vm 1: %v", err)
	}
}
