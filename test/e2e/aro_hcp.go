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
	"fmt"
	"os"
	"strings"
	"time"

	asoConfig "github.com/Azure/azure-service-operator/v2/pkg/common/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav2 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

const (
	aroHCPAPIVersion        = "v20260901preview"
	aroHCPAPIGroup          = "redhatopenshift.azure.com"
	keyVaultAPIGroup        = "keyvault.azure.com"
	aroHCPControlPlaneKind  = "HcpOpenShiftCluster"
	aroHCPNodePoolKind      = "HcpOpenShiftClustersNodePool"
	aroHCPExternalAuthKind  = "HcpOpenShiftClustersExternalAuth"
	aroHCPVaultKind         = "Vault"
	aroHCPTemplatePath      = "data/aro-hcp/cluster-template.yaml"
	aroHCPExternalAuthAppID = "ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID"
)

var _ = Describe("ARO HCP E2E", func() {
	It("provisions, scales, and deletes a cluster", func(ctx SpecContext) {
		runAROHCPClusterLifecycle(ctx)
	})
})

type aroHCPResourceReference struct {
	key client.ObjectKey
	gvk schema.GroupVersionKind
}

func runAROHCPClusterLifecycle(ctx context.Context) {
	if os.Getenv("EXP_ARO") != "true" {
		Skip("ARO HCP E2E requires EXP_ARO=true so the management-cluster controller enables the ARO feature gate")
	}

	clusterName := "aro-e2e-" + strings.ToLower(util.RandomString(6))
	namespace, cancelWatches, err := setupSpecNamespace(ctx, "aro-e2e-"+strings.ToLower(util.RandomString(6)), bootstrapClusterProxy, artifactFolder)
	Expect(err).NotTo(HaveOccurred())
	namespaceName := namespace.Name
	clusterCleanupCompleted := false
	DeferCleanup(func(ctx SpecContext) {
		defer cancelWatches()
		if skipCleanup {
			return
		}
		if !clusterCleanupCompleted {
			By(fmt.Sprintf("Preserving namespace %q because ARO HCP resource cleanup did not complete", namespaceName))
			return
		}

		By("Deleting the ARO HCP E2E namespace")
		Eventually(func(ctx context.Context) error {
			return bootstrapClusterProxy.GetClient().Delete(ctx, namespace)
		}, e2eConfig.GetIntervals("default", "wait-aro-hcp-delete")...).WithContext(ctx).Should(Succeed())
	})
	mgmtClient := bootstrapClusterProxy.GetClient()
	cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespaceName}}
	var managedResourcesToDelete []aroHCPResourceReference
	DeferCleanup(func(ctx SpecContext) {
		if skipCleanup {
			By("Skipping ARO HCP cluster cleanup because -e2e.skip-resource-cleanup is set")
			return
		}

		// ApplyCustomClusterTemplateAndWait can fail before returning its result, so
		// the cluster variable may not have been populated even though the Cluster
		// and its infrastructure resources were created. Discover them by name before
		// deleting the Cluster, while the ASO credential Secret still exists.
		clusterKey := client.ObjectKey{Namespace: namespaceName, Name: clusterName}
		clusterToDelete := &clusterv1.Cluster{}
		getClusterErr := mgmtClient.Get(ctx, clusterKey, clusterToDelete)
		if getClusterErr != nil && !apierrors.IsNotFound(getClusterErr) {
			Expect(getClusterErr).NotTo(HaveOccurred(), "failed to get ARO HCP E2E Cluster before cleanup")
			return
		}
		if getClusterErr == nil {
			managedResources, err := aroHCPManagedResourceReferences(ctx, mgmtClient, clusterToDelete)
			Expect(err).NotTo(HaveOccurred(), "failed to discover ARO HCP resources before cleanup")
			if err != nil {
				return
			}
			managedResourcesToDelete = uniqueAROResourceReferences(managedResourcesToDelete, managedResources)

			// DeleteClusterAndWait in the current CAPI test dependency applies BeEmpty
			// to a *Cluster, which fails before deletion. Use its component helpers.
			framework.DeleteCluster(ctx, framework.DeleteClusterInput{
				Deleter: mgmtClient,
				Cluster: clusterToDelete,
			})
			framework.WaitForClusterDeleted(ctx, framework.WaitForClusterDeletedInput{
				ClusterProxy:         bootstrapClusterProxy,
				ClusterctlConfigPath: clusterctlConfigPath,
				Cluster:              clusterToDelete,
				ArtifactFolder:       artifactFolder,
			}, e2eConfig.GetIntervals("default", "wait-aro-hcp-delete")...)
			clusterStillExists := &clusterv1.Cluster{}
			if err := mgmtClient.Get(ctx, clusterKey, clusterStillExists); !apierrors.IsNotFound(err) {
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "ARO HCP E2E Cluster %s still exists after deletion", clusterKey)
				return
			}
		}

		By("Waiting for ARO infrastructure and embedded ASO resources to be deleted")
		resourcesDeleted := Eventually(func(ctx context.Context) error {
			for _, resource := range managedResourcesToDelete {
				obj := &unstructured.Unstructured{}
				obj.SetGroupVersionKind(resource.gvk)
				err := mgmtClient.Get(ctx, resource.key, obj)
				if err == nil {
					return fmt.Errorf("managed resource %s %s still exists", resource.gvk.String(), resource.key)
				}
				if !apierrors.IsNotFound(err) {
					return err
				}
			}
			return nil
		}, e2eConfig.GetIntervals("default", "wait-aro-hcp-delete")...).WithContext(ctx).Should(Succeed())
		if !resourcesDeleted {
			return
		}
		clusterCleanupCompleted = true
	})

	asoCredentialSecretName := e2eConfig.MustGetVariable("ASO_CREDENTIAL_SECRET_NAME")
	asoCredentialSecretMode := e2eConfig.MustGetVariable("ASO_CREDENTIAL_SECRET_MODE")
	Expect(asoCredentialSecretMode).To(Equal(string(asoConfig.WorkloadIdentityAuthMode)), "ARO HCP E2E uses CAPZ's Workload Identity setup for ASO credentials")
	identityName := e2eConfig.MustGetVariable(ClusterIdentityName)
	identityClientID := e2eConfig.MustGetVariable(AzureClientIDUserAssignedIdentity)
	tenantID := e2eConfig.MustGetVariable(AzureTenantID)
	subscriptionID := e2eConfig.MustGetVariable(AzureSubscriptionID)
	externalAuthAppID := e2eConfig.MustGetVariable(aroHCPExternalAuthAppID)
	Expect(externalAuthAppID).NotTo(BeEmpty(), "set %s to a pre-registered Microsoft Entra application client ID", aroHCPExternalAuthAppID)
	createdAt := time.Now().UTC()
	sweeperTags := ""
	if os.Getenv("ARO_HCP_SWEEPER_TAGS") == "true" {
		sweeperTags = fmt.Sprintf("        e2e.aro-hcp-ci.redhat.com: \"true\"\n        deleteAfter.aro-hcp-ci.redhat.com: %q", createdAt.Add(6*time.Hour).Format(time.RFC3339))
	}

	template, err := os.ReadFile(aroHCPTemplatePath)
	Expect(err).NotTo(HaveOccurred(), "failed to read ARO HCP E2E template %q", aroHCPTemplatePath)
	templateVariables := map[string]string{
		"CLUSTER_NAME":               clusterName,
		"NAMESPACE":                  namespaceName,
		"CLUSTER_IDENTITY_NAME":      identityName,
		"ASO_CREDENTIAL_SECRET_NAME": asoCredentialSecretName,
		"AZURE_SUBSCRIPTION_ID":      subscriptionID,
		"AZURE_TENANT_ID":            tenantID,
		"AZURE_LOCATION":             e2eConfig.MustGetVariable(AzureLocation),
		"CREATED_AT":                 createdAt.Format(time.RFC3339),
		"SWEEPER_TAGS":               sweeperTags,
		aroHCPExternalAuthAppID:      externalAuthAppID,
	}
	templateYAML := os.Expand(string(template), func(key string) string {
		return templateVariables[key]
	})
	Expect(templateYAML).NotTo(ContainSubstring("${"), "ARO HCP template has unresolved variables")

	setAROHCPASOImage(ctx, mgmtClient)
	asoCredential := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: asoCredentialSecretName},
		StringData: map[string]string{
			asoConfig.AzureSubscriptionID: subscriptionID,
			asoConfig.AzureTenantID:       tenantID,
			asoConfig.AzureClientID:       identityClientID,
			asoConfig.AuthMode:            asoCredentialSecretMode,
		},
	}
	By(fmt.Sprintf("Creating ASO workload identity Secret %s/%s", namespaceName, asoCredentialSecretName))
	Expect(mgmtClient.Create(ctx, asoCredential)).To(Succeed(), "failed to create the ASO E2E credential Secret")
	identity := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: identityName, Namespace: namespaceName},
		Spec: infrav1.AzureClusterIdentitySpec{
			AllowedNamespaces: &infrav1.AllowedNamespaces{},
			ClientID:          identityClientID,
			TenantID:          tenantID,
			Type:              infrav1.WorkloadIdentity,
		},
	}
	Expect(mgmtClient.Create(ctx, identity)).To(Succeed(), "failed to create the ARO HCP AzureClusterIdentity")

	result := &clusterctl.ApplyCustomClusterTemplateAndWaitResult{}
	clusterctl.ApplyCustomClusterTemplateAndWait(ctx, clusterctl.ApplyCustomClusterTemplateAndWaitInput{
		ClusterProxy:                 bootstrapClusterProxy,
		CustomTemplateYAML:           []byte(templateYAML),
		ClusterName:                  clusterName,
		Namespace:                    namespaceName,
		WaitForClusterIntervals:      e2eConfig.GetIntervals("default", "wait-aro-hcp-cluster"),
		WaitForControlPlaneIntervals: e2eConfig.GetIntervals("default", "wait-aro-hcp-cluster"),
		WaitForMachinePools:          e2eConfig.GetIntervals("default", "wait-aro-hcp-machines"),
		ControlPlaneWaiters: clusterctl.ControlPlaneWaiters{
			WaitForControlPlaneInitialized: func(ctx context.Context, _ clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
				waitForAROControlPlane(ctx, mgmtClient, result.Cluster)
			},
			// ARO HCP has a hosted control plane and does not create CAPI control-plane Machines.
			WaitForControlPlaneMachinesReady: func(context.Context, clusterctl.ApplyCustomClusterTemplateAndWaitInput, *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
			},
		},
	}, result)

	cluster = result.Cluster
	controlPlane := &cplane.AROControlPlane{}
	Expect(mgmtClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: cluster.Spec.ControlPlaneRef.Name}, controlPlane)).To(Succeed())
	managedResourcesToDelete = append(managedResourcesToDelete, aroHCPResourceReferences(controlPlane.Spec.Resources, namespaceName)...)
	Expect(controlPlane.Spec.IdentityRef).NotTo(BeNil())
	Expect(controlPlane.Spec.IdentityRef.Name).To(Equal(identityName))
	Expect(controlPlane.Spec.IdentityRef.Namespace).To(Equal(namespaceName))
	expectAROResourceCredentials(controlPlane.Spec.Resources, asoCredentialSecretName)
	Expect(hasAROResource(controlPlane.Spec.Resources, aroHCPAPIGroup, aroHCPAPIVersion, aroHCPControlPlaneKind)).To(BeTrue(), "template must use the ASO API generated from ARM API 2026-09-01-preview")
	Expect(hasAROResource(controlPlane.Spec.Resources, aroHCPAPIGroup, aroHCPAPIVersion, aroHCPExternalAuthKind)).To(BeTrue(), "template must include ExternalAuth using ASO API v20260901preview")
	Expect(hasAROClusterEncryption(controlPlane.Spec.Resources)).To(BeTrue(), "template must configure etcd KMS encryption")

	aroCluster := &infrav2.AROCluster{}
	Expect(mgmtClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: cluster.Spec.InfrastructureRef.Name}, aroCluster)).To(Succeed())
	managedResourcesToDelete = append(managedResourcesToDelete, aroHCPResourceReferences(aroCluster.Spec.Resources, namespaceName)...)
	expectAROResourceCredentials(aroCluster.Spec.Resources, asoCredentialSecretName)
	Expect(aroCluster.Status.Ready).To(BeTrue())
	Expect(aroCluster.Status.Initialization).NotTo(BeNil())
	Expect(aroCluster.Status.Initialization.Provisioned).To(BeTrue())
	Expect(aroConditionIsTrue(aroCluster.Status.Conditions, infrav2.ResourcesReadyCondition)).To(BeTrue())
	Expect(hasAROResource(aroCluster.Spec.Resources, keyVaultAPIGroup, "v1api20230701", aroHCPVaultKind)).To(BeTrue(), "template must include the Key Vault in AROCluster resources")

	var machinePool *clusterv1.MachinePool
	for _, candidate := range result.MachinePools {
		if candidate.Spec.Template.Spec.InfrastructureRef.Kind == infrav2.AROMachinePoolKind {
			machinePool = candidate
			break
		}
	}
	Expect(machinePool).NotTo(BeNil(), "template must define a CAPI MachinePool backed by AROMachinePool")
	aroMachinePool := &infrav2.AROMachinePool{}
	Expect(mgmtClient.Get(ctx, types.NamespacedName{
		Namespace: namespaceName,
		Name:      machinePool.Spec.Template.Spec.InfrastructureRef.Name,
	}, aroMachinePool)).To(Succeed())
	managedResourcesToDelete = append(managedResourcesToDelete, aroHCPResourceReferences(aroMachinePool.Spec.Resources, namespaceName)...)
	expectAROResourceCredentials(aroMachinePool.Spec.Resources, asoCredentialSecretName)
	Expect(hasAROResource(aroMachinePool.Spec.Resources, aroHCPAPIGroup, aroHCPAPIVersion, aroHCPNodePoolKind)).To(BeTrue(), "template must use the ASO API generated from ARM API 2026-09-01-preview")

	initialReplicas := ptr.Deref(machinePool.Spec.Replicas, 0)
	Expect(initialReplicas).To(Equal(int32(2)), "ARO HCP E2E template must start with two workers to test 2->3->2 scaling")
	By("Waiting for the initial ARO HCP machine pool workers")
	framework.WaitForMachinePoolNodesToExist(ctx, framework.WaitForMachinePoolNodesToExistInput{
		Getter:      mgmtClient,
		MachinePool: machinePool,
	}, e2eConfig.GetIntervals("default", "wait-aro-hcp-machines")...)
	workloadCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespaceName, clusterName)
	assertAROHPReadyNodes(ctx, workloadCluster, initialReplicas)
	waitForExternalAuthReady(ctx, mgmtClient, cluster)

	By("Scaling the ARO HCP node pool out and back in")
	scaleAROMachinePool(ctx, mgmtClient, machinePool, initialReplicas+1)
	framework.WaitForMachinePoolNodesToExist(ctx, framework.WaitForMachinePoolNodesToExistInput{
		Getter:      mgmtClient,
		MachinePool: machinePool,
	}, e2eConfig.GetIntervals("default", "wait-aro-hcp-machines")...)
	assertAROMachinePoolReplicas(ctx, mgmtClient, aroMachinePool.Name, namespaceName, initialReplicas+1)
	assertAROHPReadyNodes(ctx, workloadCluster, initialReplicas+1)

	scaleAROMachinePool(ctx, mgmtClient, machinePool, initialReplicas)
	framework.WaitForMachinePoolNodesToExist(ctx, framework.WaitForMachinePoolNodesToExistInput{
		Getter:      mgmtClient,
		MachinePool: machinePool,
	}, e2eConfig.GetIntervals("default", "wait-aro-hcp-machines")...)
	assertAROMachinePoolReplicas(ctx, mgmtClient, aroMachinePool.Name, namespaceName, initialReplicas)
	assertAROHPReadyNodes(ctx, workloadCluster, initialReplicas)
}

