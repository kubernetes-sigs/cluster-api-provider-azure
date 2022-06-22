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

	"github.com/blang/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	e2e_namespace "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/namespace"
	clusterctl "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

const (
	IdentitySecretName = "cluster-identity-secret"
)

var _ = Describe("Running the Cluster API E2E tests", func() {
	var (
		ctx               = context.TODO()
		identityNamespace *corev1.Namespace
	)
	BeforeEach(func() {
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.CNIPath))
		rgName := fmt.Sprintf("capz-e2e-%s", util.RandomString(6))
		Expect(os.Setenv(AzureResourceGroup, rgName)).To(Succeed())
		Expect(os.Setenv(AzureVNetName, fmt.Sprintf("%s-vnet", rgName))).To(Succeed())

		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersionUpgradeFrom))
		v, err := semver.ParseTolerant(e2eConfig.GetVariable(capi_e2e.KubernetesVersionUpgradeFrom))
		Expect(err).NotTo(HaveOccurred())
		// Opt into Windows for versions greater than or equal to 1.22
		if v.GTE(semver.MustParse("1.22.0")) {
			Expect(os.Setenv("WINDOWS_WORKER_MACHINE_COUNT", "2")).To(Succeed())
			Expect(os.Setenv("K8S_FEATURE_GATES", "WindowsHostProcessContainers=true")).To(Succeed())
		}

		clientset := bootstrapClusterProxy.GetClientSet()
		Expect(clientset).NotTo(BeNil())
		ns := fmt.Sprintf("capz-e2e-identity-%s", util.RandomString(6))

		identityNamespace, err = e2e_namespace.Create(ctx, clientset, ns, map[string]string{})
		Expect(err).NotTo(HaveOccurred())

		spClientSecret := os.Getenv(AzureClientSecret)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      IdentitySecretName,
				Namespace: identityNamespace.Name,
				Labels: map[string]string{
					clusterctl.ClusterctlMoveHierarchyLabelName: "true",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"clientSecret": []byte(spClientSecret)},
		}
		err = bootstrapClusterProxy.GetClient().Create(ctx, secret)
		Expect(err).NotTo(HaveOccurred())

		identityName := e2eConfig.GetVariable(ClusterIdentityName)
		Expect(os.Setenv(ClusterIdentityName, identityName)).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretName, IdentitySecretName)).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretNamespace, identityNamespace.Name)).To(Succeed())
	})

	AfterEach(func() {
		redactLogs()

		Expect(os.Unsetenv(AzureResourceGroup)).To(Succeed())
		Expect(os.Unsetenv(AzureVNetName)).To(Succeed())
		Expect(os.Unsetenv(ClusterIdentityName)).To(Succeed())
		Expect(os.Unsetenv(ClusterIdentitySecretName)).To(Succeed())
		Expect(os.Unsetenv(ClusterIdentitySecretNamespace)).To(Succeed())
	})

	Context("Running the quick-start spec", func() {
		capi_e2e.QuickStartSpec(context.TODO(), func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
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
			}
		})
	})

	if os.Getenv("LOCAL_ONLY") != "true" {
		Context("Running the self-hosted spec", func() {
			SelfHostedSpec(context.TODO(), func() SelfHostedSpecInput {
				return SelfHostedSpecInput{
					E2EConfig:             e2eConfig,
					ClusterctlConfigPath:  clusterctlConfigPath,
					BootstrapClusterProxy: bootstrapClusterProxy,
					ArtifactFolder:        artifactFolder,
					SkipCleanup:           skipCleanup,
				}
			})
		})
	}

	Context("Should successfully remediate unhealthy machines with MachineHealthCheck", func() {
		capi_e2e.MachineRemediationSpec(context.TODO(), func() capi_e2e.MachineRemediationSpecInput {
			return capi_e2e.MachineRemediationSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
			}
		})
	})

	Context("Should adopt up-to-date control plane Machines without modification", func() {
		capi_e2e.KCPAdoptionSpec(context.TODO(), func() capi_e2e.KCPAdoptionSpecInput {
			return capi_e2e.KCPAdoptionSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
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
			}
		})
	})

	if os.Getenv("LOCAL_ONLY") != "true" {
		Context("API Version Upgrade", func() {

			Context("upgrade from v1alpha4 to v1beta1, and scale workload clusters created in v1alpha4", func() {
				BeforeEach(func() {
					// Unset resource group and vnet env variables, since we capi test creates 2 clusters,
					// and will result in both the clusters using the same vnet and resource group.
					Expect(os.Unsetenv(AzureResourceGroup)).To(Succeed())
					Expect(os.Unsetenv(AzureVNetName)).To(Succeed())

					// Unset windows specific variables
					Expect(os.Unsetenv("WINDOWS_WORKER_MACHINE_COUNT")).To(Succeed())
					Expect(os.Unsetenv("K8S_FEATURE_GATES")).To(Succeed())
				})
				capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
					return capi_e2e.ClusterctlUpgradeSpecInput{
						E2EConfig:                 e2eConfig,
						ClusterctlConfigPath:      clusterctlConfigPath,
						BootstrapClusterProxy:     bootstrapClusterProxy,
						ArtifactFolder:            artifactFolder,
						SkipCleanup:               skipCleanup,
						InitWithProvidersContract: "v1alpha4",
						InitWithBinary:            "https://github.com/kubernetes-sigs/cluster-api/releases/download/v0.4.7/clusterctl-{OS}-{ARCH}",
						PreInit:                   getPreInitFunc(ctx),
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
				ControlPlaneMachineCount: pointer.Int64(3),
				WorkerMachineCount:       pointer.Int64(0),
				SkipCleanup:              skipCleanup,
				SkipConformanceTests:     true,
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
				ControlPlaneMachineCount: pointer.Int64(3),
				WorkerMachineCount:       pointer.Int64(0),
				SkipCleanup:              skipCleanup,
				SkipConformanceTests:     true,
				Flavor:                   pointer.String("kcp-scale-in"),
			}
		})
	})
})

func getPreInitFunc(ctx context.Context) func(proxy framework.ClusterProxy) {
	return func(clusterProxy framework.ClusterProxy) {
		spClientSecret := os.Getenv(AzureClientSecret)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      IdentitySecretName,
				Namespace: "default",
				Labels: map[string]string{
					clusterctl.ClusterctlMoveHierarchyLabelName: "true",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"clientSecret": []byte(spClientSecret)},
		}
		err := clusterProxy.GetClient().Create(ctx, secret)
		Expect(err).NotTo(HaveOccurred())

		identityName := e2eConfig.GetVariable(ClusterIdentityName)
		Expect(os.Setenv(ClusterIdentityName, identityName)).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretName, IdentitySecretName)).To(Succeed())
		Expect(os.Setenv(ClusterIdentitySecretNamespace, "default")).To(Succeed())
	}
}
