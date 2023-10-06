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
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
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
		By("verifying the Azure Bastion Host was create successfully")
		settings, err := auth.GetSettingsFromEnvironment()
		Expect(err).To(BeNil())

		azureBastionClient := network.NewBastionHostsClient(settings.GetSubscriptionID())
		azureBastionClient.Authorizer, err = azureutil.GetAuthorizer(settings)
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
			bastion, err := azureBastionClient.Get(ctx, groupName, azureBastionName)
			if err != nil {
				return false, err
			}

			switch bastion.ProvisioningState {
			case network.ProvisioningStateSucceeded:
				return true, nil
			case network.ProvisioningStateUpdating:
				// Wait for operation to complete.
				return false, nil
			default:
				return false, errors.New(fmt.Sprintf("Azure Bastion provisioning failed with state: %q", bastion.ProvisioningState))
			}
		}
		err = wait.ExponentialBackoff(backoff, retryFn)

		Expect(err).To(BeNil())
	}
}

// SetupExistingVNet creates a resource group and a VNet to be used by a workload cluster.
func SetupExistingVNet(ctx context.Context, vnetCidr string, cpSubnetCidrs, nodeSubnetCidrs map[string]string, bastionSubnetName, bastionSubnetCidr string) func() {
	By("creating Azure clients with the workload cluster's subscription")
	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	authorizer, err := azureutil.GetAuthorizer(settings)
	Expect(err).NotTo(HaveOccurred())
	groupClient := resources.NewGroupsClient(subscriptionID)
	groupClient.Authorizer = authorizer
	vnetClient := network.NewVirtualNetworksClient(subscriptionID)
	vnetClient.Authorizer = authorizer
	nsgClient := network.NewSecurityGroupsClient(subscriptionID)
	nsgClient.Authorizer = authorizer
	routetableClient := network.NewRouteTablesClient(subscriptionID)
	routetableClient.Authorizer = authorizer

	By("creating a resource group")
	groupName := os.Getenv(AzureResourceGroup)
	_, err = groupClient.CreateOrUpdate(ctx, groupName, resources.Group{
		Location: ptr.To(os.Getenv(AzureLocation)),
		Tags: map[string]*string{
			"jobName":           ptr.To(os.Getenv(JobName)),
			"creationTimestamp": ptr.To(os.Getenv(Timestamp)),
		},
	})
	Expect(err).To(BeNil())

	By("creating a network security group")
	nsgName := "control-plane-nsg"
	securityRules := []network.SecurityRule{
		{
			Name: ptr.To("allow_ssh"),
			SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
				Description:              ptr.To("Allow SSH"),
				Priority:                 ptr.To[int32](2200),
				Protocol:                 network.SecurityRuleProtocolTCP,
				Access:                   network.SecurityRuleAccessAllow,
				Direction:                network.SecurityRuleDirectionInbound,
				SourceAddressPrefix:      ptr.To("*"),
				SourcePortRange:          ptr.To("*"),
				DestinationAddressPrefix: ptr.To("*"),
				DestinationPortRange:     ptr.To("22"),
			},
		},
		{
			Name: ptr.To("allow_apiserver"),
			SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
				Description:              ptr.To("Allow API Server"),
				SourcePortRange:          ptr.To("*"),
				DestinationPortRange:     ptr.To("6443"),
				SourceAddressPrefix:      ptr.To("*"),
				DestinationAddressPrefix: ptr.To("*"),
				Protocol:                 network.SecurityRuleProtocolTCP,
				Access:                   network.SecurityRuleAccessAllow,
				Direction:                network.SecurityRuleDirectionInbound,
				Priority:                 ptr.To[int32](2201),
			},
		},
	}
	nsgFuture, err := nsgClient.CreateOrUpdate(ctx, groupName, nsgName, network.SecurityGroup{
		Location: ptr.To(os.Getenv(AzureLocation)),
		SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
			SecurityRules: &securityRules,
		},
	})
	Expect(err).To(BeNil())
	err = nsgFuture.WaitForCompletionRef(ctx, nsgClient.Client)
	Expect(err).To(BeNil())

	By("creating a node security group")
	nsgNodeName := "node-nsg"
	securityRulesNode := []network.SecurityRule{}
	nsgNodeFuture, err := nsgClient.CreateOrUpdate(ctx, groupName, nsgNodeName, network.SecurityGroup{
		Location: ptr.To(os.Getenv(AzureLocation)),
		SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
			SecurityRules: &securityRulesNode,
		},
	})
	Expect(err).To(BeNil())
	err = nsgNodeFuture.WaitForCompletionRef(ctx, nsgClient.Client)
	Expect(err).To(BeNil())

	By("creating a node routetable")
	routeTableName := "node-routetable"
	routeTable := network.RouteTable{
		Location:                   ptr.To(os.Getenv(AzureLocation)),
		RouteTablePropertiesFormat: &network.RouteTablePropertiesFormat{},
	}
	routetableFuture, err := routetableClient.CreateOrUpdate(ctx, groupName, routeTableName, routeTable)
	Expect(err).To(BeNil())
	err = routetableFuture.WaitForCompletionRef(ctx, routetableClient.Client)
	Expect(err).To(BeNil())

	By("creating a virtual network")
	var subnets []network.Subnet
	for name, cidr := range cpSubnetCidrs {
		subnets = append(subnets, network.Subnet{
			SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
				AddressPrefix: ptr.To(cidr),
				NetworkSecurityGroup: &network.SecurityGroup{
					ID: ptr.To(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s", subscriptionID, groupName, nsgName)),
				},
			},
			Name: ptr.To(name),
		})
	}
	for name, cidr := range nodeSubnetCidrs {
		subnets = append(subnets, network.Subnet{
			SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
				AddressPrefix: ptr.To(cidr),
				NetworkSecurityGroup: &network.SecurityGroup{
					ID: ptr.To(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s", subscriptionID, groupName, nsgNodeName)),
				},
				RouteTable: &network.RouteTable{
					ID: ptr.To(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/routeTables/%s", subscriptionID, groupName, routeTableName)),
				},
			},
			Name: ptr.To(name),
		})
	}

	// Create the AzureBastion subnet.
	subnets = append(subnets, network.Subnet{
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefix: ptr.To(bastionSubnetCidr),
		},
		Name: ptr.To(bastionSubnetName),
	})

	vnetFuture, err := vnetClient.CreateOrUpdate(ctx, groupName, os.Getenv(AzureCustomVNetName), network.VirtualNetwork{
		Location: ptr.To(os.Getenv(AzureLocation)),
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

	return func() {
		Logf("deleting an existing virtual network %q", os.Getenv(AzureCustomVNetName))
		vFuture, err := vnetClient.Delete(ctx, groupName, os.Getenv(AzureCustomVNetName))
		Expect(err).NotTo(HaveOccurred())
		Expect(vFuture.WaitForCompletionRef(ctx, vnetClient.Client)).To(Succeed())

		Logf("deleting an existing route table %q", routeTableName)
		rtFuture, err := routetableClient.Delete(ctx, groupName, routeTableName)
		Expect(err).NotTo(HaveOccurred())
		Expect(rtFuture.WaitForCompletionRef(ctx, routetableClient.Client)).To(Succeed())

		Logf("deleting an existing network security group %q", nsgNodeName)
		nsgFuture, err := nsgClient.Delete(ctx, groupName, nsgNodeName)
		Expect(err).NotTo(HaveOccurred())
		Expect(nsgFuture.WaitForCompletionRef(ctx, nsgClient.Client)).To(Succeed())

		Logf("deleting an existing network security group %q", nsgName)
		nsgFuture, err = nsgClient.Delete(ctx, groupName, nsgName)
		Expect(err).NotTo(HaveOccurred())
		Expect(nsgFuture.WaitForCompletionRef(ctx, nsgClient.Client)).To(Succeed())

		Logf("verifying the existing resource group %q is empty", groupName)
		resClient := resources.NewClient(subscriptionID)
		resClient.Authorizer = authorizer
		Eventually(func() ([]resources.GenericResourceExpanded, error) {
			page, err := resClient.ListByResourceGroup(ctx, groupName, "", "provisioningState", ptr.To[int32](10))
			if err != nil {
				return nil, err
			}

			// for each resource do a GET directly for that resource to avoid hitting Azure list cache
			var foundResources []resources.GenericResourceExpanded
			for _, genericResource := range page.Values() {
				apiversion, err := getAPIVersion(*genericResource.ID)
				if err != nil {
					LogWarningf("failed to get API version for %q with %+v", *genericResource.ID, err)
				}

				_, err = resClient.GetByID(ctx, *genericResource.ID, apiversion)
				if err != nil && azure.ResourceNotFound(err) {
					// the resources is returned in the list, but it's actually 404
					continue
				}

				// unexpected error calling GET on the resource
				if err != nil {
					LogWarningf("failed GETing resource %q with %+v", *genericResource.ID, err)
					return nil, err
				}

				// if resource is still there, then append to foundResources
				foundResources = append(foundResources, genericResource)
			}
			return foundResources, nil
			// add some tolerance for Azure caching of resource group resource caching
		}, deleteOperationTimeout, retryableOperationTimeout).Should(BeEmpty(), "Expect the manually created resource group is empty after removing the manually created resources.")

		Logf("deleting the existing resource group %q", groupName)
		grpFuture, err := groupClient.Delete(ctx, groupName)
		Expect(err).NotTo(HaveOccurred())
		Expect(grpFuture.WaitForCompletionRef(ctx, nsgClient.Client)).To(Succeed())
	}
}

func getAPIVersion(resourceID string) (string, error) {
	parsed, err := azureutil.ParseResourceID(resourceID)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("unable to parse resource ID %q", resourceID))
	}

	switch parsed.Provider {
	case "Microsoft.Network":
		if parsed.ResourceType.String() == "privateDnsZones" {
			return getAPIVersionFromUserAgent(privatedns.UserAgent()), nil
		}
		return getAPIVersionFromUserAgent(network.UserAgent()), nil
	case "Microsoft.Compute":
		return getAPIVersionFromUserAgent(compute.UserAgent()), nil
	default:
		return "", fmt.Errorf("failed to find an API version for resource provider %q", parsed.Provider)
	}
}

func getAPIVersionFromUserAgent(userAgent string) string {
	splits := strings.Split(userAgent, "/")
	return splits[len(splits)-1]
}

// getClientIDforMSI fetches the client ID of a user assigned identity.
func getClientIDforMSI(resourceID string) string {
	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	authorizer, err := azureutil.GetAuthorizer(settings)
	Expect(err).NotTo(HaveOccurred())

	msiClient := msi.NewUserAssignedIdentitiesClient(subscriptionID)
	msiClient.Authorizer = authorizer

	parsed, err := azureutil.ParseResourceID(resourceID)
	Expect(err).NotTo(HaveOccurred())

	id, err := msiClient.Get(context.TODO(), parsed.ResourceGroupName, parsed.Name)
	Expect(err).NotTo(HaveOccurred())

	return id.ClientID.String()
}