func setAROHCPASOImage(ctx context.Context, mgmtClient client.Client) {
	image := os.Getenv("ASO_IMAGE")
	if image == "" {
		return
	}
	pullPolicy := corev1.PullAlways

	key := types.NamespacedName{Namespace: "capz-system", Name: "azureserviceoperator-controller-manager"}
	By(fmt.Sprintf("Setting the ASO controller image to %s", image))
	Eventually(func(ctx context.Context) error {
		deployment := &appsv1.Deployment{}
		if err := mgmtClient.Get(ctx, key, deployment); err != nil {
			return err
		}
		for i := range deployment.Spec.Template.Spec.Containers {
			if deployment.Spec.Template.Spec.Containers[i].Name != "manager" {
				continue
			}
			if deployment.Spec.Template.Spec.Containers[i].Image == image && deployment.Spec.Template.Spec.Containers[i].ImagePullPolicy == pullPolicy {
				return nil
			}
			deployment.Spec.Template.Spec.Containers[i].Image = image
			deployment.Spec.Template.Spec.Containers[i].ImagePullPolicy = pullPolicy
			return mgmtClient.Update(ctx, deployment)
		}
		return fmt.Errorf("ASO manager container was not found in deployment %s", key)
	}, e2eConfig.GetIntervals("default", "wait-aro-hcp-aso-rollout")...).WithContext(ctx).Should(Succeed())

	By("Waiting for the ASO controller rollout")
	Eventually(func(ctx context.Context) error {
		deployment := &appsv1.Deployment{}
		if err := mgmtClient.Get(ctx, key, deployment); err != nil {
			return err
		}
		replicas := int32(1)
		if deployment.Spec.Replicas != nil {
			replicas = *deployment.Spec.Replicas
		}
		if deployment.Status.ObservedGeneration < deployment.Generation || deployment.Status.UpdatedReplicas < replicas || deployment.Status.AvailableReplicas < replicas {
			return fmt.Errorf("ASO deployment %s has not rolled out yet", key)
		}
		return nil
	}, e2eConfig.GetIntervals("default", "wait-aro-hcp-aso-rollout")...).WithContext(ctx).Should(Succeed())
}

