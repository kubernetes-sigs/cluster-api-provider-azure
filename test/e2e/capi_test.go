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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

const (
	IdentitySecretName = "cluster-identity-secret"
)

var _ = Describe("Running the Cluster API E2E tests", func() {
	var (
		ctx       = context.TODO()
		specTimes = map[string]time.Time{}
	)
	BeforeEach(func() {
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.CNIPath))
		rgName := fmt.Sprintf("capz-e2e-%s", util.RandomString(6))
		Expect(os.Setenv(AzureResourceGroup, rgName)).To(Succeed())
		Expect(os.Setenv(AzureVNetName, fmt.Sprintf("%s-vnet", rgName))).To(Succeed())

		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersionUpgradeFrom))
		Expect(os.Setenv("WINDOWS_WORKER_MACHINE_COUNT", "2")).To(Succeed())

		identityName := e2eConfig.GetVariable(ClusterIdentityName)
		Expect(os.Setenv(ClusterIdentityName, identityName)).To(Succeed())

		logCheckpoint(specTimes)
	})

	AfterEach(func() {
		CheckTestBeforeCleanup()
		redactLogs()

		Expect(os.Unsetenv(AzureResourceGroup)).To(Succeed())
		Expect(os.Unsetenv(AzureVNetName)).To(Succeed())
		Expect(os.Unsetenv(ClusterIdentityName)).To(Succeed())

		logCheckpoint(specTimes)
	})

	Context("Running the quick-start spec", func() {
		capi_e2e.QuickStartSpec(context.TODO(), func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitializedNoAddons,
				},
			}
		})
	})

	Context("Running the MachineDeployment rollout spec", func() {
		capi_e2e.MachineDeploymentRolloutSpec(context.TODO(), func() capi_e2e.MachineDeploymentRolloutSpecInput {
			return capi_e2e.MachineDeploymentRolloutSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitializedNoAddons,
				},
			}
		})
	})

	if os.Getenv("USE_LOCAL_KIND_REGISTRY") != "true" {
		Context("Running the self-hosted spec", func() {
			SelfHostedSpec(context.TODO(), func() SelfHostedSpecInput {
				return SelfHostedSpecInput{
					E2EConfig:             e2eConfig,
					ClusterctlConfigPath:  clusterctlConfigPath,
					BootstrapClusterProxy: bootstrapClusterProxy,
					ArtifactFolder:        artifactFolder,
					SkipCleanup:           skipCleanup,
					ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
						WaitForControlPlaneInitialized: EnsureControlPlaneInitializedNoAddons,
					},
				}
			})
		})
	}

	// TODO: Add test using KCPRemediationSpec
	Context("Should successfully remediate unhealthy worker machines with MachineHealthCheck", func() {
		capi_e2e.MachineDeploymentRemediationSpec(context.TODO(), func() capi_e2e.MachineDeploymentRemediationSpecInput {
			return capi_e2e.MachineDeploymentRemediationSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitializedNoAddons,
				},
			}
		})
	})

	Context("Should successfully exercise machine pools", func() {
		capi_e2e.MachinePoolSpec(context.TODO(), func() capi_e2e.MachinePoolInput {
			return capi_e2e.MachinePoolInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitializedNoAddons,
				},
			}
		})
	})

	Context("Should successfully scale out and scale in a MachineDeployment", func() {
		capi_e2e.MachineDeploymentScaleSpec(context.TODO(), func() capi_e2e.MachineDeploymentScaleSpecInput {
			return capi_e2e.MachineDeploymentScaleSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitializedNoAddons,
				},
			}
		})
	})

	Context("Should successfully set and use node drain timeout", func() {
		capi_e2e.NodeDrainTimeoutSpec(context.TODO(), func() capi_e2e.NodeDrainTimeoutSpecInput {
			return capi_e2e.NodeDrainTimeoutSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitializedNoAddons,
				},
			}
		})
	})

	if os.Getenv("USE_LOCAL_KIND_REGISTRY") != "true" {
		Context("API Version Upgrade", func() {
			BeforeEach(func() {
				// Unset resource group and vnet env variables, since the upgrade test creates 2 clusters,
				// and will result in both the clusters using the same vnet and resource group.
				Expect(os.Unsetenv(AzureResourceGroup)).To(Succeed())
				Expect(os.Unsetenv(AzureVNetName)).To(Succeed())

				// Unset windows specific variables
				Expect(os.Unsetenv("WINDOWS_WORKER_MACHINE_COUNT")).To(Succeed())
			})

			Context("upgrade from an old version of v1beta1 to current, and scale workload clusters created in the old version", func() {
				capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
					return capi_e2e.ClusterctlUpgradeSpecInput{
						E2EConfig:                 e2eConfig,
						ClusterctlConfigPath:      clusterctlConfigPath,
						BootstrapClusterProxy:     bootstrapClusterProxy,
						ArtifactFolder:            artifactFolder,
						SkipCleanup:               skipCleanup,
						PreInit:                   getPreInitFunc(ctx),
						InitWithProvidersContract: "v1beta1",
						ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
							WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
						},
						InitWithKubernetesVersion:       e2eConfig.GetVariable(KubernetesVersionAPIUpgradeFrom),
						InitWithBinary:                  fmt.Sprintf("https://github.com/kubernetes-sigs/cluster-api/releases/download/%s/clusterctl-{OS}-{ARCH}", e2eConfig.GetVariable(OldCAPIUpgradeVersion)),
						InitWithCoreProvider:            "cluster-api:" + e2eConfig.GetVariable(OldCAPIUpgradeVersion),
						InitWithBootstrapProviders:      []string{"kubeadm:" + e2eConfig.GetVariable(OldCAPIUpgradeVersion)},
						InitWithControlPlaneProviders:   []string{"kubeadm:" + e2eConfig.GetVariable(OldCAPIUpgradeVersion)},
						InitWithInfrastructureProviders: []string{"azure:" + e2eConfig.GetVariable(OldProviderUpgradeVersion)},
						InitWithAddonProviders:          []string{"helm:" + e2eConfig.GetVariable(OldAddonProviderUpgradeVersion)},
					}
				})
			})

			Context("upgrade from the latest version of v1beta1 to current, and scale workload clusters created in the old version", func() {
				capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
					return capi_e2e.ClusterctlUpgradeSpecInput{
						E2EConfig:                 e2eConfig,
						ClusterctlConfigPath:      clusterctlConfigPath,
						BootstrapClusterProxy:     bootstrapClusterProxy,
						ArtifactFolder:            artifactFolder,
						SkipCleanup:               skipCleanup,
						PreInit:                   getPreInitFunc(ctx),
						InitWithProvidersContract: "v1beta1",
						ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
							WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
						},
						InitWithKubernetesVersion:       e2eConfig.GetVariable(KubernetesVersionAPIUpgradeFrom),
						InitWithBinary:                  fmt.Sprintf("https://github.com/kubernetes-sigs/cluster-api/releases/download/%s/clusterctl-{OS}-{ARCH}", e2eConfig.GetVariable(LatestCAPIUpgradeVersion)),
						InitWithCoreProvider:            "cluster-api:" + e2eConfig.GetVariable(LatestCAPIUpgradeVersion),
						InitWithBootstrapProviders:      []string{"kubeadm:" + e2eConfig.GetVariable(LatestCAPIUpgradeVersion)},
						InitWithControlPlaneProviders:   []string{"kubeadm:" + e2eConfig.GetVariable(LatestCAPIUpgradeVersion)},
						InitWithInfrastructureProviders: []string{"azure:" + e2eConfig.GetVariable(LatestProviderUpgradeVersion)},
						InitWithAddonProviders:          []string{"helm:" + e2eConfig.GetVariable(LatestAddonProviderUpgradeVersion)},
					}
				})
			})
		})
	}

	Context("Running the workload cluster upgrade spec [K8s-Upgrade]", func() {
		capi_e2e.ClusterUpgradeConformanceSpec(ctx, func() capi_e2e.ClusterUpgradeConformanceSpecInput {
			return capi_e2e.ClusterUpgradeConformanceSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				SkipConformanceTests:  true,
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}
		})
	})

	Context("Running KCP upgrade in a HA cluster [K8s-Upgrade]", func() {
		capi_e2e.ClusterUpgradeConformanceSpec(context.TODO(), func() capi_e2e.ClusterUpgradeConformanceSpecInput {
			return capi_e2e.ClusterUpgradeConformanceSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     clusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				ControlPlaneMachineCount: ptr.To[int64](3),
				WorkerMachineCount:       ptr.To[int64](0),
				SkipCleanup:              skipCleanup,
				SkipConformanceTests:     true,
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}
		})
	})

	Context("Running KCP upgrade in a HA cluster using scale in rollout [K8s-Upgrade]", func() {
		capi_e2e.ClusterUpgradeConformanceSpec(context.TODO(), func() capi_e2e.ClusterUpgradeConformanceSpecInput {
			return capi_e2e.ClusterUpgradeConformanceSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     clusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				ControlPlaneMachineCount: ptr.To[int64](3),
				WorkerMachineCount:       ptr.To[int64](0),
				SkipCleanup:              skipCleanup,
				SkipConformanceTests:     true,
				Flavor:                   ptr.To("kcp-scale-in"),
				ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				},
			}
		})
	})
})

func getPreInitFunc(ctx context.Context) func(proxy framework.ClusterProxy) {
	return func(clusterProxy framework.ClusterProxy) {
		identityName := e2eConfig.GetVariable(ClusterIdentityName)
		Expect(os.Setenv(ClusterIdentityName, identityName)).To(Succeed())
	}
}
