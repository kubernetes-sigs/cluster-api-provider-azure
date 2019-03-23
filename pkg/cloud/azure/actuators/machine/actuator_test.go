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
	"context"
	"testing"

	"github.com/ghodss/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	providerv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/fake"
	"sigs.k8s.io/cluster-api/pkg/controller/machine"
)

var (
	_ machine.Actuator = (*Actuator)(nil)
)

func newClusterProviderSpec() providerv1.AzureClusterProviderSpec {
	return providerv1.AzureClusterProviderSpec{
		ResourceGroup: "resource-group-test",
		Location:      "southcentralus",
	}
}

func providerSpecFromMachine(in *providerv1.AzureMachineProviderSpec) (*clusterv1.ProviderSpec, error) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	return &clusterv1.ProviderSpec{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func providerSpecFromCluster(in *providerv1.AzureClusterProviderSpec) (*clusterv1.ProviderSpec, error) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	return &clusterv1.ProviderSpec{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func newMachine(t *testing.T, machineConfig providerv1.AzureMachineProviderSpec, labels map[string]string) *clusterv1.Machine {
	providerSpec, err := providerSpecFromMachine(&machineConfig)
	if err != nil {
		t.Fatalf("error encoding provider config: %v", err)
	}
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "machine-test",
			Labels: labels,
		},
		Spec: clusterv1.MachineSpec{
			ProviderSpec: *providerSpec,
			Versions: clusterv1.MachineVersionInfo{
				Kubelet:      "1.9.4",
				ControlPlane: "1.9.4",
			},
		},
	}
}

func newCluster(t *testing.T) *clusterv1.Cluster {
	clusterProviderSpec := newClusterProviderSpec()
	providerSpec, err := providerSpecFromCluster(&clusterProviderSpec)
	if err != nil {
		t.Fatalf("error encoding provider config: %v", err)
	}

	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-test",
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: clusterv1.ClusterNetworkingConfig{
				Services: clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"10.96.0.0/12",
					},
				},
				Pods: clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			ProviderSpec: *providerSpec,
		},
	}
}
func createFakeScope(t *testing.T) *actuators.MachineScope {
	scope := &actuators.Scope{
		Context: context.Background(),
		Cluster: newCluster(t),
		ClusterConfig: &v1alpha1.AzureClusterProviderSpec{
			ResourceGroup:       "dummyResourceGroup",
			Location:            "dummyLocation",
			CAKeyPair:           v1alpha1.KeyPair{Cert: []byte("cert"), Key: []byte("key")},
			EtcdCAKeyPair:       v1alpha1.KeyPair{Cert: []byte("cert"), Key: []byte("key")},
			FrontProxyCAKeyPair: v1alpha1.KeyPair{Cert: []byte("cert"), Key: []byte("key")},
			SAKeyPair:           v1alpha1.KeyPair{Cert: []byte("cert"), Key: []byte("key")},
			DiscoveryHashes:     []string{"discoveryhash0"},
		},
		ClusterStatus: &v1alpha1.AzureClusterProviderStatus{},
	}
	scope.Network().APIServerIP.DNSName = "DummyDNSName"
	labels := make(map[string]string)
	labels["set"] = v1alpha1.ControlPlane
	machineConfig := providerv1.AzureMachineProviderSpec{}
	m := newMachine(t, machineConfig, labels)
	c := fake.NewSimpleClientset(m).ClusterV1alpha1()
	return &actuators.MachineScope{
		Scope:         scope,
		Machine:       m,
		MachineClient: c.Machines("dummyNamespace"),
		MachineConfig: &v1alpha1.AzureMachineProviderSpec{},
		MachineStatus: &v1alpha1.AzureMachineProviderStatus{},
	}
}

