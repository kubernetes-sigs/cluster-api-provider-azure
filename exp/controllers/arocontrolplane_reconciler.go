/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"fmt"
	"time"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/keyvaults"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/controllers"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/mutators"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const (
	// provisioningReason is used as a generic reason for resources being provisioned.
	provisioningReason = "Provisioning"
	// pauseValue is the value used for ASO pause annotations.
	pauseValue = "true"
	// hcpOpenShiftClustersExternalAuthKind is the kind name for HcpOpenShiftClustersExternalAuth resources.
	hcpOpenShiftClustersExternalAuthKind = "HcpOpenShiftClustersExternalAuth"
)

// resourceReconciler interface for ASO resource reconciliation.
type resourceReconciler interface {
	// Reconcile reconciles resources defined by this object and updates this object's status to reflect the
	// state of the specified resources.
	Reconcile(context.Context) error

	// Pause stops ASO from continuously reconciling the specified resources.
	Pause(context.Context) error

	// Delete begins deleting the specified resources and updates the object's status to reflect the state of
	// the specified resources.
	Delete(context.Context) error
}

// aroControlPlaneService is the reconciler called by the AROControlPlane controller.
type aroControlPlaneService struct {
	scope                 *scope.AROControlPlaneScope
	keyVaultSvc           azure.ServiceReconciler
	skuCache              *resourceskus.Cache
	Reconcile             func(context.Context) error
	Pause                 func(context.Context) error
	Delete                func(context.Context) error
	kubeclient            client.Client
	newResourceReconciler func(*cplane.AROControlPlane, []*unstructured.Unstructured) resourceReconciler
}

// newAROControlPlaneService populates all the services based on input scope.
func newAROControlPlaneService(scope *scope.AROControlPlaneScope) (*aroControlPlaneService, error) {
	location, err := scope.Location()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine cluster location")
	}

	skuCache, err := resourceskus.GetCache(scope, location)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}
	keyVaultSvc, err := keyvaults.New(scope)
	if err != nil {
		return nil, err
	}
	acs := &aroControlPlaneService{
		kubeclient:  scope.Client,
		scope:       scope,
		keyVaultSvc: keyVaultSvc,
		skuCache:    skuCache,
		newResourceReconciler: func(controlPlane *cplane.AROControlPlane, resources []*unstructured.Unstructured) resourceReconciler {
			return controllers.NewResourceReconciler(scope.Client, resources, controlPlane)
		},
	}
	acs.Reconcile = acs.reconcile
	acs.Pause = acs.pause
	acs.Delete = acs.delete

	return acs, nil
}

// Reconcile reconciles the AROControlPlane using resources mode.
func (s *aroControlPlaneService) reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.Reconcile")
	defer done()

	log.Info("Using resources mode for AROControlPlane reconciliation")
	return s.reconcileResources(ctx)
}

// Pause pauses the AROControlPlane resources.
func (s *aroControlPlaneService) pause(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.Pause")
	defer done()

	resources, err := mutators.ToUnstructured(ctx, s.scope.ControlPlane.Spec.Resources)
	if err != nil {
		return errors.Wrap(err, "failed to convert resources to unstructured")
	}

	resourceReconciler := s.newResourceReconciler(s.scope.ControlPlane, resources)
	return resourceReconciler.Pause(ctx)
}

// Delete deletes the AROControlPlane resources.
func (s *aroControlPlaneService) delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.Delete")
	defer done()

	log.V(4).Info("Using resources mode for AROControlPlane deletion")
	return s.deleteResources(ctx)
}

