// +build e2e

/*
Copyright 2020 The Kubernetes Authors.

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
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"os"
	"path/filepath"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AzurePrivateClusterSpecInput is the input for AzurePrivateClusterSpec.
type AzurePrivateClusterSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	ClusterctlConfigPath  string
	E2EConfig             *clusterctl.E2EConfig
	ArtifactFolder        string
}

// AzurePrivateClusterSpec implements a test that creates a workload cluster with a private API endpoint.
func AzurePrivateClusterSpec(ctx context.Context, inputGetter func() AzurePrivateClusterSpecInput) {
	var (
		specName            = "azure-private-cluster"
		input               AzurePrivateClusterSpecInput
		publicClusterProxy  framework.ClusterProxy
		publicNamespace     *corev1.Namespace
		publicCancelWatches context.CancelFunc
		cluster             *clusterv1.Cluster
		clusterName         string
	)

	input = inputGetter()
	Expect(input).ToNot(BeNil())
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	By("creating a Kubernetes client to the workload cluster")
	publicClusterProxy = input.BootstrapClusterProxy.GetWorkloadCluster(context.TODO(), input.Namespace.Name, input.ClusterName)

	Byf("Creating a namespace for hosting the %s test spec", specName)
	publicNamespace, publicCancelWatches = framework.CreateNamespaceAndWatchEvents(context.TODO(), framework.CreateNamespaceAndWatchEventsInput{
		Creator:   publicClusterProxy.GetClient(),
		ClientSet: publicClusterProxy.GetClientSet(),
		Name:      input.Namespace.Name,
		LogFolder: filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName),
	})

	Expect(publicNamespace).NotTo(BeNil())
	Expect(publicCancelWatches).NotTo(BeNil())

	By("Initializing the workload cluster")
	clusterctl.InitManagementClusterAndWatchControllerLogs(context.TODO(), clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:            publicClusterProxy,
		ClusterctlConfigPath:    input.ClusterctlConfigPath,
		InfrastructureProviders: input.E2EConfig.InfrastructureProviders(),
		LogFolder:               filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName),
	}, input.E2EConfig.GetIntervals(specName, "wait-controllers")...)

	By("Ensure public API server is stable before creating private cluster")
	Consistently(func() error {
		kubeSystem := &corev1.Namespace{}
		return publicClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
	}, "5s", "100ms").Should(BeNil(), "Failed to assert public API server stability")

	By("Creating a private workload cluster")
	clusterName = fmt.Sprintf("capz-e2e-%s", util.RandomString(6))
	Expect(os.Setenv(AzureInternalLBIP, "10.128.0.100")).NotTo(HaveOccurred())
	result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy: publicClusterProxy,
		ConfigCluster: clusterctl.ConfigClusterInput{
			LogFolder:                filepath.Join(input.ArtifactFolder, "clusters", publicClusterProxy.GetName()),
			ClusterctlConfigPath:     input.ClusterctlConfigPath,
			KubeconfigPath:           publicClusterProxy.GetKubeconfigPath(),
			InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
			Flavor:                   "private",
			Namespace:                input.Namespace.Name,
			ClusterName:              clusterName,
			KubernetesVersion:        input.E2EConfig.GetVariable(capi_e2e.KubernetesVersion),
			ControlPlaneMachineCount: pointer.Int64Ptr(3),
			WorkerMachineCount:       pointer.Int64Ptr(1),
		},
		WaitForClusterIntervals:      input.E2EConfig.GetIntervals(specName, "wait-cluster"),
		WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(specName, "wait-control-plane"),
		WaitForMachineDeployments:    input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
	})
	cluster = result.Cluster

	Expect(cluster).ToNot(BeNil())
}

// SetupExistingVNet creates a resource group and a VNet to be used by a workload cluster.
func SetupExistingVNet(ctx context.Context, vnetCidr string, cpSubnetCidrs, nodeSubnetCidrs map[string]string) {
	By("creating Azure clients with the workload cluster's subscription")
	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	authorizer, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	groupClient := resources.NewGroupsClient(subscriptionID)
	groupClient.Authorizer = authorizer
	vnetClient := network.NewVirtualNetworksClient(subscriptionID)
	vnetClient.Authorizer = authorizer
	nsgClient := network.NewSecurityGroupsClient(subscriptionID)
	nsgClient.Authorizer = authorizer

	By("creating a resource group")
	groupName := os.Getenv(AzureResourceGroup)
	_, err = groupClient.CreateOrUpdate(ctx, groupName, resources.Group{
		Location: pointer.StringPtr(os.Getenv(AzureLocation)),
		Tags: map[string]*string{
			"jobName":           pointer.StringPtr(os.Getenv(JobName)),
			"creationTimestamp": pointer.StringPtr(os.Getenv(Timestamp)),
		},
	})
	Expect(err).To(BeNil())

	By("creating a network security group")
	nsgName := "control-plane-nsg"
	securityRules := []network.SecurityRule{
		{
			Name: pointer.StringPtr("allow_ssh"),
			SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
				Description:              pointer.StringPtr("Allow SSH"),
				Priority:                 pointer.Int32Ptr(2200),
				Protocol:                 network.SecurityRuleProtocolTCP,
				Access:                   network.SecurityRuleAccessAllow,
				Direction:                network.SecurityRuleDirectionInbound,
				SourceAddressPrefix:      pointer.StringPtr("*"),
				SourcePortRange:          pointer.StringPtr("*"),
				DestinationAddressPrefix: pointer.StringPtr("*"),
				DestinationPortRange:     pointer.StringPtr("22"),
			},
		},
		{
			Name: pointer.StringPtr("allow_apiserver"),
			SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
				Description:              pointer.StringPtr("Allow API Server"),
				SourcePortRange:          pointer.StringPtr("*"),
				DestinationPortRange:     pointer.StringPtr("6443"),
				SourceAddressPrefix:      pointer.StringPtr("*"),
				DestinationAddressPrefix: pointer.StringPtr("*"),
				Protocol:                 network.SecurityRuleProtocolTCP,
				Access:                   network.SecurityRuleAccessAllow,
				Direction:                network.SecurityRuleDirectionInbound,
				Priority:                 pointer.Int32Ptr(2201),
			},
		},
	}
	nsgFuture, err := nsgClient.CreateOrUpdate(ctx, groupName, nsgName, network.SecurityGroup{
		Location: pointer.StringPtr(os.Getenv(AzureLocation)),
		SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
			SecurityRules: &securityRules,
		},
	})
	Expect(err).To(BeNil())
	err = nsgFuture.WaitForCompletionRef(ctx, nsgClient.Client)
	Expect(err).To(BeNil())

	By("creating a virtual network")
	var subnets []network.Subnet
	for name, cidr := range cpSubnetCidrs {
		subnets = append(subnets, network.Subnet{
			SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
				AddressPrefix: pointer.StringPtr(cidr),
				NetworkSecurityGroup: &network.SecurityGroup{
					ID: pointer.StringPtr(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s", subscriptionID, groupName, nsgName)),
				},
			},
			Name: pointer.StringPtr(name),
		})
	}
	for name, cidr := range nodeSubnetCidrs {
		subnets = append(subnets, network.Subnet{
			SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
				AddressPrefix: pointer.StringPtr(cidr),
			},
			Name: pointer.StringPtr(name),
		})
	}

	vnetFuture, err := vnetClient.CreateOrUpdate(ctx, groupName, os.Getenv(AzureVNetName), network.VirtualNetwork{
		Location: pointer.StringPtr(os.Getenv(AzureLocation)),
		VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
			AddressSpace: &network.AddressSpace{
				AddressPrefixes: &[]string{vnetCidr},
			},
			Subnets: &subnets,
		},
	})
	if err != nil {
		fmt.Print(err.Error())
	}
	Expect(err).To(BeNil())
	err = vnetFuture.WaitForCompletionRef(ctx, vnetClient.Client)
	Expect(err).To(BeNil())
}
