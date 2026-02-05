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
	"path"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	e2e_namespace "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/namespace"
)

// Test suite constants for e2e config variables
const (
	AddonsPath                        = "ADDONS_PATH"
	RedactLogScriptPath               = "REDACT_LOG_SCRIPT"
	AzureLocation                     = "AZURE_LOCATION"
	AzureExtendedLocationType         = "AZURE_EXTENDEDLOCATION_TYPE"
	AzureExtendedLocationName         = "AZURE_EXTENDEDLOCATION_NAME"
	AzureResourceGroup                = "AZURE_RESOURCE_GROUP"
	AzureCustomVnetResourceGroup      = "AZURE_CUSTOM_VNET_RESOURCE_GROUP"
	AzureVNetName                     = "AZURE_VNET_NAME"
	AzureCustomVNetName               = "AZURE_CUSTOM_VNET_NAME"
	AzureInternalLBIP                 = "AZURE_INTERNAL_LB_IP"
	AzureCPSubnetCidr                 = "AZURE_CP_SUBNET_CIDR"
	AzureVNetCidr                     = "AZURE_PRIVATE_VNET_CIDR"
	AzureNodeSubnetCidr               = "AZURE_NODE_SUBNET_CIDR"
	AzureBastionSubnetCidr            = "AZURE_BASTION_SUBNET_CIDR"
	ClusterIdentityName               = "CLUSTER_IDENTITY_NAME"
	ClusterIdentityNamespace          = "CLUSTER_IDENTITY_NAMESPACE"
	AzureClientID                     = "AZURE_CLIENT_ID"
	AzureClientIDUserAssignedIdentity = "AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY"
	AzureSubscriptionID               = "AZURE_SUBSCRIPTION_ID"
	AzureTenantID                     = "AZURE_TENANT_ID"
	AzureUserIdentity                 = "USER_IDENTITY"
	AzureIdentityResourceGroup        = "CI_RG"
	JobName                           = "JOB_NAME"
	Timestamp                         = "TIMESTAMP"
	AKSKubernetesVersion              = "AKS_KUBERNETES_VERSION"
	AKSKubernetesVersionUpgradeFrom   = "AKS_KUBERNETES_VERSION_UPGRADE_FROM"
	CalicoVersion                     = "CALICO_VERSION"
	ManagedClustersResourceType       = "managedClusters"
	capiImagePublisher                = "cncf-upstream"
	capiOfferName                     = "capi"
	capiWindowsOfferName              = "capi-windows"
	capiCommunityGallery              = "ClusterAPI-f72ceb4f-5159-4c26-a0fe-2ea738f0d019"
	aksClusterNameSuffix              = "aks"
	defaultNamespace                  = "default"
	AzureCNIv1Manifest                = "AZURE_CNI_V1_MANIFEST_PATH"
	OldProviderUpgradeVersion         = "OLD_PROVIDER_UPGRADE_VERSION"
	LatestProviderUpgradeVersion      = "LATEST_PROVIDER_UPGRADE_VERSION"
	OldCAPIUpgradeVersion             = "OLD_CAPI_UPGRADE_VERSION"
	LatestCAPIUpgradeVersion          = "LATEST_CAPI_UPGRADE_VERSION"
	OldAddonProviderUpgradeVersion    = "OLD_CAAPH_UPGRADE_VERSION"
	LatestAddonProviderUpgradeVersion = "LATEST_CAAPH_UPGRADE_VERSION"
	KubernetesVersionAPIUpgradeFrom   = "KUBERNETES_VERSION_API_UPGRADE_FROM"
)

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

