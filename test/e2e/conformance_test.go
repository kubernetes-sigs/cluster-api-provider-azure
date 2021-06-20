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
	"strconv"
	"strings"

	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/node"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
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
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. BootstrapClusterProxy can't be nil")
		Expect(kubetestConfigFilePath).ToNot(BeNil(), "Invalid argument. kubetestConfigFilePath can't be nil")
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
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

		Expect(os.Setenv(AzureResourceGroup, clusterName)).NotTo(HaveOccurred())
		Expect(os.Setenv(AzureVNetName, fmt.Sprintf("%s-vnet", clusterName))).NotTo(HaveOccurred())

		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	})

	Measure(specName, func(b Benchmarker) {
		var err error

		kubernetesVersion := e2eConfig.GetVariable(capi_e2e.KubernetesVersion)
		flavor := clusterctl.DefaultFlavor
		if isWindows(kubetestConfigFilePath) {
			flavor = "windows"
		}

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

			if isWindows(kubetestConfigFilePath) {
				flavor = flavor + "-windows"
			}
		}

		workerMachineCount, err := strconv.ParseInt(e2eConfig.GetVariable("CONFORMANCE_WORKER_MACHINE_COUNT"), 10, 64)
		Expect(err).NotTo(HaveOccurred())
		controlPlaneMachineCount, err := strconv.ParseInt(e2eConfig.GetVariable("CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT"), 10, 64)
		Expect(err).NotTo(HaveOccurred())

		runtime := b.Time("cluster creation", func() {
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   flavor,
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        kubernetesVersion,
					ControlPlaneMachineCount: pointer.Int64Ptr(controlPlaneMachineCount),
					WorkerMachineCount:       pointer.Int64Ptr(workerMachineCount),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
		})

		b.RecordValue("cluster creation", runtime.Seconds())
		workloadProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)

		// Windows requires a taint on control nodes nodes since not all conformance tests have ability to run
		if isWindows(kubetestConfigFilePath) {
			options := v1.ListOptions{
				LabelSelector: "kubernetes.io/os=linux",
			}

			noScheduleTaint := &corev1.Taint{
				Key:    "node-role.kubernetes.io/master",
				Value:  "",
				Effect: "NoSchedule",
			}

			err := node.TaintNode(workloadProxy.GetClientSet(), options, noScheduleTaint)
			Expect(err).NotTo(HaveOccurred())
		}

		ginkgoNodes, err := strconv.Atoi(e2eConfig.GetVariable("CONFORMANCE_NODES"))
		Expect(err).NotTo(HaveOccurred())

		runtime = b.Time("conformance suite", func() {
			err := kubetest.Run(context.Background(),
				kubetest.RunInput{
					ClusterProxy:         workloadProxy,
					NumberOfNodes:        int(workerMachineCount),
					ConfigFilePath:       kubetestConfigFilePath,
					KubeTestRepoListPath: kubetestRepoListPath,
					ConformanceImage:     e2eConfig.GetVariable("CONFORMANCE_IMAGE"),
					GinkgoNodes:          ginkgoNodes,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})
		b.RecordValue("conformance suite run time", runtime.Seconds())
	}, 1)

	AfterEach(func() {
		if result.Cluster == nil {
			// this means the cluster failed to come up. We make an attempt to find the cluster to be able to fetch logs for the failed bootstrapping.
			_ = bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{Name: clusterName, Namespace: namespace.Name}, result.Cluster)
		}

		// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, cancelWatches, result.Cluster, e2eConfig.GetIntervals, skipCleanup)

		Expect(os.Unsetenv(AzureResourceGroup)).NotTo(HaveOccurred())
		Expect(os.Unsetenv(AzureVNetName)).NotTo(HaveOccurred())
	})

})

func isWindows(kubetestConfigFilePath string) bool {
	return strings.Contains(kubetestConfigFilePath, "windows")
}
