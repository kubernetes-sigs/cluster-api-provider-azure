package machine

import (
	"testing"
)

func TestGetIP(t *testing.T) {
	rg := "ClusterAPI-test-CI-get-ip"
	clusterConfigFile := "testconfigs/cluster-ci-get-ip.yaml"
	clusterProviderConfig := mockAzureClusterProviderConfig(t, rg)
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
	clusterActuator, err := createClusterActuator()
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = clusterActuator.Reconcile(cluster)
	if err != nil {
		t.Fatalf("failed to reconcile cluster: %v", err)
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
func TestGetIPUnit(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-get-ip.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse configs: %v", err)
	}
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create mock azure client: %v", err)
	}
	expectedIP := "1.1.1.1"
	actualIP, err := azure.GetIP(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to get public IP address: %v", err)
	}
	if actualIP != expectedIP {
		t.Fatalf("actualIP does not match expectedIP: %v != %v", actualIP, expectedIP)
	}
}