func waitForAROControlPlane(ctx context.Context, getter client.Reader, cluster *clusterv1.Cluster) {
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.ControlPlaneRef.Name}
	By("Waiting for ARO HCP and etcd encryption to become ready")
	Eventually(func(ctx context.Context) error {
		controlPlane := &cplane.AROControlPlane{}
		if err := getter.Get(ctx, key, controlPlane); err != nil {
			return err
		}
		if !controlPlane.Status.Ready || controlPlane.Status.Initialization == nil || !controlPlane.Status.Initialization.ControlPlaneInitialized {
			return fmt.Errorf("AROControlPlane %s is not initialized yet", key)
		}
		if !aroConditionIsTrue(controlPlane.Status.Conditions, cplane.HcpClusterReadyCondition) {
			return fmt.Errorf("AROControlPlane %s has not reported HcpClusterReady", key)
		}
		if !aroConditionIsTrue(controlPlane.Status.Conditions, cplane.EncryptionKeyReadyCondition) {
			return fmt.Errorf("AROControlPlane %s has not reported EncryptionKeyReady", key)
		}
		return nil
	}, e2eConfig.GetIntervals("default", "wait-aro-hcp-cluster")...).WithContext(ctx).Should(Succeed())
}

func waitForExternalAuthReady(ctx context.Context, getter client.Reader, cluster *clusterv1.Cluster) {
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.ControlPlaneRef.Name}
	By("Waiting for external authentication after the ARO HCP machine pool workers are ready")
	Eventually(func(ctx context.Context) error {
		controlPlane := &cplane.AROControlPlane{}
		if err := getter.Get(ctx, key, controlPlane); err != nil {
			return err
		}
		if !aroConditionIsTrue(controlPlane.Status.Conditions, cplane.ExternalAuthReadyCondition) {
			return fmt.Errorf("AROControlPlane %s has not reported ExternalAuthReady", key)
		}
		return nil
	}, e2eConfig.GetIntervals("default", "wait-aro-hcp-cluster")...).WithContext(ctx).Should(Succeed())
}