func (s *aroControlPlaneService) reconcileKubeconfig(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.reconcileKubeconfig")
	defer done()

	// Check if reconciliation is actually needed (metadata-based validation)
	if !s.scope.ShouldReconcileKubeconfig(ctx) {
		log.V(4).Info("kubeconfig is still valid, skipping reconciliation")
		return nil
	}

	log.V(4).Info("reconciling kubeconfig secret")

	// Get the admin kubeconfig data from ASO-created secret
	kubeconfigData, err := s.getKubeconfigFromASOSecret(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig from ASO secret")
	}
	if len(kubeconfigData) == 0 {
		return errors.New("no kubeconfig data available from ASO secret")
	}

	// Parse the kubeconfig to work with it
	kubeconfigFile, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		return errors.Wrap(err, "failed to parse kubeconfig data")
	}

	// Handle certificate authority data
	var caData []byte
	if len(kubeconfigFile.Contexts) > 0 && len(kubeconfigFile.Clusters) > 0 {
		currentContext := kubeconfigFile.CurrentContext
		if currentContext == "" && len(kubeconfigFile.Contexts) == 1 {
			// Use the only context if current context is not set
			for contextName := range kubeconfigFile.Contexts {
				currentContext = contextName
				break
			}
		}

		if currentContext != "" {
			context := kubeconfigFile.Contexts[currentContext]
			if context != nil && context.Cluster != "" {
				cluster := kubeconfigFile.Clusters[context.Cluster]
				if cluster != nil {
					caData = cluster.CertificateAuthorityData
					if caData == nil && cluster.Server != "" {
						// ASO-created kubeconfig doesn't include CA certificate
						// Use insecure-skip-tls-verify as workaround
						log.V(4).Info("no CA data in kubeconfig from ASO, setting insecure skip TLS verify", "server", cluster.Server)
						cluster.InsecureSkipTLSVerify = true
					}

					if caData != nil {
						// Create/update CA secret
						caSecret := s.scope.MakeClusterCA()
						if _, err := controllerutil.CreateOrUpdate(ctx, s.kubeclient, caSecret, func() error {
							caSecret.Data = map[string][]byte{
								secret.TLSCrtDataName: caData,
								secret.TLSKeyDataName: []byte("placeholder"), // Required by CAPI
							}
							return nil
						}); err != nil {
							return errors.Wrap(err, "failed to reconcile certificate authority data secret")
						}
					}
				}
			}
		}
	}

	// Serialize the updated kubeconfig
	kubeconfigAdmin, err := clientcmd.Write(*kubeconfigFile)
	if err != nil {
		return errors.Wrap(err, "failed to serialize kubeconfig")
	}

	// Update the ASO-created kubeconfig secret directly
	// We need to preserve the secret's existing metadata (owner references from ASO)
	// and only update the data and add our tracking annotations
	secretName := secret.Name(s.scope.Cluster.Name, secret.Kubeconfig)
	kubeConfigSecret := s.scope.MakeEmptyKubeConfigSecret()

	// Get the existing secret to preserve its metadata
	if err := s.kubeclient.Get(ctx, client.ObjectKey{
		Namespace: s.scope.Namespace(),
		Name:      secretName,
	}, &kubeConfigSecret); err != nil {
		return errors.Wrap(err, "failed to get existing kubeconfig secret")
	}

	// Update the secret data with our modified kubeconfig
	if kubeConfigSecret.Data == nil {
		kubeConfigSecret.Data = make(map[string][]byte)
	}
	kubeConfigSecret.Data[secret.KubeconfigDataName] = kubeconfigAdmin

	// Ensure proper labels
	if kubeConfigSecret.Labels == nil {
		kubeConfigSecret.Labels = make(map[string]string)
	}
	kubeConfigSecret.Labels[clusterv1.ClusterNameLabel] = s.scope.ClusterName()

	// Add annotations for tracking
	if kubeConfigSecret.Annotations == nil {
		kubeConfigSecret.Annotations = make(map[string]string)
	}
	kubeConfigSecret.Annotations["aro.azure.com/kubeconfig-last-updated"] = time.Now().Format(time.RFC3339)

	// Remove refresh-needed annotation if it exists
	delete(kubeConfigSecret.Annotations, "aro.azure.com/kubeconfig-refresh-needed")

	// Update the secret (preserving existing owner references from ASO)
	if err := s.kubeclient.Update(ctx, &kubeConfigSecret); err != nil {
		return errors.Wrap(err, "failed to update kubeconfig secret")
	}

	// TODO: Add user kubeconfig support if needed
	// This would follow the same pattern as admin kubeconfig

	log.V(4).Info("successfully reconciled kubeconfig secret")
	return nil
}

