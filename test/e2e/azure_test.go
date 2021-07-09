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

	"sigs.k8s.io/cluster-api/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var _ = Describe("Workload cluster creation", func() {
	var (
		ctx           = context.TODO()
		specName      = "create-workload-cluster"
		namespace     *corev1.Namespace
		cancelWatches context.CancelFunc
		result        *clusterctl.ApplyClusterTemplateAndWaitResult
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

		clusterName = os.Getenv("CLUSTER_NAME")
		if clusterName == "" {
			clusterName = fmt.Sprintf("capz-e2e-%s", util.RandomString(6))
		}
		fmt.Fprintf(GinkgoWriter, "INFO: Cluster name is %s\n", clusterName)

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		var err error
		namespace, cancelWatches, err = setupSpecNamespace(ctx, clusterName, bootstrapClusterProxy, artifactFolder)
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Setenv(AzureResourceGroup, clusterName)).NotTo(HaveOccurred())
		Expect(os.Setenv(AzureVNetName, fmt.Sprintf("%s-vnet", clusterName))).NotTo(HaveOccurred())
		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		spClientSecret := os.Getenv(AzureClientSecret)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-identity-secret",
				Namespace: namespace.Name,
				Labels: map[string]string{
					clusterctlv1.ClusterctlMoveHierarchyLabelName: "true",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"clientSecret": []byte(spClientSecret)},
		}
		err = bootstrapClusterProxy.GetClient().Create(ctx, secret)
		Expect(err).ToNot(HaveOccurred())

		identityName := e2eConfig.GetVariable(ClusterIdentityName)
		Expect(os.Setenv(ClusterIdentityName, identityName)).NotTo(HaveOccurred())
		Expect(os.Setenv(ClusterIdentityNamespace, namespace.Name)).NotTo(HaveOccurred())
		Expect(os.Setenv(ClusterIdentitySecretName, "cluster-identity-secret")).NotTo(HaveOccurred())
		Expect(os.Setenv(ClusterIdentitySecretNamespace, namespace.Name)).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if result.Cluster == nil {
			// this means the cluster failed to come up. We make an attempt to find the cluster to be able to fetch logs for the failed bootstrapping.
			_ = bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{Name: clusterName, Namespace: namespace.Name}, result.Cluster)
		}
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, cancelWatches, result.Cluster, e2eConfig.GetIntervals, skipCleanup)
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

				clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
					ClusterProxy: bootstrapClusterProxy,
					ConfigCluster: clusterctl.ConfigClusterInput{
						LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
						ClusterctlConfigPath:     clusterctlConfigPath,
						KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
						InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
						Flavor:                   "custom-vnet",
						Namespace:                namespace.Name,
						ClusterName:              clusterName,
						KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
						ControlPlaneMachineCount: pointer.Int64Ptr(1),
						WorkerMachineCount:       pointer.Int64Ptr(1),
					},
					WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
					WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
					WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				}, result)

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
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
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
		}, result)

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
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
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
			}, result)

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
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
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
			}, result)

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
					}
				})
			})

			Context("Cordon and draining a node", func() {
				AzureMachinePoolDrainSpec(ctx, func() AzureMachinePoolDrainSpecInput {
					return AzureMachinePoolDrainSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
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
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
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
				// nvidia-gpu flavor creates a config map as part of a crs, that exceeds the annotations size limit when we do kubectl apply.
				// This is because the entire config map is stored in `last-applied` annotation for tracking.
				// The workaround is to use server side apply by passing `--server-side` flag to kubectl apply.
				// More on server side apply here: https://kubernetes.io/docs/reference/using-api/server-side-apply/
				Args: []string{"--server-side"},
			}, result)

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

	// ci-e2e.sh and Prow CI skip this test by default.
	// To include this test, set `GINKGO_SKIP=""`.
	Context("Creating a cluster that uses the external cloud provider", func() {
		It("with a 1 control plane nodes and 2 worker nodes", func() {
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "external-cloud-provider",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(2),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			}, result)

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
		})
	})

	Context("Creating an AKS cluster using a different SP identity", func() {
		It("with a single control plane node and 1 node", func() {
			kubernetesVersion, err := GetAKSKubernetesVersion(ctx, e2eConfig)
			Expect(err).To(BeNil())

			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "aks-multi-tenancy",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        kubernetesVersion,
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized:   WaitForControlPlaneInitialized,
					WaitForControlPlaneMachinesReady: WaitForControlPlaneMachinesReady,
				},
			}, result)

			Context("Validating AKS Resources", func() {
				AKSResourcesValidationSpec(ctx, func() AKSResourcesValidationSpecInput {
					return AKSResourcesValidationSpecInput{
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
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
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
			}, result)

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
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
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
			}, result)

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
})