func aroConditionIsTrue(conditions []metav1.Condition, conditionType string) bool {
	condition := aroCondition(conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func aroCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func scaleAROMachinePool(ctx context.Context, mgmtClient client.Client, machinePool *clusterv1.MachinePool, replicas int32) {
	patchHelper, err := patch.NewHelper(machinePool, mgmtClient)
	Expect(err).NotTo(HaveOccurred())
	machinePool.Spec.Replicas = ptr.To(replicas)
	Eventually(func(ctx context.Context) error {
		return patchHelper.Patch(ctx, machinePool)
	}, 3*time.Minute, 10*time.Second).WithContext(ctx).Should(Succeed())
}

func assertAROMachinePoolReplicas(ctx context.Context, getter client.Reader, name, namespace string, replicas int32) {
	Eventually(func(ctx context.Context) (int32, error) {
		machinePool := &infrav2.AROMachinePool{}
		if err := getter.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, machinePool); err != nil {
			return 0, err
		}
		if !machinePool.Status.Ready {
			return machinePool.Status.Replicas, fmt.Errorf("AROMachinePool %s is not ready", name)
		}
		return machinePool.Status.Replicas, nil
	}, e2eConfig.GetIntervals("default", "wait-aro-hcp-machines")...).WithContext(ctx).Should(Equal(replicas))
}

func assertAROHPReadyNodes(ctx context.Context, workloadCluster framework.ClusterProxy, replicas int32) {
	Eventually(func(ctx context.Context) (int32, error) {
		nodes := &corev1.NodeList{}
		if err := workloadCluster.GetClient().List(ctx, nodes); err != nil {
			return 0, err
		}
		var ready int32
		for _, node := range nodes.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					ready++
					break
				}
			}
		}
		return ready, nil
	}, e2eConfig.GetIntervals("default", "wait-aro-hcp-machines")...).WithContext(ctx).Should(BeNumerically(">=", replicas))
}

