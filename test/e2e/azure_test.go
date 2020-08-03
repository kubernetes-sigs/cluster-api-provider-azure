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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Workload cluster creation", func() {
	var (
		ctx           = context.TODO()
		specName      = "create-workload-cluster"
		namespace     *corev1.Namespace
		cancelWatches context.CancelFunc
		cluster       *clusterv1.Cluster
		clusterName   string
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))
		Expect(e2eConfig.Variables).To(HaveKey(CNIPath))

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, bootstrapClusterProxy, artifactFolder)

		clusterName = fmt.Sprintf("capz-e2e-%s", util.RandomString(6))
		Expect(os.Setenv(AzureResourceGroup, clusterName)).NotTo(HaveOccurred())
		Expect(os.Setenv(AzureVNetName, fmt.Sprintf("%s-vnet", clusterName))).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, cancelWatches, cluster, e2eConfig.GetIntervals, skipCleanup)
		Expect(os.Unsetenv(AzureResourceGroup)).NotTo(HaveOccurred())
		Expect(os.Unsetenv(AzureVNetName)).NotTo(HaveOccurred())
	})

	Describe("Creating a single control-plane cluster", func() {
		Context("With 1 worker node", func() {
			It("Deploys a workload cluster", func() {
				cluster, _, _ = clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
					ClusterProxy: bootstrapClusterProxy,
					ConfigCluster: clusterctl.ConfigClusterInput{
						LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
						ClusterctlConfigPath:     clusterctlConfigPath,
						KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
						InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
						Flavor:                   clusterctl.DefaultFlavor,
						Namespace:                namespace.Name,
						ClusterName:              clusterName,
						KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
						ControlPlaneMachineCount: pointer.Int64Ptr(1),
						WorkerMachineCount:       pointer.Int64Ptr(1),
					},
					CNIManifestPath:              e2eConfig.GetVariable(CNIPath),
					WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
					WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
					WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				})
			})

			It("has basic load balancer functionality", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

		})
	})

	Describe("Creating highly available control-plane cluster", func() {
		Context("With 3 control-plane nodes and 2 worker nodes", func() {
			It("deploys the cluster", func() {
				cluster, _, _ = clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
					ClusterProxy: bootstrapClusterProxy,
					ConfigCluster: clusterctl.ConfigClusterInput{
						LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
						ClusterctlConfigPath:     clusterctlConfigPath,
						KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
						InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
						Flavor:                   clusterctl.DefaultFlavor,
						Namespace:                namespace.Name,
						ClusterName:              clusterName,
						KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
						ControlPlaneMachineCount: pointer.Int64Ptr(3),
						WorkerMachineCount:       pointer.Int64Ptr(2),
					},
					CNIManifestPath:              e2eConfig.GetVariable(CNIPath),
					WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
					WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
					WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				})
			})

			It("Creating a accessible load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			It("Validating network policies", func() {
				AzureNetPolSpec(ctx, func() AzureNetPolSpecInput {
					return AzureNetPolSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})
		})
	})
})