func TestReconcilerSuccess(t *testing.T) {
	fakeSuccessSvc := &azure.FakeSuccessService{}

	fakeReconciler := &Reconciler{
		scope:                 createFakeScope(t),
		networkInterfacesSvc:  fakeSuccessSvc,
		virtualMachinesSvc:    fakeSuccessSvc,
		virtualMachinesExtSvc: fakeSuccessSvc,
	}

	if err := fakeReconciler.Create(context.Background()); err != nil {
		t.Errorf("failed to create machine: %+v", err)
	}

	// if err := fakeReconciler.Update(context.Background()); err != nil {
	// 	t.Errorf("failed to update machine: %+v", err)
	// }

	// if _, err := fakeReconciler.Exists(context.Background()); err != nil {
	// 	t.Errorf("failed to check if machine exists: %+v", err)
	// }

	if err := fakeReconciler.Delete(context.Background()); err != nil {
		t.Errorf("failed to delete machine: %+v", err)
	}
}

func TestReconcileFailure(t *testing.T) {
	fakeSuccessSvc := &azure.FakeSuccessService{}
	fakeFailureSvc := &azure.FakeFailureService{}

	fakeReconciler := &Reconciler{
		scope:                 createFakeScope(t),
		networkInterfacesSvc:  fakeFailureSvc,
		virtualMachinesSvc:    fakeFailureSvc,
		virtualMachinesExtSvc: fakeSuccessSvc,
	}

	if err := fakeReconciler.Create(context.Background()); err == nil {
		t.Errorf("expected createa to fail")
	}

	if err := fakeReconciler.Update(context.Background()); err == nil {
		t.Errorf("expected update to fail")
	}

	if _, err := fakeReconciler.Exists(context.Background()); err == nil {
		t.Errorf("expected exists to fail")
	}

	if err := fakeReconciler.Delete(context.Background()); err == nil {
		t.Errorf("expected delete to fail")
	}
}

