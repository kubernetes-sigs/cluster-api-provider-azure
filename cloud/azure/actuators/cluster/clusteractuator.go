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
	"fmt"
	"log"
	"os"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/golang/glog"
	"github.com/joho/godotenv"
	azureconfigv1 "github.com/platform9/azure-provider/cloud/azure/providerconfig/v1alpha1"
	"github.com/platform9/azure-provider/cloud/azure/services"
	"github.com/platform9/azure-provider/cloud/azure/services/network"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
)

type AzureClusterClient struct {
	services                 *services.AzureClients
	clusterClient            client.ClusterInterface
	azureProviderConfigCodec *azureconfigv1.AzureProviderConfigCodec
}

type ClusterActuatorParams struct {
	ClusterClient client.ClusterInterface
	Services      *services.AzureClients
}

func NewClusterActuator(params ClusterActuatorParams) (*AzureClusterClient, error) {
	_, azureProviderConfigCodec, err := azureconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}
	azureServicesClients, err := azureServicesClientOrDefault(params)
	if err != nil {
		return nil, err
	}

	return &AzureClusterClient{
		services:                 azureServicesClients,
		clusterClient:            params.ClusterClient,
		azureProviderConfigCodec: azureProviderConfigCodec,
	}, nil
}

func (azure *AzureClusterClient) Reconcile(cluster *clusterv1.Cluster) error {
	glog.Infof("Reconciling cluster %v.", cluster.Name)

	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return err
	}
	networkSGFuture, err := azure.network().CreateOrUpdateNetworkSecurityGroup(clusterConfig.ResourceGroup, "ClusterAPINSG", clusterConfig.Location)
	if err != nil {
		return err
	}
	err = azure.network().WaitForNetworkSGsCreateOrUpdateFuture(*networkSGFuture)
	if err != nil {
		return err
	}

	return err
}

func (azure *AzureClusterClient) Delete(cluster *clusterv1.Cluster) error {
	//TODO: get rid of the whole resource group?
	return fmt.Errorf("NYI: Cluster Deletions are not yet supported")
}

func azureServicesClientOrDefault(params ClusterActuatorParams) (*services.AzureClients, error) {
	if params.Services != nil {
		return params.Services, nil
	}
	//Parse in environment variables if necessary
	if os.Getenv("AZURE_SUBSCRIPTION_ID") == "" {
		err := godotenv.Load()
		if err == nil && os.Getenv("AZURE_SUBSCRIPTION_ID") == "" {
			err = errors.New("AZURE_SUBSCRIPTION_ID: \"\"")
		}
		if err != nil {
			log.Fatalf("Failed to load environment variables: %v", err)
			return nil, err
		}
	}

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Fatalf("Failed to get OAuth config: %v", err)
		return nil, err
	}
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if err != nil {
		return nil, err
	}
	azureNetworkClient := network.NewService(subscriptionID)
	azureNetworkClient.SetAuthorizer(authorizer)
	return &services.AzureClients{
		Network: azureNetworkClient,
	}, nil
}

func (azure *AzureClusterClient) network() services.AzureNetworkClient {
	return azure.services.Network
}