func setupSpecNamespace(ctx context.Context, namespaceName string, clusterProxy framework.ClusterProxy, artifactFolder string) (*corev1.Namespace, context.CancelFunc, error) {
	Byf("Creating namespace %q for hosting the cluster", namespaceName)
	Logf("starting to create namespace for hosting the %q test spec", namespaceName)
	logPath := filepath.Join(artifactFolder, "clusters", clusterProxy.GetName())
	namespace, err := e2e_namespace.Get(ctx, clusterProxy.GetClientSet(), namespaceName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}

	// namespace exists wire it up
	if err == nil {
		Byf("Creating event watcher for existing namespace %q", namespace.Name)
		watchesCtx, cancelWatches := context.WithCancel(ctx)
		go func() {
			defer GinkgoRecover()
			framework.WatchNamespaceEvents(watchesCtx, framework.WatchNamespaceEventsInput{
				ClientSet: clusterProxy.GetClientSet(),
				Name:      namespace.Name,
				LogFolder: logPath,
			})
		}()

		return namespace, cancelWatches, nil
	}

	// create and wire up namespace
	namespace, cancelWatches := framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   clusterProxy.GetClient(),
		ClientSet: clusterProxy.GetClientSet(),
		Name:      namespaceName,
		LogFolder: logPath,
	})

	return namespace, cancelWatches, nil
}

type cleanupInput struct {
	SpecName               string
	ClusterProxy           framework.ClusterProxy
	ArtifactFolder         string
	Namespace              *corev1.Namespace
	CancelWatches          context.CancelFunc
	Cluster                *clusterv1.Cluster
	IntervalsGetter        func(spec, key string) []interface{}
	SkipCleanup            bool
	SkipLogCollection      bool
	AdditionalCleanup      func()
	SkipResourceGroupCheck bool
}

func dumpSpecResourcesAndCleanup(ctx context.Context, input cleanupInput) {
	defer func() {
		input.CancelWatches()
	}()

	Logf("Dumping all the Cluster API resources in the %q namespace", input.Namespace.Name)
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:               input.ClusterProxy.GetClient(),
		KubeConfigPath:       input.ClusterProxy.GetKubeconfigPath(),
		ClusterctlConfigPath: clusterctlConfigPath,
		Namespace:            input.Namespace.Name,
		LogPath:              filepath.Join(input.ArtifactFolder, "clusters", input.ClusterProxy.GetName(), "resources"),
	})

	if input.Cluster == nil {
		By("Unable to dump workload cluster logs as the cluster is nil")
	} else if !input.SkipLogCollection {
		Byf("Dumping logs from the %q workload cluster", input.Cluster.Name)
		input.ClusterProxy.CollectWorkloadClusterLogs(ctx, input.Cluster.Namespace, input.Cluster.Name, filepath.Join(input.ArtifactFolder, "clusters", input.Cluster.Name))
	}

	if input.SkipCleanup {
		return
	}

	Logf("Deleting all clusters in the %s namespace", input.Namespace.Name)
	// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
	// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
	// instead of DeleteClusterAndWait
	deleteTimeoutConfig := "wait-delete-cluster"
	if input.Cluster != nil && strings.Contains(input.Cluster.Name, aksClusterNameSuffix) {
		deleteTimeoutConfig = "wait-delete-cluster-aks"
	}
	framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
		ClusterProxy:         input.ClusterProxy,
		ClusterctlConfigPath: clusterctlConfigPath,
		Namespace:            input.Namespace.Name,
		ArtifactFolder:       input.ArtifactFolder,
	}, input.IntervalsGetter(input.SpecName, deleteTimeoutConfig)...)

	Logf("Deleting namespace used for hosting the %q test spec", input.SpecName)
	framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
		Deleter: input.ClusterProxy.GetClient(),
		Name:    input.Namespace.Name,
	})

	if input.AdditionalCleanup != nil {
		Logf("Running additional cleanup for the %q test spec", input.SpecName)
		input.AdditionalCleanup()
	}

	Logf("Checking if any resources are left over in Azure for spec %q", input.SpecName)

	if !input.SkipResourceGroupCheck {
		ExpectResourceGroupToBe404(ctx)
	}
}

