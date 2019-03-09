/*
Copyright 2019 The Kubernetes Authors.

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

package actuators

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
)

// ScopeParams defines the input parameters used to create a new Scope.
type ScopeParams struct {
	AzureClients
	Cluster *clusterv1.Cluster
	Client  client.ClusterV1alpha1Interface
}

// NewScope creates a new Scope from the supplied parameters.
// This is meant to be called for each different actuator iteration.
func NewScope(params ScopeParams) (*Scope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil cluster")
	}

	clusterConfig, err := v1alpha1.ClusterConfigFromProviderSpec(params.Cluster.Spec.ProviderSpec)
	if err != nil {
		return nil, errors.Errorf("failed to load cluster provider config: %v", err)
	}

	clusterStatus, err := v1alpha1.ClusterStatusFromProviderStatus(params.Cluster.Status.ProviderStatus)
	if err != nil {
		return nil, errors.Errorf("failed to load cluster provider status: %v", err)
	}

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, errors.Errorf("failed to create azure session: %v", err)
	}

	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return nil, fmt.Errorf("error creating azure services. Environment variable AZURE_SUBSCRIPTION_ID is not set")
	}

	// Compute
	params.AzureClients.VM = compute.NewVirtualMachinesClient(subscriptionID)
	params.AzureClients.VM.Authorizer = authorizer

	params.AzureClients.Disks = compute.NewDisksClient(subscriptionID)
	params.AzureClients.Disks.Authorizer = authorizer

	// Network
	params.AzureClients.VirtualNetworks = network.NewVirtualNetworksClient(subscriptionID)
	params.AzureClients.VirtualNetworks.Authorizer = authorizer

	params.AzureClients.SecurityGroups = network.NewSecurityGroupsClient(subscriptionID)
	params.AzureClients.SecurityGroups.Authorizer = authorizer

	params.AzureClients.Interfaces = network.NewInterfacesClient(subscriptionID)
	params.AzureClients.Interfaces.Authorizer = authorizer

	params.AzureClients.LB = network.NewLoadBalancersClient(subscriptionID)
	params.AzureClients.LB.Authorizer = authorizer

	params.AzureClients.PublicIPAddresses = network.NewPublicIPAddressesClient(subscriptionID)
	params.AzureClients.PublicIPAddresses.Authorizer = authorizer

	// Resources
	params.AzureClients.Groups = resources.NewGroupsClient(subscriptionID)
	params.AzureClients.Groups.Authorizer = authorizer

	params.AzureClients.Deployments = resources.NewDeploymentsClient(subscriptionID)
	params.AzureClients.Deployments.Authorizer = authorizer

	params.AzureClients.Tags = resources.NewTagsClient(subscriptionID)
	params.AzureClients.Tags.Authorizer = authorizer

	var clusterClient client.ClusterInterface
	if params.Client != nil {
		clusterClient = params.Client.Clusters(params.Cluster.Namespace)
	}

	return &Scope{
		AzureClients:  params.AzureClients,
		Cluster:       params.Cluster,
		ClusterClient: clusterClient,
		ClusterConfig: clusterConfig,
		ClusterStatus: clusterStatus,
		Context:       context.Background(),
	}, nil
}

// Scope defines the basic context for an actuator to operate upon.
type Scope struct {
	AzureClients
	Cluster       *clusterv1.Cluster
	ClusterClient client.ClusterInterface
	ClusterConfig *v1alpha1.AzureClusterProviderSpec
	ClusterStatus *v1alpha1.AzureClusterProviderStatus
	Context       context.Context
}

// Network returns the cluster network object.
func (s *Scope) Network() *v1alpha1.Network {
	return &s.ClusterStatus.Network
}

// TODO: Rewrite to use ClusterConfig and NetworkSpec
// Vnet returns the cluster Vnet.
func (s *Scope) Vnet() *v1alpha1.Vnet {
	return &s.ClusterStatus.Network.Vnet
}

// TODO: Rewrite to use ClusterConfig and NetworkSpec
// Subnets returns the cluster subnets.
func (s *Scope) Subnets() v1alpha1.Subnets {
	return s.ClusterStatus.Network.Subnets
}

// SecurityGroups returns the cluster security groups as a map, it creates the map if empty.
func (s *Scope) SecurityGroups() map[v1alpha1.SecurityGroupRole]*v1alpha1.SecurityGroup {
	return s.ClusterStatus.Network.SecurityGroups
}

// Name returns the cluster name.
func (s *Scope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *Scope) Namespace() string {
	return s.Cluster.Namespace
}

// Location returns the cluster location.
func (s *Scope) Location() string {
	return s.ClusterConfig.Location
}

func (s *Scope) storeClusterConfig(cluster *clusterv1.Cluster) (*clusterv1.Cluster, error) {
	ext, err := v1alpha1.EncodeClusterSpec(s.ClusterConfig)
	if err != nil {
		return nil, err
	}

	cluster.Spec.ProviderSpec.Value = ext
	return s.ClusterClient.Update(cluster)
}

func (s *Scope) storeClusterStatus(cluster *clusterv1.Cluster) (*clusterv1.Cluster, error) {
	ext, err := v1alpha1.EncodeClusterStatus(s.ClusterStatus)
	if err != nil {
		return nil, err
	}

	cluster.Status.ProviderStatus = ext
	return s.ClusterClient.UpdateStatus(cluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *Scope) Close() {
	if s.ClusterClient == nil {
		return
	}

	latestCluster, err := s.storeClusterConfig(s.Cluster)
	if err != nil {
		klog.Errorf("[scope] failed to store provider config for cluster %q in namespace %q: %v", s.Cluster.Name, s.Cluster.Namespace, err)
		return
	}

	_, err = s.storeClusterStatus(latestCluster)
	if err != nil {
		klog.Errorf("[scope] failed to store provider status for cluster %q in namespace %q: %v", s.Cluster.Name, s.Cluster.Namespace, err)
	}
}
