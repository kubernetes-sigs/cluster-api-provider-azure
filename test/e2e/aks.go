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
	"sync"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/conditions"
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

// GetAKSKubernetesVersion gets the kubernetes version for AKS clusters as specified by the environment variable defined by versionVar.
func GetAKSKubernetesVersion(ctx context.Context, e2eConfig *clusterctl.E2EConfig, versionVar string) (string, error) {
	e2eAKSVersion := e2eConfig.GetVariable(versionVar)

	location := e2eConfig.GetVariable(AzureLocation)

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	var maxVersion string
	switch e2eAKSVersion {
	case "latest":
		maxVersion, err = GetLatestStableAKSKubernetesVersion(ctx, subscriptionID, location)
		Expect(err).NotTo(HaveOccurred())
	case "latest-1":
		maxVersion, err = GetNextLatestStableAKSKubernetesVersion(ctx, subscriptionID, location)
		Expect(err).NotTo(HaveOccurred())
	default:
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
func GetLatestStableAKSKubernetesVersion(ctx context.Context, subscriptionID, location string) (string, error) {
	return getLatestStableAKSKubernetesVersionOffset(ctx, subscriptionID, location, 0)
}

// GetNextLatestStableAKSKubernetesVersion returns the stable available
// Kubernetes version of AKS immediately preceding the latest.
func GetNextLatestStableAKSKubernetesVersion(ctx context.Context, subscriptionID, location string) (string, error) {
	return getLatestStableAKSKubernetesVersionOffset(ctx, subscriptionID, location, 1)
}

func getLatestStableAKSKubernetesVersionOffset(ctx context.Context, subscriptionID, location string, offset int) (string, error) {
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
	maxVersion = orchestratorversions[len(orchestratorversions)-1-offset]
	if semver.IsValid(maxVersion) {
		foundWorkingVersion = true
	}
	if !foundWorkingVersion {
		return "", errors.New("latest stable AKS version not found")
	}
	return maxVersion, nil
}

type AKSMachinePoolSpecInput struct {
	Cluster       *clusterv1.Cluster
	MachinePools  []*expv1.MachinePool
	WaitIntervals []interface{}
}

func AKSMachinePoolSpec(ctx context.Context, inputGetter func() AKSMachinePoolSpecInput) {
	input := inputGetter()
	var wg sync.WaitGroup

	originalReplicas := map[types.NamespacedName]int32{}
	for _, mp := range input.MachinePools {
		originalReplicas[client.ObjectKeyFromObject(mp)] = to.Int32(mp.Spec.Replicas)
	}

	By("Scaling the machine pools out")
	for _, mp := range input.MachinePools {
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  to.Int32(mp.Spec.Replicas) + 1,
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})
		}(mp)
	}
	wg.Wait()

	By("Scaling the machine pools in")
	for _, mp := range input.MachinePools {
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  to.Int32(mp.Spec.Replicas) - 1,
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})
		}(mp)
	}
	wg.Wait()

	By("Scaling the machine pools to zero")
	// System node pools cannot be scaled to 0, so only include user node pools.
	var machinePoolsToScale []*expv1.MachinePool
	for _, mp := range input.MachinePools {
		ammp := &infrav1exp.AzureManagedMachinePool{}
		err := bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{
			Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
			Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
		}, ammp)
		Expect(err).NotTo(HaveOccurred())

		if ammp.Spec.Mode != string(infrav1exp.NodePoolModeSystem) {
			machinePoolsToScale = append(machinePoolsToScale, mp)
		}
	}

	framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
		ClusterProxy:              bootstrapClusterProxy,
		Cluster:                   input.Cluster,
		Replicas:                  0,
		MachinePools:              machinePoolsToScale,
		WaitForMachinePoolToScale: input.WaitIntervals,
	})

	By("Restoring initial replica count")
	for _, mp := range input.MachinePools {
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  originalReplicas[client.ObjectKeyFromObject(mp)],
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})
		}(mp)
	}
	wg.Wait()
}

type AKSAutoscaleSpecInput struct {
	Cluster       *clusterv1.Cluster
	MachinePool   *expv1.MachinePool
	WaitIntervals []interface{}
}

