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
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	"k8s.io/apimachinery/pkg/types"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DiscoverAndWaitForAKSControlPlaneInput contains the fields the required for checking the status of azure managed control plane.
type DiscoverAndWaitForAKSControlPlaneInput struct {
	Lister  framework.Lister
	Getter  framework.Getter
	Cluster *clusterv1.Cluster
}

// WaitForAKSControlPlaneInitialized waits for the Azure managed control plane to be initialized.
// This will be invoked by cluster api e2e framework.
func WaitForAKSControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, result *clusterctl.ApplyClusterTemplateAndWaitResult) {
	client := input.ClusterProxy.GetClient()
	DiscoverAndWaitForAKSControlPlaneInitialized(ctx, DiscoverAndWaitForAKSControlPlaneInput{
		Lister:  client,
		Getter:  client,
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

// WaitForAKSControlPlaneReady waits for the azure managed control plane to be ready.
// This will be invoked by cluster api e2e framework.
func WaitForAKSControlPlaneReady(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, result *clusterctl.ApplyClusterTemplateAndWaitResult) {
	client := input.ClusterProxy.GetClient()
	DiscoverAndWaitForAKSControlPlaneReady(ctx, DiscoverAndWaitForAKSControlPlaneInput{
		Lister:  client,
		Getter:  client,
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

// DiscoverAndWaitForAKSControlPlaneInitialized gets the Azure managed control plane associated with the cluster
// and waits for at least one machine in the "system" node pool to exist.
func DiscoverAndWaitForAKSControlPlaneInitialized(ctx context.Context, input DiscoverAndWaitForAKSControlPlaneInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoverAndWaitForAKSControlPlaneInitialized")
	Expect(input.Lister).NotTo(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForAKSControlPlaneInitialized")
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForAKSControlPlaneInitialized")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	Expect(controlPlane).NotTo(BeNil())

	Logf("Waiting for the first AKS machine in the %s/%s 'system' node pool to exist", controlPlane.Namespace, controlPlane.Name)
	WaitForAtLeastOneSystemNodePoolMachineToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
		Lister:       input.Lister,
		Getter:       input.Getter,
		ControlPlane: controlPlane,
		ClusterName:  input.Cluster.Name,
		Namespace:    input.Cluster.Namespace,
	}, intervals...)
}

// DiscoverAndWaitForAKSControlPlaneReady gets the Azure managed control plane associated with the cluster
// and waits for all the machines in the 'system' node pool to exist.
func DiscoverAndWaitForAKSControlPlaneReady(ctx context.Context, input DiscoverAndWaitForAKSControlPlaneInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoverAndWaitForAKSControlPlaneReady")
	Expect(input.Lister).NotTo(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForAKSControlPlaneReady")
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForAKSControlPlaneReady")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	Expect(controlPlane).NotTo(BeNil())

	Logf("Waiting for all AKS machines in the %s/%s 'system' node pool to exist", controlPlane.Namespace, controlPlane.Name)
	WaitForAllControlPlaneAndMachinesToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
		Lister:       input.Lister,
		Getter:       input.Getter,
		ControlPlane: controlPlane,
		ClusterName:  input.Cluster.Name,
		Namespace:    input.Cluster.Namespace,
	}, intervals...)
}

// GetAzureManagedControlPlaneByClusterInput contains the fields the required for fetching the azure managed control plane.
type GetAzureManagedControlPlaneByClusterInput struct {
	Lister      framework.Lister
	ClusterName string
	Namespace   string
}

// GetAzureManagedControlPlaneByCluster returns the AzureManagedControlPlane object for a cluster.
// Important! this method relies on labels that are created by the CAPI controllers during the first reconciliation, so
// it is necessary to ensure this is already happened before calling it.
func GetAzureManagedControlPlaneByCluster(ctx context.Context, input GetAzureManagedControlPlaneByClusterInput) *infrav1exp.AzureManagedControlPlane {
	controlPlaneList := &infrav1exp.AzureManagedControlPlaneList{}
	Expect(input.Lister.List(ctx, controlPlaneList, byClusterOptions(input.ClusterName, input.Namespace)...)).To(Succeed(), "Failed to list AzureManagedControlPlane object for Cluster %s/%s", input.Namespace, input.ClusterName)
	Expect(len(controlPlaneList.Items)).NotTo(BeNumerically(">", 1), "Cluster %s/%s should not have more than 1 AzureManagedControlPlane object", input.Namespace, input.ClusterName)
	if len(controlPlaneList.Items) == 1 {
		return &controlPlaneList.Items[0]
	}
	return nil
}

// WaitForControlPlaneAndMachinesReadyInput contains the fields required for checking the status of azure managed control plane machines.
type WaitForControlPlaneAndMachinesReadyInput struct {
	Lister       framework.Lister
	Getter       framework.Getter
	ControlPlane *infrav1exp.AzureManagedControlPlane
	ClusterName  string
	Namespace    string
}

// WaitForAtLeastOneSystemNodePoolMachineToExist waits for at least one machine in the "system" node pool to exist.
func WaitForAtLeastOneSystemNodePoolMachineToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	By("Waiting for at least one node to exist in the 'system' node pool")
	WaitForAKSSystemNodePoolMachinesToExist(ctx, input, atLeastOne, intervals...)
}

// WaitForAllControlPlaneAndMachinesToExist waits for all machines in the "system" node pool to exist.
func WaitForAllControlPlaneAndMachinesToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	By("Waiting for all nodes to exist in the 'system' node pool")
	WaitForAKSSystemNodePoolMachinesToExist(ctx, input, all, intervals...)
}

// controlPlaneReplicas represents the count of control plane machines.
type controlPlaneReplicas string

const (
	atLeastOne controlPlaneReplicas = "atLeastOne"
	all        controlPlaneReplicas = "all"
)

// value returns the integer equivalent of controlPlaneReplicas
func (r controlPlaneReplicas) value(mp *expv1.MachinePool) int {
	switch r {
	case atLeastOne:
		return 1
	case all:
		return int(*mp.Spec.Replicas)
	}
	return 0
}

// WaitForAKSSystemNodePoolMachinesToExist waits for a certain number of machines in the "system" node pool to exist.
func WaitForAKSSystemNodePoolMachinesToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, minReplicas controlPlaneReplicas, intervals ...interface{}) {
	Eventually(func() bool {
		opt1 := client.InNamespace(input.Namespace)
		opt2 := client.MatchingLabels(map[string]string{
			infrav1exp.LabelAgentPoolMode: string(infrav1exp.NodePoolModeSystem),
			clusterv1.ClusterLabelName:    input.ClusterName,
		})

		ammpList := &infrav1exp.AzureManagedMachinePoolList{}

		if err := input.Lister.List(ctx, ammpList, opt1, opt2); err != nil {
			LogWarningf("Failed to get machinePool: %+v", err)
			return false
		}

		for _, pool := range ammpList.Items {
			// Fetch the owning MachinePool.
			for _, ref := range pool.OwnerReferences {
				if ref.Kind != "MachinePool" {
					continue
				}

				ownerMachinePool := &expv1.MachinePool{}
				if err := input.Getter.Get(ctx, types.NamespacedName{Namespace: input.Namespace, Name: ref.Name},
					ownerMachinePool); err != nil {
					LogWarningf("Failed to get machinePool: %+v", err)
					return false
				}
				if len(ownerMachinePool.Status.NodeRefs) >= minReplicas.value(ownerMachinePool) {
					return true
				}
			}
		}

		return false
	}, intervals...).Should(Equal(true), "System machine pools not detected")
}

// GetAKSKubernetesVersion gets the kubernetes version for AKS clusters.
func GetAKSKubernetesVersion(ctx context.Context, e2eConfig *clusterctl.E2EConfig) (string, error) {
	e2eAKSVersion := e2eConfig.GetVariable(AKSKubernetesVersion)
	e2eAKSMaxVersion := e2eConfig.GetVariable(AKSKubernetesMaxVersion)

	location := e2eConfig.GetVariable(AzureLocation)

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	var maxVersion string
	if e2eAKSVersion == "latest" {
		maxVersion, err = GetLatestStableAKSKubernetesVersion(ctx, subscriptionID, location, e2eAKSMaxVersion)
		Expect(err).NotTo(HaveOccurred())
	} else {
		maxVersion, err = GetWorkingAKSKubernetesVersion(ctx, subscriptionID, location, e2eAKSVersion)
		Expect(err).NotTo(HaveOccurred())
	}

	return maxVersion, nil
}

// byClusterOptions returns a set of ListOptions that will identify all the objects belonging to a Cluster.
func byClusterOptions(name, namespace string) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterLabelName: name,
		},
	}
}

