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
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/mod/semver"
	"k8s.io/apimachinery/pkg/types"
	infraexpv1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
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
// and waits for atleast one control plane machine to be up.
func DiscoverAndWaitForControlPlaneInitialized(ctx context.Context, input DiscoverAndWaitForControlPlaneMachinesInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoverAndWaitForControlPlaneInitialized")
	Expect(input.Lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForControlPlaneInitialized")
	Expect(input.Cluster).ToNot(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForControlPlaneInitialized")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	Expect(controlPlane).ToNot(BeNil())

	Logf("Waiting for the first control plane machine managed by %s/%s to be provisioned", controlPlane.Namespace, controlPlane.Name)
	WaitForAtLeastOneControlPlaneAndMachineToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
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
	Expect(input.Lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForControlPlaneReady")
	Expect(input.Cluster).ToNot(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForControlPlaneReady")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	Expect(controlPlane).ToNot(BeNil())

	Logf("Waiting for the first control plane machine managed by %s/%s to be provisioned", controlPlane.Namespace, controlPlane.Name)
	WaitForAllControlPlaneAndMachinesToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
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
	Expect(len(controlPlaneList.Items)).ToNot(BeNumerically(">", 1), "Cluster %s/%s should not have more than 1 AzureManagedControlPlane object", input.Namespace, input.ClusterName)
	if len(controlPlaneList.Items) == 1 {
		return &controlPlaneList.Items[0]
	}
	return nil
}

// WaitForControlPlaneAndMachinesReadyInput contains the fields required for checking the status of azure managed control plane machines.
type WaitForControlPlaneAndMachinesReadyInput struct {
	Getter       framework.Getter
	ControlPlane *infraexpv1.AzureManagedControlPlane
	ClusterName  string
	Namespace    string
}

// WaitForAtLeastOneControlPlaneAndMachineToExist waits for atleast one control plane machine to be provisioned.
func WaitForAtLeastOneControlPlaneAndMachineToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	By("Waiting for atleast one control plane node to exist")
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
	Eventually(func() (bool, error) {
		controlPlaneMachinePool := &clusterv1exp.MachinePool{}
		if err := input.Getter.Get(ctx, types.NamespacedName{Namespace: input.Namespace, Name: input.ControlPlane.Spec.DefaultPoolRef.Name},
			controlPlaneMachinePool); err != nil {
			Logf("Failed to get machinePool: %+v", err)
			return false, err
		}
		return len(controlPlaneMachinePool.Status.NodeRefs) >= minReplicas.value(controlPlaneMachinePool), nil

	}, intervals...).Should(Equal(true))
}

// GetAKSKubernetesVersion gets the kubernetes version for AKS clusters.
func GetAKSKubernetesVersion(ctx context.Context, e2eConfig *clusterctl.E2EConfig) (string, error) {
	e2eAKSVersion := e2eConfig.GetVariable(AKSKubernetesVersion)

	location := e2eConfig.GetVariable(AzureLocation)

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	authorizer, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	containerServiceClient := containerservice.NewContainerServicesClient(subscriptionID)
	containerServiceClient.Authorizer = authorizer

	result, err := containerServiceClient.ListOrchestrators(ctx, location, ManagedClustersResourceType)
	if err != nil {
		return "", err
	}

	// For 1.19 release this will be 1.19.0
	baseVersion := fmt.Sprintf("%s.0", semver.MajorMinor(e2eAKSVersion))
	maxVersion := fmt.Sprintf("%s.0", semver.MajorMinor(e2eAKSVersion))
	for _, o := range *result.Orchestrators {
		orchVersion := fmt.Sprintf("v%s", *o.OrchestratorVersion)
		// test k8s version matches with one of  the supported aks versions.
		if orchVersion == e2eAKSVersion {
			return e2eAKSVersion, nil
		}

		// find the highest aks version for a given major.minor
		if semver.MajorMinor(orchVersion) == semver.MajorMinor(maxVersion) && semver.Compare(orchVersion, maxVersion) > 0 {
			maxVersion = orchVersion
		}
	}

	// This means there is no version supported by AKS for this major.minor
	if semver.Compare(maxVersion, baseVersion) == 0 {
		return "", errors.New(fmt.Sprintf("No AKS versions found for %s", semver.MajorMinor(baseVersion)))
	}

	return maxVersion, nil
}

// byClusterOptions returns a set of ListOptions that allows to identify all the objects belonging to a Cluster.
func byClusterOptions(name, namespace string) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterLabelName: name,
		},
	}
}
