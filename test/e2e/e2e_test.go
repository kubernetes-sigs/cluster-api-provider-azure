package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/platform9/azure-provider/cloud/azure/actuators/machine"
	azureconfigv1 "github.com/platform9/azure-provider/cloud/azure/providerconfig/v1alpha1"
	"github.com/platform9/azure-provider/cloud/azure/services"
	"github.com/platform9/azure-provider/cloud/azure/services/resourcemanagement"
)

// do some testing with the K8s go client
var (
	_, b, _, _          = runtime.Caller(0)
	testBasePath        = filepath.Dir(filepath.Dir(b))
	generatedConfigPath = filepath.Join(filepath.Dir(testBasePath), "generatedconfigs")
)

type Clients struct {
	kube                     KubeClient
	azure                    services.AzureClients
	azureProviderConfigCodec *azureconfigv1.AzureProviderConfigCodec
}

func TestMasterMachineCreated(t *testing.T) {
	clients, err := createTestClients()
	if err != nil {
		t.Fatalf("failed to create test clients: %v", err)
	}

	// kube: verify virtual machine was created sucessfully and healthy
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

	// validate virtual machine fields match the spec
}

func createTestClients() (*Clients, error) {
	kubeconfig := os.Getenv("KUBE_CONFIG")
	if kubeconfig == "" {
		return nil, fmt.Errorf("KUBE_CONFIG environment variable is not set")
	}
	kubeClient, err := NewKubeClient(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return nil, fmt.Errorf("AZURE_SUBSCRIPTION_ID environment variable is not set")
	}

	azureServicesClient, err := NewAzureServicesClient(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure services client: %v", err)
	}

	azureProviderConfigCodec, err := azureconfigv1.NewCodec()
	if err != nil {
		return nil, fmt.Errorf("error creating codec for provider: %v", err)
	}
	return &Clients{kube: *kubeClient, azure: *azureServicesClient, azureProviderConfigCodec: azureProviderConfigCodec}, nil
}