// ExpectResourceGroupToBe404 performs a GET request to Azure to determine if the cluster resource group still exists.
// If it does still exist, it means the cluster was not deleted and is leaking Azure resources.
func ExpectResourceGroupToBe404(ctx context.Context) {
	resourceGroup := os.Getenv(AzureResourceGroup)
	if resourceGroup == "" {
		return
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())
	groupsClient, err := armresources.NewResourceGroupsClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = groupsClient.Get(ctx, resourceGroup, nil)
	Expect(azure.ResourceNotFound(err)).To(BeTrue(), "The resource group in Azure still exists. After deleting the cluster all of the Azure resources should also be deleted.")
}

func createRestConfig(ctx context.Context, tmpdir, namespace, clusterName string) *rest.Config {
	cluster := client.ObjectKey{
		Namespace: namespace,
		Name:      clusterName,
	}
	kubeConfigData, err := kubeconfig.FromSecret(ctx, bootstrapClusterProxy.GetClient(), cluster)
	Expect(err).NotTo(HaveOccurred())

	kubeConfigPath := path.Join(tmpdir, clusterName+".kubeconfig")
	Expect(os.WriteFile(kubeConfigPath, kubeConfigData, 0o600)).To(Succeed())

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	Expect(err).NotTo(HaveOccurred())

	return config
}

// setupAKSNetworking sets up VNet peering and private DNS for AKS management cluster scenarios.
// This is required when using an AKS cluster as the management cluster because NRMS security rules
// block direct Internet access to workload clusters.
func setupAKSNetworking(ctx context.Context, clusterName string) {
	aksResourceGroup := os.Getenv("AKS_RESOURCE_GROUP")
	if aksResourceGroup == "" {
		Logf("AKS_RESOURCE_GROUP not set, skipping VNet peering setup")
		return
	}

	aksMgmtVnetName := os.Getenv("AKS_MGMT_VNET_NAME")
	if aksMgmtVnetName == "" {
		Logf("AKS_MGMT_VNET_NAME not set, skipping VNet peering setup")
		return
	}

	azureLocation := os.Getenv("AZURE_LOCATION")
	internalLBIP := os.Getenv("AZURE_INTERNAL_LB_PRIVATE_IP")
	apiServerDNSSuffix := os.Getenv("APISERVER_LB_DNS_SUFFIX")
	if apiServerDNSSuffix == "" {
		apiServerDNSSuffix = util.RandomString(10)
		Logf("Generated APISERVER_LB_DNS_SUFFIX: %s", apiServerDNSSuffix)
	}

	By(fmt.Sprintf("Setting up VNet peering and private DNS for cluster %s", clusterName))

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	subscriptionID := getSubscriptionID(Default)

	// Create VNet peering
	createVnetPeering(ctx, cred, subscriptionID, aksResourceGroup, aksMgmtVnetName, clusterName)

	// Create private DNS zone
	dnsZoneName := fmt.Sprintf("%s-%s.%s.cloudapp.azure.com", clusterName, apiServerDNSSuffix, azureLocation)
	createPrivateDNSZone(ctx, cred, subscriptionID, aksResourceGroup, aksMgmtVnetName, clusterName, dnsZoneName, internalLBIP)

	Logf("VNet peering and private DNS setup completed successfully")
}

