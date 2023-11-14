//go:build e2e
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
	"os"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
	SkipCleanup           bool
	CancelWatches         context.CancelFunc
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
	Expect(input).NotTo(BeNil())
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	By("creating a Kubernetes client to the workload cluster")
	publicClusterProxy = input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)

	Byf("Creating a namespace for hosting the %s test spec", specName)
	Logf("starting to create namespace for hosting the %s test spec", specName)
	publicNamespace, publicCancelWatches = framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   publicClusterProxy.GetClient(),
		ClientSet: publicClusterProxy.GetClientSet(),
		Name:      input.Namespace.Name,
		LogFolder: filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName),
	})

	Expect(publicNamespace).NotTo(BeNil())
	Expect(publicCancelWatches).NotTo(BeNil())

	By("Initializing the workload cluster")
	clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:            publicClusterProxy,
		ClusterctlConfigPath:    input.ClusterctlConfigPath,
		InfrastructureProviders: input.E2EConfig.InfrastructureProviders(),
		AddonProviders:          input.E2EConfig.AddonProviders(),
		LogFolder:               filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName),
	}, input.E2EConfig.GetIntervals(specName, "wait-controllers")...)

	By("Ensure public API server is stable before creating private cluster")
	Consistently(func() error {
		ns := &corev1.Namespace{}
		return publicClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: kubesystem}, ns)
	}, "5s", "100ms").Should(BeNil(), "Failed to assert public API server stability")

	// **************
	// Get the Client ID for the user assigned identity
	subscriptionID := os.Getenv(AzureSubscriptionID)
	identityRG, ok := os.LookupEnv(AzureIdentityResourceGroup)
	if !ok {
		identityRG = "capz-ci"
	}
	userID, ok := os.LookupEnv(AzureUserIdentity)
	if !ok {
		userID = "cloud-provider-user-identity"
	}
	resourceID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s", subscriptionID, identityRG, userID)
	os.Setenv("UAMI_CLIENT_ID", getClientIDforMSI(resourceID))

	os.Setenv("CLUSTER_IDENTITY_NAME", "cluster-identity-user-assigned")
	os.Setenv("CLUSTER_IDENTITY_NAMESPACE", input.Namespace.Name)
	// *************

	By("Creating a private workload cluster")
	clusterName = fmt.Sprintf("capz-e2e-%s-%s", util.RandomString(6), "private")
	Expect(os.Setenv(AzureVNetName, clusterName+"-vnet")).To(Succeed())
	Expect(os.Setenv(AzureVNetCidr, "10.255.0.0/16")).To(Succeed())
	Expect(os.Setenv(AzureInternalLBIP, "10.255.0.100")).To(Succeed())
	Expect(os.Setenv(AzureCPSubnetCidr, "10.255.0.0/24")).To(Succeed())
	Expect(os.Setenv(AzureNodeSubnetCidr, "10.255.1.0/24")).To(Succeed())
	Expect(os.Setenv(AzureBastionSubnetCidr, "10.255.255.224/27")).To(Succeed())
	result := &clusterctl.ApplyClusterTemplateAndWaitResult{}

	// NOTE: We don't add control plane waiters here because Helm install will fail since the apiserver is private and not reachable from the prow cluster.
	// As a workaround, we use in-tree cloud-provider-azure on the private cluster until a Helm integration is available.
	clusterctl.ApplyClusterTemplateAndWait(ctx, createApplyClusterTemplateInput(
		specName,
		withClusterProxy(publicClusterProxy),
		withFlavor("private"),
		withNamespace(input.Namespace.Name),
		withClusterName(clusterName),
		withControlPlaneMachineCount(3),
		withWorkerMachineCount(1),
		withClusterInterval(specName, "wait-private-cluster"),
		withControlPlaneInterval(specName, "wait-control-plane-ha"),
	), result)
	cluster = result.Cluster

	Expect(cluster).NotTo(BeNil())

	defer func() {
		// Delete the private cluster, so that all of the Azure resources will be cleaned up when the public
		// cluster is deleted at the end of the test. If we don't delete this cluster, the Azure resource delete
		// verification will fail.
		cleanInput := cleanupInput{
			SpecName:               specName,
			Cluster:                cluster,
			ClusterProxy:           publicClusterProxy,
			Namespace:              input.Namespace,
			CancelWatches:          publicCancelWatches,
			IntervalsGetter:        e2eConfig.GetIntervals,
			SkipCleanup:            input.SkipCleanup,
			SkipLogCollection:      skipLogCollection,
			ArtifactFolder:         input.ArtifactFolder,
			SkipResourceGroupCheck: true, // We don't expect the resource group to be deleted since the private cluster does not own the resource group.
		}
		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	}()

	// Check that azure bastion is provisioned successfully.
	{
		By("verifying the Azure Bastion Host was created successfully")
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		Expect(err).NotTo(HaveOccurred())

		azureBastionClient, err := armnetwork.NewBastionHostsClient(getSubscriptionID(Default), cred, nil)
		Expect(err).To(BeNil())

		groupName := os.Getenv(AzureResourceGroup)
		azureBastionName := fmt.Sprintf("%s-azure-bastion", clusterName)

		backoff := wait.Backoff{
			Duration: retryBackoffInitialDuration,
			Factor:   retryBackoffFactor,
			Jitter:   retryBackoffJitter,
			Steps:    retryBackoffSteps,
		}
		retryFn := func() (bool, error) {
			resp, err := azureBastionClient.Get(ctx, groupName, azureBastionName, nil)
			if err != nil {
				return false, err
			}

			bastion := resp.BastionHost
			switch ptr.Deref(bastion.Properties.ProvisioningState, "") {
			case armnetwork.ProvisioningStateSucceeded:
				return true, nil
			case armnetwork.ProvisioningStateUpdating:
				// Wait for operation to complete.
				return false, nil
			default:
				return false, errors.New(fmt.Sprintf("Azure Bastion provisioning failed with state: %q", ptr.Deref(bastion.Properties.ProvisioningState, "(nil)")))
			}
		}
		err = wait.ExponentialBackoff(backoff, retryFn)

		Expect(err).To(BeNil())
	}
}

