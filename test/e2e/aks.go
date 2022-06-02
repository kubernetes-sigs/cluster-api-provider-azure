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
	infraexpv1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitForControlPlaneInitialized waits for the azure managed control plane to be initialized.
// This will be invoked by cluster api e2e framework.
func WaitForControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, result *clusterctl.ApplyClusterTemplateAndWaitResult) {
	client := input.ClusterProxy.GetClient()
	DiscoverAndWaitForControlPlaneInitialized(ctx, DiscoverAndWaitForControlPlaneMachinesInput{
		Lister:  client,
		Getter:  client,
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

// WaitForControlPlaneMachinesReady waits for the azure managed control plane to be ready.
// This will be invoked by cluster api e2e framework.
func WaitForControlPlaneMachinesReady(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, result *clusterctl.ApplyClusterTemplateAndWaitResult) {
	client := input.ClusterProxy.GetClient()
	DiscoverAndWaitForControlPlaneReady(ctx, DiscoverAndWaitForControlPlaneMachinesInput{
		Lister:  client,
		Getter:  client,
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

// DiscoverAndWaitForControlPlaneMachinesInput contains the fields the required for checking the status of azure managed control plane.
type DiscoverAndWaitForControlPlaneMachinesInput struct {
	Lister  framework.Lister
	Getter  framework.Getter
	Cluster *clusterv1.Cluster
}

// DiscoverAndWaitForControlPlaneInitialized gets the azure managed control plane associated with the cluster,
// and waits for at least one control plane machine to be up.
func DiscoverAndWaitForControlPlaneInitialized(ctx context.Context, input DiscoverAndWaitForControlPlaneMachinesInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoverAndWaitForControlPlaneInitialized")
	Expect(input.Lister).NotTo(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForControlPlaneInitialized")
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForControlPlaneInitialized")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	Expect(controlPlane).NotTo(BeNil())

	Logf("Waiting for the first control plane machine managed by %s/%s to be provisioned", controlPlane.Namespace, controlPlane.Name)
	WaitForAtLeastOneControlPlaneAndMachineToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
		Lister:       input.Lister,
		Getter:       input.Getter,
		ControlPlane: controlPlane,
		ClusterName:  input.Cluster.Name,
		Namespace:    input.Cluster.Namespace,
	}, intervals...)
}

// DiscoverAndWaitForControlPlaneReady gets the azure managed control plane associated with the cluster,
// and waits for all the control plane machines to be up.
func DiscoverAndWaitForControlPlaneReady(ctx context.Context, input DiscoverAndWaitForControlPlaneMachinesInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoverAndWaitForControlPlaneReady")
	Expect(input.Lister).NotTo(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForControlPlaneReady")
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForControlPlaneReady")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	Expect(controlPlane).NotTo(BeNil())

	Logf("Waiting for the first control plane machine managed by %s/%s to be provisioned", controlPlane.Namespace, controlPlane.Name)
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
func GetAzureManagedControlPlaneByCluster(ctx context.Context, input GetAzureManagedControlPlaneByClusterInput) *infraexpv1.AzureManagedControlPlane {
	controlPlaneList := &infraexpv1.AzureManagedControlPlaneList{}
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
	ControlPlane *infraexpv1.AzureManagedControlPlane
	ClusterName  string
	Namespace    string
}

// WaitForAtLeastOneControlPlaneAndMachineToExist waits for at least one control plane machine to be provisioned.
func WaitForAtLeastOneControlPlaneAndMachineToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	By("Waiting for at least one control plane node to exist")
	WaitForControlPlaneMachinesToExist(ctx, input, atLeastOne, intervals...)
}

// WaitForAllControlPlaneAndMachinesToExist waits for all control plane machines to be provisioned.
func WaitForAllControlPlaneAndMachinesToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	By("Waiting for all control plane nodes to exist")
	WaitForControlPlaneMachinesToExist(ctx, input, all, intervals...)
}

// controlPlaneReplicas represents the count of control plane machines.
type controlPlaneReplicas string

const (
	atLeastOne controlPlaneReplicas = "atLeastOne"
	all        controlPlaneReplicas = "all"
)

// value returns the integer equivalent of controlPlaneReplicas
func (r controlPlaneReplicas) value(mp *clusterv1exp.MachinePool) int {
	switch r {
	case atLeastOne:
		return 1
	case all:
		return int(*mp.Spec.Replicas)
	}
	return 0
}

// WaitForControlPlaneMachinesToExist waits for a certain number of control plane machines to be provisioned represented.
func WaitForControlPlaneMachinesToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, minReplicas controlPlaneReplicas, intervals ...interface{}) {
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

				ownerMachinePool := &clusterv1exp.MachinePool{}
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

	}, intervals...).Should(Equal(true), "System machine pools not ready")
}

// GetAKSKubernetesVersion gets the kubernetes version for AKS clusters.
func GetAKSKubernetesVersion(ctx context.Context, e2eConfig *clusterctl.E2EConfig) (string, error) {
	e2eAKSVersion := e2eConfig.GetVariable(AKSKubernetesVersion)

	location := e2eConfig.GetVariable(AzureLocation)

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	var maxVersion string
	if e2eAKSVersion == "latest" {
		maxVersion, err = GetLatestStableAKSKubernetesVersion(ctx, subscriptionID, location)
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
		return "", errors.Wrap(err, "GetSettingsFromEnvironment failed.")
	}
	authorizer, err := settings.GetAuthorizer()
	if err != nil {
		return "", errors.Wrap(err, "Failed to create an Authorizer.")
	}
	containerServiceClient := containerservice.NewContainerServicesClient(subscriptionID)
	containerServiceClient.Authorizer = authorizer
	result, err := containerServiceClient.ListOrchestrators(ctx, location, ManagedClustersResourceType)
	if err != nil {
		return "", errors.Wrap(err, "Failed to list Orchestrators.")
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
func GetLatestStableAKSKubernetesVersion(ctx context.Context, subscriptionID, location string) (string, error) {
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return "", errors.Wrap(err, "GetSettingsFromEnvironment failed.")
	}
	authorizer, err := settings.GetAuthorizer()
	if err != nil {
		return "", errors.Wrap(err, "Failed to create an Authorizer.")
	}
	containerServiceClient := containerservice.NewContainerServicesClient(subscriptionID)
	containerServiceClient.Authorizer = authorizer
	result, err := containerServiceClient.ListOrchestrators(ctx, location, ManagedClustersResourceType)
	if err != nil {
		return "", errors.Wrap(err, "Failed to list Orchestrators.")
	}

	var orchestratorversions []string
	var foundWorkingVersion bool
	var orchVersion string
	var maxVersion string

	for _, o := range *result.Orchestrators {
		orchVersion = *o.OrchestratorVersion
		// semver comparisons require a "v" prefix
		if orchVersion[:1] != "v" && o.IsPreview == nil {
			orchVersion = fmt.Sprintf("v%s", *o.OrchestratorVersion)
		}
		orchestratorversions = append(orchestratorversions, orchVersion)
	}
	semver.Sort(orchestratorversions)
	maxVersion = orchestratorversions[len(orchestratorversions)-1]
	if semver.IsValid(maxVersion) {
		foundWorkingVersion = true
	}
	if !foundWorkingVersion {
		return "", errors.New(fmt.Sprintf("Latest stable AKS version not found."))
	}
	return maxVersion, nil
}
