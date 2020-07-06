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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Running the Cluster API E2E tests", func() {
	var (
		cloudConfig string
	)

	BeforeEach(func() {
		rgName := fmt.Sprintf("capz-e2e-%s", util.RandomString(6))
		Expect(os.Setenv(AzureResourceGroup, rgName)).NotTo(HaveOccurred())
		Expect(os.Setenv(AzureVNetName, fmt.Sprintf("%s-vnet", rgName))).NotTo(HaveOccurred())

		var err error
		cloudConfig, err = getCloudProviderConfig(rgName)
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Setenv(AzureJson, cloudConfig)).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		redactLogs()
		Expect(os.Unsetenv(AzureResourceGroup)).NotTo(HaveOccurred())
		Expect(os.Unsetenv(AzureVNetName)).NotTo(HaveOccurred())
		Expect(os.Unsetenv(AzureJson)).NotTo(HaveOccurred())
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

	// TODO: fix and enable
	//Context("Running the KCP upgrade spec", func() {
	//	capi_e2e.KCPUpgradeSpec(context.TODO(), func() capi_e2e.KCPUpgradeSpecInput {
	//		return capi_e2e.KCPUpgradeSpecInput{
	//			E2EConfig:             e2eConfig,
	//			ClusterctlConfigPath:  clusterctlConfigPath,
	//			BootstrapClusterProxy: bootstrapClusterProxy,
	//			ArtifactFolder:        artifactFolder,
	//			SkipCleanup:           skipCleanup,
	//		}
	//	})
	//})

	Context("Running the MachineDeployment upgrade spec", func() {
		capi_e2e.MachineDeploymentUpgradesSpec(context.TODO(), func() capi_e2e.MachineDeploymentUpgradesSpecInput {
			return capi_e2e.MachineDeploymentUpgradesSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
			}
		})
	})

	// TODO: fix and enable
	// Context("Running the self-hosted spec", func() {
	// 	capi_e2e.SelfHostedSpec(context.TODO(), func() capi_e2e.SelfHostedSpecInput {
	// 		return capi_e2e.SelfHostedSpecInput{
	// 			E2EConfig:             e2eConfig,
	// 			ClusterctlConfigPath:  clusterctlConfigPath,
	// 			BootstrapClusterProxy: bootstrapClusterProxy,
	// 			ArtifactFolder:        artifactFolder,
	// 			SkipCleanup:           skipCleanup,
	// 		}
	// 	})
	// })
})
