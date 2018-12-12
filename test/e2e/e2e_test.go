package e2e

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/platform9/azure-provider/pkg/cloud/azure/actuators/machine"

	"github.com/platform9/azure-provider/pkg/cloud/azure/services/resourcemanagement"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_, b, _, _          = runtime.Caller(0)
	testBasePath        = filepath.Dir(filepath.Dir(b))
	generatedConfigPath = filepath.Join(filepath.Dir(testBasePath), "generatedconfigs")
)

func TestMasterMachineExists(t *testing.T) {
	clients, err := createTestClients()
	if err != nil {
		t.Fatalf("failed to create test clients: %v", err)
	}

	// kube: verify virtual machine was created successfully and healthy
	machineList, err := clients.kube.ListMachine("default", metav1.ListOptions{LabelSelector: "set=master"})
	if len(machineList.Items) != 1 {
		t.Fatalf("expected only one machine with label master in the default namespace")
	}

	// azure: check if virtual machine exists
	masterMachine := machineList.Items[0]
	resourceGroup := masterMachine.ObjectMeta.Annotations[string(machine.ResourceGroup)]
	vm, err := clients.azure.Compute.VmIfExists(resourceGroup, resourcemanagement.GetVMName(&masterMachine))
	if err != nil {
		t.Fatalf("error checking if vm exists: %v", err)
	}
	if vm == nil {
		t.Fatalf("couldn't find vm for machine: %v", masterMachine.Name)
	}
}

func TestCreateNode(t *testing.T) {
	clients, err := createTestClients()
	if err != nil {
		t.Fatalf("failed to create test clients: %v", err)
	}

	// make sure master machine exists
	machineList, err := clients.kube.ListMachine("default", metav1.ListOptions{LabelSelector: "set=master"})
	if len(machineList.Items) != 1 {
		t.Fatalf("expected only one machine with label master in the default namespace")
	}
	masterMachine := machineList.Items[0]
	resourceGroup := masterMachine.ObjectMeta.Annotations[string(machine.ResourceGroup)]

	configVals, err := genMachineParams()
	if err != nil {
		t.Fatalf("error generating params for machine: %v", err)
	}
	machineToCreate, err := machineFromConfigFile(filepath.Join(testBasePath, "e2e/fixtures/node-machine.yaml"), configVals)
	if err != nil {
		t.Fatalf("error parsing node machine config file: %v", err)
	}

	machine, err := clients.kube.CreateMachine("default", machineToCreate)
	if err != nil {
		t.Fatalf("error creating machine: %v", err)
	}

	// at this point, the machine is created by kube
	// need to wait for deployments and validate it succeeds on the Azure side

	// wait till deployment is created
	state := "Accepted"
	deployment := resources.DeploymentExtended{}
	for state == "Accepted" || state == "Running" {
		deployment, err = clients.azure.Resourcemanagement.GetDeployment(resourceGroup, machine.ObjectMeta.Name)
		if deployment.StatusCode != 404 && err != nil {
			t.Fatalf("error querying azure for deployment: %v", err)
		}
		if deployment.Properties != nil {
			state = *deployment.Properties.ProvisioningState
		}
		time.Sleep(30 * time.Second)
	}

	if state != "Succeeded" {
		t.Fatalf("Azure deployment for machine: %v failed with state: %v", machine.ObjectMeta.Name, state)
	}

	// deployment succeeded on azure, now validate virtual machine is up and running
	vm, err := clients.azure.Compute.VmIfExists(resourceGroup, resourcemanagement.GetVMName(machine))
	if err != nil {
		t.Fatalf("error checking if vm exists: %v", err)
	}
	if vm == nil {
		t.Fatalf("couldn't find vm for machine: %v", machine.Name)
	}
}