func hasAROResource(resources []runtime.RawExtension, group, version, kind string) bool {
	for _, resource := range aroUnstructuredResources(resources) {
		if resource.GroupVersionKind().Group == group && resource.GroupVersionKind().Version == version && resource.GetKind() == kind {
			return true
		}
	}
	return false
}

func hasAROClusterEncryption(resources []runtime.RawExtension) bool {
	for _, resource := range aroUnstructuredResources(resources) {
		if resource.GroupVersionKind().Group != aroHCPAPIGroup || resource.GroupVersionKind().Version != aroHCPAPIVersion || resource.GetKind() != aroHCPControlPlaneKind {
			continue
		}
		_, found, err := unstructured.NestedFieldNoCopy(resource.Object, "spec", "properties", "etcd", "dataEncryption", "customerManaged", "kms")
		Expect(err).NotTo(HaveOccurred())
		if found {
			return true
		}
	}
	return false
}

func expectAROResourceCredentials(resources []runtime.RawExtension, secretName string) {
	asoResources := aroUnstructuredResources(resources)
	Expect(asoResources).NotTo(BeEmpty())
	for _, resource := range asoResources {
		Expect(resource.GetAnnotations()).To(HaveKeyWithValue("serviceoperator.azure.com/credential-from", secretName))
	}
}

