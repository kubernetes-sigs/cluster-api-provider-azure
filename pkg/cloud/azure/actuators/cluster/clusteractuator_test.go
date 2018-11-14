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
	"errors"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-01-01/network"
	azureconfigv1 "github.com/platform9/azure-provider/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"github.com/platform9/azure-provider/pkg/cloud/azure/services"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestActuatorCreateSuccess(t *testing.T) {
	azureServicesClient := services.AzureClients{Network: &services.MockAzureNetworkClient{}}
	params := ClusterActuatorParams{Services: &azureServicesClient}
	_, err := NewClusterActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
}

func TestNewAzureClientParamsPassed(t *testing.T) {
	azureServicesClient := services.AzureClients{Network: &services.MockAzureNetworkClient{}}
	params := ClusterActuatorParams{Services: &azureServicesClient}
	client, err := azureServicesClientOrDefault(params)
	if err != nil {
		t.Fatalf("unable to create azure services client: %v", err)
	}
	if client.Compute != nil {
		t.Fatal("expected azure services client to be nil")
	}
}

func TestReconcileSuccess(t *testing.T) {
	azureServicesClient := mockReconcileSuccess()
	params := ClusterActuatorParams{Services: &azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewClusterActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err != nil {
		t.Fatalf("unexpected error calling Reconcile: %v", err)
	}
}
func TestReconcileFailureNG(t *testing.T) {
	azureServicesClient := mockReconcileFailureNG()
	params := ClusterActuatorParams{Services: &azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewClusterActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}
func TestReconcileFailureVnet(t *testing.T) {
	azureServicesClient := mockReconcileFailureVnet()
	params := ClusterActuatorParams{Services: &azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewClusterActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}
func TestReconcileFailureRG(t *testing.T) {
	azureServicesClient := mockReconcileFailureRG()
	params := ClusterActuatorParams{Services: &azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewClusterActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Reconcile(cluster)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}
func TestDeleteSuccess(t *testing.T) {
	azureServicesClient := mockDeleteRGExists()
	params := ClusterActuatorParams{Services: &azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewClusterActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Delete(cluster)
	if err != nil {
		t.Fatalf("unexpected error calling Delete: %v", err)
	}
}

func TestDeleteFailureSGNotExists(t *testing.T) {
	azureServicesClient := mockDeleteRGNotExists()
	params := ClusterActuatorParams{Services: &azureServicesClient}
	cluster := newCluster(t)

	actuator, err := NewClusterActuator(params)
	if err != nil {
		t.Fatalf("unable to create cluster actuator: %v", err)
	}
	err = actuator.Delete(cluster)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func mockReconcileSuccess() services.AzureClients {
	networkMock := services.MockAzureNetworkClient{
		MockCreateOrUpdateNetworkSecurityGroup: func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
			return &network.SecurityGroupsCreateOrUpdateFuture{}, nil
		},
		MockCreateOrUpdateVnet: func(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error) {
			return &network.VirtualNetworksCreateOrUpdateFuture{}, nil
		},
	}
	return services.AzureClients{Resourcemanagement: &services.MockAzureResourceManagementClient{}, Network: &networkMock}
}

func mockReconcileFailureNG() services.AzureClients {
	networkMock := services.MockAzureNetworkClient{
		MockCreateOrUpdateNetworkSecurityGroup: func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
			return nil, errors.New("failed to create resource")
		},
	}
	return services.AzureClients{Resourcemanagement: &services.MockAzureResourceManagementClient{}, Network: &networkMock}
}

func mockReconcileFailureVnet() services.AzureClients {
	networkMock := services.MockAzureNetworkClient{
		MockCreateOrUpdateNetworkSecurityGroup: func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
			return &network.SecurityGroupsCreateOrUpdateFuture{}, nil
		},
		MockCreateOrUpdateVnet: func(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error) {
			return nil, errors.New("failed to create resource")
		},
	}
	return services.AzureClients{Resourcemanagement: &services.MockAzureResourceManagementClient{}, Network: &networkMock}
}

func mockReconcileFailureRG() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCreateOrUpdateGroup: func(resourceGroupName string, location string) (resources.Group, error) {
			return resources.Group{}, errors.New("failed to create resource")
		},
	}
	return services.AzureClients{Resourcemanagement: &resourcemanagementMock, Network: &services.MockAzureNetworkClient{}}
}

func mockDeleteRGExists() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
		},
		MockDeleteGroup: func(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
			return resources.GroupsDeleteFuture{}, nil
		},
	}
	return services.AzureClients{Resourcemanagement: &resourcemanagementMock}
}

func mockDeleteRGNotExists() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 404}}, nil
		},
	}
	return services.AzureClients{Resourcemanagement: &resourcemanagementMock}
}

func newClusterProviderConfig() azureconfigv1.AzureClusterProviderConfig {
	return azureconfigv1.AzureClusterProviderConfig{
		ResourceGroup: "resource-group-test",
		Location:      "westus2",
	}
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