// createVnetPeering creates bidirectional VNet peering between the AKS management cluster and workload cluster.
func createVnetPeering(ctx context.Context, cred *azidentity.DefaultAzureCredential, subscriptionID, aksResourceGroup, aksMgmtVnetName, clusterName string) {
	vnetClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	peeringClient, err := armnetwork.NewVirtualNetworkPeeringsClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	workloadVnetName := fmt.Sprintf("%s-vnet", clusterName)

	// Wait for workload VNet to exist
	By("Waiting for workload cluster VNet to be created")
	var workloadVnet armnetwork.VirtualNetwork
	Eventually(func(g Gomega) {
		resp, err := vnetClient.Get(ctx, clusterName, workloadVnetName, nil)
		g.Expect(err).NotTo(HaveOccurred())
		workloadVnet = resp.VirtualNetwork
	}, "10m", "10s").Should(Succeed(), "Timed out waiting for workload VNet %s", workloadVnetName)

	// Get management VNet
	mgmtVnetResp, err := vnetClient.Get(ctx, aksResourceGroup, aksMgmtVnetName, nil)
	Expect(err).NotTo(HaveOccurred())

	// Create peering from management to workload
	By("Creating VNet peering from management to workload cluster")
	mgmtToWorkloadPeering := armnetwork.VirtualNetworkPeering{
		Properties: &armnetwork.VirtualNetworkPeeringPropertiesFormat{
			RemoteVirtualNetwork: &armnetwork.SubResource{
				ID: workloadVnet.ID,
			},
			AllowVirtualNetworkAccess: ptr.To(true),
			AllowForwardedTraffic:     ptr.To(true),
		},
	}
	mgmtPeeringPoller, err := peeringClient.BeginCreateOrUpdate(ctx, aksResourceGroup, aksMgmtVnetName, fmt.Sprintf("mgmt-to-%s", clusterName), mgmtToWorkloadPeering, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = mgmtPeeringPoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred())

	// Create peering from workload to management
	By("Creating VNet peering from workload to management cluster")
	workloadToMgmtPeering := armnetwork.VirtualNetworkPeering{
		Properties: &armnetwork.VirtualNetworkPeeringPropertiesFormat{
			RemoteVirtualNetwork: &armnetwork.SubResource{
				ID: mgmtVnetResp.ID,
			},
			AllowVirtualNetworkAccess: ptr.To(true),
			AllowForwardedTraffic:     ptr.To(true),
		},
	}
	workloadPeeringPoller, err := peeringClient.BeginCreateOrUpdate(ctx, clusterName, workloadVnetName, fmt.Sprintf("%s-to-mgmt", clusterName), workloadToMgmtPeering, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = workloadPeeringPoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred())

	Logf("VNet peering completed successfully")
}

// createPrivateDNSZone creates a private DNS zone and links it to both VNets.
func createPrivateDNSZone(ctx context.Context, cred *azidentity.DefaultAzureCredential, subscriptionID, aksResourceGroup, aksMgmtVnetName, clusterName, dnsZoneName, internalLBIP string) {
	dnsClient, err := armprivatedns.NewPrivateZonesClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	linkClient, err := armprivatedns.NewVirtualNetworkLinksClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	recordClient, err := armprivatedns.NewRecordSetsClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	workloadVnetID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s-vnet", subscriptionID, clusterName, clusterName)
	mgmtVnetID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s", subscriptionID, aksResourceGroup, aksMgmtVnetName)

	// Create private DNS zone
	By(fmt.Sprintf("Creating private DNS zone %s", dnsZoneName))
	zonePoller, err := dnsClient.BeginCreateOrUpdate(ctx, clusterName, dnsZoneName, armprivatedns.PrivateZone{
		Location: ptr.To("global"),
	}, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = zonePoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred())

	// Link to workload VNet
	By("Linking private DNS zone to workload VNet")
	workloadLinkPoller, err := linkClient.BeginCreateOrUpdate(ctx, clusterName, dnsZoneName, fmt.Sprintf("%s-link", clusterName), armprivatedns.VirtualNetworkLink{
		Location: ptr.To("global"),
		Properties: &armprivatedns.VirtualNetworkLinkProperties{
			VirtualNetwork: &armprivatedns.SubResource{
				ID: ptr.To(workloadVnetID),
			},
			RegistrationEnabled: ptr.To(false),
		},
	}, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = workloadLinkPoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred())

	// Link to management VNet
	By("Linking private DNS zone to management VNet")
	mgmtLinkPoller, err := linkClient.BeginCreateOrUpdate(ctx, clusterName, dnsZoneName, "mgmt-link", armprivatedns.VirtualNetworkLink{
		Location: ptr.To("global"),
		Properties: &armprivatedns.VirtualNetworkLinkProperties{
			VirtualNetwork: &armprivatedns.SubResource{
				ID: ptr.To(mgmtVnetID),
			},
			RegistrationEnabled: ptr.To(false),
		},
	}, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = mgmtLinkPoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred())

	// Create A record pointing to internal LB IP
	By(fmt.Sprintf("Creating DNS A record pointing to %s", internalLBIP))
	_, err = recordClient.CreateOrUpdate(ctx, clusterName, dnsZoneName, armprivatedns.RecordTypeA, "@", armprivatedns.RecordSet{
		Properties: &armprivatedns.RecordSetProperties{
			TTL: ptr.To[int64](3600),
			ARecords: []*armprivatedns.ARecord{
				{IPv4Address: ptr.To(internalLBIP)},
			},
		},
	}, nil)
	Expect(err).NotTo(HaveOccurred())

	Logf("Private DNS zone setup completed successfully")
}

