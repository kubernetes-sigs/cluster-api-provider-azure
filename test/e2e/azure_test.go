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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
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
		specTimes     = map[string]time.Time{}
	)

	BeforeEach(func() {
		logCheckpoint(specTimes)

		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersion))
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.CNIPath))

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

		logCheckpoint(specTimes)
	})

	if os.Getenv("LOCAL_ONLY") != "true" {
		Context("Creating a private cluster", func() {
			It("Creates a public management cluster in the same vnet", func() {
				Context("Creating a custom virtual network", func() {
					Expect(os.Setenv(AzureVNetName, "custom-vnet")).NotTo(HaveOccurred())
					cpCIDR := "10.128.0.0/16"
					Expect(os.Setenv(AzureCPSubnetCidr, cpCIDR)).NotTo(HaveOccurred())
					nodeCIDR := "10.129.0.0/16"
					Expect(os.Setenv(AzureNodeSubnetCidr, nodeCIDR)).NotTo(HaveOccurred())
					SetupExistingVNet(ctx,
						"10.0.0.0/8",
						map[string]string{fmt.Sprintf("%s-controlplane-subnet", clusterName): "10.0.0.0/16", "private-cp-subnet": cpCIDR},
						map[string]string{fmt.Sprintf("%s-node-subnet", clusterName): "10.1.0.0/16", "private-node-subnet": nodeCIDR})
				})

				result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
					ClusterProxy: bootstrapClusterProxy,
					ConfigCluster: clusterctl.ConfigClusterInput{
						LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
						ClusterctlConfigPath:     clusterctlConfigPath,
						KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
						InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
						Flavor:                   clusterctl.DefaultFlavor,
						Namespace:                namespace.Name,
						ClusterName:              clusterName,
						KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
						ControlPlaneMachineCount: pointer.Int64Ptr(1),
						WorkerMachineCount:       pointer.Int64Ptr(1),
					},
					WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
					WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
					WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				})
				cluster = result.Cluster

				Context("Validating time synchronization", func() {
					AzureTimeSyncSpec(ctx, func() AzureTimeSyncSpecInput {
						return AzureTimeSyncSpecInput{
							BootstrapClusterProxy: bootstrapClusterProxy,
							Namespace:             namespace,
							ClusterName:           clusterName,
						}
					})
				})

				Context("Creating a private cluster from the management cluster", func() {
					AzurePrivateClusterSpec(ctx, func() AzurePrivateClusterSpecInput {
						return AzurePrivateClusterSpecInput{
							BootstrapClusterProxy: bootstrapClusterProxy,
							Namespace:             namespace,
							ClusterName:           clusterName,
							ClusterctlConfigPath:  clusterctlConfigPath,
							E2EConfig:             e2eConfig,
							ArtifactFolder:        artifactFolder,
						}
					})
				})
			})
		})
	} else {
		fmt.Fprintf(GinkgoWriter, "INFO: skipping test requires pushing container images to external repository")
	}

	It("With 3 control-plane nodes and 2 worker nodes", func() {
		result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   clusterctl.DefaultFlavor,
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64Ptr(3),
				WorkerMachineCount:       pointer.Int64Ptr(2),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		})
		cluster = result.Cluster

		Context("Validating time synchronization", func() {
			AzureTimeSyncSpec(ctx, func() AzureTimeSyncSpecInput {
				return AzureTimeSyncSpecInput{
					BootstrapClusterProxy: bootstrapClusterProxy,
					Namespace:             namespace,
					ClusterName:           clusterName,
				}
			})
		})

		Context("Validating failure domains", func() {
			AzureFailureDomainsSpec(ctx, func() AzureFailureDomainsSpecInput {
				return AzureFailureDomainsSpecInput{
					BootstrapClusterProxy: bootstrapClusterProxy,
					Cluster:               result.Cluster,
					Namespace:             namespace,
					ClusterName:           clusterName,
				}
			})
		})

		Context("Creating an accessible load balancer", func() {
			AzureLBSpec(ctx, func() AzureLBSpecInput {
				return AzureLBSpecInput{
					BootstrapClusterProxy: bootstrapClusterProxy,
					Namespace:             namespace,
					ClusterName:           clusterName,
					SkipCleanup:           skipCleanup,
				}
			})
		})

		Context("Validating network policies", func() {
			AzureNetPolSpec(ctx, func() AzureNetPolSpecInput {
				return AzureNetPolSpecInput{
					BootstrapClusterProxy: bootstrapClusterProxy,
					Namespace:             namespace,
					ClusterName:           clusterName,
					SkipCleanup:           skipCleanup,
				}
			})
		})

		Context("Validating accelerated networking", func() {
			AzureAcceleratedNetworkingSpec(ctx, func() AzureAcceleratedNetworkingSpecInput {
				return AzureAcceleratedNetworkingSpecInput{
					ClusterName: clusterName,
				}
			})
		})
	})

	Context("Creating a ipv6 control-plane cluster", func() {
		It("With ipv6 worker node", func() {
			result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ipv6",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(3),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			})
			cluster = result.Cluster

			Context("Validating time synchronization", func() {
				AzureTimeSyncSpec(ctx, func() AzureTimeSyncSpecInput {
					return AzureTimeSyncSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			Context("Creating an accessible ipv6 load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						IPv6:                  true,
					}
				})
			})
		})
	})

	Context("Creating a VMSS cluster", func() {
		It("with a single control plane node and an AzureMachinePool with 2 nodes", func() {
			result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "machine-pool",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(2),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			})
			cluster = result.Cluster

			Context("Validating time synchronization", func() {
				AzureTimeSyncSpec(ctx, func() AzureTimeSyncSpecInput {
					return AzureTimeSyncSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			Context("Creating an accessible load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						IsVMSS:                true,
					}
				})
			})
		})
	})

	// ci-e2e.sh and Prow CI skip this test by default, since N-series GPUs are relatively expensive
	// and may require specific quota limits on the subscription.
	// To include this test, set `GINKGO_SKIP=""`.
	// You can override the default SKU `Standard_NV6` and `Standard_LRS` storage by setting
	// the `AZURE_GPU_NODE_MACHINE_TYPE` and `AZURE_GPU_NODE_STORAGE_TYPE` environment variables.
	// See https://azure.microsoft.com/en-us/pricing/details/virtual-machines/linux/ for pricing.
	Context("Creating a GPU-enabled cluster", func() {
		It("with a single control plane node and 1 node", func() {
			result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "nvidia-gpu",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			})
			cluster = result.Cluster

			Context("Running a GPU-based calculation", func() {
				AzureGPUSpec(ctx, func() AzureGPUSpecInput {
					return AzureGPUSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			Context("Validating accelerated networking", func() {
				AzureAcceleratedNetworkingSpec(ctx, func() AzureAcceleratedNetworkingSpecInput {
					return AzureAcceleratedNetworkingSpecInput{
						ClusterName: clusterName,
					}
				})
			})
		})
	})

	Context("Creating a cluster using a different SP identity", func() {
		BeforeEach(func() {
			spClientSecret := os.Getenv("AZURE_MULTI_TENANCY_SECRET")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sp-identity-secret",
					Namespace: namespace.Name,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"clientSecret": []byte(spClientSecret)},
			}
			err := bootstrapClusterProxy.GetClient().Create(ctx, secret)
			Expect(err).ToNot(HaveOccurred())
		})

		It("with a single control plane node and 1 node", func() {
			spClientID := os.Getenv("AZURE_MULTI_TENANCY_ID")
			identityName := e2eConfig.GetVariable(MultiTenancyIdentityName)
			os.Setenv("CLUSTER_IDENTITY_NAME", identityName)
			os.Setenv("CLUSTER_IDENTITY_NAMESPACE", namespace.Name)
			os.Setenv("AZURE_CLUSTER_IDENTITY_CLIENT_ID", spClientID)
			os.Setenv("AZURE_CLUSTER_IDENTITY_SECRET_NAME", "sp-identity-secret")
			os.Setenv("AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE", namespace.Name)

			result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "multi-tenancy",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			})
			cluster = result.Cluster

			Context("Validating identity", func() {
				AzureServicePrincipalIdentitySpec(ctx, func() AzureServicePrincipalIdentitySpecInput {
					return AzureServicePrincipalIdentitySpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})
		})
	})

	Context("Creating a Windows Enabled cluster", func() {
		// Requires 3 control planes due to https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/857
		It("With 3 control-plane nodes and 1 Linux worker node and 1 Windows worker node", func() {
			result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "windows",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(3),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			})
			cluster = result.Cluster

			Context("Creating an accessible load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			Context("Creating an accessible load balancer for windows", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						Windows:               true,
					}
				})
			})
		})
	})

	Context("Creating a Windows enabled VMSS cluster", func() {
		It("with a single control plane node and an Linux AzureMachinePool with 1 nodes and Windows AzureMachinePool with 1 node", func() {
			result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "machine-pool-windows",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			})
			cluster = result.Cluster

			Context("Creating an accessible load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						IsVMSS:                true,
					}
				})
			})

			Context("Creating an accessible load balancer for windows", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						Windows:               true,
						IsVMSS:                true,
					}
				})
			})
		})
	})
})
