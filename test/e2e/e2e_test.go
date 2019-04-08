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

package e2e

import (
	"os"
	"testing"

	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators/machine"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/resources"
)

// do some testing with the K8s go client
//var (
//	_, b, _, _          = runtime.Caller(0)
//	testBasePath        = filepath.Dir(filepath.Dir(b))
//	generatedConfigPath = filepath.Join(filepath.Dir(testBasePath), "generatedconfigs")
//)

type Clients struct {
	kube  KubeClient
	azure actuators.AzureClients
}

func TestMasterMachineCreated(t *testing.T) {
	kubeConfig := os.Getenv("KUBE_CONFIG")
	if kubeConfig == "" {
		t.Skip("KUBE_CONFIG environment variable is not set")
	}
	clients, err := createTestClients(kubeConfig)
	if err != nil {
		t.Fatalf("failed to create test clients: %v", err)
	}

	// kube: verify virtual machine was created successfully and healthy
	machineList, err := clients.kube.ListMachine("default", metav1.ListOptions{LabelSelector: "set=master"})
	if err != nil {
		t.Fatalf("error to while trying to retrieve machine list: %v", err)
	}
	if len(machineList.Items) != 1 {
		t.Fatalf("expected only one machine with label master in the default namespace")
	}

	// azure: check if virtual machine exists
	masterMachine := machineList.Items[0]
	resourceGroup := masterMachine.ObjectMeta.Annotations[string(machine.ResourceGroup)]
	vm, err := clients.azure.Compute.VMIfExists(resourceGroup, resources.GetVMName(&masterMachine))
	if err != nil {
		t.Fatalf("error checking if vm exists: %v", err)
	}
	if vm == nil {
		t.Fatalf("couldn't find vm for machine: %v", masterMachine.Name)
	}

	// validate virtual machine fields match the spec
}

func createTestClients(kubeConfig string) (*Clients, error) {
	kubeClient, err := NewKubeClient(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubernetes client")
	}

	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return nil, errors.New("AZURE_SUBSCRIPTION_ID environment variable is not set")
	}

	azureServicesClient, err := NewAzureServicesClient(subscriptionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create azure services client")
	}
	return &Clients{kube: *kubeClient, azure: *azureServicesClient}, nil
}
