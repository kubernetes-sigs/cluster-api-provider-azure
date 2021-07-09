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
	"os/exec"
	"path"
	"path/filepath"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	e2e_namespace "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/namespace"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
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
	AzureInternalLBIP              = "AZURE_INTERNAL_LB_IP"
	AzureCPSubnetCidr              = "AZURE_CP_SUBNET_CIDR"
	AzureNodeSubnetCidr            = "AZURE_NODE_SUBNET_CIDR"
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
	ManagedClustersResourceType    = "managedClusters"
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

func dumpSpecResourcesAndCleanup(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string, namespace *corev1.Namespace, cancelWatches context.CancelFunc, cluster *clusterv1.Cluster, intervalsGetter func(spec, key string) []interface{}, skipCleanup bool) {
	defer func() {
		cancelWatches()
		redactLogs()
	}()

	if cluster == nil {
		By("Unable to dump workload cluster logs as the cluster is nil")
	} else {
		Byf("Dumping logs from the %q workload cluster", cluster.Name)
		clusterProxy.CollectWorkloadClusterLogs(ctx, cluster.Namespace, cluster.Name, filepath.Join(artifactFolder, "clusters", cluster.Name))
	}

	Byf("Dumping all the Cluster API resources in the %q namespace", namespace.Name)
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    clusterProxy.GetClient(),
		Namespace: namespace.Name,
		LogPath:   filepath.Join(artifactFolder, "clusters", clusterProxy.GetName(), "resources"),
	})

	if skipCleanup {
		return
	}

	Byf("Deleting all clusters in the %s namespace", namespace.Name)
	// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
	// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
	// instead of DeleteClusterAndWait
	framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
		Client:    clusterProxy.GetClient(),
		Namespace: namespace.Name,
	}, intervalsGetter(specName, "wait-delete-cluster")...)

	Byf("Deleting namespace used for hosting the %q test spec", specName)
	framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
		Deleter: clusterProxy.GetClient(),
		Name:    namespace.Name,
	})
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
