package machine

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
)

func TestCreateGroup(t *testing.T) {
	rg := "ClusterAPI-test-CI-create-rg"
	clusterConfigFile := "testconfigs/cluster-ci-create-rg.yaml"
	cluster, _, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	clusterConfig := mockAzureClusterProviderConfig(t, rg)
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	defer deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	group, err := azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	groupsClient := resources.NewGroupsClient(azure.SubscriptionID)
	groupsClient.Authorizer = azure.Authorizer
	_, err = groupsClient.Get(azure.ctx, *group.Name)
	if err != nil {
		t.Fatalf("unable to get created resource group, %v: %v", group.Name, err)
	}
}

func TestCreateGroupUnit(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-create-rg.yaml"
	cluster, _, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create mock azure client: %v", err)
	}
	group, err := azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	if group == nil {
		t.Fatalf("unable to get created resource group: %v", err)
	}
}

func TestCheckResourceGroupExists(t *testing.T) {
	rg := "ClusterAPI-test-CI-rg-exists"
	clusterConfigFile := "testconfigs/cluster-ci-rg-exists.yaml"
	cluster, _, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	clusterConfig := mockAzureClusterProviderConfig(t, rg)
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	exists, err := azure.checkResourceGroupExists(cluster)
	if exists {
		t.Fatalf("got resource group exists that should not have existed")
	}
	if err != nil {
		t.Fatalf("error checking if resource group exists: %v", err)
	}
	defer deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	exists, err = azure.checkResourceGroupExists(cluster)
	if !exists {
		t.Fatalf("got resource group does not exist that should have existed")
	}
	if err != nil {
		t.Fatalf("error checking if resource group exists: %v", err)
	}
}

func TestCheckResourceGroupExistsUnit(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-rg-exists.yaml"
	cluster, _, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create mock azure client: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("could not create new resouce group: %v", err)
	}
	exists, err := azure.checkResourceGroupExists(cluster)
	if !exists {
		t.Fatalf("did not get resource group that should have existed")
	}
	if err != nil {
		t.Fatalf("error checking if resource group exists: %v", err)
	}
}

func deleteTestResourceGroup(t *testing.T, azure *AzureClient, resourceGroupName string) {
	t.Helper()
	//Clean up the mess
	groupsClient := resources.NewGroupsClient(azure.SubscriptionID)
	groupsClient.Authorizer = azure.Authorizer
	groupsDeleteFuture, _ := groupsClient.Delete(azure.ctx, resourceGroupName)
	_ = groupsDeleteFuture.Future.WaitForCompletion(azure.ctx, groupsClient.BaseClient.Client)
}
