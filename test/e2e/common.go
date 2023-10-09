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
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	e2e_namespace "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/namespace"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kubeadmv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test suite constants for e2e config variables
const (
	AddonsPath                      = "ADDONS_PATH"
	RedactLogScriptPath             = "REDACT_LOG_SCRIPT"
	AzureLocation                   = "AZURE_LOCATION"
	AzureExtendedLocationType       = "AZURE_EXTENDEDLOCATION_TYPE"
	AzureExtendedLocationName       = "AZURE_EXTENDEDLOCATION_NAME"
	AzureResourceGroup              = "AZURE_RESOURCE_GROUP"
	AzureVNetName                   = "AZURE_VNET_NAME"
	AzureCustomVNetName             = "AZURE_CUSTOM_VNET_NAME"
	AzureInternalLBIP               = "AZURE_INTERNAL_LB_IP"
	AzureCPSubnetCidr               = "AZURE_CP_SUBNET_CIDR"
	AzureVNetCidr                   = "AZURE_PRIVATE_VNET_CIDR"
	AzureNodeSubnetCidr             = "AZURE_NODE_SUBNET_CIDR"
	AzureBastionSubnetCidr          = "AZURE_BASTION_SUBNET_CIDR"
	ClusterIdentityName             = "CLUSTER_IDENTITY_NAME"
	ClusterIdentityNamespace        = "CLUSTER_IDENTITY_NAMESPACE"
	ClusterIdentitySecretName       = "AZURE_CLUSTER_IDENTITY_SECRET_NAME"      //nolint:gosec // Not a secret itself, just its name
	ClusterIdentitySecretNamespace  = "AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE" //nolint:gosec // Not a secret itself, just its name
	AzureClientSecret               = "AZURE_CLIENT_SECRET"                     //nolint:gosec // Not a secret itself, just its name
	AzureClientID                   = "AZURE_CLIENT_ID"
	AzureSubscriptionID             = "AZURE_SUBSCRIPTION_ID"
	AzureUserIdentity               = "USER_IDENTITY"
	AzureIdentityResourceGroup      = "CI_RG"
	JobName                         = "JOB_NAME"
	Timestamp                       = "TIMESTAMP"
	AKSKubernetesVersion            = "AKS_KUBERNETES_VERSION"
	AKSKubernetesVersionUpgradeFrom = "AKS_KUBERNETES_VERSION_UPGRADE_FROM"
	FlatcarKubernetesVersion        = "FLATCAR_KUBERNETES_VERSION"
	FlatcarVersion                  = "FLATCAR_VERSION"
	SecurityScanFailThreshold       = "SECURITY_SCAN_FAIL_THRESHOLD"
	SecurityScanContainer           = "SECURITY_SCAN_CONTAINER"
	CalicoVersion                   = "CALICO_VERSION"
	ManagedClustersResourceType     = "managedClusters"
	capiImagePublisher              = "cncf-upstream"
	capiOfferName                   = "capi"
	capiWindowsOfferName            = "capi-windows"
	aksClusterNameSuffix            = "aks"
	flatcarCAPICommunityGallery     = "flatcar4capi-742ef0cb-dcaa-4ecb-9cb0-bfd2e43dccc0"
	defaultNamespace                = "default"
	AzureCNIv1Manifest              = "AZURE_CNI_V1_MANIFEST_PATH"
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
		redactLogs()
	}()

	Logf("Dumping all the Cluster API resources in the %q namespace", input.Namespace.Name)
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    input.ClusterProxy.GetClient(),
		Namespace: input.Namespace.Name,
		LogPath:   filepath.Join(input.ArtifactFolder, "clusters", input.ClusterProxy.GetName(), "resources"),
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
	if strings.Contains(input.Cluster.Name, aksClusterNameSuffix) {
		deleteTimeoutConfig = "wait-delete-cluster-aks"
	}
	framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
		Client:    input.ClusterProxy.GetClient(),
		Namespace: input.Namespace.Name,
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
	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())
	groupsClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = groupsClient.Get(ctx, os.Getenv(AzureResourceGroup), nil)
	Expect(azure.ResourceNotFound(err)).To(BeTrue(), "The resource group in Azure still exists. After deleting the cluster all of the Azure resources should also be deleted.")
}

