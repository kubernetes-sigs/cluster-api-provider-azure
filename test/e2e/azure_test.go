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
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Workload cluster creation", func() {
	var (
		ctx                    = context.TODO()
		specName               = "create-workload-cluster"
		namespace              *corev1.Namespace
		cancelWatches          context.CancelFunc
		result                 *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName            string
		clusterNamePrefix      string
		additionalCleanup      func()
		specTimes              = map[string]time.Time{}
		skipResourceGroupCheck = false
	)

	BeforeEach(func() {
		logCheckpoint(specTimes)

		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(e2eConfig).NotTo(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0o755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersion))

		// CLUSTER_NAME and CLUSTER_NAMESPACE allows for testing existing clusters
		// if CLUSTER_NAMESPACE is set don't generate a new prefix otherwise
		// the correct namespace won't be found and a new cluster will be created
		clusterNameSpace := os.Getenv("CLUSTER_NAMESPACE")
		if clusterNameSpace == "" {
			clusterNamePrefix = fmt.Sprintf("capz-e2e-%s", util.RandomString(6))
		} else {
			clusterNamePrefix = clusterNameSpace
		}

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		var err error
		namespace, cancelWatches, err = setupSpecNamespace(ctx, clusterNamePrefix, bootstrapClusterProxy, artifactFolder)
		Expect(err).NotTo(HaveOccurred())

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
		_, err = bootstrapClusterProxy.GetClientSet().CoreV1().Secrets(namespace.Name).Get(ctx, secret.Name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			Expect(err).ShouldNot(HaveOccurred())
		}
		if err != nil {
			Logf("Creating cluster identity secret \"%s\"", secret.Name)
			err = bootstrapClusterProxy.GetClient().Create(ctx, secret)
			Expect(err).NotTo(HaveOccurred())
		} else {
			Logf("Using existing cluster identity secret")
		}

		identityName := e2eConfig.GetVariable(ClusterIdentityName)
		Expect(os.Setenv(ClusterIdentityName, identityName)).To(Succeed())
		Expect(os.Setenv(ClusterIdentityNamespace, namespace.Name)).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretName, "cluster-identity-secret")).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretNamespace, namespace.Name)).To(Succeed())
		additionalCleanup = nil
	})

	AfterEach(func() {
		if result.Cluster == nil {
			// this means the cluster failed to come up. We make an attempt to find the cluster to be able to fetch logs for the failed bootstrapping.
			_ = bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{Name: clusterName, Namespace: namespace.Name}, result.Cluster)
		}

		CheckTestBeforeCleanup()

		cleanInput := cleanupInput{
			SpecName:               specName,
			Cluster:                result.Cluster,
			ClusterProxy:           bootstrapClusterProxy,
			Namespace:              namespace,
			CancelWatches:          cancelWatches,
			IntervalsGetter:        e2eConfig.GetIntervals,
			SkipCleanup:            skipCleanup,
			SkipLogCollection:      skipLogCollection,
			AdditionalCleanup:      additionalCleanup,
			ArtifactFolder:         artifactFolder,
			SkipResourceGroupCheck: skipResourceGroupCheck,
		}
		dumpSpecResourcesAndCleanup(ctx, cleanInput)
		Expect(os.Unsetenv(AzureResourceGroup)).To(Succeed())
		Expect(os.Unsetenv(AzureVNetName)).To(Succeed())
		Expect(os.Unsetenv(ClusterIdentityName)).To(Succeed())
		Expect(os.Unsetenv(ClusterIdentityNamespace)).To(Succeed())
		Expect(os.Unsetenv(ClusterIdentitySecretName)).To(Succeed())
		Expect(os.Unsetenv(ClusterIdentitySecretNamespace)).To(Succeed())

		Expect(os.Unsetenv("WINDOWS_WORKER_MACHINE_COUNT")).To(Succeed())
		Expect(os.Unsetenv("K8S_FEATURE_GATES")).To(Succeed())

		logCheckpoint(specTimes)
	})

	if os.Getenv("USE_LOCAL_KIND_REGISTRY") != "true" {
		// This spec expects a user-assigned identity with Contributor role assignment named "cloud-provider-user-identity" in a "capz-ci"
		// resource group. Override these defaults by setting the USER_IDENTITY and CI_RG environment variables.
		Context("Creating a private cluster [OPTIONAL]", func() {
			It("Creates a public management cluster in a custom vnet", func() {
				clusterName = getClusterName(clusterNamePrefix, "public-custom-vnet")
				By("Creating a custom virtual network", func() {
					Expect(os.Setenv(AzureCustomVNetName, "custom-vnet")).To(Succeed())
					additionalCleanup = SetupExistingVNet(ctx,
						"10.0.0.0/16",
						map[string]string{fmt.Sprintf("%s-controlplane-subnet", os.Getenv(AzureCustomVNetName)): "10.0.0.0/24"},
						map[string]string{fmt.Sprintf("%s-node-subnet", os.Getenv(AzureCustomVNetName)): "10.0.1.0/24"},
						fmt.Sprintf("%s-azure-bastion-subnet", os.Getenv(AzureCustomVNetName)),
						"10.0.2.0/24",
					)
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
					ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
						WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
					},
				}, result)

				By("Creating a private cluster from the management cluster", func() {
					AzurePrivateClusterSpec(ctx, func() AzurePrivateClusterSpecInput {
						return AzurePrivateClusterSpecInput{
							BootstrapClusterProxy: bootstrapClusterProxy,
							Namespace:             namespace,
							ClusterName:           clusterName,
							ClusterctlConfigPath:  clusterctlConfigPath,
							E2EConfig:             e2eConfig,
							ArtifactFolder:        artifactFolder,
							SkipCleanup:           skipCleanup,
							CancelWatches:         cancelWatches,
						}
					})
				})

				By("PASSED!")
			})
		})
	} else {
		fmt.Fprintf(GinkgoWriter, "INFO: skipping test requires pushing container images to external repository")
	}

	Context("Creating a highly available cluster [REQUIRED]", func() {
		It("With 3 control-plane nodes and 2 Linux and 2 Windows worker nodes", func() {
			clusterName = getClusterName(clusterNamePrefix, "ha")

			// Opt into using windows with prow template
			Expect(os.Setenv("WINDOWS_WORKER_MACHINE_COUNT", "2")).To(Succeed())
			Expect(os.Setenv("K8S_FEATURE_GATES", "WindowsHostProcessContainers=true")).To(Succeed())

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
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane-ha"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}, result)

			By("Verifying expected VM extensions are present on the node", func() {
				AzureVMExtensionsSpec(ctx, func() AzureVMExtensionsSpecInput {
					return AzureVMExtensionsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			By("Validating failure domains", func() {
				AzureFailureDomainsSpec(ctx, func() AzureFailureDomainsSpecInput {
					return AzureFailureDomainsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Cluster:               result.Cluster,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			By("Creating an accessible load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			By("Validating network policies", func() {
				AzureNetPolSpec(ctx, func() AzureNetPolSpecInput {
					return AzureNetPolSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			By("Creating an accessible load balancer for windows", func() {
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

			By("PASSED!")
		})
	})

	Context("Creating a ipv6 control-plane cluster [REQUIRED]", func() {
		It("With ipv6 worker node", func() {
			clusterName = getClusterName(clusterNamePrefix, "ipv6")
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
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane-ha"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}, result)

			By("Verifying expected VM extensions are present on the node", func() {
				AzureVMExtensionsSpec(ctx, func() AzureVMExtensionsSpecInput {
					return AzureVMExtensionsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			By("Creating an accessible ipv6 load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						// Setting IPFamily to ipv6 is not required for single-stack IPv6 clusters. The clusterIP
						// will be automatically assigned IPv6 address. However, setting this config so that
						// we can use the same test code for both single-stack and dual-stack IPv6 clusters.
						IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol},
					}
				})
			})

			By("PASSED!")
		})
	})

	Context("Creating a VMSS cluster [REQUIRED]", func() {
		It("with a single control plane node and an AzureMachinePool with 2 Linux and 2 Windows worker nodes", func() {
			clusterName = getClusterName(clusterNamePrefix, "vmss")

			// Opt into using windows with prow template
			Expect(os.Setenv("WINDOWS_WORKER_MACHINE_COUNT", "2")).To(Succeed())
			Expect(os.Setenv("K8S_FEATURE_GATES", "WindowsHostProcessContainers=true")).To(Succeed())

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
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}, result)

			By("Verifying expected VM extensions are present on the node", func() {
				AzureVMExtensionsSpec(ctx, func() AzureVMExtensionsSpecInput {
					return AzureVMExtensionsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			By("Creating an accessible load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			By("Creating an accessible load balancer for windows", func() {
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

			// TODO: Implement more robust cordon and drain test
			// By("Cordon and draining a node", func() {
			// 	AzureMachinePoolDrainSpec(ctx, func() AzureMachinePoolDrainSpecInput {
			// 		return AzureMachinePoolDrainSpecInput{
			// 			BootstrapClusterProxy: bootstrapClusterProxy,
			// 			Namespace:             namespace,
			// 			ClusterName:           clusterName,
			// 			SkipCleanup:           skipCleanup,
			// 		}
			// 	})
			// })

			By("PASSED!")
		})
	})

	// ci-e2e.sh and Prow CI skip this test by default, since N-series GPUs are relatively expensive
	// and may require specific quota limits on the subscription.
	// To include this test, set `GINKGO_SKIP=""`.
	// You can override the default SKU `Standard_NV6` and `Standard_LRS` storage by setting
	// the `AZURE_GPU_NODE_MACHINE_TYPE` and `AZURE_GPU_NODE_STORAGE_TYPE` environment variables.
	// See https://azure.microsoft.com/en-us/pricing/details/virtual-machines/linux/ for pricing.
	Context("Creating a GPU-enabled cluster [OPTIONAL]", func() {
		It("with a single control plane node and 1 node", func() {
			clusterName = getClusterName(clusterNamePrefix, "gpu")
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
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-gpu-nodes"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
				// nvidia-gpu flavor creates a config map as part of a crs, that exceeds the annotations size limit when we do kubectl apply.
				// This is because the entire config map is stored in `last-applied` annotation for tracking.
				// The workaround is to use server side apply by passing `--server-side` flag to kubectl apply.
				// More on server side apply here: https://kubernetes.io/docs/reference/using-api/server-side-apply/
				Args: []string{"--server-side"},
			}, result)

			By("Verifying expected VM extensions are present on the node", func() {
				AzureVMExtensionsSpec(ctx, func() AzureVMExtensionsSpecInput {
					return AzureVMExtensionsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			By("Running a GPU-based calculation", func() {
				AzureGPUSpec(ctx, func() AzureGPUSpecInput {
					return AzureGPUSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			By("PASSED!")
		})
	})

	// ci-e2e.sh and Prow CI skip this test by default. To include this test, set `GINKGO_SKIP=""`.
	// This spec expects a user-assigned identity named "cloud-provider-user-identity" in a "capz-ci"
	// resource group. Override these defaults by setting the USER_IDENTITY and CI_RG environment variables.
	Context("Creating a cluster that uses the external cloud provider and machinepools [OPTIONAL]", func() {
		It("with 1 control plane node and 1 machinepool", func() {
			By("using user-assigned identity")
			clusterName = getClusterName(clusterNamePrefix, "flex")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "external-cloud-provider-vmss-flex",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        "v1.26.0",
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}, result)

			By("Verifying machinepool resources", func() {
				AzureMachinePoolsSpec(ctx, func() AzureMachinePoolsSpecInput {
					return AzureMachinePoolsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			By("Verifying expected VM extensions are present on the node", func() {
				AzureVMExtensionsSpec(ctx, func() AzureVMExtensionsSpecInput {
					return AzureVMExtensionsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			By("Creating an accessible load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			By("PASSED!")
		})
	})

	// ci-e2e.sh and Prow CI skip this test by default. To include this test, set `GINKGO_SKIP=""`.
	// This spec expects a user-assigned identity named "cloud-provider-user-identity" in a "capz-ci"
	// resource group. Override these defaults by setting the USER_IDENTITY and CI_RG environment variables.
	Context("Creating a cluster that uses the external cloud provider and external azurediskcsi driver [OPTIONAL]", func() {
		It("with a 1 control plane nodes and 2 worker nodes", func() {
			By("using user-assigned identity")
			clusterName = getClusterName(clusterNamePrefix, "oot")
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
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}, result)

			By("Verifying expected VM extensions are present on the node", func() {
				AzureVMExtensionsSpec(ctx, func() AzureVMExtensionsSpecInput {
					return AzureVMExtensionsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			By("Creating an accessible load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			By("Creating a deployment that uses persistent volume", func() {
				AzureDiskCSISpec(ctx, func() AzureDiskCSISpecInput {
					return AzureDiskCSISpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
			})

			By("PASSED!")
		})
	})

	// You can override the default SKU `Standard_D2s_v3` by setting the
	// `AZURE_AKS_NODE_MACHINE_TYPE` environment variable.
	Context("Creating an AKS cluster [Managed Kubernetes]", func() {
		It("with a single control plane node and 1 node", func() {
			clusterName = getClusterName(clusterNamePrefix, aksClusterNameSuffix)
			kubernetesVersionUpgradeFrom, err := GetAKSKubernetesVersion(ctx, e2eConfig, AKSKubernetesVersionUpgradeFrom)
			Expect(err).To(BeNil())
			kubernetesVersion, err := GetAKSKubernetesVersion(ctx, e2eConfig, AKSKubernetesVersion)
			Expect(err).To(BeNil())

			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "aks",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        kubernetesVersionUpgradeFrom,
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized:   WaitForAKSControlPlaneInitialized,
					WaitForControlPlaneMachinesReady: WaitForAKSControlPlaneReady,
				},
			}, result)

			By("Upgrading the Kubernetes version of the cluster", func() {
				AKSUpgradeSpec(ctx, func() AKSUpgradeSpecInput {
					return AKSUpgradeSpecInput{
						Cluster:                    result.Cluster,
						MachinePools:               result.MachinePools,
						KubernetesVersionUpgradeTo: kubernetesVersion,
						WaitForControlPlane:        e2eConfig.GetIntervals(specName, "wait-machine-upgrade"),
						WaitForMachinePools:        e2eConfig.GetIntervals(specName, "wait-machine-pool-upgrade"),
					}
				})
			})

			By("Exercising machine pools", func() {
				AKSMachinePoolSpec(ctx, func() AKSMachinePoolSpecInput {
					return AKSMachinePoolSpecInput{
						Cluster:       result.Cluster,
						MachinePools:  result.MachinePools,
						WaitIntervals: e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes")}
				})
			})

			By("creating a machine pool with public IP addresses from a prefix", func() {
				// This test is also currently serving as the canonical
				// "create/delete node pool" test. Eventually, that should be
				// made more distinct from this public IP prefix test.
				AKSPublicIPPrefixSpec(ctx, func() AKSPublicIPPrefixSpecInput {
					return AKSPublicIPPrefixSpecInput{
						Cluster:           result.Cluster,
						KubernetesVersion: kubernetesVersion,
						WaitIntervals:     e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
					}
				})
			})

			By("modifying nodepool autoscaling configuration", func() {
				AKSAutoscaleSpec(ctx, func() AKSAutoscaleSpecInput {
					return AKSAutoscaleSpecInput{
						Cluster:       result.Cluster,
						MachinePool:   result.MachinePools[0],
						WaitIntervals: e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
					}
				})
			})

			By("modifying additionalTags configuration", func() {
				AKSAdditionalTagsSpec(ctx, func() AKSAdditionalTagsSpecInput {
					return AKSAdditionalTagsSpecInput{
						Cluster:       result.Cluster,
						MachinePools:  result.MachinePools,
						WaitForUpdate: e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
					}
				})
			})

			By("modifying the azure cluster-autoscaler settings", func() {
				AKSAzureClusterAutoscalerSettingsSpec(ctx, func() AKSAzureClusterAutoscalerSettingsSpecInput {
					return AKSAzureClusterAutoscalerSettingsSpecInput{
						Cluster:       result.Cluster,
						WaitIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
					}
				})
			})
		})
	})

	// ci-e2e.sh and Prow CI skip this test by default. To include this test, set `GINKGO_SKIP=""`.
	// This spec expects a user-assigned identity named "cloud-provider-user-identity" in a "capz-ci"
	// resource group. Override these defaults by setting the USER_IDENTITY and CI_RG environment variables.
	Context("Creating a dual-stack cluster [OPTIONAL]", func() {
		It("With dual-stack worker node", func() {
			By("using user-assigned identity")
			clusterName = getClusterName(clusterNamePrefix, "dual-stack")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "dual-stack",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(3),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane-ha"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}, result)

			By("Verifying expected VM extensions are present on the node", func() {
				AzureVMExtensionsSpec(ctx, func() AzureVMExtensionsSpecInput {
					return AzureVMExtensionsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			// dual-stack external IP for dual-stack clusters is not yet supported
			// first ip family in ipFamilies is used for the primary clusterIP and cloud-provider
			// determines the elb/ilb ip family based on the primary clusterIP
			By("Creating an accessible ipv4 load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						IPFamilies:            []corev1.IPFamily{corev1.IPv4Protocol},
					}
				})
			})

			By("Creating an accessible ipv6 load balancer", func() {
				AzureLBSpec(ctx, func() AzureLBSpecInput {
					return AzureLBSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						IPFamilies:            []corev1.IPFamily{corev1.IPv6Protocol},
					}
				})
			})

			By("PASSED!")
		})
	})

	Context("Creating clusters using clusterclass [OPTIONAL]", func() {
		It("with a single control plane node, one linux worker node, and one windows worker node", func() {
			// use "cc" as spec name because natgw pip name exceeds limit.
			clusterName = getClusterName(clusterNamePrefix, "cc")

			// Opt into using windows with prow template
			Expect(os.Setenv("WINDOWS_WORKER_MACHINE_COUNT", "1")).To(Succeed())
			Expect(os.Setenv("K8S_FEATURE_GATES", "WindowsHostProcessContainers=true")).To(Succeed())

			// Create a cluster using the cluster class created above
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "topology",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(capi_e2e.KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}, result)

			By("Verifying expected VM extensions are present on the node", func() {
				AzureVMExtensionsSpec(ctx, func() AzureVMExtensionsSpecInput {
					return AzureVMExtensionsSpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
					}
				})
			})

			By("PASSED!")
		})
	})
})