func aroUnstructuredResources(resources []runtime.RawExtension) []*unstructured.Unstructured {
	result := make([]*unstructured.Unstructured, 0, len(resources))
	for _, resource := range resources {
		obj := &unstructured.Unstructured{}
		if len(resource.Raw) > 0 {
			Expect(obj.UnmarshalJSON(resource.Raw)).To(Succeed())
		} else if resource.Object != nil {
			content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(resource.Object)
			Expect(err).NotTo(HaveOccurred())
			obj.Object = content
		} else {
			Fail("ARO HCP E2E template contains an empty embedded ASO resource")
		}
		result = append(result, obj)
	}
	return result
}

func aroHCPResourceReferences(resources []runtime.RawExtension, namespace string) []aroHCPResourceReference {
	result := make([]aroHCPResourceReference, 0, len(resources))
	for _, resource := range aroUnstructuredResources(resources) {
		result = append(result, aroHCPResourceReference{
			key: client.ObjectKey{Namespace: namespace, Name: resource.GetName()},
			gvk: resource.GroupVersionKind(),
		})
	}
	return result
}

func aroHCPManagedResourceReferences(ctx context.Context, getter client.Client, cluster *clusterv1.Cluster) ([]aroHCPResourceReference, error) {
	var references []aroHCPResourceReference
	appendObjectReference := func(ref clusterv1.ContractVersionedObjectReference, groupVersion schema.GroupVersion) {
		references = append(references, aroHCPResourceReference{
			key: client.ObjectKey{Namespace: cluster.Namespace, Name: ref.Name},
			gvk: groupVersion.WithKind(ref.Kind),
		})
	}

	if cluster.Spec.ControlPlaneRef.Name != "" {
		appendObjectReference(cluster.Spec.ControlPlaneRef, cplane.GroupVersion)
		controlPlane := &cplane.AROControlPlane{}
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.ControlPlaneRef.Name}
		if err := getter.Get(ctx, key, controlPlane); err == nil {
			references = append(references, aroHCPResourceReferences(controlPlane.Spec.Resources, cluster.Namespace)...)
		} else if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	if cluster.Spec.InfrastructureRef.Name != "" {
		appendObjectReference(cluster.Spec.InfrastructureRef, infrav2.GroupVersion)
		aroCluster := &infrav2.AROCluster{}
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.InfrastructureRef.Name}
		if err := getter.Get(ctx, key, aroCluster); err == nil {
			references = append(references, aroHCPResourceReferences(aroCluster.Spec.Resources, cluster.Namespace)...)
		} else if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	machinePools := &clusterv1.MachinePoolList{}
	if err := getter.List(ctx, machinePools, client.InNamespace(cluster.Namespace), client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name}); err != nil {
		return nil, err
	}
	for i := range machinePools.Items {
		infrastructureRef := machinePools.Items[i].Spec.Template.Spec.InfrastructureRef
		if infrastructureRef.Kind != infrav2.AROMachinePoolKind {
			continue
		}
		if infrastructureRef.Name == "" {
			continue
		}
		appendObjectReference(infrastructureRef, infrav2.GroupVersion)
		aroMachinePool := &infrav2.AROMachinePool{}
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: infrastructureRef.Name}
		if err := getter.Get(ctx, key, aroMachinePool); err == nil {
			references = append(references, aroHCPResourceReferences(aroMachinePool.Spec.Resources, cluster.Namespace)...)
		} else if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	return references, nil
}

func uniqueAROResourceReferences(referenceSets ...[]aroHCPResourceReference) []aroHCPResourceReference {
	var result []aroHCPResourceReference
	seen := make(map[string]struct{})
	for _, references := range referenceSets {
		for _, reference := range references {
			key := reference.gvk.String() + "/" + reference.key.String()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, reference)
		}
	}
	return result
}