/*

const controlPlaneTestVersion = "1.1.1.1"

func TestActuatorCreateSuccess(t *testing.T) {
	azureServicesClient := actuators.AzureClients{Network: &azure.MockAzureNetworkClient{}}
	params := MachineActuatorParams{Services: &azureServicesClient}
	_, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
}
func TestActuatorCreateFailure(t *testing.T) {
	if err := os.Setenv("AZURE_ENVIRONMENT", "dummy"); err != nil {
		t.Fatalf("error when setting AZURE_ENVIRONMENT environment variable")
	}
	_, err := NewMachineActuator(ActuatorParams{})
	if err == nil {
		t.Fatalf("expected error when creating the cluster actuator but gone none")
	}
	os.Unsetenv("AZURE_ENVIRONMENT")
}
func TestNewAzureClientParamsPassed(t *testing.T) {
	azureServicesClient := actuators.AzureClients{Compute: &azure.MockAzureComputeClient{}}
	params := MachineActuatorParams{Services: &azureServicesClient}
	client, err := actuators.NewScope(params)
	if err != nil {
		t.Fatalf("unable to create azure services client: %v", err)
	}
	// ensures that the passed azure services client is the one used
	if client.Compute == nil {
		t.Fatal("expected compute client to not be nil")
	}
	if client.Network != nil {
		t.Fatal("expected network client to be nil")
	}
	if client.Resources != nil {
		t.Fatal("expected resource management client to be nil")
	}
}

func TestNewAzureClientNoParamsPassed(t *testing.T) {
	if err := os.Setenv("AZURE_SUBSCRIPTION_ID", "dummy"); err != nil {
		t.Fatalf("error when setting AZURE_SUBSCRIPTION_ID environment variable")
	}
	client, err := azureServicesClientOrDefault(ActuatorParams{})
	if err != nil {
		t.Fatalf("unable to create azure services client: %v", err)
	}
	// cluster actuator doesn't utilize compute client
	if client.Compute == nil {
		t.Fatal("expected compute client to not be nil")
	}
	// clients should be initialized
	if client.Resources == nil {
		t.Fatal("expected resource management client to not be nil")
	}
	if client.Network == nil {
		t.Fatal("expected network client to not be nil")
	}
	os.Unsetenv("AZURE_SUBSCRIPTION_ID")
}

func TestNewAzureClientAuthorizerFailure(t *testing.T) {
	if err := os.Setenv("AZURE_ENVIRONMENT", "dummy"); err != nil {
		t.Fatalf("error when setting environment variable")
	}
	_, err := azureServicesClientOrDefault(ActuatorParams{})
	if err == nil {
		t.Fatalf("expected error when creating the azure services client but got none")
	}
	os.Unsetenv("AZURE_ENVIRONMENT")
}

func TestNewAzureClientSubscriptionFailure(t *testing.T) {
	_, err := azureServicesClientOrDefault(ActuatorParams{})
	if err == nil {
		t.Fatalf("expected error when creating the azure services client but got none")
	}
}
func TestCreateSuccess(t *testing.T) {
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockDeploymentCreateOrUpdateSuccess())
	mergo.Merge(&resourceManagementMock, azure.MockRgExists())
	mergo.Merge(&resourceManagementMock, azure.MockDeloymentGetResultSuccess())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Create(context.Background(), cluster, machine)
	if err != nil {
		t.Fatalf("unable to create machine: %v", err)
	}
}
func TestCreateFailureClusterParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	err = actuator.Create(context.Background(), cluster, machine)
	if err == nil {
		t.Fatal("expected error when creating machine, but got none")
	}
}

func TestCreateFailureMachineParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	machine.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	err = actuator.Create(context.Background(), cluster, machine)
	if err == nil {
		t.Fatal("expected error when creating machine, but got none")
	}
}

func TestCreateFailureDeploymentValidation(t *testing.T) {
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockDeploymentValidate())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Create(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error when creating machine, but got none")
	}
}

func TestCreateFailureDeploymentCreation(t *testing.T) {
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockDeploymentCreateOrUpdateFailure())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Create(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error when calling create, but got none")
	}
}

func TestCreateFailureDeploymentFutureError(t *testing.T) {
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockDeploymentCreateOrUpdateSuccess())
	mergo.Merge(&resourceManagementMock, azure.MockDeploymentCreateOrUpdateFutureFailure())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Create(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error when calling create, but got none")
	}
}

func TestCreateFailureDeploymentResult(t *testing.T) {
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockDeploymentCreateOrUpdateSuccess())
	mergo.Merge(&resourceManagementMock, azure.MockDeloymentGetResultFailure())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Create(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error when calling create, but got none")
	}
}

func TestExistsSuccess(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockRgExists())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock, Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	ok, err := actuator.Exists(context.Background(), cluster, machine)
	if err != nil {
		t.Fatalf("unexpected error calling Exists: %v", err)
	}
	if !ok {
		t.Fatalf("machine: %v does not exist", machine.ObjectMeta.Name)
	}
}

func TestExistsFailureClusterParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	_, err = actuator.Exists(context.Background(), cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling exists, but got none")
	}
}

func TestExistsFailureMachineParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	machine.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	_, err = actuator.Exists(context.Background(), cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling exists, but got none")
	}
}

func TestExistsFailureRGNotExists(t *testing.T) {
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockRgNotExists())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	ok, err := actuator.Exists(context.Background(), cluster, machine)
	if err != nil {
		t.Fatalf("unexpected error calling Exists: %v", err)
	}
	if ok {
		t.Fatalf("expected machine: %v to not exist", machine.ObjectMeta.Name)
	}
}
func TestExistsFailureRGCheckFailure(t *testing.T) {
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockRgCheckFailure())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	ok, err := actuator.Exists(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error when calling exists, but got none")
	}
	if ok {
		t.Fatalf("expected machine: %v to not exist", machine.ObjectMeta.Name)
	}
}
func TestExistsFailureVMNotExists(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMNotExists())
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockRgExists())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock, Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	ok, err := actuator.Exists(context.Background(), cluster, machine)
	if err != nil {
		t.Fatalf("unexpected error calling Exists: %v", err)
	}
	if ok {
		t.Fatalf("expected machine: %v to not exist", machine.ObjectMeta.Name)
	}
}

func TestExistsFailureVMCheckFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMCheckFailure())
	resourceManagementMock := azure.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, azure.MockRgExists())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock, Resources: &resourceManagementMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	ok, err := actuator.Exists(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error when calling exists, but got none")
	}
	if ok {
		t.Fatalf("expected machine: %v to not exist", machine.ObjectMeta.Name)
	}
}

func TestUpdateFailureClusterParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	err = actuator.Update(context.Background(), cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling exists, but got none")
	}
}

func TestUpdateFailureMachineParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	machine.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	err = actuator.Update(context.Background(), cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling exists, but got none")
	}
}

// PREVIOUSLY COMMENTED OUT TESTS
// func TestUpdateVMNotExists(t *testing.T) {
// 	azureServicesClient := mockVMNotExists()
// 	params := ActuatorParams{Services: &azureServicesClient}
// func TestUpdateVMNotExists(t *testing.T) {
// 	azureServicesClient := mockVMNotExists()
// 	params := ActuatorParams{Services: &azureServicesClient}

// 	machineConfig := newMachineProviderSpec()
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	actuator, err := NewMachineActuator(params)
// 	err = actuator.Update(cluster, machine)
// 	if err == nil {
// 		t.Fatal("expected error calling Update but got none")
// 	}
// }

// func TestUpdateMachineNotExists(t *testing.T) {
// 	azureServicesClient := mockVMExists()
// 	machineConfig := newMachineProviderSpec()
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	params := ActuatorParams{Services: &azureServicesClient}
// 	actuator, err := NewMachineActuator(params)
// 	err = actuator.Update(cluster, machine)
// 	if err == nil {
// 		t.Fatal("expected error calling Update but got none")
// 	}
// }

// func TestUpdateNoSpecChange(t *testing.T) {
// 	azureServicesClient := mockVMExists()
// 	machineConfig := newMachineProviderSpec()
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	params := ActuatorParams{Services: &azureServicesClient, V1Alpha1Client: fake.NewSimpleClientset(machine).ClusterV1alpha1()}
// 	actuator, err := NewMachineActuator(params)
// 	err = actuator.Update(cluster, machine)
// 	if err != nil {
// 		t.Fatal("unexpected error calling Update")
// 	}
// }

// func TestUpdateMasterKubeletChange(t *testing.T) {
// 	azureServicesClient := mockVMExists()
// 	machineConfig := newMachineProviderSpec()
// 	// set as master machine
// 	machineConfig.Roles = []providerv1.MachineRole{providerv1.Master}
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	params := ActuatorParams{Services: &azureServicesClient, V1Alpha1Client: fake.NewSimpleClientset(machine).ClusterV1alpha1()}
// 	actuator, err := NewMachineActuator(params)
// 	goalMachine := machine
// 	goalMachine.Spec.Versions.Kubelet = "1.12.5"

// 	err = actuator.Update(cluster, goalMachine)
// 	if err != nil {
// 		t.Fatalf("unexpected error calling Update: %v", err)
// 	}
// }

// func TestUpdateMasterControlPlaneChange(t *testing.T) {
// 	azureServicesClient := mockVMExists()
// 	machineConfig := newMachineProviderSpec()
// 	// set as master machine
// 	machineConfig.Roles = []providerv1.MachineRole{providerv1.Master}
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	params := ActuatorParams{Services: &azureServicesClient, V1Alpha1Client: fake.NewSimpleClientset(machine).ClusterV1alpha1()}
// 	actuator, err := NewMachineActuator(params)
// 	goalMachine := machine
// 	goalMachine.Spec.Versions.ControlPlane = "1.12.5"

// 	err = actuator.Update(cluster, goalMachine)
// 	if err != nil {
// 		t.Fatalf("unexpected error calling Update: %v", err)
// 	}
// }
// func TestUpdateMasterControlPlaneChangeRunCommandFailure(t *testing.T) {
// 	azureServicesClient := mockVMExists()
// 	machineConfig := newMachineProviderSpec()
// 	// set as master machine
// 	machineConfig.Roles = []providerv1.MachineRole{providerv1.Master}
// 	machine := newMachine(t, machineConfig)
// 	cluster := newCluster(t)

// 	params := ActuatorParams{Services: &azureServicesClient, V1Alpha1Client: fake.NewSimpleClientset(machine).ClusterV1alpha1()}
// 	actuator, err := NewMachineActuator(params)
// 	goalMachine := machine
// 	goalMachine.Spec.Versions.ControlPlane = "1.12.5"

// 	err = actuator.Update(cluster, goalMachine)
// 	if err != nil {
// 		t.Fatalf("unexpected error calling Update: %v", err)
// 	}
// }

func TestUpdateMasterFailureMachineParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	err = actuator.updateMaster(cluster, machine, machine)
	if err == nil {
		t.Fatal("expected error when calling updateMaster, but got none")
	}
}

func TestUpdateMasterControlPlaneSuccess(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)
	m2.Spec.Versions.ControlPlane = controlPlaneTestVersion
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.updateMaster(cluster, m1, m2)
	if err != nil {
		t.Fatalf("unexpected error calling updateMaster: %v", err)
	}
}

func TestUpdateMasterControlPlaneCmdRunFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockRunCommandFailure())

	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)
	m2.Spec.Versions.ControlPlane = controlPlaneTestVersion
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.updateMaster(cluster, m1, m2)
	if err == nil {
		t.Fatalf("expected error calling updateMaster but got none")
	}
}

func TestUpdateMasterControlPlaneFutureFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockRunCommandFutureFailure())

	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)
	m2.Spec.Versions.ControlPlane = controlPlaneTestVersion
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.updateMaster(cluster, m1, m2)
	if err == nil {
		t.Fatalf("expected error calling updateMaster but got none")
	}
}

func TestUpdateMasterKubeletSuccess(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)
	m2.Spec.Versions.Kubelet = controlPlaneTestVersion
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.updateMaster(cluster, m1, m2)
	if err != nil {
		t.Fatalf("unexpected error calling updateMaster: %v", err)
	}
}

func TestUpdateMasterKubeletFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockRunCommandFailure())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)
	m2.Spec.Versions.Kubelet = controlPlaneTestVersion
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.updateMaster(cluster, m1, m2)
	if err == nil {
		t.Fatalf("expected error calling updateMaster but got none")
	}
}

func TestUpdateMasterKubeletFutureFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockRunCommandFutureFailure())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)
	m2.Spec.Versions.Kubelet = controlPlaneTestVersion
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.updateMaster(cluster, m1, m2)
	if err == nil {
		t.Fatalf("expected error calling updateMaster but got none")
	}
}

func TestShouldUpdateSameMachine(t *testing.T) {
	params := MachineActuatorParams{Services: &actuators.AzureClients{}}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	shouldUpdate := actuator.shouldUpdate(m1, m2)
	if shouldUpdate {
		t.Fatalf("expected shouldUpdate to return false but got true")
	}
}

func TestShouldUpdateVersionChange(t *testing.T) {
	params := MachineActuatorParams{Services: &actuators.AzureClients{}}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)
	m2.Spec.Versions.ControlPlane = controlPlaneTestVersion

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	shouldUpdate := actuator.shouldUpdate(m1, m2)
	if !shouldUpdate {
		t.Fatalf("expected shouldUpdate to return true but got false")
	}
}
func TestShouldUpdateObjectMetaChange(t *testing.T) {
	params := MachineActuatorParams{Services: &actuators.AzureClients{}}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)
	m2.Spec.ObjectMeta.Namespace = "namespace-update"

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	shouldUpdate := actuator.shouldUpdate(m1, m2)
	if !shouldUpdate {
		t.Fatalf("expected shouldUpdate to return true but got false")
	}
}
func TestShouldUpdateProviderSpecChange(t *testing.T) {
	params := MachineActuatorParams{Services: &actuators.AzureClients{}}
	m1Config := newMachineProviderSpec()
	m1 := newMachine(t, m1Config)
	m2Config := m1Config
	m2Config.Location = "new-region"
	m2 := newMachine(t, m2Config)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	shouldUpdate := actuator.shouldUpdate(m1, m2)
	if !shouldUpdate {
		t.Fatalf("expected shouldUpdate to return true but got false")
	}
}

func TestShouldUpdateNameChange(t *testing.T) {
	params := MachineActuatorParams{Services: &actuators.AzureClients{}}
	machineConfig := newMachineProviderSpec()
	m1 := newMachine(t, machineConfig)
	m2 := newMachine(t, machineConfig)
	m2.Spec.ObjectMeta.Name = "name-update"

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	shouldUpdate := actuator.shouldUpdate(m1, m2)
	if shouldUpdate != true {
		t.Fatalf("expected shouldUpdate to return true but got false")
	}
}

func TestDeleteSuccess(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock, Network: &azure.MockAzureNetworkClient{}}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err != nil {
		t.Fatalf("unable to delete machine: %v", err)
	}
}

func TestDeleteFailureClusterParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling exists, but got none")
	}
}

func TestDeleteFailureMachineParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	machine.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling exists, but got none")
	}
}

func TestDeleteFailureVMNotExists(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMNotExists())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestDeleteFailureVMDeletionFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	mergo.Merge(&computeMock, azure.MockVMDeleteFailure())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestDeleteFailureVMCheckFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMCheckFailure())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestDeleteFailureVMDeleteFutureFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	mergo.Merge(&computeMock, azure.MockVMDeleteFutureFailure())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}
func TestDeleteFailureDiskDeleteFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	mergo.Merge(&computeMock, azure.MockDisksDeleteFailure())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestDeleteFailureDiskDeleteFutureFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	mergo.Merge(&computeMock, azure.MockDisksDeleteFutureFailure())
	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}
func TestDeleteFailureNICResourceName(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExistsNICInvalid())

	azureServicesClient := actuators.AzureClients{Compute: &computeMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}
func TestDeleteFailureNICDeleteFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockNicDeleteFailure())

	azureServicesClient := actuators.AzureClients{Compute: &computeMock, Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestDeleteFailureNICDeleteFutureFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockNicDeleteFutureFailure())

	azureServicesClient := actuators.AzureClients{Compute: &computeMock, Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestDeleteFailurePublicIPDeleteFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockPublicIPDeleteFailure())

	azureServicesClient := actuators.AzureClients{Compute: &computeMock, Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestDeleteFailurePublicIPDeleteFutureFailure(t *testing.T) {
	computeMock := azure.MockAzureComputeClient{}
	mergo.Merge(&computeMock, azure.MockVMExists())
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockPublicIPDeleteFutureFailure())

	azureServicesClient := actuators.AzureClients{Compute: &computeMock, Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(context.Background(), cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestGetKubeConfigFailureClusterParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	_, err = actuator.GetKubeConfig(cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling GetKubeConfig, but got none")
	}
}

func TestGetKubeConfigFailureMachineParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	machine.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	_, err = actuator.GetKubeConfig(cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling GetKubeConfig, but got none")
	}
}

func TestGetKubeConfigBase64Error(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machineConfig.SSHPrivateKey = "===="
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = actuator.GetKubeConfig(cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling GetKubeConfig, but got none")
	}
}

func TestGetKubeConfigIPAddressFailure(t *testing.T) {
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockCreateOrUpdatePublicIPAddressFailure())
	azureServicesClient := actuators.AzureClients{Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}

	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = actuator.GetKubeConfig(cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling GetKubeConfig, but got none")
	}
}

func TestGetIPFailureClusterParsing(t *testing.T) {
	cluster := newCluster(t)
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)

	actuator, err := NewMachineActuator(MachineActuatorParams{Services: &actuators.AzureClients{}})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}
	_, err = actuator.GetIP(cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling GetIP, but got none")
	}
}

func TestGetKubeConfigValidPrivateKey(t *testing.T) {
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockCreateOrUpdatePublicIPAddress("127.0.0.1"))
	azureServicesClient := actuators.AzureClients{Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}

	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = actuator.GetKubeConfig(cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling GetIP, but got none")
	}
}
func TestGetKubeConfigInvalidBase64(t *testing.T) {
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockCreateOrUpdatePublicIPAddress("127.0.0.1"))
	azureServicesClient := actuators.AzureClients{Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}

	machineConfig := newMachineProviderSpec()
	machineConfig.SSHPrivateKey = "====="
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = actuator.GetKubeConfig(cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling GetIP, but got none")
	}
}
func TestGetKubeConfigInvalidPrivateKey(t *testing.T) {
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockCreateOrUpdatePublicIPAddress("127.0.0.1"))
	azureServicesClient := actuators.AzureClients{Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}

	machineConfig := newMachineProviderSpec()
	machineConfig.SSHPrivateKey = "aGVsbG8="
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = actuator.GetKubeConfig(cluster, machine)
	if err == nil {
		t.Fatal("expected error when calling GetIP, but got none")
	}
}
func TestGetIPSuccess(t *testing.T) {
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockCreateOrUpdatePublicIPAddress("127.0.0.1"))
	azureServicesClient := actuators.AzureClients{Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	ip, err := actuator.GetIP(cluster, machine)
	if err != nil {
		t.Fatalf("unexpected error when calling GetIP: %v", err)
	}
	if ip != "127.0.0.1" {
		t.Fatalf("expected ip address to be 127.0.0.1 but got: %v", ip)
	}
}

func TestGetIPFailure(t *testing.T) {
	networkMock := azure.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, azure.MockCreateOrUpdatePublicIPAddressFailure())
	azureServicesClient := actuators.AzureClients{Network: &networkMock}

	params := ActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderSpec()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}

	_, err = actuator.GetIP(cluster, machine)
	if err == nil {
		t.Fatal("expected error calling GetIP but got none")
	}
}
func newMachineProviderSpec() providerv1.AzureMachineProviderSpec {
	var privateKey = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIBPQIBAAJBALqbHeRLCyOdykC5SDLqI49ArYGYG1mqaH9/GnWjGavZM02fos4l
c2w6tCchcUBNtJvGqKwhC5JEnx3RYoSX2ucCAwEAAQJBAKn6O+tFFDt4MtBsNcDz
GDsYDjQbCubNW+yvKbn4PJ0UZoEebwmvH1ouKaUuacJcsiQkKzTHleu4krYGUGO1
mEECIQD0dUhj71vb1rN1pmTOhQOGB9GN1mygcxaIFOWW8znLRwIhAMNqlfLijUs6
rY+h1pJa/3Fh1HTSOCCCCWA0NRFnMANhAiEAwddKGqxPO6goz26s2rHQlHQYr47K
vgPkZu2jDCo7trsCIQC/PSfRsnSkEqCX18GtKPCjfSH10WSsK5YRWAY3KcyLAQIh
AL70wdUu5jMm2ex5cZGkZLRB50yE6rBiHCd5W1WdTFoe
-----END RSA PRIVATE KEY-----
`)

	return providerv1.AzureMachineProviderSpec{
		Location: "southcentralus",
		VMSize:   "Standard_B2ms",
		Image: providerv1.Image{
			Publisher: "Canonical",
			Offer:     "UbuntuServer",
			SKU:       "16.04-LTS",
			Version:   "latest",
		},
		OSDisk: providerv1.OSDisk{
			OSType: "Linux",
			ManagedDisk: providerv1.ManagedDisk{
				StorageAccountType: "Premium_LRS",
			},
			DiskSizeGB: 30,
		},
		SSHPrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
}
*/