func AKSAutoscaleSpec(ctx context.Context, inputGetter func() AKSAutoscaleSpecInput) {
	input := inputGetter()

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	auth, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	agentpoolClient := containerservice.NewAgentPoolsClient(subscriptionID)
	agentpoolClient.Authorizer = auth
	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	amcp := &infrav1exp.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, types.NamespacedName{
		Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
		Name:      input.Cluster.Spec.ControlPlaneRef.Name,
	}, amcp)
	Expect(err).NotTo(HaveOccurred())

	ammp := &infrav1exp.AzureManagedMachinePool{}
	err = mgmtClient.Get(ctx, client.ObjectKeyFromObject(input.MachinePool), ammp)
	Expect(err).NotTo(HaveOccurred())

	resourceGroupName := amcp.Spec.ResourceGroupName
	managedClusterName := amcp.Name
	agentPoolName := *ammp.Spec.Name
	getAgentPool := func() (containerservice.AgentPool, error) {
		return agentpoolClient.Get(ctx, resourceGroupName, managedClusterName, agentPoolName)
	}

	toggleAutoscaling := func() {
		err = mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)
		Expect(err).NotTo(HaveOccurred())

		enabled := ammp.Spec.Scaling != nil
		var enabling string
		if enabled {
			enabling = "Disabling"
			ammp.Spec.Scaling = nil
		} else {
			enabling = "Enabling"
			ammp.Spec.Scaling = &infrav1exp.ManagedMachinePoolScaling{
				MinSize: to.Int32Ptr(1),
				MaxSize: to.Int32Ptr(2),
			}
		}
		By(enabling + " autoscaling")
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).NotTo(HaveOccurred())
	}

	validateUntoggled := validateAKSAutoscaleDisabled
	validateToggled := validateAKSAutoscaleEnabled
	autoscalingInitiallyEnabled := ammp.Spec.Scaling != nil
	if autoscalingInitiallyEnabled {
		validateToggled, validateUntoggled = validateUntoggled, validateToggled
	}

	validateUntoggled(getAgentPool, inputGetter)
	toggleAutoscaling()
	validateToggled(getAgentPool, inputGetter)
	toggleAutoscaling()
	validateUntoggled(getAgentPool, inputGetter)
}

func validateAKSAutoscaleDisabled(agentPoolGetter func() (containerservice.AgentPool, error), inputGetter func() AKSAutoscaleSpecInput) {
	By("Validating autoscaler disabled")
	Eventually(func(g Gomega) {
		agentpool, err := agentPoolGetter()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(to.Bool(agentpool.EnableAutoScaling)).To(BeFalse())
	}, inputGetter().WaitIntervals...).Should(Succeed())
}

func validateAKSAutoscaleEnabled(agentPoolGetter func() (containerservice.AgentPool, error), inputGetter func() AKSAutoscaleSpecInput) {
	By("Validating autoscaler enabled")
	Eventually(func(g Gomega) {
		agentpool, err := agentPoolGetter()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(to.Bool(agentpool.EnableAutoScaling)).To(BeTrue())
	}, inputGetter().WaitIntervals...).Should(Succeed())
}

type AKSPublicIPPrefixSpecInput struct {
	Cluster           *clusterv1.Cluster
	KubernetesVersion string
	WaitIntervals     []interface{}
}