// EnsureControlPlaneInitialized waits for the cluster KubeadmControlPlane object to be initialized
// and then waits for cloud-provider-azure components installed via CAAPH.
// Fulfills the clusterctl.Waiter type so that it can be used as ApplyClusterTemplateAndWaitInput data
// in the flow of a clusterctl.ApplyClusterTemplateAndWait E2E test scenario.
func EnsureControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
	getter := input.ClusterProxy.GetClient()
	cluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
		Getter:    getter,
		Name:      input.ClusterName,
		Namespace: input.Namespace,
	})
	kubeadmControlPlane := &controlplanev1.KubeadmControlPlane{}
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}

	By("Ensuring KubeadmControlPlane is initialized")
	Eventually(func(g Gomega) {
		g.Expect(getter.Get(ctx, key, kubeadmControlPlane)).To(Succeed(), "Failed to get KubeadmControlPlane object %s/%s", cluster.Namespace, cluster.Spec.ControlPlaneRef.Name)
		g.Expect(ptr.Deref(kubeadmControlPlane.Status.Initialization.ControlPlaneInitialized, false)).To(BeTrue(), "KubeadmControlPlane is not yet initialized")
	}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "KubeadmControlPlane object %s/%s was not initialized in time", cluster.Namespace, cluster.Spec.ControlPlaneRef.Name)

	// Setup VNet peering for AKS management cluster scenarios.
	// This must happen after the workload cluster's VNet exists but before we try to access the API server.
	setupAKSNetworking(ctx, input.ClusterName)

	By("Ensuring API Server is reachable before applying Helm charts")
	Eventually(func(g Gomega) {
		ns := &corev1.Namespace{}
		clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
		g.Expect(clusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: kubesystem}, ns)).To(Succeed(), "Failed to get kube-system namespace")
	}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "API Server was not reachable in time")

	cloudProviderValue := ""
	for _, extraArg := range kubeadmControlPlane.Spec.KubeadmConfigSpec.ClusterConfiguration.ControllerManager.ExtraArgs {
		if extraArg.Name != "cloud-provider" {
			continue
		}
		cloudProviderValue = ptr.Deref(extraArg.Value, "")
	}
	if cloudProviderValue != infrav1.AzureNetworkPluginName {
		// There is a co-dependency between cloud-provider and CNI so we install both together if cloud-provider is external.
		EnsureCNIAndCloudProviderAzureHelmChart(ctx, input)
	} else {
		EnsureCNI(ctx, input)
	}
	controlPlane := discoveryAndWaitForControlPlaneInitialized(ctx, input, result)
	EnsureAzureDiskCSIDriverHelmChart(ctx, input)
	result.ControlPlane = controlPlane
}

