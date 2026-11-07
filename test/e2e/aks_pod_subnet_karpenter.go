//go:build e2e
// +build e2e

/*
Copyright 2026 The Kubernetes Authors.

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
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	asoauthorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	asomanagedidentityv1 "github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asoannotations "github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
)

const (
	podSubnetProbeDeploymentName = "pod-subnet-probe"
	podSubnetProbeImage          = "docker.io/library/busybox:latest"

	karpenterNamespace          = "kube-system"
	karpenterReleaseName        = "karpenter"
	karpenterServiceAccountName = "karpenter-sa"
	karpenterNodePoolLabel      = "karpenter.sh/nodepool"
	// Pod subnet is only wired up for VM-based provisioning; the AKS machine API
	// rejects podSubnetID.
	karpenterProvisionMode = "aksscriptless"

	// Subnet azureNames from the aks-aso-pod-subnet flavor.
	nodeSubnetName = "node-subnet"
	podSubnetName  = "pod-subnet"
	podSubnet2Name = "pod-subnet-2"
)

var (
	karpenterNodePoolGVR = schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodepools",
	}
	karpenterNodeClassGVR = schema.GroupVersionResource{
		Group:    "karpenter.azure.com",
		Version:  "v1beta1",
		Resource: "aksnodeclasses",
	}
	karpenterNodeClaimGVR = schema.GroupVersionResource{
		Group:    "karpenter.sh",
		Version:  "v1",
		Resource: "nodeclaims",
	}
)

// AKSPodSubnetKarpenterSpecInput is the input for AKSPodSubnetKarpenterSpec.
type AKSPodSubnetKarpenterSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	ResourceGroup         string
	NodeSubnetCIDR        string
	PodSubnetCIDR         string
	PodSubnet2CIDR        string
	ChartRef              string
	ChartVersion          string
	ImageRepository       string
	ImageTag              string
	WaitIntervals         []interface{}
}

// karpenterPodSubnetCase is one Karpenter nodepool under test alongside the pod subnet its nodes
// are expected to draw pod IPs from.
type karpenterPodSubnetCase struct {
	nodePoolName  string
	nodeClassName string
	probeName     string
	// podSubnetID is the nodeclass level override. Empty leaves it unset so the nodeclass
	// inherits the cluster default instead.
	podSubnetID string
	want        *net.IPNet
}

// AKSPodSubnetKarpenterSpec implements a test that verifies nodes provisioned by self-hosted
// Karpenter on an Azure CNI pod subnet cluster give their pods IPs from the pod subnet.
//
// It covers both the cluster default pod subnet and a nodeclass level override, asserting that
// each nodepool's pods stay inside their own pod subnet. The regression it guards against produces
// Karpenter nodes whose pods hold IPs from the wrong subnet while still reporting Running, so the
// assertions are on the pod IP CIDR rather than on pod phase.
func AKSPodSubnetKarpenterSpec(ctx context.Context, inputGetter func() AKSPodSubnetKarpenterSpecInput) {
	var (
		specName = "aks-pod-subnet-karpenter"
		input    AKSPodSubnetKarpenterSpecInput
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)
	Expect(input.ResourceGroup).NotTo(BeEmpty(), "Invalid argument. input.ResourceGroup can't be empty when calling %s spec", specName)
	Expect(input.ChartRef).NotTo(BeEmpty(), "Invalid argument. input.ChartRef can't be empty when calling %s spec. Set KARPENTER_CHART_OCI_REF to the Karpenter chart to test.", specName)
	Expect(input.ChartVersion).NotTo(BeEmpty(), "Invalid argument. input.ChartVersion can't be empty when calling %s spec. Set KARPENTER_CHART_VERSION to the chart built from the same commit as the image, since an unset version resolves to the registry's latest.", specName)
	Expect(input.ImageRepository).NotTo(BeEmpty(), "Invalid argument. input.ImageRepository can't be empty when calling %s spec. Set KARPENTER_IMAGE_REPOSITORY to a build with Azure CNI pod subnet support.", specName)
	Expect(input.ImageTag).NotTo(BeEmpty(), "Invalid argument. input.ImageTag can't be empty when calling %s spec. Set KARPENTER_IMAGE_TAG to a build with Azure CNI pod subnet support.", specName)

	prefixes := parsePodSubnetPrefixes(input.NodeSubnetCIDR, input.PodSubnetCIDR, input.PodSubnet2CIDR)

	By("creating a Kubernetes client to the workload cluster")
	clusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("provisioning Karpenter's workload identity and role assignments")
	// Created here rather than in the cluster template because CAPZ's controller has no RBAC for
	// the managedidentity.azure.com or authorization.azure.com API groups, so listing them in
	// AzureASOManagedControlPlane.spec.resources wedges the control plane reconcile.
	createKarpenterIdentity(ctx, input)
	// Registered before Karpenter is installed so LIFO tears the identity down last, after
	// Karpenter has drained the VMs it created.
	defer deleteKarpenterIdentity(ctx, input)

	By("collecting the cluster's Karpenter configuration from Azure")
	values, defaultPodSubnetID := buildKarpenterValues(ctx, input, clusterProxy)

	By("installing Karpenter via Helm")
	values = append(values,
		"--set-string", fmt.Sprintf("controller.image.repository=%s", input.ImageRepository),
		"--set-string", fmt.Sprintf("controller.image.tag=%s", input.ImageTag),
		// The chart pins an image digest by default, which would override the tag above.
		"--set-string", "controller.image.digest=",
	)
	InstallHelmChartOCI(ctx, clusterProxy, karpenterNamespace, input.ChartRef, input.ChartVersion, karpenterReleaseName, values...)

	cases := []karpenterPodSubnetCase{
		{
			nodePoolName:  "pod-subnet-default",
			nodeClassName: "pod-subnet-default",
			probeName:     podSubnetProbeDeploymentName,
			want:          prefixes.pod,
		},
		{
			nodePoolName:  "pod-subnet-override",
			nodeClassName: "pod-subnet-override",
			probeName:     podSubnetProbeDeploymentName + "-override",
			podSubnetID:   siblingSubnetID(defaultPodSubnetID, podSubnet2Name),
			want:          prefixes.pod2,
		},
	}

	By("creating a Karpenter NodePool and AKSNodeClass per pod subnet")
	dynamicClient := newDynamicClient(clusterProxy)
	for _, c := range cases {
		createKarpenterNodePool(ctx, dynamicClient, c)
	}
	// Karpenter's VMs and NICs live in the node resource group but attach to subnets ASO owns in
	// the cluster resource group, and teardown deletes the two groups without ordering between
	// them. Removing the NodePools here makes Karpenter drain its own VMs first, so a leftover NIC
	// can't block the VNet delete with InUseSubnetCannotBeDeleted.
	defer deleteKarpenterNodePools(ctx, dynamicClient, cases, input.WaitIntervals)

	By("scheduling a workload that only a Karpenter node can satisfy")
	// Selecting on the NodePool label means no existing AKS node can run these pods, so Karpenter
	// must provision one per nodepool. That is deterministic, unlike relying on resource pressure.
	// Both are created before either is awaited so the nodes provision in parallel.
	for _, c := range cases {
		nodeSelector := map[string]string{karpenterNodePoolLabel: c.nodePoolName}
		createPodSubnetProbeDeployment(ctx, clusterProxy, corev1.NamespaceDefault, c.probeName, nodeSelector)
	}
	defer deletePodSubnetProbeDeployments(ctx, clientset, corev1.NamespaceDefault, cases)
	for _, c := range cases {
		waitForPodSubnetProbeDeployment(ctx, clusterProxy, corev1.NamespaceDefault, c.probeName, specName)
	}

	for _, c := range cases {
		Byf("waiting for Karpenter to provision a node for nodepool %s", c.nodePoolName)
		karpenterNodes := waitForKarpenterNodes(ctx, clusterProxy, c.nodePoolName, input.WaitIntervals)
		Expect(karpenterNodes).NotTo(BeEmpty(), "expected Karpenter to provision at least one node for nodepool %s", c.nodePoolName)
		Logf("nodepool %s provisioned nodes: %s", c.nodePoolName, strings.Join(karpenterNodes, ", "))

		nodeNames := nodeNameSet(karpenterNodes...)

		Byf("verifying nodes for nodepool %s have IPs from the node subnet", c.nodePoolName)
		expectNodeIPsInNodeSubnet(ctx, clientset, prefixes, nodeNames)

		Byf("verifying pods for nodepool %s have IPs from pod subnet %s", c.nodePoolName, c.want)
		expectPodIPsInPodSubnet(ctx, clientset, prefixes, nodeNames, c.want)
	}
}

// siblingSubnetID returns the ARM ID of the subnet named name sitting in the same virtual network
// as subnetID.
func siblingSubnetID(subnetID, name string) string {
	i := strings.LastIndex(subnetID, "/")
	Expect(i).To(BeNumerically(">", 0), "expected subnet ID %q to end in a subnet name", subnetID)
	return subnetID[:i+1] + name
}

// karpenterRoleDefinitions are the built-in roles self-hosted Karpenter needs to create VMs and
// NICs and to attach the kubelet identity to them, mirroring the az-perm target in the provider's
// own Makefile-az.mk.
var karpenterRoleDefinitions = []struct {
	name string
	guid string
}{
	{"vm-contributor", "9980e02c-c2be-4d73-94e8-173b1dc7cf3c"},      // Virtual Machine Contributor
	{"network-contributor", "4d97b98b-1d4f-4787-a291-c67834d212e7"}, // Network Contributor
	{"identity-operator", "f1a07417-d97a-45cb-824c-7a7467783830"},   // Managed Identity Operator
}

// karpenterIdentityObjects builds the ASO resources backing Karpenter's workload identity: a user
// assigned identity, a federated credential binding it to Karpenter's service account, and role
// assignments granting it the Azure permissions it needs. The role assignments are scoped to the
// node resource group, where Karpenter creates VMs and NICs, plus the virtual network, which lives
// in the cluster resource group and so is not covered by that scope.
func karpenterIdentityObjects(input AKSPodSubnetKarpenterSpecInput) []client.Object {
	ns := input.Namespace.Name
	// ASO looks for a namespaced secret literally named "aso-credential"; CAPZ's e2e creates one
	// under a different name, so every resource has to point at it explicitly.
	credentialFrom := map[string]string{
		asoannotations.PerResourceSecret: e2eConfig.MustGetVariable("ASO_CREDENTIAL_SECRET_NAME"),
	}
	identityName := fmt.Sprintf("%s-karpenter", input.ClusterName)
	identityConfigMap := fmt.Sprintf("%s-karpenter-identity", input.ClusterName)
	nodeResourceGroupID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s-nodes", getSubscriptionID(Default), input.ClusterName)

	principalIDRef := &genruntime.ConfigMapReference{Name: identityConfigMap, Key: "principalId"}

	objs := []client.Object{
		&asomanagedidentityv1.UserAssignedIdentity{
			ObjectMeta: metav1.ObjectMeta{Name: identityName, Namespace: ns, Annotations: credentialFrom},
			Spec: asomanagedidentityv1.UserAssignedIdentity_Spec{
				Owner:    &genruntime.KnownResourceReference{Name: input.ClusterName},
				Location: ptr.To(os.Getenv(AzureLocation)),
				OperatorSpec: &asomanagedidentityv1.UserAssignedIdentityOperatorSpec{
					ConfigMaps: &asomanagedidentityv1.UserAssignedIdentityOperatorConfigMaps{
						ClientId:    &genruntime.ConfigMapDestination{Name: identityConfigMap, Key: "clientId"},
						PrincipalId: &genruntime.ConfigMapDestination{Name: identityConfigMap, Key: "principalId"},
					},
				},
			},
		},
		&asomanagedidentityv1.FederatedIdentityCredential{
			ObjectMeta: metav1.ObjectMeta{Name: identityName, Namespace: ns, Annotations: credentialFrom},
			Spec: asomanagedidentityv1.FederatedIdentityCredential_Spec{
				Owner:     &genruntime.KnownResourceReference{Name: identityName},
				Audiences: []string{"api://AzureADTokenExchange"},
				// Published by the ManagedCluster once its OIDC issuer is provisioned.
				IssuerFromConfig: &genruntime.ConfigMapReference{
					Name: fmt.Sprintf("%s-oidc", input.ClusterName),
					Key:  "issuer",
				},
				Subject: ptr.To(fmt.Sprintf("system:serviceaccount:%s:%s", karpenterNamespace, karpenterServiceAccountName)),
			},
		},
	}

	for _, role := range karpenterRoleDefinitions {
		objs = append(objs, newKarpenterRoleAssignment(
			fmt.Sprintf("%s-karpenter-%s", input.ClusterName, role.name), ns,
			&genruntime.ArbitraryOwnerReference{ARMID: nodeResourceGroupID},
			principalIDRef, role.guid, credentialFrom,
		))
	}
	objs = append(objs, newKarpenterRoleAssignment(
		fmt.Sprintf("%s-karpenter-vnet-contributor", input.ClusterName), ns,
		&genruntime.ArbitraryOwnerReference{
			Group: "network.azure.com",
			Kind:  "VirtualNetwork",
			Name:  fmt.Sprintf("%s-vnet", input.ClusterName),
		},
		principalIDRef, "4d97b98b-1d4f-4787-a291-c67834d212e7", credentialFrom, // Network Contributor
	))

	return objs
}

// newKarpenterRoleAssignment builds one ASO RoleAssignment. ASO generates the role assignment's
// Azure name, which must be a UUID, from the stable naming convention.
func newKarpenterRoleAssignment(name, namespace string, owner *genruntime.ArbitraryOwnerReference, principalID *genruntime.ConfigMapReference, roleGUID string, annotations map[string]string) *asoauthorizationv1.RoleAssignment {
	return &asoauthorizationv1.RoleAssignment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Annotations: annotations},
		Spec: asoauthorizationv1.RoleAssignment_Spec{
			Owner:                 owner,
			PrincipalIdFromConfig: principalID,
			PrincipalType:         ptr.To(asoauthorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal),
			RoleDefinitionReference: &genruntime.WellKnownResourceReference{
				ResourceReference: genruntime.ResourceReference{
					ARMID: fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", getSubscriptionID(Default), roleGUID),
				},
			},
			OperatorSpec: &asoauthorizationv1.RoleAssignmentOperatorSpec{
				NamingConvention: ptr.To("stable"),
			},
		},
	}
}

// createKarpenterIdentity creates Karpenter's identity resources and waits for ASO to reconcile
// them. The federated credential and role assignments resolve their issuer and principal ID from
// ConfigMaps other resources publish, so ASO retries them until those become available.
func createKarpenterIdentity(ctx context.Context, input AKSPodSubnetKarpenterSpecInput) {
	mgmtClient := input.BootstrapClusterProxy.GetClient()
	objs := karpenterIdentityObjects(input)

	for _, obj := range objs {
		Byf("creating %T %s/%s", obj, obj.GetNamespace(), obj.GetName())
		Expect(mgmtClient.Create(ctx, obj)).To(Succeed())
	}

	for _, obj := range objs {
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			g.Expect(asoResourceReady(obj)).To(BeTrue(), "%T %s is not ready yet", obj, obj.GetName())
		}, input.WaitIntervals...).Should(Succeed())
	}
}

// deleteKarpenterIdentity removes Karpenter's identity resources. It is best effort: the cluster
// resource group is deleted during teardown regardless, which removes the Azure resources.
func deleteKarpenterIdentity(ctx context.Context, input AKSPodSubnetKarpenterSpecInput) {
	mgmtClient := input.BootstrapClusterProxy.GetClient()
	objs := karpenterIdentityObjects(input)
	// Reverse order so the role assignments and federated credential go before the identity
	// they depend on.
	for i := len(objs) - 1; i >= 0; i-- {
		obj := objs[i]
		Byf("deleting %T %s/%s", obj, obj.GetNamespace(), obj.GetName())
		if err := mgmtClient.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			Logf("failed to delete %T %s/%s: %v", obj, obj.GetNamespace(), obj.GetName(), err)
		}
	}
}

// asoResourceReady reports whether an ASO resource has its Ready condition set to True.
func asoResourceReady(obj client.Object) bool {
	conditioner, ok := obj.(conditions.Conditioner)
	if !ok {
		return false
	}
	for _, cond := range conditioner.GetConditions() {
		if cond.Type == conditions.ConditionTypeReady {
			return cond.Status == metav1.ConditionTrue
		}
	}
	return false
}

// buildKarpenterValues assembles the Helm --set arguments self-hosted Karpenter needs, mirroring
// the provider's hack/deploy/configure-values.sh. It also returns the cluster default pod subnet
// ID that nodeclasses inherit when they set no override of their own.
func buildKarpenterValues(ctx context.Context, input AKSPodSubnetKarpenterSpecInput, clusterProxy framework.ClusterProxy) ([]string, string) {
	subscriptionID := getSubscriptionID(Default)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	managedClustersClient, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	resp, err := managedClustersClient.Get(ctx, input.ResourceGroup, input.ClusterName, nil)
	Expect(err).NotTo(HaveOccurred())
	mc := resp.ManagedCluster
	Expect(mc.Properties).NotTo(BeNil())
	Expect(mc.Properties.AgentPoolProfiles).NotTo(BeEmpty(), "expected the cluster to have at least one agent pool")

	// The pools sit on different pod subnets, and the API does not promise an order, so the pool
	// backing the cluster default is selected by subnet name rather than by index.
	pool := agentPoolForPodSubnet(mc.Properties.AgentPoolProfiles, podSubnetName)
	vnetSubnetID := ptr.Deref(pool.VnetSubnetID, "")
	podSubnetID := ptr.Deref(pool.PodSubnetID, "")
	Expect(vnetSubnetID).NotTo(BeEmpty(), "expected agent pool %s to have a node subnet", ptr.Deref(pool.Name, ""))
	// The v1api20240901 API version this flavor pins has no podIpAllocationMode field, so pools
	// created through it always use dynamic allocation.
	Expect(podSubnetID).NotTo(BeEmpty(), "expected agent pool %s to have a pod subnet", ptr.Deref(pool.Name, ""))

	kubeletIdentity := mc.Properties.IdentityProfile["kubeletidentity"]
	Expect(kubeletIdentity).NotTo(BeNil(), "expected the cluster to have a kubelet identity")

	networkProfile := mc.Properties.NetworkProfile
	Expect(networkProfile).NotTo(BeNil())

	vnetGUID := lookupVNetGUID(ctx, cred, subscriptionID, vnetSubnetID)
	bootstrapToken := readBootstrapToken(ctx, clusterProxy)
	karpenterClientID := readKarpenterIdentityClientID(ctx, input)

	// Indices are explicit because the chart appends controller.env verbatim to the container's
	// env list.
	env := []struct{ name, value string }{
		{"CLUSTER_NAME", input.ClusterName},
		{"CLUSTER_ENDPOINT", clusterProxy.GetRESTConfig().Host},
		{"KUBELET_BOOTSTRAP_TOKEN", bootstrapToken},
		{"SSH_PUBLIC_KEY", readSSHPublicKey()},
		{"NETWORK_PLUGIN", networkProfileValue(string(ptr.Deref(networkProfile.NetworkPlugin, "")))},
		{"NETWORK_PLUGIN_MODE", networkProfileValue(string(ptr.Deref(networkProfile.NetworkPluginMode, "")))},
		{"NETWORK_POLICY", networkProfileValue(string(ptr.Deref(networkProfile.NetworkPolicy, "")))},
		{"NETWORK_DATAPLANE", networkProfileValue(string(ptr.Deref(networkProfile.NetworkDataplane, "")))},
		{"VNET_SUBNET_ID", vnetSubnetID},
		{"POD_SUBNET_ID", podSubnetID},
		// Required alongside POD_SUBNET_ID so Karpenter does not have to infer the mode. Builds
		// before this option existed ignore it.
		{"POD_IP_ALLOCATION_MODE", "DynamicIndividual"},
		{"VNET_GUID", vnetGUID},
		{"NODE_IDENTITIES", ptr.Deref(kubeletIdentity.ResourceID, "")},
		{"AZURE_SUBSCRIPTION_ID", subscriptionID},
		{"LOCATION", ptr.Deref(mc.Location, "")},
		{"KUBELET_IDENTITY_CLIENT_ID", ptr.Deref(kubeletIdentity.ClientID, "")},
		{"AZURE_NODE_RESOURCE_GROUP", ptr.Deref(mc.Properties.NodeResourceGroup, "")},
		{"ARM_RESOURCE_GROUP", input.ResourceGroup},
		{"PROVISION_MODE", karpenterProvisionMode},
		{"USE_SIG", "false"},
		{"ENABLE_AZURE_SDK_LOGGING", "false"},
		// Leader election is unnecessary at one replica and slows startup.
		{"DISABLE_LEADER_ELECTION", "true"},
	}

	// --set infers types, so anything that must stay a string uses --set-string: Kubernetes
	// requires strings for label values and EnvVar.value, and the chart renders both with a bare
	// toYaml. replicas and the feature gate are genuinely numeric and boolean.
	args := []string{
		"--set", "replicas=1",
		"--set", "settings.featureGates.staticCapacity=true",
		"--set-string", "logLevel=debug",
		"--set-string", fmt.Sprintf("serviceAccount.name=%s", karpenterServiceAccountName),
		"--set-string", fmt.Sprintf("serviceAccount.annotations.%s=%s",
			escapeHelmKey("azure.workload.identity/client-id"), karpenterClientID),
		"--set-string", fmt.Sprintf("podLabels.%s=true", escapeHelmKey("azure.workload.identity/use")),
	}
	for i, e := range env {
		args = append(args,
			"--set-string", fmt.Sprintf("controller.env[%d].name=%s", i, e.name),
			"--set-string", fmt.Sprintf("controller.env[%d].value=%s", i, escapeHelmValue(e.value)),
		)
	}
	return args, podSubnetID
}

// agentPoolForPodSubnet returns the agent pool whose pod subnet is named name.
func agentPoolForPodSubnet(pools []*armcontainerservice.ManagedClusterAgentPoolProfile, name string) *armcontainerservice.ManagedClusterAgentPoolProfile {
	for _, pool := range pools {
		if pool != nil && strings.HasSuffix(ptr.Deref(pool.PodSubnetID, ""), "/"+name) {
			return pool
		}
	}
	Fail(fmt.Sprintf("expected one of the cluster's agent pools to be on pod subnet %q", name))
	return nil
}

// escapeHelmKey escapes the dots and slashes in a Kubernetes annotation or label key so Helm
// treats it as one map key instead of a nested path.
func escapeHelmKey(key string) string {
	return strings.NewReplacer(".", `\.`, "/", `\/`).Replace(key)
}

// escapeHelmValue escapes the characters Helm's --set parser treats as structure, so values like
// comma-separated resource IDs survive intact. --set-string uses the same structural parser, so
// the escaping is still required. Replacer makes a single pass, so the backslashes introduced
// here are not escaped again.
func escapeHelmValue(value string) string {
	return strings.NewReplacer(`\`, `\\`, ",", `\,`, ".", `\.`, "=", `\=`).Replace(value)
}

// networkProfileValue normalizes AKS network profile fields, which report "none" for features
// that are switched off while Karpenter expects an empty string.
func networkProfileValue(value string) string {
	if value == "none" {
		return ""
	}
	return value
}

// lookupVNetGUID resolves the resource GUID of the VNet that owns the given subnet. Karpenter
// needs it to build the node's network configuration.
func lookupVNetGUID(ctx context.Context, cred *azidentity.DefaultAzureCredential, subscriptionID, subnetID string) string {
	parsed, err := azureutil.ParseResourceID(subnetID)
	Expect(err).NotTo(HaveOccurred(), "failed to parse subnet ID %q", subnetID)
	Expect(parsed.Parent).NotTo(BeNil(), "expected subnet ID %q to name a parent virtual network", subnetID)

	vnetsClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	resp, err := vnetsClient.Get(ctx, parsed.Parent.ResourceGroupName, parsed.Parent.Name, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.Properties).NotTo(BeNil())

	guid := ptr.Deref(resp.Properties.ResourceGUID, "")
	Expect(guid).NotTo(BeEmpty(), "expected virtual network %s to report a resource GUID", parsed.Parent.Name)
	return guid
}

// readBootstrapToken returns a kubeadm-style bootstrap token from the workload cluster, which
// Karpenter hands to new nodes so they can join.
func readBootstrapToken(ctx context.Context, clusterProxy framework.ClusterProxy) string {
	secrets, err := clusterProxy.GetClientSet().CoreV1().Secrets(metav1.NamespaceSystem).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("type=%s", corev1.SecretTypeBootstrapToken),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(secrets.Items).NotTo(BeEmpty(), "expected a bootstrap token secret in %s", metav1.NamespaceSystem)

	secret := secrets.Items[0]
	tokenID := string(secret.Data["token-id"])
	tokenSecret := string(secret.Data["token-secret"])
	Expect(tokenID).NotTo(BeEmpty(), "bootstrap token secret %s has no token-id", secret.Name)
	Expect(tokenSecret).NotTo(BeEmpty(), "bootstrap token secret %s has no token-secret", secret.Name)

	return fmt.Sprintf("%s.%s", tokenID, tokenSecret)
}

// readKarpenterIdentityClientID reads the client ID that ASO published for Karpenter's user
// assigned identity. The flavor asks ASO to write it to a ConfigMap in the management cluster.
func readKarpenterIdentityClientID(ctx context.Context, input AKSPodSubnetKarpenterSpecInput) string {
	name := fmt.Sprintf("%s-karpenter-identity", input.ClusterName)
	configMap := &corev1.ConfigMap{}
	Eventually(func() error {
		return input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{
			Namespace: input.Namespace.Name,
			Name:      name,
		}, configMap)
	}, input.WaitIntervals...).Should(Succeed(), "expected ASO to publish ConfigMap %s/%s with Karpenter's identity", input.Namespace.Name, name)

	clientID := configMap.Data["clientId"]
	Expect(clientID).NotTo(BeEmpty(), "ConfigMap %s/%s has no clientId", input.Namespace.Name, name)
	return clientID
}

// readSSHPublicKey returns the SSH public key e2e provisions nodes with. Karpenter requires one
// to build its launch template.
func readSSHPublicKey() string {
	if b64 := os.Getenv("AZURE_SSH_PUBLIC_KEY_B64"); b64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		Expect(err).NotTo(HaveOccurred(), "failed to decode AZURE_SSH_PUBLIC_KEY_B64")
		return sshPublicKeyWithComment(string(decoded))
	}
	if keyfile := os.Getenv("AZURE_SSH_PUBLIC_KEY_FILE"); keyfile != "" {
		contents, err := os.ReadFile(keyfile)
		Expect(err).NotTo(HaveOccurred(), "failed to read AZURE_SSH_PUBLIC_KEY_FILE %q", keyfile)
		return sshPublicKeyWithComment(string(contents))
	}
	Fail("no SSH public key available: set AZURE_SSH_PUBLIC_KEY_B64 or AZURE_SSH_PUBLIC_KEY_FILE")
	return ""
}

// sshPublicKeyWithComment normalizes a public key to the "<key> azureuser" form Karpenter's
// tooling produces.
func sshPublicKeyWithComment(key string) string {
	trimmed := strings.TrimSpace(key)
	Expect(trimmed).NotTo(BeEmpty(), "SSH public key is empty")
	if len(strings.Fields(trimmed)) >= 3 {
		return trimmed
	}
	return trimmed + " azureuser"
}

// createKarpenterNodePool creates the NodePool and AKSNodeClass Karpenter provisions from. When
// the case sets a pod subnet, the nodeclass overrides the cluster default with it.
func createKarpenterNodePool(ctx context.Context, dynamicClient dynamic.Interface, c karpenterPodSubnetCase) {
	nodeClassSpec := map[string]interface{}{
		"imageFamily": "Ubuntu",
	}
	if c.podSubnetID != "" {
		nodeClassSpec["podSubnetID"] = c.podSubnetID
	}
	nodeClass := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": karpenterNodeClassGVR.Group + "/" + karpenterNodeClassGVR.Version,
			"kind":       "AKSNodeClass",
			"metadata": map[string]interface{}{
				"name": c.nodeClassName,
			},
			"spec": nodeClassSpec,
		},
	}
	created, err := dynamicClient.Resource(karpenterNodeClassGVR).Create(ctx, nodeClass, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	if c.podSubnetID != "" {
		// An AKSNodeClass CRD older than the Karpenter build under test has no podSubnetID, and
		// the API server prunes unknown fields silently, so read it back rather than trusting the
		// create to have preserved it.
		got, found, err := unstructured.NestedString(created.Object, "spec", "podSubnetID")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue(), "the AKSNodeClass CRD has no podSubnetID field, so it was dropped on create. The chart selected by KARPENTER_CHART_OCI_REF and KARPENTER_CHART_VERSION predates the Karpenter build in KARPENTER_IMAGE_TAG; point both at a chart built from the same commit.")
		Expect(got).To(Equal(c.podSubnetID))
	}

	nodePool := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": karpenterNodePoolGVR.Group + "/" + karpenterNodePoolGVR.Version,
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name": c.nodePoolName,
			},
			"spec": map[string]interface{}{
				// Hold nodes still for the duration of the test rather than letting
				// consolidation replace them mid-assertion.
				"disruption": map[string]interface{}{
					"consolidationPolicy": "WhenEmpty",
					"consolidateAfter":    "1h",
				},
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"expireAfter": "Never",
						"nodeClassRef": map[string]interface{}{
							"group": karpenterNodeClassGVR.Group,
							"kind":  "AKSNodeClass",
							"name":  c.nodeClassName,
						},
						"requirements": []interface{}{
							map[string]interface{}{
								"key":      "kubernetes.io/arch",
								"operator": "In",
								"values":   []interface{}{"amd64"},
							},
							map[string]interface{}{
								"key":      "kubernetes.io/os",
								"operator": "In",
								"values":   []interface{}{"linux"},
							},
							map[string]interface{}{
								"key":      "karpenter.sh/capacity-type",
								"operator": "In",
								"values":   []interface{}{"on-demand"},
							},
							map[string]interface{}{
								"key":      "karpenter.azure.com/sku-family",
								"operator": "In",
								"values":   []interface{}{"D"},
							},
						},
					},
				},
			},
		},
	}
	_, err = dynamicClient.Resource(karpenterNodePoolGVR).Create(ctx, nodePool, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

// deleteKarpenterNodePools removes each NodePool and waits for Karpenter to finish reclaiming what
// it provisioned. Deleting a NodePool cascades to its NodeClaims, and each NodeClaim is only
// removed once Karpenter has deleted the backing VM and NIC. This waits on NodeClaims rather than
// Nodes because an instance that never registered has no Node to wait on, which would leave the
// VM behind for cluster teardown to trip over.
func deleteKarpenterNodePools(ctx context.Context, dynamicClient dynamic.Interface, cases []karpenterPodSubnetCase, intervals []interface{}) {
	By("deleting the Karpenter NodePools so Karpenter reclaims the VMs it created")
	for _, c := range cases {
		if err := dynamicClient.Resource(karpenterNodePoolGVR).Delete(ctx, c.nodePoolName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			// Returning here would leave the VMs for cluster teardown to trip over, so fail loudly.
			Expect(err).NotTo(HaveOccurred(), "failed to delete Karpenter NodePool %s", c.nodePoolName)
		}
	}

	for _, c := range cases {
		Eventually(func(g Gomega) {
			nodeClaims, err := dynamicClient.Resource(karpenterNodeClaimGVR).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", karpenterNodePoolLabel, c.nodePoolName),
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(nodeClaims.Items).To(BeEmpty(), "Karpenter has not finished reclaiming the instances for nodepool %s yet", c.nodePoolName)
		}, intervals...).Should(Succeed())
	}

	for _, c := range cases {
		if err := dynamicClient.Resource(karpenterNodeClassGVR).Delete(ctx, c.nodeClassName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			Logf("failed to delete Karpenter AKSNodeClass %s: %v", c.nodeClassName, err)
		}
	}
}

// waitForKarpenterNodes waits until at least one node provisioned by the given nodepool is
// registered and ready, and returns the names of those nodes.
func waitForKarpenterNodes(ctx context.Context, clusterProxy framework.ClusterProxy, nodePoolName string, intervals []interface{}) []string {
	var names []string
	Eventually(func(g Gomega) {
		nodes, err := clusterProxy.GetClientSet().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", karpenterNodePoolLabel, nodePoolName),
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(nodes.Items).NotTo(BeEmpty(), "no node has registered for nodepool %s yet", nodePoolName)

		names = nil
		for _, node := range nodes.Items {
			ready := false
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
					ready = true
					break
				}
			}
			g.Expect(ready).To(BeTrue(), "Karpenter node %s is not ready yet", node.Name)
			names = append(names, node.Name)
		}
	}, intervals...).Should(Succeed())
	return names
}

// podSubnetPrefixes holds the node and pod subnet CIDRs of a cluster using Azure CNI
// with a pod subnet. pod is the cluster default; pod2 backs the nodeclass level override.
type podSubnetPrefixes struct {
	node *net.IPNet
	pod  *net.IPNet
	pod2 *net.IPNet
}

// parsePodSubnetPrefixes parses the node and pod subnet CIDRs and asserts none of them overlap.
func parsePodSubnetPrefixes(nodeCIDR, podCIDR, pod2CIDR string) podSubnetPrefixes {
	Expect(nodeCIDR).NotTo(BeEmpty(), "Invalid argument. Node subnet CIDR can't be empty")
	Expect(podCIDR).NotTo(BeEmpty(), "Invalid argument. Pod subnet CIDR can't be empty")
	Expect(pod2CIDR).NotTo(BeEmpty(), "Invalid argument. Second pod subnet CIDR can't be empty")

	_, node, err := net.ParseCIDR(nodeCIDR)
	Expect(err).NotTo(HaveOccurred(), "failed to parse node subnet CIDR %q", nodeCIDR)
	_, pod, err := net.ParseCIDR(podCIDR)
	Expect(err).NotTo(HaveOccurred(), "failed to parse pod subnet CIDR %q", podCIDR)
	_, pod2, err := net.ParseCIDR(pod2CIDR)
	Expect(err).NotTo(HaveOccurred(), "failed to parse second pod subnet CIDR %q", pod2CIDR)

	// Overlapping prefixes would make the assertions meaningless, since an IP from one subnet
	// would also satisfy the check for another.
	for _, pair := range [][2]*net.IPNet{{node, pod}, {node, pod2}, {pod, pod2}} {
		Expect(pair[0].Contains(pair[1].IP) || pair[1].Contains(pair[0].IP)).To(BeFalse(),
			"subnets %s and %s must not overlap", pair[0], pair[1])
	}

	return podSubnetPrefixes{node: node, pod: pod, pod2: pod2}
}

// expectNodeIPsInNodeSubnet asserts each node's internal IP comes from the node subnet.
// When nodeNames is empty every node is checked, otherwise only the named ones.
func expectNodeIPsInNodeSubnet(ctx context.Context, clientset kubernetes.Interface, prefixes podSubnetPrefixes, nodeNames map[string]struct{}) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(nodes.Items).NotTo(BeEmpty(), "expected at least one node")

	checked := 0
	for _, node := range nodes.Items {
		if len(nodeNames) > 0 {
			if _, ok := nodeNames[node.Name]; !ok {
				continue
			}
		}
		for _, addr := range node.Status.Addresses {
			if addr.Type != corev1.NodeInternalIP {
				continue
			}
			// Pod subnet clusters are IPv4 only here, so skip anything that isn't a v4 address.
			ip := net.ParseIP(addr.Address)
			if ip == nil || ip.To4() == nil {
				continue
			}
			Expect(prefixes.node.Contains(ip)).To(BeTrue(),
				"node %s has internal IP %s outside the node subnet %s", node.Name, addr.Address, prefixes.node)
			checked++
		}
	}
	Expect(checked).To(BeNumerically(">", 0), "expected to check at least one node internal IP")
}

// expectPodIPsInPodSubnet asserts every running pod that uses the cluster network holds an IP from
// want and from no other subnet in the cluster. When nodeNames is empty pods on all nodes are
// checked, otherwise only pods scheduled onto the named nodes.
//
// The failures this guards against produce healthy Running pods holding IPs from the node subnet
// or from the other pod subnet, so asserting on phase, or only that the IP is in want, would miss
// them.
func expectPodIPsInPodSubnet(ctx context.Context, clientset kubernetes.Interface, prefixes podSubnetPrefixes, nodeNames map[string]struct{}, want *net.IPNet) {
	pods, err := clientset.CoreV1().Pods(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	// Every subnet a pod IP must not come from: the node subnet, plus whichever pod subnet is not
	// the expected one.
	forbidden := []*net.IPNet{prefixes.node}
	for _, other := range []*net.IPNet{prefixes.pod, prefixes.pod2} {
		if other.String() != want.String() {
			forbidden = append(forbidden, other)
		}
	}

	checked := 0
	for _, pod := range pods.Items {
		// Host network pods share the node's IP by design, so they say nothing about pod IPAM.
		if pod.Spec.HostNetwork {
			continue
		}
		if pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" {
			continue
		}
		if len(nodeNames) > 0 {
			if _, ok := nodeNames[pod.Spec.NodeName]; !ok {
				continue
			}
		}

		ip := net.ParseIP(pod.Status.PodIP)
		Expect(ip).NotTo(BeNil(), "pod %s/%s has unparsable IP %q", pod.Namespace, pod.Name, pod.Status.PodIP)
		if ip.To4() == nil {
			continue
		}

		Expect(want.Contains(ip)).To(BeTrue(),
			"pod %s/%s on node %s has IP %s outside its pod subnet %s",
			pod.Namespace, pod.Name, pod.Spec.NodeName, pod.Status.PodIP, want)
		for _, other := range forbidden {
			Expect(other.Contains(ip)).To(BeFalse(),
				"pod %s/%s on node %s has IP %s from %s instead of its own pod subnet %s",
				pod.Namespace, pod.Name, pod.Spec.NodeName, pod.Status.PodIP, other, want)
		}
		checked++
	}
	Expect(checked).To(BeNumerically(">", 0), "expected to check at least one cluster-networked pod")
	Logf("checked %d pod IPs against pod subnet %s", checked, want)
}

// createPodSubnetProbeDeployment schedules a workload that uses the cluster network.
// nodeSelector may be nil to let it land anywhere.
func createPodSubnetProbeDeployment(ctx context.Context, clusterProxy framework.ClusterProxy, namespace, name string, nodeSelector map[string]string) {
	labels := map[string]string{"app": name}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					NodeSelector: nodeSelector,
					Containers: []corev1.Container{
						{
							Name:    "probe",
							Image:   podSubnetProbeImage,
							Command: []string{"sh", "-c", "tail -f /dev/null"},
						},
					},
				},
			},
		},
	}

	Byf("creating deployment %s/%s", namespace, name)
	_, err := clusterProxy.GetClientSet().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

// waitForPodSubnetProbeDeployment waits for a probe workload to become available, which only
// happens once Karpenter has provisioned a node its pods can run on.
func waitForPodSubnetProbeDeployment(ctx context.Context, clusterProxy framework.ClusterProxy, namespace, name, specName string) {
	Byf("waiting for deployment %s/%s to be available", namespace, name)
	WaitForDeploymentsAvailable(ctx, GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, name, namespace, specName),
		e2eConfig.GetIntervals(specName, "wait-deployment")...)
}

// deletePodSubnetProbeDeployments removes each probe workload, tolerating already deleted ones.
func deletePodSubnetProbeDeployments(ctx context.Context, clientset kubernetes.Interface, namespace string, cases []karpenterPodSubnetCase) {
	for _, c := range cases {
		Byf("deleting deployment %s/%s", namespace, c.probeName)
		if err := clientset.AppsV1().Deployments(namespace).Delete(ctx, c.probeName, metav1.DeleteOptions{}); err != nil {
			Logf("failed to delete deployment %s/%s: %v", namespace, c.probeName, err)
		}
	}
}

// nodeNameSet builds a lookup set from node names.
func nodeNameSet(names ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	return set
}

// PodSubnetNetworkInput is the input for CreatePodSubnetNetwork.
type PodSubnetNetworkInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	VNetCIDR              string
	NodeSubnetCIDR        string
	PodSubnetCIDR         string
	PodSubnet2CIDR        string
	WaitIntervals         []interface{}
}

// CreatePodSubnetNetwork provisions the resource group, virtual network and the node and pod
// subnets, and waits for each to finish provisioning in Azure.
//
// This runs before the cluster template is applied because AKS validates that the subnets exist
// when the ManagedCluster is created, and nothing orders the two. Losing that race is not
// self-correcting: AKS returns SubnetNotFound, which ASO classifies as RetryVerySlow and so does
// not retry for a full sync period. The template still declares these same resources, which CAPZ
// applies with ForceOwnership and so adopts rather than conflicts with.
func CreatePodSubnetNetwork(ctx context.Context, input PodSubnetNetworkInput) {
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil")
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty")

	mgmtClient := input.BootstrapClusterProxy.GetClient()
	ns := input.Namespace.Name
	vnetName := fmt.Sprintf("%s-vnet", input.ClusterName)
	credentialFrom := map[string]string{
		asoannotations.PerResourceSecret: e2eConfig.MustGetVariable("ASO_CREDENTIAL_SECRET_NAME"),
	}

	resourceGroup := &asoresourcesv1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{Name: input.ClusterName, Namespace: ns, Annotations: credentialFrom},
		Spec: asoresourcesv1.ResourceGroup_Spec{
			Location: ptr.To(os.Getenv(AzureLocation)),
		},
	}
	virtualNetwork := &asonetworkv1.VirtualNetwork{
		ObjectMeta: metav1.ObjectMeta{Name: vnetName, Namespace: ns, Annotations: credentialFrom},
		Spec: asonetworkv1.VirtualNetwork_Spec{
			Owner:        &genruntime.KnownResourceReference{Name: input.ClusterName},
			Location:     ptr.To(os.Getenv(AzureLocation)),
			AddressSpace: &asonetworkv1.AddressSpace{AddressPrefixes: []string{input.VNetCIDR}},
		},
	}

	// Azure serializes writes to the subnets of one virtual network, so these are created one at
	// a time to avoid AnotherOperationInProgress.
	subnets := []*asonetworkv1.VirtualNetworksSubnet{
		newPodSubnetSubnet(fmt.Sprintf("%s-node-subnet", input.ClusterName), ns, nodeSubnetName, vnetName, input.NodeSubnetCIDR, credentialFrom),
		newPodSubnetSubnet(fmt.Sprintf("%s-%s", input.ClusterName, podSubnetName), ns, podSubnetName, vnetName, input.PodSubnetCIDR, credentialFrom),
		newPodSubnetSubnet(fmt.Sprintf("%s-%s", input.ClusterName, podSubnet2Name), ns, podSubnet2Name, vnetName, input.PodSubnet2CIDR, credentialFrom),
	}

	ordered := []client.Object{resourceGroup, virtualNetwork}
	for _, subnet := range subnets {
		ordered = append(ordered, subnet)
	}
	for _, obj := range ordered {
		Byf("creating %T %s/%s", obj, obj.GetNamespace(), obj.GetName())
		Expect(mgmtClient.Create(ctx, obj)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			g.Expect(asoResourceReady(obj)).To(BeTrue(), "%T %s is not ready yet", obj, obj.GetName())
		}, input.WaitIntervals...).Should(Succeed())
	}
}

// newPodSubnetSubnet builds one subnet of the cluster's virtual network. The singular
// addressPrefix avoids SubscriptionNotRegisteredForFeature for
// Microsoft.Network/AllowMultipleAddressPrefixesOnSubnet.
func newPodSubnetSubnet(name, namespace, azureName, vnetName, cidr string, annotations map[string]string) *asonetworkv1.VirtualNetworksSubnet {
	return &asonetworkv1.VirtualNetworksSubnet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Annotations: annotations},
		Spec: asonetworkv1.VirtualNetworksSubnet_Spec{
			AzureName:     azureName,
			Owner:         &genruntime.KnownResourceReference{Name: vnetName},
			AddressPrefix: ptr.To(cidr),
		},
	}
}