// GetWorkingAKSKubernetesVersion returns an available Kubernetes version of AKS given a desired semver version, if possible.
// If the desired version is available, we return it.
// If the desired version is not available, we check for any available patch version using desired version's Major.Minor semver release.
// If no versions are available in the desired version's Major.Minor semver release, we return an error.
func GetWorkingAKSKubernetesVersion(ctx context.Context, subscriptionID, location, version string) (string, error) {
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return "", errors.Wrap(err, "failed to get settings from environment")
	}
	authorizer, err := settings.GetAuthorizer()
	if err != nil {
		return "", errors.Wrap(err, "failed to create an Authorizer")
	}
	containerServiceClient := containerservice.NewContainerServicesClient(subscriptionID)
	containerServiceClient.Authorizer = authorizer
	result, err := containerServiceClient.ListOrchestrators(ctx, location, ManagedClustersResourceType)
	if err != nil {
		return "", errors.Wrap(err, "failed to list Orchestrators")
	}

	var latestStableVersionDesired bool
	// We're not doing much input validation here,
	// we assume that if the prefix is 'stable-' that the remainder of the string is in the format <Major>.<Minor>
	if isStableVersion, _ := validateStableReleaseString(version); isStableVersion {
		latestStableVersionDesired = true
		// Form a fully valid semver version @ the initial patch release (".0")
		version = fmt.Sprintf("%s.0", version[7:])
	}

	// semver comparisons below require a "v" prefix
	if version[:1] != "v" {
		version = fmt.Sprintf("v%s", version)
	}
	// Create a var of the patch ".0" equivalent of the inputted version
	baseVersion := fmt.Sprintf("%s.0", semver.MajorMinor(version))
	maxVersion := fmt.Sprintf("%s.0", semver.MajorMinor(version))
	var foundWorkingVersion bool
	for _, o := range *result.Orchestrators {
		orchVersion := *o.OrchestratorVersion
		// semver comparisons require a "v" prefix
		if orchVersion[:1] != "v" {
			orchVersion = fmt.Sprintf("v%s", *o.OrchestratorVersion)
		}
		// if the inputted version matches with an available AKS version we can return immediately
		if orchVersion == version && !latestStableVersionDesired {
			return version, nil
		}

		// or, keep track of the highest aks version for a given major.minor
		if semver.MajorMinor(orchVersion) == semver.MajorMinor(maxVersion) && semver.Compare(orchVersion, maxVersion) >= 0 {
			maxVersion = orchVersion
			foundWorkingVersion = true
		}
	}

	// This means there is no version supported by AKS for this major.minor
	if !foundWorkingVersion {
		return "", errors.New(fmt.Sprintf("No AKS versions found for %s", semver.MajorMinor(baseVersion)))
	}

	return maxVersion, nil
}

