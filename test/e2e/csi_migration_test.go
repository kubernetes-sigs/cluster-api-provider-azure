//go:build e2e
// +build e2e

/*
Copyright 2022 The Kubernetes Authors.

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
	"strings"
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
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("[K8s-Upgrade] Running the CSI migration tests", func() {
	var (
		ctx                      = context.TODO()
		specName                 = "csi-migration"
		namespace                *corev1.Namespace
		cancelWatches            context.CancelFunc
		result                   *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName              string
		clusterNamePrefix        string
		additionalCleanup        func()
		specTimes                = map[string]time.Time{}
		preCSIKubernetesVersion  string
		postCSIKubernetesVersion string
	)

	BeforeEach(func() {
		logCheckpoint(specTimes)
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(e2eConfig).NotTo(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0o755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersionUpgradeFrom))
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersionUpgradeTo))

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

		cleanInput := cleanupInput{
			SpecName:          specName,
			Cluster:           result.Cluster,
			ClusterProxy:      bootstrapClusterProxy,
			Namespace:         namespace,
			CancelWatches:     cancelWatches,
			IntervalsGetter:   e2eConfig.GetIntervals,
			SkipCleanup:       skipCleanup,
			SkipLogCollection: skipLogCollection,
			AdditionalCleanup: additionalCleanup,
			ArtifactFolder:    artifactFolder,
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

	Context("[CSI Migration] Running CSI migration test", func() {
		BeforeEach(func() {
			preCSIKubernetesVersion = e2eConfig.GetVariable(capi_e2e.KubernetesVersionUpgradeFrom)
			postCSIKubernetesVersion = e2eConfig.GetVariable(capi_e2e.KubernetesVersionUpgradeTo)
			By(fmt.Sprintf("Log: From %s TO %s", preCSIKubernetesVersion, postCSIKubernetesVersion))
			if !strings.Contains(preCSIKubernetesVersion, "1.22") && !strings.Contains(postCSIKubernetesVersion, "1.23") {
				Skip(fmt.Sprintf("Skipping CSI migration test as upgrade from version is %s and upgrade to "+
					"version is %s", preCSIKubernetesVersion, postCSIKubernetesVersion))
			}
		})

		Context("CSI=internal CCM=internal AzureDiskCSIMigration=false: upgrade to v1.23", func() {
			It("should create volumes dynamically with intree cloud provider", func() {
				By("Creating workload cluster v1.22 using user-assigned identity")
				clusterName = getClusterName(clusterNamePrefix, "intree-providers")
				configCluster := defaultConfigCluster(clusterName, namespace.Name)
				configCluster.KubernetesVersion = preCSIKubernetesVersion
				configCluster.WorkerMachineCount = pointer.Int64Ptr(3)
				configCluster.Flavor = "user-assigned-managed-identity"
				// Create the workload cluster with k8s version preCSIKubernetesVersion
				createClusterWithControlPlaneWaiters(ctx, configCluster, clusterctl.ControlPlaneWaiters{WaitForControlPlaneInitialized: EnsureControlPlaneInitialized}, result)
				// create a stateful deployment and confirm it is working
				By("Create a stateful deployment and verify it is working.")
				deployment := AzureDiskCSISpec(ctx, func() AzureDiskCSISpecInput {
					return AzureDiskCSISpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						DriverType:            DriverTypeInternal,
					}
				})

				// Upgrade the workload cluster
				By("Upgrade the workload cluster to v1.23")
				configCluster.KubernetesVersion = postCSIKubernetesVersion
				configCluster.Flavor = "azurediskcsi-migration-off"
				upgradedCluster, kcp := createClusterWithControlPlaneWaiters(ctx, configCluster, clusterctl.ControlPlaneWaiters{}, result)
				// Wait for control plane to be upgraded successfully
				By("Waiting for control-plane machines to have the upgraded kubernetes version")
				framework.WaitForControlPlaneMachinesToBeUpgraded(ctx, framework.WaitForControlPlaneMachinesToBeUpgradedInput{
					Lister:                   bootstrapClusterProxy.GetClient(),
					Cluster:                  upgradedCluster,
					MachineCount:             int(*kcp.Spec.Replicas),
					KubernetesUpgradeVersion: postCSIKubernetesVersion,
				}, e2eConfig.GetIntervals(specName, "wait-controlplane-upgrade")...)

				// Check if the stateful deployment created in v1.22 is still healthy after the upgrade
				By("Checking v1.22 stateful deployment still healthy after the upgrade")
				clusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)
				Expect(clusterProxy).NotTo(BeNil())
				clientset := clusterProxy.GetClientSet()
				Expect(clientset).NotTo(BeNil())
				waitForDeploymentAvailable(ctx, deployment, clientset, "check-stateful-deployment-after-upgrade")

				// Create a new stateful deployment in the upgraded cluster and verify it is running
				By("Create a new stateful deployment on upgraded k8s of v1.23")
				AzureDiskCSISpec(ctx, func() AzureDiskCSISpecInput {
					return AzureDiskCSISpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						DriverType:            DriverTypeInternal,
					}
				})
				By("CSI=internal CCM=internal AzureDiskCSIMigration=false: upgrade to v1.23 PASSED!!")
			})
		})

		Context("CSI=external CCM=internal AzureDiskCSIMigration=true: upgrade to v1.23", func() {
			It("should create volumes dynamically with intree cloud provider", func() {
				By("Creating workload cluster v1.22 using user-assigned identity")
				clusterName = getClusterName(clusterNamePrefix, "external-azurediskcsi-driver")
				configCluster := defaultConfigCluster(clusterName, namespace.Name)
				configCluster.KubernetesVersion = preCSIKubernetesVersion
				configCluster.WorkerMachineCount = pointer.Int64Ptr(3)
				configCluster.Flavor = "user-assigned-managed-identity"
				// Create the workload cluster with k8s version PreCSIKubernetesVer
				createClusterWithControlPlaneWaiters(ctx, configCluster, clusterctl.ControlPlaneWaiters{WaitForControlPlaneInitialized: EnsureControlPlaneInitialized}, result)
				// create a stateful deployment and confirm it is working
				By("Create a stateful deployment and verify it is working.")
				deployment := AzureDiskCSISpec(ctx, func() AzureDiskCSISpecInput {
					return AzureDiskCSISpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						DriverType:            DriverTypeInternal,
					}
				})

				// Upgrade the workload cluster
				By("Upgrade the workload cluster to v1.23")
				configCluster.KubernetesVersion = postCSIKubernetesVersion
				// This flavour uses external csi driver and in tree cloud provider
				configCluster.Flavor = "external-azurediskcsi-driver"
				upgradedCluster, kcp := createClusterWithControlPlaneWaiters(ctx, configCluster, clusterctl.ControlPlaneWaiters{}, result)
				// Wait for control plane to be upgraded successfully
				By("Waiting for control-plane machines to have the upgraded kubernetes version")
				framework.WaitForControlPlaneMachinesToBeUpgraded(ctx, framework.WaitForControlPlaneMachinesToBeUpgradedInput{
					Lister:                   bootstrapClusterProxy.GetClient(),
					Cluster:                  upgradedCluster,
					MachineCount:             int(*kcp.Spec.Replicas),
					KubernetesUpgradeVersion: postCSIKubernetesVersion,
				}, e2eConfig.GetIntervals(specName, "wait-controlplane-upgrade")...)

				// Check if the stateful deployment created in v1.22 is still healthy after the upgrade
				By("Checking v1.22 stateful deployment still healthy after the upgrade")
				clusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)
				Expect(clusterProxy).NotTo(BeNil())
				clientset := clusterProxy.GetClientSet()
				Expect(clientset).NotTo(BeNil())
				waitForDeploymentAvailable(ctx, deployment, clientset, "check-stateful-deployment-after-upgrade")

				// Create a new stateful deployment in the upgraded cluster and verify it is running
				By("Create a new stateful deployment on upgraded k8s of v1.23")
				AzureDiskCSISpec(ctx, func() AzureDiskCSISpecInput {
					return AzureDiskCSISpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
				By("CSI=external CCM=internal AzureDiskCSIMigration=true: upgrade to v1.23 PASSED!!")
			})
		})

		Context("CSI=external CCM=external AzureDiskCSIMigration=true: upgrade to v1.23", func() {
			It("should create volumes dynamically with intree cloud provider", func() {
				By("Creating workload cluster v1.22 using user-assigned identity")
				clusterName = getClusterName(clusterNamePrefix, "external-providers")
				configCluster := defaultConfigCluster(clusterName, namespace.Name)
				configCluster.KubernetesVersion = preCSIKubernetesVersion
				configCluster.WorkerMachineCount = pointer.Int64Ptr(3)
				configCluster.Flavor = "user-assigned-managed-identity"
				// Create the workload cluster with k8s version PreCSIKubernetesVer
				createClusterWithControlPlaneWaiters(ctx, configCluster, clusterctl.ControlPlaneWaiters{WaitForControlPlaneInitialized: EnsureControlPlaneInitialized}, result)
				// create a stateful deployment and confirm it is working
				By("Create a stateful deployment and verify it is working.")
				deployment := AzureDiskCSISpec(ctx, func() AzureDiskCSISpecInput {
					return AzureDiskCSISpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
						DriverType:            DriverTypeInternal,
					}
				})

				// Upgrade the workload cluster
				By("Upgrade the workload cluster to v1.23")
				configCluster.KubernetesVersion = postCSIKubernetesVersion
				configCluster.Flavor = "external-cloud-provider"
				upgradedCluster, kcp := createClusterWithControlPlaneWaiters(ctx, configCluster, clusterctl.ControlPlaneWaiters{}, result)

				// Wait for control plane to be upgraded successfully
				By("Waiting for control-plane machines to have the upgraded kubernetes version")
				framework.WaitForControlPlaneMachinesToBeUpgraded(ctx, framework.WaitForControlPlaneMachinesToBeUpgradedInput{
					Lister:                   bootstrapClusterProxy.GetClient(),
					Cluster:                  upgradedCluster,
					MachineCount:             int(*kcp.Spec.Replicas),
					KubernetesUpgradeVersion: postCSIKubernetesVersion,
					// install helm charts
				}, e2eConfig.GetIntervals(specName, "wait-controlplane-upgrade")...)

				// Check if the stateful deployment created in v1.22 is still healthy after the upgrade
				By("Checking v1.22 stateful deployment still healthy after the upgrade")
				clusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)
				Expect(clusterProxy).NotTo(BeNil())
				clientset := clusterProxy.GetClientSet()
				Expect(clientset).NotTo(BeNil())
				waitForDeploymentAvailable(ctx, deployment, clientset, "check-stateful-deployment-after-upgrade")

				// Create a new stateful deployment in the upgraded cluster and verify it is running
				By("Create a new stateful deployment on upgraded k8s of v1.23")
				AzureDiskCSISpec(ctx, func() AzureDiskCSISpecInput {
					return AzureDiskCSISpecInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						SkipCleanup:           skipCleanup,
					}
				})
				By("CSI=external CCM=external AzureDiskCSIMigration=true: upgrade to v1.23 PASSED!!")
			})
		})
	})
})
