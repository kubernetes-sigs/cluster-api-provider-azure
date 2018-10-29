package machine

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/wrappers"
)

// Test creating a deployment with no existing deployments
func TestCreateOrUpdateDeployment(t *testing.T) {
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

func TestDeleteSingleVM(t *testing.T) {
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