// GetLatestStableAKSKubernetesVersion returns the latest stable available Kubernetes version of AKS.
func GetLatestStableAKSKubernetesVersion(ctx context.Context, subscriptionID, location, maxVersion string) (string, error) {
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return "", errors.Wrap(err, "failed to get settings from environment")
	}
	authorizer, err := settings.GetAuthorizer()
	if err != nil {
		return "", errors.Wrap(err, "failed to create an Authorizer")
	}
	containerServiceClient := containerservice.NewContainerServicesClient(subscriptionID)
	containerServiceClient.Authorizer = authorizer
	result, err := containerServiceClient.ListOrchestrators(ctx, location, ManagedClustersResourceType)
	if err != nil {
		return "", errors.Wrap(err, "failed to list Orchestrators")
	}

	maxVersionFound, found := getMaxVersion(*result.Orchestrators, maxVersion)
	if !found {
		return "", errors.New("latest stable AKS version not found")
	}
	return maxVersionFound, nil
}

func getMaxVersion(orchestrators []containerservice.OrchestratorVersionProfile, max string) (string, bool) {
	var foundWorkingVersion bool
	var maxVersionFound string
	var rationalizedOrchestrators []string
	var maxVersion string
	var err error
	if max != "" {
		maxVersion, err = semverPrependV(max)
		if err != nil {
			return "", false
		}
	}
	for _, o := range orchestrators {
		ver, err := semverPrependV(*o.OrchestratorVersion)
		if err != nil {
			return "", false
		}
		rationalizedOrchestrators = append(rationalizedOrchestrators, ver)
	}
	for _, o := range rationalizedOrchestrators {
		if maxVersion == "" ||
			semver.MajorMinor(semver.Canonical(maxVersion)) == semver.MajorMinor(o) ||
			semver.Compare(o, maxVersion) <= 0 {
			if semver.Compare(o, maxVersionFound) == 1 {
				maxVersionFound = o
			}
		}
	}
	maxVersionFound, err = semverPrependV(maxVersionFound)
	if err != nil {
		return "", false
	}
	if semver.IsValid(maxVersionFound) {
		foundWorkingVersion = true
	}
	return maxVersionFound, foundWorkingVersion
}

func semverPrependV(version string) (string, error) {
	ret := version
	if version != "" {
		if version[:1] != "v" {
			ret = "v" + version
		}
	}
	if !semver.IsValid(semver.Canonical(ret)) {
		return "", errors.New("not a valid semver")
	}
	return ret, nil
}
