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
	"strconv"
	"strings"

	"github.com/blang/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/node"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/test/framework/kubetest"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Conformance Tests", func() {
	var (
		ctx           = context.TODO()
		cancelWatches context.CancelFunc
		result        *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName   string
		namespace     *corev1.Namespace
		specName      = "conformance-tests"
		repoList      = ""
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. BootstrapClusterProxy can't be nil")
		Expect(kubetestConfigFilePath).NotTo(BeNil(), "Invalid argument. kubetestConfigFilePath can't be nil")
		Expect(e2eConfig).NotTo(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersion))

		clusterName = os.Getenv("CLUSTER_NAME")
		if clusterName == "" {
			clusterName = fmt.Sprintf("capz-conf-%s", util.RandomString(6))
		}
		fmt.Fprintf(GinkgoWriter, "INFO: Cluster name is %s\n", clusterName)

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		var err error
		namespace, cancelWatches, err = setupSpecNamespace(ctx, clusterName, bootstrapClusterProxy, artifactFolder)
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Setenv(AzureResourceGroup, clusterName)).To(Succeed())
		Expect(os.Setenv(AzureVNetName, fmt.Sprintf("%s-vnet", clusterName))).To(Succeed())

		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		spClientSecret := os.Getenv(AzureClientSecret)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-identity-secret",
				Namespace: namespace.Name,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"clientSecret": []byte(spClientSecret)},
		}
		err = bootstrapClusterProxy.GetClient().Create(ctx, secret)
		Expect(err).NotTo(HaveOccurred())

		identityName := e2eConfig.GetVariable(ClusterIdentityName)
		Expect(os.Setenv(ClusterIdentityName, identityName)).To(Succeed())
		Expect(os.Setenv(ClusterIdentityNamespace, namespace.Name)).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretName, "cluster-identity-secret")).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretNamespace, namespace.Name)).To(Succeed())
	})

	It(specName, func() {
		experiment := gmeasure.NewExperiment(specName)
		AddReportEntry(experiment.Name, experiment)

		var err error

		kubernetesVersion := e2eConfig.GetVariable(capi_e2e.KubernetesVersion)
		flavor := e2eConfig.GetVariable("CONFORMANCE_FLAVOR")

		// clusters with CI artifacts or PR artifacts are based on a known CI version
		// PR artifacts will replace the CI artifacts during kubeadm init
		if useCIArtifacts || usePRArtifacts {
			kubernetesVersion, err = resolveCIVersion(kubernetesVersion)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Setenv("CI_VERSION", kubernetesVersion)).To(Succeed())

			if useCIArtifacts {
				flavor = "conformance-ci-artifacts"
			} else if usePRArtifacts {
				flavor = "conformance-presubmit-artifacts"
			}
		}

		// use the ipv6 flavor if ipv6 IP family is specified.
		if e2eConfig.GetVariable(capi_e2e.IPFamily) == "IPv6" {
			flavor += "-ipv6"
			kubetestConfigFilePath = strings.Replace(kubetestConfigFilePath, ".yaml", "-ipv6.yaml", 1)
		} else if e2eConfig.GetVariable(capi_e2e.IPFamily) == "dual" {
			flavor += "-dual-stack"
			kubetestConfigFilePath = strings.Replace(kubetestConfigFilePath, ".yaml", "-dual-stack.yaml", 1)
		}

		// Starting with Kubernetes v1.25, the kubetest config file needs to be compatible with Ginkgo V2.
		v125 := semver.MustParse("1.25.0-alpha.0.0")
		v, err := semver.ParseTolerant(kubernetesVersion)
		Expect(err).NotTo(HaveOccurred())
		if v.GTE(v125) {
			// Use the Ginkgo V2 config file.
			kubetestConfigFilePath = getGinkgoV2ConfigFilePath(kubetestConfigFilePath)
		}

		// Set the worker counts for conformance tests that use Windows
		// This is a work around until we can update cluster-api test framework to be aware of windows node counts.
		conformanceNodeCount := e2eConfig.GetVariable("CONFORMANCE_WORKER_MACHINE_COUNT")
		numOfConformanceNodes, err := strconv.ParseInt(conformanceNodeCount, 10, 64)
		Expect(err).NotTo(HaveOccurred())

		linuxWorkerMachineCount := numOfConformanceNodes
		if isWindows(kubetestConfigFilePath) {
			Expect(os.Setenv("WINDOWS_WORKER_MACHINE_COUNT", conformanceNodeCount)).To(Succeed())

			// Conformance for windows doesn't require any linux worker machines.
			// The templates use WORKER_MACHINE_COUNT for linux machines for backwards compatibility so clear it
			linuxWorkerMachineCount = 0
		}

		controlPlaneMachineCount, err := strconv.ParseInt(e2eConfig.GetVariable("CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT"), 10, 64)
		Expect(err).NotTo(HaveOccurred())

		stopwatch := experiment.NewStopwatch()
		clusterctl.ApplyClusterTemplateAndWait(ctx, createApplyClusterTemplateInput(
			specName,
			withFlavor(flavor),
			withNamespace(namespace.Name),
			withClusterName(clusterName),
			withKubernetesVersion(kubernetesVersion),
			withControlPlaneMachineCount(controlPlaneMachineCount),
			withWorkerMachineCount(linuxWorkerMachineCount),
			withControlPlaneWaiters(clusterctl.ControlPlaneWaiters{
				WaitForControlPlaneInitialized: EnsureControlPlaneInitializedNoAddons,
			}),
		), result)
		stopwatch.Record("cluster creation")

		workloadProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)

		if isWindows(kubetestConfigFilePath) {
			// Windows requires a taint on control nodes nodes since not all conformance tests have ability to run
			options := metav1.ListOptions{
				LabelSelector: "kubernetes.io/os=linux",
			}

			noScheduleTaint := &corev1.Taint{
				Key:    "node-role.kubernetes.io/control-plane",
				Value:  "",
				Effect: "NoSchedule",
			}

			if v, err := semver.ParseTolerant(kubernetesVersion); err == nil {
				if v.LT(semver.MustParse("1.24.0-alpha.0.0")) {
					noScheduleTaint = &corev1.Taint{
						Key:    "node-role.kubernetes.io/master",
						Value:  "",
						Effect: "NoSchedule",
					}
				}
			}

			err = node.TaintNode(workloadProxy.GetClientSet(), options, noScheduleTaint)
			Expect(err).NotTo(HaveOccurred())

			// Windows requires a repo-list when running e2e tests against K8s versions prior to v1.25
			// because some test images published to the k8s gcr do not have Windows flavors.
			repoList, err = resolveKubetestRepoListPath(kubernetesVersion, kubetestRepoListPath)
			Expect(err).NotTo(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "INFO: Using repo-list '%s' for version '%s'\n", repoList, kubernetesVersion)
		}

		ginkgoNodes, err := strconv.Atoi(e2eConfig.GetVariable("CONFORMANCE_NODES"))
		Expect(err).NotTo(HaveOccurred())

		stopwatch.Reset()
		err = kubetest.Run(context.Background(),
			kubetest.RunInput{
				ClusterProxy:         workloadProxy,
				NumberOfNodes:        int(numOfConformanceNodes),
				ConfigFilePath:       kubetestConfigFilePath,
				KubeTestRepoListPath: repoList,
				ConformanceImage:     e2eConfig.GetVariable("CONFORMANCE_IMAGE"),
				GinkgoNodes:          ginkgoNodes,
			},
		)
		Expect(err).NotTo(HaveOccurred())
		stopwatch.Record("conformance suite")
	})

	AfterEach(func() {
		if result.Cluster == nil {
			// this means the cluster failed to come up. We make an attempt to find the cluster to be able to fetch logs for the failed bootstrapping.
			_ = bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{Name: clusterName, Namespace: namespace.Name}, result.Cluster)
		}

		CheckTestBeforeCleanup()

		cleanInput := cleanupInput{
			SpecName:          specName,
			Cluster:           result.Cluster,
			ClusterProxy:      bootstrapClusterProxy,
			Namespace:         namespace,
			CancelWatches:     cancelWatches,
			IntervalsGetter:   e2eConfig.GetIntervals,
			SkipCleanup:       skipCleanup,
			SkipLogCollection: skipLogCollection,
			ArtifactFolder:    artifactFolder,
		}
		// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
		dumpSpecResourcesAndCleanup(ctx, cleanInput)

		Expect(os.Unsetenv(AzureResourceGroup)).To(Succeed())
		Expect(os.Unsetenv(AzureVNetName)).To(Succeed())
	})

})

func isWindows(kubetestConfigFilePath string) bool {
	return strings.Contains(kubetestConfigFilePath, "windows")
}

func getGinkgoV2ConfigFilePath(kubetestConfigFilePath string) string {
	if !strings.HasSuffix(kubetestConfigFilePath, "-ginkgo-v2.yaml") {
		return strings.Replace(kubetestConfigFilePath, ".yaml", "-ginkgo-v2.yaml", 1)
	}
	return kubetestConfigFilePath
}
