/*
Copyright 2018 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package azure_provider

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	v1alpha1 "github.com/platform9/azure-provider/azureproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

const (
	machineConfigFile = "testconfigs/machines.yaml"
)

func TestCreate(t *testing.T) {
	clusterConfigFile := "cluster-ci-create.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files :%v", err)
	}
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	var clusterConfig v1alpha1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		t.Fatalf("unable to parse cluster provider config: %v", err)
	}
	defer deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	for _, machine := range machines {
		err = azure.Create(cluster, machine)
		if err != nil {
			t.Fatalf("unable to create machine: %v", err)
		}
	}
}

func TestUpdate(t *testing.T) {
	// TODO: write test
	return
}

func TestDelete(t *testing.T) {
	clusterConfigFile := "cluster-ci-delete.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files :%v", err)
	}
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	var clusterConfig v1alpha1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		t.Fatalf("unable to parse cluster provider config: %v", err)
	}
	var machineConfig v1alpha1.AzureMachineProviderConfig
	err = azure.decodeMachineProviderConfig(machines[0].Spec.ProviderConfig, &machineConfig)
	if err != nil {
		t.Fatalf("unable to parse machine provider config: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	_, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create deployment: %v", err)
	}
	err = azure.Delete(cluster, machines[0])
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to delete cluster: %v", err)
	}
}

func TestExists(t *testing.T) {
	// TODO: write test
	return
}

func TestParseProviderConfigs(t *testing.T) {
	clusterConfigFile := "cluster-ci-parse-providers.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	var clusterConfig v1alpha1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		t.Fatalf("unable to parse cluster provider config: %v", err)
	}
	for _, machine := range machines {
		var machineConfig v1alpha1.AzureMachineProviderConfig
		err = azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig, &machineConfig)
		if err != nil {
			t.Fatalf("unable to parse machine provider config: %v", err)
		}
	}
}

/*func TestGetKubeConfig(t *testing.T) {
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	azure.GetKubeConfig()
	message := "Enable succeeded: \n[stdout]\nhello world!\n\n[stderr]\n"
}*/

func readConfigs(t *testing.T, clusterConfigPath string, machinesConfigPath string) (*clusterv1.Cluster, []*clusterv1.Machine, error) {
	t.Helper()

	data, err := ioutil.ReadFile(clusterConfigPath)
	if err != nil {
		return nil, nil, err
	}
	cluster := &clusterv1.Cluster{}
	err = yaml.Unmarshal(data, cluster)
	if err != nil {
		return nil, nil, err
	}

	data, err = ioutil.ReadFile(machinesConfigPath)
	if err != nil {
		return nil, nil, err
	}
	list := &clusterv1.MachineList{}
	err = yaml.Unmarshal(data, &list)
	if err != nil {
		return nil, nil, err
	}

	var machines []*clusterv1.Machine
	for index, machine := range list.Items {
		if machine.Spec.ProviderConfig.Value == nil {
			return nil, nil, fmt.Errorf("Machine %d's value is nil", index)
		}
		machines = append(machines, machine.DeepCopy())
	}

	return cluster, machines, nil
}

func mockAzureMachineProviderConfig(t *testing.T) *v1alpha1.AzureMachineProviderConfig {
	t.Helper()
	return &v1alpha1.AzureMachineProviderConfig{
		Location: "eastus",
		VMSize:   "Standard_B1s",
		Image: v1alpha1.Image{
			Publisher: "Canonical",
			Offer:     "UbuntuServer",
			SKU:       "16.04-LTS",
			Version:   "latest",
		},
		OSDisk: v1alpha1.OSDisk{
			OSType: "Linux",
			ManagedDisk: v1alpha1.ManagedDisk{
				StorageAccountType: "Premium_LRS",
			},
			DiskSizeGB: 30,
		},
	}
}

func mockAzureClusterProviderConfig(t *testing.T, rg string) *v1alpha1.AzureClusterProviderConfig {
	t.Helper()
	return &v1alpha1.AzureClusterProviderConfig{
		ResourceGroup: rg,
		Location:      "eastus",
	}
}