func AKSPublicIPPrefixSpec(ctx context.Context, inputGetter func() AKSPublicIPPrefixSpecInput) {
	input := inputGetter()

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	auth, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	infraControlPlane := &infrav1exp.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
	Expect(err).NotTo(HaveOccurred())

	resourceGroupName := infraControlPlane.Spec.ResourceGroupName

	publicIPPrefixClient := network.NewPublicIPPrefixesClient(subscriptionID)
	publicIPPrefixClient.Authorizer = auth

	By("Creating public IP prefix with 2 addresses")
	publicIPPrefixFuture, err := publicIPPrefixClient.CreateOrUpdate(ctx, resourceGroupName, input.Cluster.Name, network.PublicIPPrefix{
		Location: to.StringPtr(infraControlPlane.Spec.Location),
		Sku: &network.PublicIPPrefixSku{
			Name: network.PublicIPPrefixSkuNameStandard,
		},
		PublicIPPrefixPropertiesFormat: &network.PublicIPPrefixPropertiesFormat{
			PrefixLength: to.Int32Ptr(31), // In bits. This provides 2 addresses.
		},
	})
	Expect(err).NotTo(HaveOccurred())
	var publicIPPrefix network.PublicIPPrefix
	Eventually(func() error {
		publicIPPrefix, err = publicIPPrefixFuture.Result(publicIPPrefixClient)
		return err
	}, input.WaitIntervals...).Should(Succeed(), "failed to create public IP prefix")

	By("Creating node pool with 3 nodes")
	infraMachinePool := &infrav1exp.AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pool3",
			Namespace: input.Cluster.Namespace,
		},
		Spec: infrav1exp.AzureManagedMachinePoolSpec{
			Mode:                 "User",
			SKU:                  "Standard_D2s_v3",
			EnableNodePublicIP:   to.BoolPtr(true),
			NodePublicIPPrefixID: to.StringPtr("/subscriptions/" + subscriptionID + "/resourceGroups/" + resourceGroupName + "/providers/Microsoft.Network/publicipprefixes/" + *publicIPPrefix.Name),
		},
	}
	err = mgmtClient.Create(ctx, infraMachinePool)
	Expect(err).NotTo(HaveOccurred())

	machinePool := &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: infraMachinePool.Namespace,
			Name:      infraMachinePool.Name,
		},
		Spec: expv1.MachinePoolSpec{
			ClusterName: input.Cluster.Name,
			Replicas:    to.Int32Ptr(3),
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: to.StringPtr(""),
					},
					ClusterName: input.Cluster.Name,
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: infrav1exp.GroupVersion.String(),
						Kind:       "AzureManagedMachinePool",
						Name:       infraMachinePool.Name,
					},
					Version: to.StringPtr(input.KubernetesVersion),
				},
			},
		},
	}
	err = mgmtClient.Create(ctx, machinePool)
	Expect(err).NotTo(HaveOccurred())

	defer func() {
		By("Deleting the node pool")
		err := mgmtClient.Delete(ctx, machinePool)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), &expv1.MachinePool{})
			return apierrors.IsNotFound(err)
		}, input.WaitIntervals...).Should(BeTrue(), "Deleted MachinePool %s/%s still exists", machinePool.Namespace, machinePool.Name)

		Eventually(func() bool {
			err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraMachinePool), &infrav1exp.AzureManagedMachinePool{})
			return apierrors.IsNotFound(err)
		}, input.WaitIntervals...).Should(BeTrue(), "Deleted AzureManagedMachinePool %s/%s still exists", infraMachinePool.Namespace, infraMachinePool.Name)
	}()

	By("Verifying the AzureManagedMachinePool converges to a failed ready status")
	Eventually(func(g Gomega) {
		infraMachinePool := &infrav1exp.AzureManagedMachinePool{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), infraMachinePool)
		g.Expect(err).NotTo(HaveOccurred())
		cond := conditions.Get(infraMachinePool, infrav1.AgentPoolsReadyCondition)
		g.Expect(cond).NotTo(BeNil())
		g.Expect(cond.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(cond.Reason).To(Equal(infrav1.FailedReason))
		g.Expect(cond.Message).To(HavePrefix("failed to find vm scale set"))
	}, input.WaitIntervals...).Should(Succeed())

	By("Scaling the MachinePool to 2 nodes")
	err = mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), machinePool)
	Expect(err).NotTo(HaveOccurred())
	machinePool.Spec.Replicas = to.Int32Ptr(2)
	err = mgmtClient.Update(ctx, machinePool)
	Expect(err).NotTo(HaveOccurred())

	By("Verifying the AzureManagedMachinePool becomes ready")
	Eventually(func(g Gomega) {
		infraMachinePool := &infrav1exp.AzureManagedMachinePool{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), infraMachinePool)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.IsTrue(infraMachinePool, infrav1.AgentPoolsReadyCondition)).To(BeTrue())
	}, input.WaitIntervals...).Should(Succeed())
}

type AKSUpgradeSpecInput struct {
	Cluster                    *clusterv1.Cluster
	MachinePools               []*expv1.MachinePool
	KubernetesVersionUpgradeTo string
	WaitForControlPlane        []interface{}
	WaitForMachinePools        []interface{}
}

func AKSUpgradeSpec(ctx context.Context, inputGetter func() AKSUpgradeSpecInput) {
	input := inputGetter()

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	auth, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())

	managedClustersClient := containerservice.NewManagedClustersClient(subscriptionID)
	managedClustersClient.Authorizer = auth

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	infraControlPlane := &infrav1exp.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
	Expect(err).NotTo(HaveOccurred())

	By("Upgrading the control plane")
	infraControlPlane.Spec.Version = input.KubernetesVersionUpgradeTo
	Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())

	Eventually(func() (string, error) {
		aksCluster, err := managedClustersClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name)
		if err != nil {
			return "", err
		}
		if aksCluster.ManagedClusterProperties == nil || aksCluster.ManagedClusterProperties.KubernetesVersion == nil {
			return "", errors.New("Kubernetes version unknown")
		}
		return "v" + *aksCluster.KubernetesVersion, nil
	}, input.WaitForControlPlane...).Should(Equal(input.KubernetesVersionUpgradeTo))

	By("Upgrading the machinepool instances")
	framework.UpgradeMachinePoolAndWait(ctx, framework.UpgradeMachinePoolAndWaitInput{
		ClusterProxy:                   bootstrapClusterProxy,
		Cluster:                        input.Cluster,
		UpgradeVersion:                 input.KubernetesVersionUpgradeTo,
		WaitForMachinePoolToBeUpgraded: input.WaitForMachinePools,
		MachinePools:                   input.MachinePools,
	})
}
