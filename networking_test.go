package azureactuator

import (
	"testing"
)

func TestGetIP(t *testing.T) {
	clusterProviderConfig := mockAzureClusterProviderConfig(t)
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse configs: %v", err)
	}

	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	defer deleteTestResourceGroup(t, azure, clusterProviderConfig.ResourceGroup)
	_, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create deployment: %v", err)
	}

	_, err = azure.GetIP(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to get public IP address: %v", err)
	}
}
