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
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2020-10-01/resources"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	e2e_namespace "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/namespace"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Test suite constants for e2e config variables
const (
	RedactLogScriptPath            = "REDACT_LOG_SCRIPT"
	AzureLocation                  = "AZURE_LOCATION"
	AzureResourceGroup             = "AZURE_RESOURCE_GROUP"
	AzureVNetName                  = "AZURE_VNET_NAME"
	AzureCustomVNetName            = "AZURE_CUSTOM_VNET_NAME"
	AzureInternalLBIP              = "AZURE_INTERNAL_LB_IP"
	AzureCPSubnetCidr              = "AZURE_CP_SUBNET_CIDR"
	AzureVNetCidr                  = "AZURE_PRIVATE_VNET_CIDR"
	AzureNodeSubnetCidr            = "AZURE_NODE_SUBNET_CIDR"
	AzureBastionSubnetCidr         = "AZURE_BASTION_SUBNET_CIDR"
	MultiTenancyIdentityName       = "MULTI_TENANCY_IDENTITY_NAME"
	ClusterIdentityName            = "CLUSTER_IDENTITY_NAME"
	ClusterIdentityNamespace       = "CLUSTER_IDENTITY_NAMESPACE"
	ClusterIdentitySecretName      = "AZURE_CLUSTER_IDENTITY_SECRET_NAME"
	ClusterIdentitySecretNamespace = "AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE"
	AzureClientSecret              = "AZURE_CLIENT_SECRET"
	AzureClientId                  = "AZURE_CLIENT_ID"
	JobName                        = "JOB_NAME"
	Timestamp                      = "TIMESTAMP"
	AKSKubernetesVersion           = "AKS_KUBERNETES_VERSION"
	SecurityScanFailThreshold      = "SECURITY_SCAN_FAIL_THRESHOLD"
	SecurityScanContainer          = "SECURITY_SCAN_CONTAINER"
	ManagedClustersResourceType    = "managedClusters"
	capiImagePublisher             = "cncf-upstream"
	capiOfferName                  = "capi"
	capiWindowsOfferName           = "capi-windows"
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
	SpecName          string
	ClusterProxy      framework.ClusterProxy
	ArtifactFolder    string
	Namespace         *corev1.Namespace
	CancelWatches     context.CancelFunc
	Cluster           *clusterv1.Cluster
	IntervalsGetter   func(spec, key string) []interface{}
	SkipCleanup       bool
	AdditionalCleanup func()
}

func dumpSpecResourcesAndCleanup(ctx context.Context, input cleanupInput) {
	defer func() {
		input.CancelWatches()
		redactLogs()
	}()

	if input.Cluster == nil {
		By("Unable to dump workload cluster logs as the cluster is nil")
	} else {
		Byf("Dumping logs from the %q workload cluster", input.Cluster.Name)
		input.ClusterProxy.CollectWorkloadClusterLogs(ctx, input.Cluster.Namespace, input.Cluster.Name, filepath.Join(input.ArtifactFolder, "clusters", input.Cluster.Name))
	}

	Byf("Dumping all the Cluster API resources in the %q namespace", input.Namespace.Name)
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    input.ClusterProxy.GetClient(),
		Namespace: input.Namespace.Name,
		LogPath:   filepath.Join(input.ArtifactFolder, "clusters", input.ClusterProxy.GetName(), "resources"),
	})

	if input.SkipCleanup {
		return
	}

	Byf("Deleting all clusters in the %s namespace", input.Namespace.Name)
	// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
	// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
	// instead of DeleteClusterAndWait
	framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
		Client:    input.ClusterProxy.GetClient(),
		Namespace: input.Namespace.Name,
	}, input.IntervalsGetter(input.SpecName, "wait-delete-cluster")...)

	Byf("Deleting namespace used for hosting the %q test spec", input.SpecName)
	framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
		Deleter: input.ClusterProxy.GetClient(),
		Name:    input.Namespace.Name,
	})

	if input.AdditionalCleanup != nil {
		Byf("Running additional cleanup for the %q test spec", input.SpecName)
		input.AdditionalCleanup()
	}

	Byf("Checking if any resources are left over in Azure for spec %q", input.SpecName)
	ExpectResourceGroupToBe404(ctx)
}

// ExpectResourceGroupToBe404 performs a GET request to Azure to determine if the cluster resource group still exists.
// If it does still exist, it means the cluster was not deleted and is leaking Azure resources.
func ExpectResourceGroupToBe404(ctx context.Context) {
	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	authorizer, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	groupsClient := resources.NewGroupsClient(subscriptionID)
	groupsClient.Authorizer = authorizer
	_, err = groupsClient.Get(ctx, os.Getenv(AzureResourceGroup))
	Expect(azure.ResourceNotFound(err)).To(BeTrue(), "The resource group in Azure still exists. After deleting the cluster all of the Azure resources should also be deleted.")
}

func redactLogs() {
	By("Redacting sensitive information from logs")
	Expect(e2eConfig.Variables).To(HaveKey(RedactLogScriptPath))
	cmd := exec.Command(e2eConfig.GetVariable(RedactLogScriptPath))
	cmd.Run()
}

func createRestConfig(ctx context.Context, tmpdir, namespace, clusterName string) *rest.Config {
	cluster := crclient.ObjectKey{
		Namespace: namespace,
		Name:      clusterName,
	}
	kubeConfigData, err := kubeconfig.FromSecret(ctx, bootstrapClusterProxy.GetClient(), cluster)
	Expect(err).NotTo(HaveOccurred())

	kubeConfigPath := path.Join(tmpdir, clusterName+".kubeconfig")
	Expect(ioutil.WriteFile(kubeConfigPath, kubeConfigData, 0640)).To(Succeed())

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	Expect(err).NotTo(HaveOccurred())

	return config
}