func redactLogs() {
	By("Redacting sensitive information from logs")
	Expect(e2eConfig.Variables).To(HaveKey(RedactLogScriptPath))
	//nolint:gosec // Ignore warning about running a command constructed from user input
	cmd := exec.Command(e2eConfig.GetVariable(RedactLogScriptPath))
	if err := cmd.Run(); err != nil {
		LogWarningf("Redact logs command failed: %v", err)
	}
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

// EnsureControlPlaneInitialized waits for the cluster KubeadmControlPlane object to be initialized
// and then installs cloud-provider-azure components via Helm.
// Fulfills the clusterctl.Waiter type so that it can be used as ApplyClusterTemplateAndWaitInput data
// in the flow of a clusterctl.ApplyClusterTemplateAndWait E2E test scenario.
func EnsureControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
	ensureControlPlaneInitialized(ctx, input, result, true)
}

// EnsureControlPlaneInitializedNoAddons waits for the cluster KubeadmControlPlane object to be initialized
// and then installs cloud-provider-azure components via Helm.
// Fulfills the clusterctl.Waiter type so that it can be used as ApplyClusterTemplateAndWaitInput data
// in the flow of a clusterctl.ApplyClusterTemplateAndWait E2E test scenario.
func EnsureControlPlaneInitializedNoAddons(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
	ensureControlPlaneInitialized(ctx, input, result, false)
}

// ensureControlPlaneInitialized waits for the cluster KubeadmControlPlane object to be initialized
// and then installs cloud-provider-azure components via Helm.
// Fulfills the clusterctl.Waiter type so that it can be used as ApplyClusterTemplateAndWaitInput data
// in the flow of a clusterctl.ApplyClusterTemplateAndWait E2E test scenario.
func ensureControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult, installHelmCharts bool) {
	getter := input.ClusterProxy.GetClient()
	cluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
		Getter:    getter,
		Name:      input.ClusterName,
		Namespace: input.Namespace,
	})
	kubeadmControlPlane := &kubeadmv1.KubeadmControlPlane{}
	key := client.ObjectKey{
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}

	By("Ensuring KubeadmControlPlane is initialized")
	Eventually(func(g Gomega) {
		g.Expect(getter.Get(ctx, key, kubeadmControlPlane)).To(Succeed(), "Failed to get KubeadmControlPlane object %s/%s", cluster.Spec.ControlPlaneRef.Namespace, cluster.Spec.ControlPlaneRef.Name)
		g.Expect(kubeadmControlPlane.Status.Initialized).To(BeTrue(), "KubeadmControlPlane is not yet initialized")
	}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "KubeadmControlPlane object %s/%s was not initialized in time", cluster.Spec.ControlPlaneRef.Namespace, cluster.Spec.ControlPlaneRef.Name)

	By("Ensuring API Server is reachable before applying Helm charts")
	Eventually(func(g Gomega) {
		ns := &corev1.Namespace{}
		clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
		g.Expect(clusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: kubesystem}, ns)).To(Succeed(), "Failed to get kube-system namespace")
	}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "API Server was not reachable in time")

	_, hasWindows := cluster.Labels["cni-windows"]
	if kubeadmControlPlane.Spec.KubeadmConfigSpec.ClusterConfiguration.ControllerManager.ExtraArgs["cloud-provider"] != "azure" {
		// There is a co-dependency between cloud-provider and CNI so we install both together if cloud-provider is external.
		InstallCNIAndCloudProviderAzureHelmChart(ctx, input, installHelmCharts, cluster.Spec.ClusterNetwork.Pods.CIDRBlocks, hasWindows)
	} else {
		EnsureCNI(ctx, input, installHelmCharts, cluster.Spec.ClusterNetwork.Pods.CIDRBlocks, hasWindows)
	}
	controlPlane := discoveryAndWaitForControlPlaneInitialized(ctx, input, result)
	InstallAzureDiskCSIDriverHelmChart(ctx, input, hasWindows)
	result.ControlPlane = controlPlane
}

// CheckTestBeforeCleanup checks to see if the current running Ginkgo test failed, and prints
// a status message regarding cleanup.
func CheckTestBeforeCleanup() {
	if CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
		Logf("FAILED!")
	}
	Logf("Cleaning up after \"%s\" spec", CurrentSpecReport().FullText())
}

func discoveryAndWaitForControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) *kubeadmv1.KubeadmControlPlane {
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
			KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
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