// reconcileResources handles reconciliation when spec.resources is specified.
func (s *aroControlPlaneService) reconcileResources(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.reconcileResources")
	defer done()

	log.V(4).Info("Reconciling AROControlPlane using resources mode")

	// Wait for AROCluster infrastructure to be ready before creating HcpOpenShiftCluster
	// This ensures all networking, identities, and role assignments exist first
	infraReady, err := s.isInfrastructureReady(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check infrastructure readiness")
	}
	if !infraReady {
		log.V(4).Info("Waiting for AROCluster infrastructure to be ready")
		return nil
	}
	log.Info("AROCluster infrastructure is ready, proceeding with HcpOpenShiftCluster creation")

	// Check if HcpOpenShiftCluster has ETCD encryption configured
	encryptionConfigured := false
	for _, resource := range s.scope.ControlPlane.Spec.Resources {
		// Convert RawExtension to Unstructured
		u := &unstructured.Unstructured{}
		if err := u.UnmarshalJSON(resource.Raw); err != nil {
			log.V(4).Info("Failed to unmarshal resource, skipping encryption check", "error", err)
			continue
		}

		if u.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
			u.GroupVersionKind().Kind == "HcpOpenShiftCluster" {
			// Check if etcd.dataEncryption.customerManaged.kms is configured
			etcdPath := []string{"spec", "properties", "etcd", "dataEncryption", "customerManaged", "kms"}
			if _, hasKMS, _ := unstructured.NestedFieldNoCopy(u.UnstructuredContent(), etcdPath...); hasKMS {
				encryptionConfigured = true
				log.V(4).Info("HcpOpenShiftCluster has ETCD encryption with KMS configured")
				break
			}
		}
	}

	// Ensure KeyVault encryption key exists before reconciling resources
	// This is required for HcpOpenShiftCluster ETCD encryption
	keyVaultSvc, err := s.getService("keyvault")
	if err != nil {
		return errors.Wrap(err, "failed to get keyvault service")
	}

	// Set condition before reconciling KeyVault (if encryption is configured)
	if encryptionConfigured {
		vaultName, keyName, keyVersion := s.scope.GetVaultInfo()

		if vaultName == nil || keyName == nil {
			// Vault/key not configured yet
			conditions.Set(s.scope.ControlPlane, metav1.Condition{
				Type:    string(cplane.EncryptionKeyReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  "WaitingForVault",
				Message: "Waiting for KeyVault to be created",
			})
		} else if keyVersion == nil {
			// Vault exists but key not created yet
			conditions.Set(s.scope.ControlPlane, metav1.Condition{
				Type:    string(cplane.EncryptionKeyReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  "CreatingKey",
				Message: fmt.Sprintf("Creating encryption key '%s' in vault '%s'", ptr.Deref(keyName, ""), ptr.Deref(vaultName, "")),
			})
		}
	}

	if err := keyVaultSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to ensure KeyVault encryption key")
	}

	// Update condition after KeyVault reconciliation (if encryption is configured)
	if encryptionConfigured {
		vaultName, keyName, keyVersion := s.scope.GetVaultInfo()
		if keyVersion != nil {
			// Key is ready
			conditions.Set(s.scope.ControlPlane, metav1.Condition{
				Type:   string(cplane.EncryptionKeyReadyCondition),
				Status: metav1.ConditionTrue,
				Reason: "KeyReady",
				Message: fmt.Sprintf("Encryption key '%s' version '%s' ready in vault '%s'",
					ptr.Deref(keyName, ""), ptr.Deref(keyVersion, ""), ptr.Deref(vaultName, "")),
			})
		}
	}

	// Apply mutators to set defaults, owner references, and encryption key version
	// The encryption key mutator must run after keyvault service to get the actual key version
	resources, err := mutators.ApplyMutators(
		ctx,
		s.scope.ControlPlane.Spec.Resources,
		mutators.SetHcpOpenShiftClusterDefaults(s.kubeclient, s.scope.ControlPlane, s.scope.Cluster),
		mutators.SetHcpOpenShiftClusterEncryptionKey(s.scope),
	)
	if err != nil {
		return errors.Wrap(err, "failed to apply mutators")
	}

	// Check if ExternalAuth is defined in original resources (before filtering)
	hasExternalAuthInSpec := false
	for _, resource := range resources {
		if resource.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
			resource.GroupVersionKind().Kind == hcpOpenShiftClustersExternalAuthKind {
			hasExternalAuthInSpec = true
			break
		}
	}

	// Filter out ExternalAuth resources if no node pool is ready yet
	// This prevents ASO from attempting to create ExternalAuth before node pools exist,
	// which would result in terminal Azure API errors
	var externalAuthFiltered bool
	resources, externalAuthFiltered, err = s.filterExternalAuthUntilNodePoolReady(ctx, resources)
	if err != nil {
		return errors.Wrap(err, "failed to filter ExternalAuth resources")
	}

	// Set condition if ExternalAuth was filtered out (waiting for node pool)
	if hasExternalAuthInSpec && externalAuthFiltered {
		conditions.Set(s.scope.ControlPlane, metav1.Condition{
			Type:    string(cplane.ExternalAuthReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "WaitingForNodePool",
			Message: "ExternalAuth creation deferred until at least one node pool is ready",
		})
	}

	// Use the ResourceReconciler to apply resources
	resourceReconciler := s.newResourceReconciler(s.scope.ControlPlane, resources)

	if err := resourceReconciler.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile ASO resources")
	}

	// Set conditions for infrastructure resources based on their ready status
	s.setInfrastructureConditions(ctx)

	// Find HcpOpenShiftCluster to extract status information
	// We need to do this BEFORE checking if all resources are ready, because:
	// 1. HcpOpenShiftClustersExternalAuth requires a ready node pool
	// 2. Node pools require control plane to be initialized first
	// 3. Control plane initialization requires HcpOpenShiftCluster API URL
	var hcpClusterName string
	for _, resource := range resources {
		if resource.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
			resource.GroupVersionKind().Kind == "HcpOpenShiftCluster" {
			hcpClusterName = resource.GetName()
			break
		}
	}

	if hcpClusterName == "" {
		return errors.New("no HcpOpenShiftCluster found in resources")
	}

	// Get the HcpOpenShiftCluster to extract status
	hcpCluster := &asoredhatopenshiftv1.HcpOpenShiftCluster{}
	if err := s.kubeclient.Get(ctx, client.ObjectKey{
		Namespace: s.scope.ControlPlane.Namespace,
		Name:      hcpClusterName,
	}, hcpCluster); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return errors.Wrap(err, "failed to get HcpOpenShiftCluster")
		}
		// HcpOpenShiftCluster doesn't exist yet (being created)
		// Set HcpClusterReadyCondition to False before returning to ensure proper state
		conditions.Set(s.scope.ControlPlane, metav1.Condition{
			Type:    string(cplane.HcpClusterReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "HcpOpenShiftClusterNotFound",
			Message: fmt.Sprintf("HcpOpenShiftCluster %s does not exist yet", hcpClusterName),
		})
		log.V(4).Info("HcpOpenShiftCluster not found yet, skipping status extraction", "name", hcpClusterName)
		return nil
	}

	// Extract status information from HcpOpenShiftCluster
	if hcpCluster.Status.Id != nil {
		s.scope.ControlPlane.Status.ID = *hcpCluster.Status.Id
	}

	if hcpCluster.Status.Properties != nil {
		if hcpCluster.Status.Properties.Console != nil && hcpCluster.Status.Properties.Console.Url != nil {
			s.scope.ControlPlane.Status.ConsoleURL = *hcpCluster.Status.Properties.Console.Url
		}

		if hcpCluster.Status.Properties.Api != nil && hcpCluster.Status.Properties.Api.Url != nil {
			s.scope.ControlPlane.Status.APIURL = *hcpCluster.Status.Properties.Api.Url
		}

		if hcpCluster.Status.Properties.Version != nil && hcpCluster.Status.Properties.Version.Id != nil {
			s.scope.ControlPlane.Status.Version = *hcpCluster.Status.Properties.Version.Id
		}
	}

	// Mark HcpCluster condition based on Ready status from HcpOpenShiftCluster
	// Extract the actual error details if the cluster is not ready
	ready := false
	var readyCondition *asoconditions.Condition
	for i, condition := range hcpCluster.Status.Conditions {
		if condition.Type == asoconditions.ConditionTypeReady {
			readyCondition = &hcpCluster.Status.Conditions[i]
			if condition.Status == metav1.ConditionTrue {
				ready = true
			}
			break
		}
	}

	if ready {
		conditions.Set(s.scope.ControlPlane, metav1.Condition{
			Type:   string(cplane.HcpClusterReadyCondition),
			Status: metav1.ConditionTrue,
			Reason: "Succeeded",
		})
	} else {
		// Extract error details from HcpOpenShiftCluster's Ready condition
		reason := provisioningReason
		message := "HcpOpenShiftCluster is not yet ready"

		if readyCondition != nil {
			if readyCondition.Reason != "" {
				reason = readyCondition.Reason
			}
			if readyCondition.Message != "" {
				message = readyCondition.Message
			}

			// If there's an error or warning severity, prepend it to the message for visibility
			if readyCondition.Severity == asoconditions.ConditionSeverityError || readyCondition.Severity == asoconditions.ConditionSeverityWarning {
				message = fmt.Sprintf("[%s] %s", readyCondition.Severity, message)
			}
		}

		conditions.Set(s.scope.ControlPlane, metav1.Condition{
			Type:    string(cplane.HcpClusterReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		})
	}

	// Set ExternalAuth condition if ExternalAuth resource is defined in spec.resources
	// Check if any resource is an ExternalAuth
	hasExternalAuth := false
	for _, resource := range resources {
		if resource.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
			resource.GroupVersionKind().Kind == hcpOpenShiftClustersExternalAuthKind {
			hasExternalAuth = true
			break
		}
	}
	if hasExternalAuth {
		s.setExternalAuthCondition(ctx, resources)
	}

	// Reconcile kubeconfig from ASO secret
	if err := s.reconcileKubeconfig(ctx); err != nil {
		log.V(4).Info("kubeconfig not yet available, will retry", "error", err.Error())
		// Don't fail reconciliation, just wait for next iteration
	}

	return nil
}

// setExternalAuthCondition sets the ExternalAuthReady condition based on HcpOpenShiftClustersExternalAuth status.
func (s *aroControlPlaneService) setExternalAuthCondition(ctx context.Context, resources []*unstructured.Unstructured) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.setExternalAuthCondition")
	defer done()

	// Find HcpOpenShiftClustersExternalAuth resource
	var externalAuthName string
	for _, resource := range resources {
		if resource.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
			resource.GroupVersionKind().Kind == hcpOpenShiftClustersExternalAuthKind {
			externalAuthName = resource.GetName()
			break
		}
	}

	if externalAuthName == "" {
		// No ExternalAuth resource defined
		return
	}

	// Get the HcpOpenShiftClustersExternalAuth to check status
	externalAuth := &asoredhatopenshiftv1.HcpOpenShiftClustersExternalAuth{}
	if err := s.kubeclient.Get(ctx, client.ObjectKey{
		Namespace: s.scope.ControlPlane.Namespace,
		Name:      externalAuthName,
	}, externalAuth); err != nil {
		if client.IgnoreNotFound(err) != nil {
			// Unexpected error getting the resource
			log.Error(err, "failed to get HcpOpenShiftClustersExternalAuth", "name", externalAuthName)
			conditions.Set(s.scope.ControlPlane, metav1.Condition{
				Type:    string(cplane.ExternalAuthReadyCondition),
				Status:  metav1.ConditionUnknown,
				Reason:  "GetResourceFailed",
				Message: fmt.Sprintf("Failed to get ExternalAuth resource: %v", err),
			})
			return
		}
		// Resource not found yet (still being created)
		log.V(4).Info("ExternalAuth resource not found yet, will retry", "name", externalAuthName)
		conditions.Set(s.scope.ControlPlane, metav1.Condition{
			Type:    string(cplane.ExternalAuthReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "ResourceNotFound",
			Message: fmt.Sprintf("ExternalAuth resource %s not yet created", externalAuthName),
		})
		return
	}

	// Check if the resource is paused
	const pauseAnnotation = "reconcile.azure-service-operator.io/pause"
	if externalAuth.GetAnnotations()[pauseAnnotation] == pauseValue {
		conditions.Set(s.scope.ControlPlane, metav1.Condition{
			Type:    string(cplane.ExternalAuthReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "WaitingForNodePool",
			Message: "Waiting for at least one node pool to be ready before configuring external authentication",
		})
		return
	}

	// Check Ready condition from ExternalAuth
	ready := false
	var readyCondition *asoconditions.Condition
	for i, condition := range externalAuth.Status.Conditions {
		if condition.Type == asoconditions.ConditionTypeReady {
			readyCondition = &externalAuth.Status.Conditions[i]
			if condition.Status == metav1.ConditionTrue {
				ready = true
			}
			break
		}
	}

	if ready {
		conditions.Set(s.scope.ControlPlane, metav1.Condition{
			Type:   string(cplane.ExternalAuthReadyCondition),
			Status: metav1.ConditionTrue,
			Reason: "Succeeded",
		})
	} else {
		// Extract error details from ExternalAuth's Ready condition
		reason := provisioningReason
		message := "External authentication is being configured"

		if readyCondition != nil {
			if readyCondition.Reason != "" {
				reason = readyCondition.Reason
			}
			if readyCondition.Message != "" {
				message = readyCondition.Message
			}

			// If there's an error or warning severity, prepend it to the message for visibility
			if readyCondition.Severity == asoconditions.ConditionSeverityError || readyCondition.Severity == asoconditions.ConditionSeverityWarning {
				message = fmt.Sprintf("[%s] %s", readyCondition.Severity, message)
			}
		}

		conditions.Set(s.scope.ControlPlane, metav1.Condition{
			Type:    string(cplane.ExternalAuthReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		})
	}
}

// deleteResources handles deletion when spec.resources is specified.
func (s *aroControlPlaneService) deleteResources(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.deleteResources")
	defer done()

	log.V(4).Info("Deleting AROControlPlane using resources mode")

	// Use the ResourceReconciler to delete resources
	// Pass nil for resources to indicate all should be deleted
	resourceReconciler := s.newResourceReconciler(s.scope.ControlPlane, nil)

	if err := resourceReconciler.Delete(ctx); err != nil {
		return errors.Wrap(err, "failed to delete ASO resources")
	}

	// Check if there are still resources being deleted
	// The ResourceReconciler updates the status with resources that are still deleting
	for _, status := range s.scope.ControlPlane.Status.Resources {
		if !status.Ready {
			log.V(4).Info("waiting for resource to be deleted", "resource", status.Resource.Name)
			return azure.WithTransientError(errors.New("waiting for resources to be deleted"), 15*time.Second)
		}
	}

	return nil
}

func (s *aroControlPlaneService) getService(name string) (azure.ServiceReconciler, error) {
	if name == "keyvault" {
		return s.keyVaultSvc, nil
	}
	return nil, errors.Errorf("service %s not found", name)
}

// getKubeconfigFromASOSecret reads the kubeconfig from the ASO-created secret.
func (s *aroControlPlaneService) getKubeconfigFromASOSecret(ctx context.Context) ([]byte, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.getKubeconfigFromASOSecret")
	defer done()

	// The ASO secret name follows CAPI convention: {cluster-name}-kubeconfig
	secretName := secret.Name(s.scope.Cluster.Name, secret.Kubeconfig)
	asoSecret := s.scope.MakeEmptyKubeConfigSecret()

	log.V(4).Info("reading kubeconfig from ASO secret", "secretName", secretName, "namespace", s.scope.Namespace())

	err := s.kubeclient.Get(ctx, client.ObjectKey{
		Namespace: s.scope.Namespace(),
		Name:      secretName,
	}, &asoSecret)

	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.V(4).Info("ASO kubeconfig secret not yet created, will retry")
			return nil, errors.New("ASO kubeconfig secret not yet available")
		}
		return nil, errors.Wrap(err, "failed to get ASO kubeconfig secret")
	}

	// Extract kubeconfig data from secret
	kubeconfigData, ok := asoSecret.Data[secret.KubeconfigDataName]
	if !ok || len(kubeconfigData) == 0 {
		return nil, errors.Errorf("key %s not found in ASO kubeconfig secret", secret.KubeconfigDataName)
	}

	return kubeconfigData, nil
}

// setInfrastructureConditions sets infrastructure-related conditions based on resource statuses.
func (s *aroControlPlaneService) setInfrastructureConditions(_ context.Context) {
	// Map resource kinds to condition types
	kindToCondition := map[string]clusterv1beta1.ConditionType{
		"ResourceGroup":         infrav1.ResourceGroupReadyCondition,
		"VirtualNetwork":        infrav1.VNetReadyCondition,
		"NetworkSecurityGroup":  infrav1.SecurityGroupsReadyCondition,
		"VirtualNetworksSubnet": infrav1.SubnetsReadyCondition,
		"Vault":                 "VaultReady", // Custom condition for Key Vault
		"UserAssignedIdentity":  "UserAssignedIdentitiesReady",
		"RoleAssignment":        infrav1.RoleAssignmentReadyCondition,
		"HcpOpenShiftCluster":   clusterv1beta1.ConditionType(cplane.HcpClusterReadyCondition),
	}

	// Group resources by kind and check if all resources of each kind are ready
	resourcesByKind := make(map[string][]infrav1.ResourceStatus)
	for _, status := range s.scope.ControlPlane.Status.Resources {
		kind := status.Resource.Kind
		resourcesByKind[kind] = append(resourcesByKind[kind], status)
	}

	// Set conditions for each infrastructure component
	for kind, condType := range kindToCondition {
		resources, exists := resourcesByKind[kind]
		if !exists {
			// Skip if no resources of this kind
			continue
		}

		// Check if all resources of this kind are ready
		allReady := true
		notReadyResources := []string{}
		for _, res := range resources {
			if !res.Ready {
				allReady = false
				notReadyResources = append(notReadyResources, res.Resource.Name)
			}
		}

		if allReady {
			conditions.Set(s.scope.ControlPlane, metav1.Condition{
				Type:   string(condType),
				Status: metav1.ConditionTrue,
				Reason: "Succeeded",
			})
		} else {
			conditions.Set(s.scope.ControlPlane, metav1.Condition{
				Type:    string(condType),
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.CreatingReason,
				Message: fmt.Sprintf("Waiting for %s to be ready: %v", kind, notReadyResources),
			})
		}
	}
}

// isInfrastructureReady checks if the AROCluster infrastructure (networking, identities, roles) is ready
// before creating the HcpOpenShiftCluster. This prevents errors when HcpOpenShiftCluster references
// resources that don't exist yet.
func (s *aroControlPlaneService) isInfrastructureReady(ctx context.Context) (bool, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.isInfrastructureReady")
	defer done()

	// Get the AROCluster from the cluster's infrastructure ref
	aroCluster := &infrav1exp.AROCluster{}
	aroClusterKey := client.ObjectKey{
		Namespace: s.scope.Cluster.Namespace,
		Name:      s.scope.Cluster.Spec.InfrastructureRef.Name,
	}

	if err := s.kubeclient.Get(ctx, aroClusterKey, aroCluster); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return false, errors.Wrapf(err, "failed to get AROCluster %s/%s", aroClusterKey.Namespace, aroClusterKey.Name)
		}
		// AROCluster not found - can't determine readiness
		log.V(4).Info("AROCluster not found, assuming infrastructure not ready", "aroCluster", aroClusterKey.Name)
		return false, nil
	}

	// Check if AROCluster has resources to manage
	if len(aroCluster.Spec.Resources) == 0 {
		// No resources to manage, infrastructure is ready
		log.V(4).Info("AROCluster has no resources to manage, infrastructure ready")
		return true, nil
	}

	// Check the ResourcesReady condition
	for _, condition := range aroCluster.Status.Conditions {
		if condition.Type == string(infrav1exp.ResourcesReadyCondition) {
			if condition.Status == metav1.ConditionTrue {
				log.V(4).Info("AROCluster infrastructure is ready", "aroCluster", aroClusterKey.Name)
				return true, nil
			}
			log.V(4).Info("AROCluster infrastructure not ready yet",
				"aroCluster", aroClusterKey.Name,
				"reason", condition.Reason,
				"message", condition.Message)
			return false, nil
		}
	}

	// No ResourcesReady condition found - infrastructure not ready
	log.V(4).Info("AROCluster has no ResourcesReady condition, infrastructure not ready", "aroCluster", aroClusterKey.Name)
	return false, nil
}

