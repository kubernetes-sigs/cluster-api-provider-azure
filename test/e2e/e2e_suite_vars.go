//go:build e2e
// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	kubesystem  = "kube-system"
	activitylog = "azure-activity-logs"
	nodesDir    = "nodes"
)

// Test suite flags
var (
	// configPath is the path to the e2e config file.
	configPath string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool

	// skipLogCollection prevents the log collection process from running.
	skipLogCollection bool
)

// Test suite global vars
var (
	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// bootstrapClusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	bootstrapClusterProvider bootstrap.ClusterProvider

	// bootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	bootstrapClusterProxy framework.ClusterProxy

	// kubetestConfigFilePath is the path to the kubetest configuration file
	kubetestConfigFilePath string

	// kubetestRepoListPath
	kubetestRepoListPath string

	// useCIArtifacts specifies whether or not to use the latest build from the main branch of the Kubernetes repository
	useCIArtifacts bool

	// usePRArtifacts specifies whether or not to use the build from a PR of the Kubernetes repository
	usePRArtifacts bool

	// cloudProviderAzurePath specifies the path to the cloud-provider-azure HelmChartProxy
	cloudProviderAzurePath string

	// cloudProviderAzureCIPath specifies the path to the cloud-provider-azure HelmChartProxy with CI artifacts enabled
	cloudProviderAzureCIPath string

	// calicoPath specifies the path to the calico HelmChartProxy
	calicoPath string

	// calicoIPv6Path specifies the path to the calico HelmChartProxy with IPv6
	calicoIPv6Path string

	// calicoDualStackPath specifies the path to the calico HelmChartProxy with dual stack
	calicoDualStackPath string

	// azureDiskCSIDriverPath specifies the path to the azure disk CSI driver HelmChartProxy
	azureDiskCSIDriverPath string
)