// SetupExistingVNet creates a resource group and a VNet to be used by a workload cluster.
func SetupExistingVNet(ctx context.Context, vnetCidr string, cpSubnetCidrs, nodeSubnetCidrs map[string]string, bastionSubnetName, bastionSubnetCidr string) func() {
	By("creating Azure clients with the workload cluster's subscription")
	subscriptionID := getSubscriptionID(Default)
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	groupClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())
	vnetClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())
	nsgClient, err := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())
	routetableClient, err := armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	By("creating a resource group")
	groupName := os.Getenv(AzureCustomVnetResourceGroup)
	_, err = groupClient.CreateOrUpdate(ctx, groupName, armresources.ResourceGroup{
		Location: ptr.To(os.Getenv(AzureLocation)),
		Tags: map[string]*string{
			"jobName":           ptr.To(os.Getenv(JobName)),
			"creationTimestamp": ptr.To(os.Getenv(Timestamp)),
		},
	}, nil)
	Expect(err).To(BeNil())

	By("creating a network security group")
	nsgName := "control-plane-nsg"
	securityRules := []*armnetwork.SecurityRule{
		{
			Name: ptr.To("allow_ssh"),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Description:              ptr.To("Allow SSH"),
				Priority:                 ptr.To[int32](2200),
				Protocol:                 ptr.To(armnetwork.SecurityRuleProtocolTCP),
				Access:                   ptr.To(armnetwork.SecurityRuleAccessAllow),
				Direction:                ptr.To(armnetwork.SecurityRuleDirectionInbound),
				SourceAddressPrefix:      ptr.To("*"),
				SourcePortRange:          ptr.To("*"),
				DestinationAddressPrefix: ptr.To("*"),
				DestinationPortRange:     ptr.To("22"),
			},
		},
		{
			Name: ptr.To("allow_apiserver"),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Description:              ptr.To("Allow API Server"),
				SourcePortRange:          ptr.To("*"),
				DestinationPortRange:     ptr.To("6443"),
				SourceAddressPrefix:      ptr.To("*"),
				DestinationAddressPrefix: ptr.To("*"),
				Protocol:                 ptr.To(armnetwork.SecurityRuleProtocolTCP),
				Access:                   ptr.To(armnetwork.SecurityRuleAccessAllow),
				Direction:                ptr.To(armnetwork.SecurityRuleDirectionInbound),
				Priority:                 ptr.To[int32](2201),
			},
		},
	}
	nsgPoller, err := nsgClient.BeginCreateOrUpdate(ctx, groupName, nsgName, armnetwork.SecurityGroup{
		Location: ptr.To(os.Getenv(AzureLocation)),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: securityRules,
		},
	}, nil)
	Expect(err).To(BeNil())
	_, err = nsgPoller.PollUntilDone(ctx, nil)
	Expect(err).To(BeNil())

	By("creating a node security group")
	nsgNodeName := "node-nsg"
	securityRulesNode := []*armnetwork.SecurityRule{}
	nsgNodePoller, err := nsgClient.BeginCreateOrUpdate(ctx, groupName, nsgNodeName, armnetwork.SecurityGroup{
		Location: ptr.To(os.Getenv(AzureLocation)),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: securityRulesNode,
		},
	}, nil)
	Expect(err).To(BeNil())
	_, err = nsgNodePoller.PollUntilDone(ctx, nil)
	Expect(err).To(BeNil())

	By("creating a node routetable")
	routeTableName := "node-routetable"
	routeTable := armnetwork.RouteTable{
		Location:   ptr.To(os.Getenv(AzureLocation)),
		Properties: &armnetwork.RouteTablePropertiesFormat{},
	}
	routetablePoller, err := routetableClient.BeginCreateOrUpdate(ctx, groupName, routeTableName, routeTable, nil)
	Expect(err).To(BeNil())
	_, err = routetablePoller.PollUntilDone(ctx, nil)
	Expect(err).To(BeNil())

	By("creating a virtual network")
	var subnets []*armnetwork.Subnet
	for name, cidr := range cpSubnetCidrs {
		subnets = append(subnets, &armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix: ptr.To(cidr),
				NetworkSecurityGroup: &armnetwork.SecurityGroup{
					ID: ptr.To(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s", subscriptionID, groupName, nsgName)),
				},
			},
			Name: ptr.To(name),
		})
	}
	for name, cidr := range nodeSubnetCidrs {
		subnets = append(subnets, &armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix: ptr.To(cidr),
				NetworkSecurityGroup: &armnetwork.SecurityGroup{
					ID: ptr.To(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s", subscriptionID, groupName, nsgNodeName)),
				},
				RouteTable: &armnetwork.RouteTable{
					ID: ptr.To(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/routeTables/%s", subscriptionID, groupName, routeTableName)),
				},
			},
			Name: ptr.To(name),
		})
	}

	// Create the AzureBastion subnet.
	subnets = append(subnets, &armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: ptr.To(bastionSubnetCidr),
		},
		Name: ptr.To(bastionSubnetName),
	})

	vnetPoller, err := vnetClient.BeginCreateOrUpdate(ctx, groupName, os.Getenv(AzureCustomVNetName), armnetwork.VirtualNetwork{
		Location: ptr.To(os.Getenv(AzureLocation)),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{ptr.To(vnetCidr)},
			},
			Subnets: subnets,
		},
	}, nil)
	if err != nil {
		fmt.Print(err.Error())
	}
	Expect(err).To(BeNil())
	_, err = vnetPoller.PollUntilDone(ctx, nil)
	Expect(err).To(BeNil())

	return func() {
		Logf("deleting an existing virtual network %q", os.Getenv(AzureCustomVNetName))
		vPoller, err := vnetClient.BeginDelete(ctx, groupName, os.Getenv(AzureCustomVNetName), nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = vPoller.PollUntilDone(ctx, nil)
		Expect(err).NotTo(HaveOccurred())

		Logf("deleting an existing route table %q", routeTableName)
		rtPoller, err := routetableClient.BeginDelete(ctx, groupName, routeTableName, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = rtPoller.PollUntilDone(ctx, nil)
		Expect(err).NotTo(HaveOccurred())

		Logf("deleting an existing network security group %q", nsgNodeName)
		nsgPoller, err := nsgClient.BeginDelete(ctx, groupName, nsgNodeName, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = nsgPoller.PollUntilDone(ctx, nil)
		Expect(err).NotTo(HaveOccurred())

		Logf("deleting an existing network security group %q", nsgName)
		nsgPoller, err = nsgClient.BeginDelete(ctx, groupName, nsgName, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = nsgPoller.PollUntilDone(ctx, nil)
		Expect(err).NotTo(HaveOccurred())

		Logf("verifying the existing resource group %q is empty", groupName)
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		Expect(err).NotTo(HaveOccurred())
		resClient, err := armresources.NewClient(getSubscriptionID(Default), cred, nil)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]*armresources.GenericResourceExpanded, error) {
			var foundResources []*armresources.GenericResourceExpanded
			opts := armresources.ClientListByResourceGroupOptions{
				Expand: ptr.To("provisioningState"),
				Top:    ptr.To[int32](10),
			}
			pager := resClient.NewListByResourceGroupPager(groupName, &opts)
			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					return nil, err
				}
				foundResources = append(foundResources, page.Value...)
			}
			return foundResources, nil
			// add some tolerance for Azure caching of resource group resources
		}, deleteOperationTimeout, retryableOperationTimeout).Should(BeEmpty(), "Expect the manually created resource group is empty after removing the manually created resources.")

		Logf("deleting the existing resource group %q", groupName)
		grpPoller, err := groupClient.BeginDelete(ctx, groupName, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = grpPoller.PollUntilDone(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
	}
}

// getClientIDforMSI fetches the client ID of a user assigned identity.
func getClientIDforMSI(resourceID string) string {
	subscriptionID := getSubscriptionID(Default)
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	msiClient, err := armmsi.NewUserAssignedIdentitiesClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	parsed, err := azureutil.ParseResourceID(resourceID)
	Expect(err).NotTo(HaveOccurred())

	resp, err := msiClient.Get(context.TODO(), parsed.ResourceGroupName, parsed.Name, nil)
	Expect(err).NotTo(HaveOccurred())

	return *resp.Properties.ClientID
}
