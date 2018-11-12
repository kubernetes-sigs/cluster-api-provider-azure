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
package machine

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-04-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	azureconfigv1 "github.com/platform9/azure-provider/cloud/azure/providerconfig/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/fake"

	"github.com/platform9/azure-provider/cloud/azure/services"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewMachineActuatorSuccess(t *testing.T) {
	params := MachineActuatorParams{}
	_, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
}

func TestCreateSuccess(t *testing.T) {
	azureServicesClient := mockDeploymentSuccess()
	params := MachineActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Create(cluster, machine)
	if err != nil {
		t.Fatalf("unable to create machine: %v", err)
	}
}
func TestCreateFailure(t *testing.T) {
	azureServicesClient := mockDeploymentFailure()
	params := MachineActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Create(cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestExistsSuccess(t *testing.T) {
	azureServicesClient := mockVMExists()
	params := MachineActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	ok, err := actuator.Exists(cluster, machine)
	if err != nil {
		t.Fatalf("unexpected error calling Exists: %v", err)
	}
	if !ok {
		t.Fatalf("machine: %v does not exist", machine.ObjectMeta.Name)
	}
}

func TestExistsFailureRGNotExists(t *testing.T) {
	azureServicesClient := mockResourceGroupNotExists()
	params := MachineActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	ok, err := actuator.Exists(cluster, machine)
	if err != nil {
		t.Fatalf("unexpected error calling Exists: %v", err)
	}
	if ok {
		t.Fatalf("expected machine: %v to not exist", machine.ObjectMeta.Name)
	}
}
func TestExistsFailureVMNotExists(t *testing.T) {
	azureServicesClient := mockVMNotExists()
	params := MachineActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	ok, err := actuator.Exists(cluster, machine)
	if err != nil {
		t.Fatalf("unexpected error calling Exists: %v", err)
	}
	if ok {
		t.Fatalf("expected machine: %v to not exist", machine.ObjectMeta.Name)
	}
}
func TestUpdateVMNotExists(t *testing.T) {
	azureServicesClient := mockVMNotExists()
	params := MachineActuatorParams{Services: &azureServicesClient}

	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	err = actuator.Update(cluster, machine)
	if err == nil {
		t.Fatal("expected error calling Update but got none")
	}
}
func TestUpdateMachineNotExists(t *testing.T) {
	azureServicesClient := mockVMExists()
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	params := MachineActuatorParams{Services: &azureServicesClient, V1Alpha1Client: fake.NewSimpleClientset().ClusterV1alpha1()}
	actuator, err := NewMachineActuator(params)
	err = actuator.Update(cluster, machine)
	if err == nil {
		t.Fatal("expected error calling Update but got none")
	}
}

// func TestUpdateNoSpecChange(t *testing.T) {
// 	azureServicesClient := mockVMExists()
// 	machineConfig := newMachineProviderConfig()
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	params := MachineActuatorParams{Services: &azureServicesClient, V1Alpha1Client: fake.NewSimpleClientset(machine).ClusterV1alpha1()}
// 	actuator, err := NewMachineActuator(params)
// 	err = actuator.Update(cluster, machine)
// 	if err != nil {
// 		t.Fatal("unexpected error calling Update")
// 	}
// }

// func TestUpdateMasterKubeletChange(t *testing.T) {
// 	azureServicesClient := mockVMExists()
// 	machineConfig := newMachineProviderConfig()
// 	// set as master machine
// 	machineConfig.Roles = []azureconfigv1.MachineRole{azureconfigv1.Master}
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	params := MachineActuatorParams{Services: &azureServicesClient, V1Alpha1Client: fake.NewSimpleClientset(machine).ClusterV1alpha1()}
// 	actuator, err := NewMachineActuator(params)
// 	goalMachine := machine
// 	goalMachine.Spec.Versions.Kubelet = "1.10.0"

// 	err = actuator.Update(cluster, goalMachine)
// 	if err != nil {
// 		t.Fatalf("unexpected error calling Update: %v", err)
// 	}
// }

// func TestUpdateMasterControlPlaneChange(t *testing.T) {
// 	azureServicesClient := mockVMExists()
// 	machineConfig := newMachineProviderConfig()
// 	// set as master machine
// 	machineConfig.Roles = []azureconfigv1.MachineRole{azureconfigv1.Master}
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	params := MachineActuatorParams{Services: &azureServicesClient, V1Alpha1Client: fake.NewSimpleClientset(machine).ClusterV1alpha1()}
// 	actuator, err := NewMachineActuator(params)
// 	goalMachine := machine
// 	goalMachine.Spec.Versions.ControlPlane = "1.10.0"

// 	err = actuator.Update(cluster, goalMachine)
// 	if err != nil {
// 		t.Fatalf("unexpected error calling Update: %v", err)
// 	}
// }
// func TestUpdateMasterControlPlaneChangeRunCommandFailure(t *testing.T) {
// 	azureServicesClient := mockVMExists()
// 	machineConfig := newMachineProviderConfig()
// 	// set as master machine
// 	machineConfig.Roles = []azureconfigv1.MachineRole{azureconfigv1.Master}
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	params := MachineActuatorParams{Services: &azureServicesClient, V1Alpha1Client: fake.NewSimpleClientset(machine).ClusterV1alpha1()}
// 	actuator, err := NewMachineActuator(params)
// 	goalMachine := machine
// 	goalMachine.Spec.Versions.ControlPlane = "1.10.0"

// 	err = actuator.Update(cluster, goalMachine)
// 	if err != nil {
// 		t.Fatalf("unexpected error calling Update: %v", err)
// 	}
// }

func TestDeleteSuccess(t *testing.T) {
	azureServicesClient := mockDeleteSuccess()
	params := MachineActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(cluster, machine)
	if err != nil {
		t.Fatalf("unable to delete machine: %v", err)
	}
}

func TestDeleteFailureVMNotExists(t *testing.T) {
	azureServicesClient := mockVMNotExists()
	params := MachineActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	err = actuator.Delete(cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func mockDeploymentFailure() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCreateOrUpdateDeployment: func(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderConfig, machineConfig *azureconfigv1.AzureMachineProviderConfig) (*resources.DeploymentsCreateOrUpdateFuture, error) {
			return nil, errors.New("failed to create resource")
		},
	}
	return services.AzureClients{Compute: &services.MockAzureComputeClient{}, Resourcemanagement: &resourcemanagementMock, Network: &services.MockAzureNetworkClient{}}
}

func mockDeploymentSuccess() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCreateOrUpdateDeployment: func(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderConfig, machineConfig *azureconfigv1.AzureMachineProviderConfig) (*resources.DeploymentsCreateOrUpdateFuture, error) {
			return &resources.DeploymentsCreateOrUpdateFuture{}, nil
		},
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
		},
		MockGetDeploymentResult: func(future resources.DeploymentsCreateOrUpdateFuture) (resources.DeploymentExtended, error) {
			return resources.DeploymentExtended{Name: to.StringPtr("deployment-test")}, nil
		},
	}
	return services.AzureClients{Compute: &services.MockAzureComputeClient{}, Resourcemanagement: &resourcemanagementMock, Network: &services.MockAzureNetworkClient{}}
}