// Returns: filtered resources, whether ExternalAuth was filtered out, error.
func (s *aroControlPlaneService) filterExternalAuthUntilNodePoolReady(ctx context.Context, resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, bool, error) {
	_, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.filterExternalAuthUntilNodePoolReady")
	defer done()

	// Check if any HcpOpenShiftClustersNodePool is ready
	nodePoolList := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePoolList{}
	if err := s.kubeclient.List(ctx, nodePoolList, client.InNamespace(s.scope.Namespace())); err != nil {
		return nil, false, fmt.Errorf("failed to list HcpOpenShiftClustersNodePool resources: %w", err)
	}

	log.V(4).Info("Checking node pool readiness", "nodePoolCount", len(nodePoolList.Items))
	hasReadyNodePool := false
	for _, nodePool := range nodePoolList.Items {
		for _, condition := range nodePool.Status.Conditions {
			if condition.Type == asoconditions.ConditionTypeReady && condition.Status == metav1.ConditionTrue {
				hasReadyNodePool = true
				log.V(4).Info("Found ready node pool", "name", nodePool.Name)
				break
			}
		}
		if hasReadyNodePool {
			break
		}
	}

	// If node pool is ready, keep all resources
	if hasReadyNodePool {
		log.V(4).Info("Node pool is ready, keeping all resources including ExternalAuth")
		return resources, false, nil
	}

	// Filter out ExternalAuth resources
	filtered := make([]*unstructured.Unstructured, 0, len(resources))
	filteredCount := 0
	for _, resource := range resources {
		if resource.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
			resource.GroupVersionKind().Kind == hcpOpenShiftClustersExternalAuthKind {
			log.V(4).Info("Filtering out ExternalAuth resource (no ready node pool)", "name", resource.GetName())
			filteredCount++
			continue
		}
		filtered = append(filtered, resource)
	}

	if filteredCount > 0 {
		log.V(4).Info("Filtered ExternalAuth resources", "filtered", filteredCount, "remaining", len(filtered))
	}

	return filtered, filteredCount > 0, nil
}