// ensureContolPlaneReplicasMatch waits for the control plane machine replicas to be created.
func ensureContolPlaneReplicasMatch(ctx context.Context, proxy framework.ClusterProxy, ns, clusterName string, replicas int, intervals []interface{}) {
	By("Waiting for all control plane nodes to exist")
	inClustersNamespaceListOption := client.InNamespace(ns)
	// ControlPlane labels
	matchClusterListOption := client.MatchingLabels{
		clusterv1.MachineControlPlaneLabel: "",
		clusterv1.ClusterNameLabel:         clusterName,
	}

	Eventually(func() (int, error) {
		machineList := &clusterv1.MachineList{}
		lister := proxy.GetClient()
		if err := lister.List(ctx, machineList, inClustersNamespaceListOption, matchClusterListOption); err != nil {
			Logf("Failed to list the machines: %+v", err)
			return 0, err
		}
		count := 0
		for _, machine := range machineList.Items {
			if meta.IsStatusConditionTrue(machine.GetConditions(), clusterv1.MachineReadyCondition) {
				count++
			}
		}
		return count, nil
	}, intervals...).Should(Equal(replicas), "Timed out waiting for %d control plane machines to exist", replicas)
}

// CheckTestBeforeCleanup checks to see if the current running Ginkgo test failed, and prints
// a status message regarding cleanup.
func CheckTestBeforeCleanup() {
	if CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
		Logf("FAILED!")
	}
	Logf("Cleaning up after \"%s\" spec", CurrentSpecReport().FullText())
}

func discoveryAndWaitForControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) *controlplanev1.KubeadmControlPlane {
	return framework.DiscoveryAndWaitForControlPlaneInitialized(ctx, framework.DiscoveryAndWaitForControlPlaneInitializedInput{
		Lister:  input.ClusterProxy.GetClient(),
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

func createApplyClusterTemplateInput(specName string, changes ...func(*clusterctl.ApplyClusterTemplateAndWaitInput)) clusterctl.ApplyClusterTemplateAndWaitInput {
	input := clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy: bootstrapClusterProxy,
		ConfigCluster: clusterctl.ConfigClusterInput{
			LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
			ClusterctlConfigPath:     clusterctlConfigPath,
			KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
			InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
			Flavor:                   clusterctl.DefaultFlavor,
			Namespace:                "default",
			ClusterName:              "cluster",
			KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
			ControlPlaneMachineCount: ptr.To[int64](1),
			WorkerMachineCount:       ptr.To[int64](1),
		},
		WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
		WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
		WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
		CNIManifestPath:              "",
	}
	for _, change := range changes {
		change(&input)
	}

	return input
}

func withClusterProxy(proxy framework.ClusterProxy) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ClusterProxy = proxy
	}
}

func withFlavor(flavor string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.Flavor = flavor
	}
}

func withNamespace(namespace string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.Namespace = namespace
	}
}

func withClusterName(clusterName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.ClusterName = clusterName
	}
}

func withKubernetesVersion(version string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.KubernetesVersion = version
	}
}

func withControlPlaneMachineCount(count int64) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.ControlPlaneMachineCount = ptr.To[int64](count)
	}
}

func withWorkerMachineCount(count int64) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.WorkerMachineCount = ptr.To[int64](count)
	}
}

func withClusterInterval(specName string, intervalName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		if intervalName != "" {
			input.WaitForClusterIntervals = e2eConfig.GetIntervals(specName, intervalName)
		}
	}
}

func withControlPlaneInterval(specName string, intervalName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		if intervalName != "" {
			input.WaitForControlPlaneIntervals = e2eConfig.GetIntervals(specName, intervalName)
		}
	}
}

func withMachineDeploymentInterval(specName string, intervalName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		if intervalName != "" {
			input.WaitForMachineDeployments = e2eConfig.GetIntervals(specName, intervalName)
		}
	}
}

func withMachinePoolInterval(specName string, intervalName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		if intervalName != "" {
			input.WaitForMachinePools = e2eConfig.GetIntervals(specName, intervalName)
		}
	}
}

func withControlPlaneWaiters(waiters clusterctl.ControlPlaneWaiters) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ControlPlaneWaiters = waiters
	}
}

func withPostMachinesProvisioned(postMachinesProvisioned func()) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.PostMachinesProvisioned = postMachinesProvisioned
	}
}

func withAzureCNIv1Manifest(manifestPath string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.CNIManifestPath = manifestPath
	}
}
