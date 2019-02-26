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

package cluster

import (
	"sigs.k8s.io/cluster-api/pkg/controller/cluster"
)

var (
	_ cluster.Actuator = (*Actuator)(nil)
)

// TODO: Reimplement tests
/*
import (
	"os"
	"testing"

	"github.com/imdario/mergo"

	providerv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"github.com/ghodss/yaml"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services"
)

func TestActuatorCreateSuccess(t *testing.T) {
	azureServicesClient := actuators.AzureClients{Network: &services.MockAzureNetworkClient{}}
	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
}

func TestActuatorCreateFailure(t *testing.T) {
	if err := os.Setenv("AZURE_ENVIRONMENT", "dummy"); err != nil {
		t.Fatalf("error when setting AZURE_ENVIRONMENT environment variable")
	}
	_, err := NewActuator(actuators.ScopeParams{})
	if err == nil {
		t.Fatalf("expected error when creating the cluster actuator but got none")
	}
	os.Unsetenv("AZURE_ENVIRONMENT")
}

func TestNewAzureClientParamsPassed(t *testing.T) {
	azureServicesClient := actuators.AzureClients{Compute: &services.MockAzureComputeClient{}}
	params := actuators.ScopeParams{AzureClients: azureServicesClient}
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
	client, err := actuators.NewScope(actuators.ScopeParams{})
	if err != nil {
		t.Fatalf("unable to create azure services client: %v", err)
	}
	// cluster actuator doesn't utilize compute client
	if client.Compute != nil {
		t.Fatal("expected compute client to be nil")
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
	_, err := actuators.NewScope(actuators.ScopeParams{})
	if err == nil {
		t.Fatalf("expected error when creating the azure services client but got none")
	}
	os.Unsetenv("AZURE_ENVIRONMENT")
}

func TestNewAzureClientSubscriptionFailure(t *testing.T) {
	_, err := actuators.NewScope(actuators.ScopeParams{})
	if err == nil {
		t.Fatalf("expected error when creating the azure services client but got none")
	}
}

func TestReconcileSuccess(t *testing.T) {
	networkMock := services.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, services.MockNsgCreateOrUpdateSuccess())
	mergo.Merge(&networkMock, services.MockVnetCreateOrUpdateSuccess())
	azureServicesClient := actuators.AzureClients{Resources: &services.MockAzureResourcesClient{}, Network: &networkMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err != nil {
		t.Fatalf("unexpected error calling Reconcile: %v", err)
	}
}

func TestReconcileFailureParsing(t *testing.T) {
	cluster := newCluster(t)
	actuator, err := NewActuator(actuators.ScopeParams{})
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}

	err = actuator.Reconcile(cluster)
	if err == nil {
		t.Fatal("expected error when calling Reconcile but got none")
	}
}
func TestReconcileFailureRGCreation(t *testing.T) {
	resourceManagementMock := services.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, services.MockRgCreateOrUpdateFailure())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err == nil {
		t.Fatalf("expected error when reconciling cluster, but got none")
	}
}

func TestReconcileFailureNSGCreation(t *testing.T) {
	networkMock := services.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, services.MockNsgCreateOrUpdateFailure())
	azureServicesClient := actuators.AzureClients{Resources: &services.MockAzureResourcesClient{}, Network: &networkMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err == nil {
		t.Fatalf("expected error when reconciling cluster, but got none")
	}
}

func TestReconcileFailureNSGFutureError(t *testing.T) {
	networkMock := services.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, services.MockNsgCreateOrUpdateSuccess())
	mergo.Merge(&networkMock, services.MockNsgCreateOrUpdateFutureFailure())
	azureServicesClient := actuators.AzureClients{Resources: &services.MockAzureResourcesClient{}, Network: &networkMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err == nil {
		t.Fatalf("expected error when reconciling cluster, but got none")
	}
}

func TestReconcileFailureVnetCreation(t *testing.T) {
	networkMock := services.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, services.MockNsgCreateOrUpdateSuccess())
	mergo.Merge(&networkMock, services.MockVnetCreateOrUpdateFailure())
	azureServicesClient := actuators.AzureClients{Resources: &services.MockAzureResourcesClient{}, Network: &networkMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err == nil {
		t.Fatalf("expected error when reconciling cluster, but got none")
	}
}

func TestReconcileFailureVnetFutureError(t *testing.T) {
	networkMock := services.MockAzureNetworkClient{}
	mergo.Merge(&networkMock, services.MockNsgCreateOrUpdateSuccess())
	mergo.Merge(&networkMock, services.MockVnetCreateOrUpdateSuccess())
	mergo.Merge(&networkMock, services.MockVnetCreateOrUpdateFutureFailure())
	azureServicesClient := actuators.AzureClients{Resources: &services.MockAzureResourcesClient{}, Network: &networkMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err == nil {
		t.Fatalf("expected error when reconciling cluster, but got none")
	}
}

func TestDeleteSuccess(t *testing.T) {
	resourceManagementMock := services.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, services.MockRgExists())
	mergo.Merge(&resourceManagementMock, services.MockRgDeleteSuccess())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Delete(cluster)
	if err != nil {
		t.Fatalf("unexpected error calling Delete: %v", err)
	}
}
func TestDeleteFailureParsing(t *testing.T) {
	azureServicesClient := actuators.AzureClients{Resources: &services.MockAzureResourcesClient{}, Network: &services.MockAzureNetworkClient{}}
	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: bytes}

	err = actuator.Delete(cluster)
	if err == nil {
		t.Fatal("expected error when calling Delete but got none")
	}
}
func TestDeleteFailureRGNotExists(t *testing.T) {
	resourceManagementMock := services.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, services.MockRgNotExists())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Delete(cluster)
	if err == nil {
		t.Fatalf("expected error when deleting cluster, but got none")
	}
}

func TestDeleteFailureRGCheckFailure(t *testing.T) {
	resourceManagementMock := services.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, services.MockRgCheckFailure())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Delete(cluster)
	if err == nil {
		t.Fatalf("expected error when deleting cluster, but got none")
	}
}

func TestDeleteFailureRGDeleteFailure(t *testing.T) {
	resourceManagementMock := services.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, services.MockRgExists())
	mergo.Merge(&resourceManagementMock, services.MockRgDeleteFailure())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Delete(cluster)
	if err == nil {
		t.Fatalf("expected error when deleting cluster, but got none")
	}
}

func TestDeleteFailureRGDeleteFutureFailure(t *testing.T) {
	resourceManagementMock := services.MockAzureResourcesClient{}
	mergo.Merge(&resourceManagementMock, services.MockRgExists())
	mergo.Merge(&resourceManagementMock, services.MockRgDeleteFutureFailure())
	azureServicesClient := actuators.AzureClients{Resources: &resourceManagementMock}

	params := actuators.ScopeParams{AzureClients: azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Delete(cluster)
	if err == nil {
		t.Fatalf("expected error when deleting cluster, but got none")
	}
}

func TestClusterProviderFromProviderSpecParsingError(t *testing.T) {
	bytes, err := yaml.Marshal("dummy")
	if err != nil {
		t.Fatalf("error while marshalling yaml")
	}
	providerSpec := &clusterv1.ProviderSpec{
		Value: &runtime.RawExtension{Raw: bytes},
	}
	_, err = clusterProviderFromProviderSpec(*providerSpec)
	if err == nil {
		t.Fatalf("expected error when parsing provider config, but got none")
	}
}

func newClusterProviderSpec() providerv1.AzureClusterProviderSpec {
	return providerv1.AzureClusterProviderSpec{
		ResourceGroup: "resource-group-test",
		Location:      "eastus2",
	}
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

func newCluster(t *testing.T) *v1alpha1.Cluster {
	clusterProviderSpec := newClusterProviderSpec()
	providerSpec, err := providerSpecFromCluster(&clusterProviderSpec)
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
			ProviderSpec: *providerSpec,
		},
	}
}
*/