func mockVMExists() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
		},
	}
	computeMock := services.MockAzureComputeClient{
		MockVmIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			networkProfile := compute.NetworkProfile{NetworkInterfaces: &[]compute.NetworkInterfaceReference{compute.NetworkInterfaceReference{ID: to.StringPtr("001")}}}
			OsDiskName := fmt.Sprintf("OS_Disk_%v", name)
			storageProfile := compute.StorageProfile{OsDisk: &compute.OSDisk{Name: &OsDiskName}}
			vmProperties := compute.VirtualMachineProperties{StorageProfile: &storageProfile, NetworkProfile: &networkProfile}
			return &compute.VirtualMachine{Name: &name, VirtualMachineProperties: &vmProperties}, nil
		},
	}
	return services.AzureClients{Compute: &computeMock, Resourcemanagement: &resourcemanagementMock, Network: &services.MockAzureNetworkClient{}}
}

func mockVMNotExists() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
		},
	}
	// default compute mock returns nil VM
	return services.AzureClients{Compute: &services.MockAzureComputeClient{}, Resourcemanagement: &resourcemanagementMock, Network: &services.MockAzureNetworkClient{}}
}

func mockResourceGroupNotExists() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
		},
	}
	return services.AzureClients{Compute: &services.MockAzureComputeClient{}, Resourcemanagement: &resourcemanagementMock, Network: &services.MockAzureNetworkClient{}}
}

