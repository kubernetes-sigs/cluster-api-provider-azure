package azure_provider

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
)

func TestCreateGroup(t *testing.T) {
	cluster, _, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	clusterConfig := mockAzureClusterProviderConfig(t)
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

func deleteTestResourceGroup(t *testing.T, azure *AzureClient, resourceGroupName string) {
	t.Helper()
	//Clean up the mess
	groupsClient := resources.NewGroupsClient(azure.SubscriptionID)
	groupsClient.Authorizer = azure.Authorizer
	groupsDeleteFuture, _ := groupsClient.Delete(azure.ctx, resourceGroupName)
	_ = groupsDeleteFuture.Future.WaitForCompletion(azure.ctx, groupsClient.BaseClient.Client)
}
