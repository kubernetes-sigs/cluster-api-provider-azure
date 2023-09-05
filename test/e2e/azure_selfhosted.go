//go:build e2e
// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SelfHostedSpecInput is the input for SelfHostedSpec.
type SelfHostedSpecInput struct {
	E2EConfig             *clusterctl.E2EConfig
	ClusterctlConfigPath  string
	BootstrapClusterProxy framework.ClusterProxy
	ArtifactFolder        string
	SkipCleanup           bool
	ControlPlaneWaiters   clusterctl.ControlPlaneWaiters
}

// SelfHostedSpec implements a test that verifies Cluster API creating a cluster, pivoting to a self-hosted cluster.
func SelfHostedSpec(ctx context.Context, inputGetter func() SelfHostedSpecInput) {
	var (
		specName         = "self-hosted"
		input            SelfHostedSpecInput
		namespace        *corev1.Namespace
		cancelWatches    context.CancelFunc
		clusterResources *clusterctl.ApplyClusterTemplateAndWaitResult

		selfHostedClusterProxy  framework.ClusterProxy
		selfHostedNamespace     *corev1.Namespace
		selfHostedCancelWatches context.CancelFunc
		selfHostedCluster       *clusterv1.Cluster
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		Expect(input.E2EConfig).NotTo(BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
		Expect(input.ClusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. input.ClusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(input.ArtifactFolder, 0o750)).To(Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)
		Expect(input.E2EConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersion))

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		var err error
		namespace, cancelWatches, err = setupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)
		Expect(err).NotTo(HaveOccurred())
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		spClientSecret := os.Getenv(AzureClientSecret)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-identity-secret",
				Namespace: namespace.Name,
				Labels: map[string]string{
					clusterctlv1.ClusterctlMoveHierarchyLabel: "true",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"clientSecret": []byte(spClientSecret)},
		}
		err = bootstrapClusterProxy.GetClient().Create(ctx, secret)
		Expect(err).NotTo(HaveOccurred())

		identityName := input.E2EConfig.GetVariable(ClusterIdentityName)
		Expect(os.Setenv(ClusterIdentityName, identityName)).To(Succeed())
		Expect(os.Setenv(ClusterIdentityNamespace, namespace.Name)).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretName, "cluster-identity-secret")).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretNamespace, namespace.Name)).To(Succeed())
	})

	// Management clusters do not support Windows nodes because of cert manager
	// We are using the capi specs located in test/e2e/data/infrastructure-azure/v1beta1 that only have linux nodes
	// to act as the management cluster until Windows nodes are supported for management nodes
	// Tracking support for cert manager: https://github.com/jetstack/cert-manager/issues/3606
	It("Should pivot the bootstrap cluster to a self-hosted cluster", func() {
		By("Creating a workload cluster")
		clusterctl.ApplyClusterTemplateAndWait(ctx, createApplyClusterTemplateInput(
			specName,
			withFlavor("management"),
			withNamespace(namespace.Name),
			withClusterName(fmt.Sprintf("%s-%s", specName, util.RandomString(6))),
			withControlPlaneMachineCount(1),
			withWorkerMachineCount(1),
			withControlPlaneWaiters(input.ControlPlaneWaiters),
		), clusterResources)

		By("Turning the workload cluster into a management cluster")
		cluster := clusterResources.Cluster
		// Get a ClusterBroker so we can interact with the workload cluster
		selfHostedClusterProxy = input.BootstrapClusterProxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name)

		Byf("Creating a namespace for hosting the %s test spec", specName)
		selfHostedNamespace, selfHostedCancelWatches = framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
			Creator:   selfHostedClusterProxy.GetClient(),
			ClientSet: selfHostedClusterProxy.GetClientSet(),
			Name:      namespace.Name,
			LogFolder: filepath.Join(input.ArtifactFolder, "clusters", "bootstrap"),
		})

		By("Initializing the workload cluster")
		clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
			ClusterProxy:            selfHostedClusterProxy,
			ClusterctlConfigPath:    input.ClusterctlConfigPath,
			InfrastructureProviders: input.E2EConfig.InfrastructureProviders(),
			AddonProviders:          input.E2EConfig.AddonProviders(),
			LogFolder:               filepath.Join(input.ArtifactFolder, "clusters", cluster.Name),
		}, input.E2EConfig.GetIntervals(specName, "wait-controllers")...)

		By("Ensure API servers are stable before doing move")
		// Nb. This check was introduced to prevent doing move to self-hosted in an aggressive way and thus avoid flakes.
		// More specifically, we were observing the test failing to get objects from the API server during move, so we
		// are now testing the API servers are stable before starting move.
		Consistently(func() error {
			ns := &corev1.Namespace{}
			return input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: kubesystem}, ns)
		}, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")
		Consistently(func() error {
			ns := &corev1.Namespace{}
			return selfHostedClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: kubesystem}, ns)
		}, "5s", "100ms").Should(BeNil(), "Failed to assert self-hosted API server stability")

		By("Moving the cluster to self hosted")
		clusterctl.Move(ctx, clusterctl.MoveInput{
			LogFolder:            filepath.Join(input.ArtifactFolder, "clusters", "bootstrap"),
			ClusterctlConfigPath: input.ClusterctlConfigPath,
			FromKubeconfigPath:   input.BootstrapClusterProxy.GetKubeconfigPath(),
			ToKubeconfigPath:     selfHostedClusterProxy.GetKubeconfigPath(),
			Namespace:            namespace.Name,
		})

		Log("Waiting for the cluster to be reconciled after moving to self hosted")
		selfHostedCluster = framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
			Getter:    selfHostedClusterProxy.GetClient(),
			Namespace: selfHostedNamespace.Name,
			Name:      cluster.Name,
		}, input.E2EConfig.GetIntervals(specName, "wait-cluster")...)

		controlPlane := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
			Lister:      selfHostedClusterProxy.GetClient(),
			ClusterName: selfHostedCluster.Name,
			Namespace:   selfHostedCluster.Namespace,
		})
		Expect(controlPlane).NotTo(BeNil())

		By("PASSED!")
	})

	AfterEach(func() {
		if input.SkipCleanup {
			return
		}
		if selfHostedNamespace != nil {
			// Dump all Cluster API related resources to artifacts before pivoting back.
			framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
				Lister:    selfHostedClusterProxy.GetClient(),
				Namespace: namespace.Name,
				LogPath:   filepath.Join(input.ArtifactFolder, "clusters", clusterResources.Cluster.Name, "resources"),
			})
		}
		if selfHostedCluster != nil {
			By("Ensure API servers are stable before doing move")
			// Nb. This check was introduced to prevent doing move back to bootstrap in an aggressive way and thus avoid flakes.
			// More specifically, we were observing the test failing to get objects from the API server during move, so we
			// are now testing the API servers are stable before starting move.
			Consistently(func() error {
				ns := &corev1.Namespace{}
				return input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: kubesystem}, ns)
			}, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")
			Consistently(func() error {
				ns := &corev1.Namespace{}
				return selfHostedClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: kubesystem}, ns)
			}, "5s", "100ms").Should(BeNil(), "Failed to assert self-hosted API server stability")

			By("Moving the cluster back to bootstrap")
			clusterctl.Move(ctx, clusterctl.MoveInput{
				LogFolder:            filepath.Join(input.ArtifactFolder, "clusters", clusterResources.Cluster.Name),
				ClusterctlConfigPath: input.ClusterctlConfigPath,
				FromKubeconfigPath:   selfHostedClusterProxy.GetKubeconfigPath(),
				ToKubeconfigPath:     input.BootstrapClusterProxy.GetKubeconfigPath(),
				Namespace:            selfHostedNamespace.Name,
			})

			Log("Waiting for the cluster to be reconciled after moving back to booststrap")
			clusterResources.Cluster = framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
				Getter:    input.BootstrapClusterProxy.GetClient(),
				Namespace: namespace.Name,
				Name:      clusterResources.Cluster.Name,
			}, input.E2EConfig.GetIntervals(specName, "wait-cluster")...)
		}
		if selfHostedCancelWatches != nil {
			selfHostedCancelWatches()
		}

		cleanInput := cleanupInput{
			SpecName:          specName,
			Cluster:           clusterResources.Cluster,
			ClusterProxy:      input.BootstrapClusterProxy,
			Namespace:         namespace,
			CancelWatches:     cancelWatches,
			IntervalsGetter:   input.E2EConfig.GetIntervals,
			SkipCleanup:       input.SkipCleanup,
			SkipLogCollection: skipLogCollection,
			ArtifactFolder:    input.ArtifactFolder,
		}
		// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})
}