func mockDeleteSuccess() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCreateOrUpdateDeployment: func(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderConfig, machineConfig *azureconfigv1.AzureMachineProviderConfig) (*resources.DeploymentsCreateOrUpdateFuture, error) {
			return &resources.DeploymentsCreateOrUpdateFuture{}, nil
		},
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
		},
	}
	computeMock := services.MockAzureComputeClient{
		MockVmIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			nicId := "001"
			networkProfile := compute.NetworkProfile{NetworkInterfaces: &[]compute.NetworkInterfaceReference{compute.NetworkInterfaceReference{ID: &nicId}}}
			OsDiskName := fmt.Sprintf("OS_Disk_%v", name)
			storageProfile := compute.StorageProfile{OsDisk: &compute.OSDisk{Name: &OsDiskName}}
			vmProperties := compute.VirtualMachineProperties{StorageProfile: &storageProfile, NetworkProfile: &networkProfile}
			return &compute.VirtualMachine{Name: &name, VirtualMachineProperties: &vmProperties}, nil
		},
	}
	return services.AzureClients{Compute: &computeMock, Resourcemanagement: &resourcemanagementMock, Network: &services.MockAzureNetworkClient{}}
}

func newMachineProviderConfig() azureconfigv1.AzureMachineProviderConfig {
	return azureconfigv1.AzureMachineProviderConfig{
		Location: "southcentralus",
		VMSize:   "Standard_B2ms",
		Image: azureconfigv1.Image{
			Publisher: "Canonical",
			Offer:     "UbuntuServer",
			SKU:       "16.04-LTS",
			Version:   "latest",
		},
		OSDisk: azureconfigv1.OSDisk{
			OSType: "Linux",
			ManagedDisk: azureconfigv1.ManagedDisk{
				StorageAccountType: "Premium_LRS",
			},
			DiskSizeGB: 30,
		},
	}
}

func newClusterProviderConfig() azureconfigv1.AzureClusterProviderConfig {
	return azureconfigv1.AzureClusterProviderConfig{
		ResourceGroup: "resource-group-test",
		Location:      "southcentralus",
	}
}

func providerConfigFromMachine(in *azureconfigv1.AzureMachineProviderConfig) (*clusterv1.ProviderConfig, error) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	return &clusterv1.ProviderConfig{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func providerConfigFromCluster(in *azureconfigv1.AzureClusterProviderConfig) (*clusterv1.ProviderConfig, error) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	return &clusterv1.ProviderConfig{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func newMachine(t *testing.T, machineConfig azureconfigv1.AzureMachineProviderConfig) *v1alpha1.Machine {
	providerConfig, err := providerConfigFromMachine(&machineConfig)
	if err != nil {
		t.Fatalf("error encoding provider config: %v", err)
	}
	return &v1alpha1.Machine{
		ObjectMeta: v1.ObjectMeta{
			Name: "machine-test",
		},
		Spec: v1alpha1.MachineSpec{
			ProviderConfig: *providerConfig,
			Versions: v1alpha1.MachineVersionInfo{
				Kubelet:      "1.9.4",
				ControlPlane: "1.9.4",
			},
		},
	}
}

func newCluster(t *testing.T) *v1alpha1.Cluster {
	clusterProviderConfig := newClusterProviderConfig()
	providerConfig, err := providerConfigFromCluster(&clusterProviderConfig)
	if err != nil {
		t.Fatalf("error encoding provider config: %v", err)
	}

	return &v1alpha1.Cluster{
		TypeMeta: v1.TypeMeta{
			Kind: "Cluster",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "cluster-test",
		},
		Spec: v1alpha1.ClusterSpec{
			ClusterNetwork: v1alpha1.ClusterNetworkingConfig{
				Services: v1alpha1.NetworkRanges{
					CIDRBlocks: []string{
						"10.96.0.0/12",
					},
				},
				Pods: v1alpha1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			ProviderConfig: *providerConfig,
		},
	}
}
