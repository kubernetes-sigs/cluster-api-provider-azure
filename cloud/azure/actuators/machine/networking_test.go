package machine

import (
	"testing"
)

func TestGetIP(t *testing.T) {
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
