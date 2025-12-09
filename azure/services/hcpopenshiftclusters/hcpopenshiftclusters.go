/*
Copyright 2025 The Kubernetes Authors.

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

// Package hcpopenshiftclusters provides ASO-based HCP OpenShift cluster management.
package hcpopenshiftclusters

import (
	"context"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "hcpopenshiftclusters"

// Service provides ASO-based operations on HCP OpenShift clusters.
type Service struct {
	Scope  *scope.AROControlPlaneScope
	client client.Client
}

// New creates a new ASO-based HCP OpenShift cluster service.
func New(aroScope *scope.AROControlPlaneScope) (*Service, error) {
	return &Service{
		Scope:  aroScope,
		client: aroScope.Client,
	}, nil
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile creates or updates the HcpOpenShiftCluster ASO resource.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftclusters.Service.Reconcile")
	defer done()

	log.V(4).Info("reconciling HcpOpenShiftCluster with ASO")

	// Build the HcpOpenShiftCluster spec from the scope
	hcpCluster, err := s.buildHcpOpenShiftCluster(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to build HcpOpenShiftCluster")
	}

	log.V(4).Info("applying HcpOpenShiftCluster ASO resource", "name", hcpCluster.Name)

	// Apply the HcpOpenShiftCluster using server-side apply
	err = s.client.Patch(ctx, hcpCluster, client.Apply, client.FieldOwner("capz-manager"), client.ForceOwnership)
	if err != nil {
		return errors.Wrap(err, "failed to apply HcpOpenShiftCluster")
	}

	// Fetch the applied resource to get status and UID
	appliedCluster := &asoredhatopenshiftv1.HcpOpenShiftCluster{}
	if err := s.client.Get(ctx, client.ObjectKeyFromObject(hcpCluster), appliedCluster); err != nil {
		return errors.Wrap(err, "failed to get applied HcpOpenShiftCluster")
	}

	// Ensure the kubeconfig secret exists with proper ownership AFTER HcpOpenShiftCluster is created
	// ASO's extension expects the secret to already exist and be owned by the HcpOpenShiftCluster
	_, err = s.ensureKubeconfigSecretWithOwner(ctx, appliedCluster)
	if err != nil {
		return errors.Wrap(err, "failed to ensure kubeconfig secret exists")
	}

	// Check Azure provisioning state
	var provState asoredhatopenshiftv1.ProvisioningState_STATUS
	if appliedCluster.Status.Properties != nil && appliedCluster.Status.Properties.ProvisioningState != nil {
		provState = *appliedCluster.Status.Properties.ProvisioningState
	}

	// Add label to trigger ASO reconciliation if:
	// 1. Secret exists (either just created or already exists)
	// 2. Label is not already set
	// 3. Azure provisioning state is "Succeeded" (safe to update)
	labelNeeded := appliedCluster.Labels["aro.azure.com/kubeconfig-secret-ready"] != "true"
	if labelNeeded && provState == asoredhatopenshiftv1.ProvisioningState_STATUS_Succeeded {
		if appliedCluster.Labels == nil {
			appliedCluster.Labels = make(map[string]string)
		}
		appliedCluster.Labels["aro.azure.com/kubeconfig-secret-ready"] = "true"
		log.V(4).Info("added label to trigger ASO secret reconciliation", "name", appliedCluster.Name, "provisioningState", provState)

		// Update the HcpOpenShiftCluster with the label
		if err := s.client.Update(ctx, appliedCluster); err != nil {
			return errors.Wrap(err, "failed to update HcpOpenShiftCluster with label")
		}
	} else if labelNeeded && provState != asoredhatopenshiftv1.ProvisioningState_STATUS_Succeeded {
		log.V(4).Info("skipping label update until Azure provisioning succeeds", "name", appliedCluster.Name, "currentState", provState)
	}

	// Get both Azure provisioning state and ASO Ready condition for complete status view
	var provisioningState asoredhatopenshiftv1.ProvisioningState_STATUS
	if appliedCluster.Status.Properties != nil && appliedCluster.Status.Properties.ProvisioningState != nil {
		provisioningState = *appliedCluster.Status.Properties.ProvisioningState
	}
	readyCondition := findCondition(appliedCluster.Status.Conditions, conditions.ConditionTypeReady)

	// Log combined status for visibility
	if readyCondition != nil {
		log.V(4).Info("HcpOpenShiftCluster status",
			"azureProvisioningState", provisioningState,
			"asoConditionStatus", readyCondition.Status,
			"asoConditionReason", readyCondition.Reason,
			"asoConditionMessage", readyCondition.Message)
	} else {
		log.V(4).Info("HcpOpenShiftCluster status", "azureProvisioningState", provisioningState)
	}

	// Mirror the HcpOpenShiftCluster Ready condition to AROControlPlane
	s.setHcpClusterReadyCondition(readyCondition, provisioningState)

	// Check Azure provisioning state first (authoritative for Azure resource status)
	if provisioningState != "" {
		// Azure reports provisioning failed
		if provisioningState == asoredhatopenshiftv1.ProvisioningState_STATUS_Failed {
			return errors.Errorf("HcpOpenShiftCluster provisioning failed in Azure")
		}

		// Azure provisioning not yet complete
		if provisioningState != asoredhatopenshiftv1.ProvisioningState_STATUS_Succeeded {
			log.V(4).Info("HcpOpenShiftCluster Azure provisioning in progress", "state", provisioningState)
			return azure.WithTransientError(
				errors.Errorf("cluster Azure provisioning state: %s", provisioningState),
				30)
		}

		// Azure provisioning succeeded, now check ASO Ready condition
		if readyCondition != nil {
			// ASO operator reports error
			if readyCondition.Status == metav1.ConditionFalse && readyCondition.Severity == conditions.ConditionSeverityError {
				return errors.Errorf("HcpOpenShiftCluster ASO reconciliation failed: %s", readyCondition.Message)
			}

			// ASO still reconciling (even though Azure provisioning succeeded)
			if readyCondition.Status == metav1.ConditionFalse {
				log.V(4).Info("HcpOpenShiftCluster ASO reconciliation in progress",
					"reason", readyCondition.Reason)
				return azure.WithTransientError(
					errors.Errorf("cluster ASO reconciliation: %s - %s", readyCondition.Reason, readyCondition.Message),
					15)
			}
		}

		// Both Azure provisioning succeeded and ASO is ready (or no condition yet) - extract API URL
		if appliedCluster.Status.Properties != nil &&
			appliedCluster.Status.Properties.Api != nil &&
			appliedCluster.Status.Properties.Api.Url != nil {
			apiURL := appliedCluster.Status.Properties.Api.Url
			log.V(4).Info("setting API URL from HcpOpenShiftCluster", "url", *apiURL)
			s.Scope.SetAPIURL(apiURL)
		} else {
			log.V(4).Info("HcpOpenShiftCluster API URL not yet available")
			return azure.WithTransientError(errors.New("cluster API URL not yet available"), 15)
		}

		// Mark control plane as initialized when cluster is successfully provisioned
		// This is required by the Cluster API contract for machine pool creation
		// IMPORTANT: This must happen BEFORE checking console URL, because machine pools
		// need to be created before external auth can be configured (which provides console URL)
		if provisioningState == asoredhatopenshiftv1.ProvisioningState_STATUS_Succeeded {
			log.V(4).Info("marking control plane as initialized")
			s.Scope.SetControlPlaneInitialized(true)
		}

		// Extract Console URL if available
		// Console URL appears after external auth is configured
		if appliedCluster.Status.Properties != nil &&
			appliedCluster.Status.Properties.Console != nil &&
			appliedCluster.Status.Properties.Console.Url != nil {
			consoleURL := appliedCluster.Status.Properties.Console.Url
			log.V(4).Info("setting Console URL from HcpOpenShiftCluster", "url", *consoleURL)
			s.Scope.SetConsoleURL(consoleURL)
		} else if s.Scope.ControlPlane.Spec.EnableExternalAuthProviders {
			// Console URL not available yet - this is expected before external auth is configured
			// Don't block - allow external auth service to run which will trigger console URL creation
			log.V(4).Info("HcpOpenShiftCluster Console URL not yet available (will be set after external auth is configured)")
		}
	} else {
		// No Azure provisioning state yet - cluster just created
		log.V(4).Info("HcpOpenShiftCluster Azure provisioning state not yet available, will requeue")
		return azure.WithTransientError(errors.New("cluster Azure status not yet available"), 15)
	}

	log.V(4).Info("successfully reconciled HcpOpenShiftCluster")
	return nil
}

// Delete deletes the HcpOpenShiftCluster ASO resource.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftclusters.Service.Delete")
	defer done()

	log.V(4).Info("deleting HcpOpenShiftCluster ASO resource")

	hcpCluster := &asoredhatopenshiftv1.HcpOpenShiftCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.getClusterName(),
			Namespace: s.Scope.Namespace(),
		},
	}

	// Try to get the resource first to check if it exists
	err := s.client.Get(ctx, client.ObjectKeyFromObject(hcpCluster), hcpCluster)
	if apierrors.IsNotFound(err) {
		log.V(4).Info("HcpOpenShiftCluster already deleted")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to get HcpOpenShiftCluster for deletion")
	}

	// Mirror the HcpOpenShiftCluster Ready condition to AROControlPlane during deletion
	var provisioningState asoredhatopenshiftv1.ProvisioningState_STATUS
	if hcpCluster.Status.Properties != nil && hcpCluster.Status.Properties.ProvisioningState != nil {
		provisioningState = *hcpCluster.Status.Properties.ProvisioningState
	}
	readyCondition := findCondition(hcpCluster.Status.Conditions, conditions.ConditionTypeReady)
	s.setHcpClusterReadyCondition(readyCondition, provisioningState)

	// If the resource exists and doesn't have a deletion timestamp, delete it
	if hcpCluster.DeletionTimestamp.IsZero() {
		log.V(4).Info("initiating HcpOpenShiftCluster deletion")
		if err := s.client.Delete(ctx, hcpCluster); err != nil {
			return errors.Wrap(err, "failed to delete HcpOpenShiftCluster")
		}
	}

	// Resource is being deleted, wait for it to be fully removed
	log.V(4).Info("waiting for HcpOpenShiftCluster deletion to complete")
	return azure.WithTransientError(errors.New("HcpOpenShiftCluster deletion in progress"), 120) // 2 minutes
}

// IsManaged returns true if the HcpOpenShiftCluster is managed by this service.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	// ASO resources are always managed if they exist
	hcpCluster := &asoredhatopenshiftv1.HcpOpenShiftCluster{}
	err := s.client.Get(ctx, client.ObjectKey{
		Name:      s.getClusterName(),
		Namespace: s.Scope.Namespace(),
	}, hcpCluster)

	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, "failed to check if HcpOpenShiftCluster exists")
	}

	return true, nil
}

// Pause pauses the HcpOpenShiftCluster reconciliation.
func (s *Service) Pause(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftclusters.Service.Pause")
	defer done()

	log.V(4).Info("pausing HcpOpenShiftCluster reconciliation")

	hcpCluster := &asoredhatopenshiftv1.HcpOpenShiftCluster{}
	err := s.client.Get(ctx, client.ObjectKey{
		Name:      s.getClusterName(),
		Namespace: s.Scope.Namespace(),
	}, hcpCluster)

	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to get HcpOpenShiftCluster")
	}

	// Pause ASO reconciliation by setting the reconcile-policy annotation
	if hcpCluster.Annotations == nil {
		hcpCluster.Annotations = make(map[string]string)
	}
	hcpCluster.Annotations["serviceoperator.azure.com/reconcile-policy"] = "skip"

	if err := s.client.Update(ctx, hcpCluster); err != nil {
		return errors.Wrap(err, "failed to pause HcpOpenShiftCluster")
	}

	log.V(4).Info("successfully paused HcpOpenShiftCluster")
	return nil
}

// buildHcpOpenShiftCluster builds the HcpOpenShiftCluster ASO resource from the scope.
func (s *Service) buildHcpOpenShiftCluster(ctx context.Context) (*asoredhatopenshiftv1.HcpOpenShiftCluster, error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftclusters.Service.buildHcpOpenShiftCluster")
	defer done()

	// Get the basic cluster information
	clusterName := s.getClusterName()
	namespace := s.Scope.Namespace()

	// Create the HcpOpenShiftCluster resource
	hcpCluster := &asoredhatopenshiftv1.HcpOpenShiftCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: asoredhatopenshiftv1.GroupVersion.Identifier(),
			Kind:       "HcpOpenShiftCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: s.Scope.ControlPlane.APIVersion,
					Kind:       s.Scope.ControlPlane.Kind,
					Name:       s.Scope.ControlPlane.Name,
					UID:        s.Scope.ControlPlane.UID,
					Controller: ptr.To(true),
				},
			},
		},
		Spec: asoredhatopenshiftv1.HcpOpenShiftCluster_Spec{
			AzureName: clusterName,
			Location:  ptr.To(s.Scope.Location()),
		},
	}

	// Set the resource group owner reference
	// The actual resource group should be created by the groups service before this
	hcpCluster.Spec.Owner = s.Scope.GetResourceGroupOwnerReference()

	// Set operatorSpec with kubeconfig secret
	secretName := secret.Name(s.Scope.Cluster.Name, secret.Kubeconfig)
	hcpCluster.Spec.OperatorSpec = &asoredhatopenshiftv1.HcpOpenShiftClusterOperatorSpec{
		Secrets: &asoredhatopenshiftv1.HcpOpenShiftClusterOperatorSecrets{
			AdminCredentials: &genruntime.SecretDestination{
				Name: secretName,
				Key:  secret.KubeconfigDataName,
			},
		},
	}

	// Build properties from scope
	hcpCluster.Spec.Properties = s.Scope.HcpOpenShiftClusterProperties()

	// Set identity if managed identities are configured
	if identities := s.Scope.UserAssignedIdentities(); len(identities) > 0 {
		hcpCluster.Spec.Identity = s.Scope.ManagedServiceIdentity()
	}

	// Set tags
	if tags := s.Scope.AdditionalTags(); len(tags) > 0 {
		hcpCluster.Spec.Tags = tags
	}

	return hcpCluster, nil
}

// getClusterName returns the cluster name for the HcpOpenShiftCluster resource.
func (s *Service) getClusterName() string {
	return s.Scope.ControlPlane.Spec.AroClusterName
}

// findCondition finds a condition by type in the ASO conditions list.
func findCondition(conditionsList []conditions.Condition, conditionType conditions.ConditionType) *conditions.Condition {
	for i := range conditionsList {
		if conditionsList[i].Type == conditionType {
			return &conditionsList[i]
		}
	}
	return nil
}

// setHcpClusterReadyCondition mirrors the HcpOpenShiftCluster Ready condition to the AROControlPlane.
func (s *Service) setHcpClusterReadyCondition(readyCondition *conditions.Condition, provisioningState asoredhatopenshiftv1.ProvisioningState_STATUS) {
	aroControlPlane := s.Scope.ControlPlane

	// If we don't have a Ready condition yet, set condition based on provisioning state
	if readyCondition == nil {
		if provisioningState == "" {
			// Cluster just created, provisioning not started
			capiconditions.MarkFalse(
				aroControlPlane,
				cplane.HcpClusterReadyCondition,
				"Provisioning",
				clusterv1.ConditionSeverityInfo,
				"HcpOpenShiftCluster provisioning starting",
			)
		} else {
			// Have provisioning state but no Ready condition yet
			capiconditions.MarkFalse(
				aroControlPlane,
				cplane.HcpClusterReadyCondition,
				string(provisioningState),
				clusterv1.ConditionSeverityInfo,
				"HcpOpenShiftCluster provisioning state: %s", provisioningState,
			)
		}
		return
	}

	// Mirror the ASO Ready condition
	switch readyCondition.Status {
	case metav1.ConditionTrue:
		// ASO reports cluster is ready
		capiconditions.MarkTrue(aroControlPlane, cplane.HcpClusterReadyCondition)

	case metav1.ConditionFalse:
		// ASO reports not ready - check severity
		severity := clusterv1.ConditionSeverityInfo
		switch readyCondition.Severity {
		case conditions.ConditionSeverityError:
			severity = clusterv1.ConditionSeverityError
		case conditions.ConditionSeverityWarning:
			severity = clusterv1.ConditionSeverityWarning
		}

		capiconditions.MarkFalse(
			aroControlPlane,
			cplane.HcpClusterReadyCondition,
			readyCondition.Reason,
			severity,
			"%s",
			readyCondition.Message,
		)

	case metav1.ConditionUnknown:
		// ASO reports unknown state
		capiconditions.MarkUnknown(
			aroControlPlane,
			cplane.HcpClusterReadyCondition,
			readyCondition.Reason,
			"%s",
			readyCondition.Message,
		)
	}
}

// ensureKubeconfigSecretWithOwner creates an empty kubeconfig secret with proper ownership if it doesn't exist.
// This is required because ASO's HcpOpenShiftCluster extension expects the secret to exist
// and be owned by the HcpOpenShiftCluster before it can populate it with admin credentials.
// Returns true if the secret was created, false if it already existed.
func (s *Service) ensureKubeconfigSecretWithOwner(ctx context.Context, hcpCluster *asoredhatopenshiftv1.HcpOpenShiftCluster) (bool, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftclusters.Service.ensureKubeconfigSecretWithOwner")
	defer done()

	secretName := secret.Name(s.Scope.Cluster.Name, secret.Kubeconfig)
	namespace := s.Scope.Namespace()

	// Check if secret already exists
	existingSecret := &corev1.Secret{}
	err := s.client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      secretName,
	}, existingSecret)

	if err == nil {
		// Secret already exists - check if it has the correct owner
		hasOwner := false
		for _, owner := range existingSecret.OwnerReferences {
			if owner.UID == hcpCluster.UID {
				hasOwner = true
				break
			}
		}

		if !hasOwner {
			// Add the HcpOpenShiftCluster as an owner
			log.V(4).Info("adding HcpOpenShiftCluster as owner to existing secret", "secretName", secretName)
			existingSecret.OwnerReferences = append(existingSecret.OwnerReferences, metav1.OwnerReference{
				APIVersion: hcpCluster.APIVersion,
				Kind:       hcpCluster.Kind,
				Name:       hcpCluster.Name,
				UID:        hcpCluster.UID,
			})
			if err := s.client.Update(ctx, existingSecret); err != nil {
				return false, errors.Wrap(err, "failed to add owner reference to existing secret")
			}
			return true, nil
		}

		log.V(4).Info("kubeconfig secret already exists with correct owner", "secretName", secretName)
		return false, nil
	}

	if !apierrors.IsNotFound(err) {
		return false, errors.Wrap(err, "failed to check if kubeconfig secret exists")
	}

	// Secret doesn't exist - create it with HcpOpenShiftCluster as owner
	log.V(4).Info("creating empty kubeconfig secret for ASO to populate", "secretName", secretName, "owner", hcpCluster.Name)

	emptySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: hcpCluster.APIVersion,
					Kind:       hcpCluster.Kind,
					Name:       hcpCluster.Name,
					UID:        hcpCluster.UID,
				},
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			secret.KubeconfigDataName: []byte(""),
		},
	}

	if err := s.client.Create(ctx, emptySecret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Race condition - another reconcile created it
			log.V(4).Info("kubeconfig secret was created by another reconcile", "secretName", secretName)
			return false, nil
		}
		return false, errors.Wrap(err, "failed to create empty kubeconfig secret")
	}

	log.V(4).Info("successfully created empty kubeconfig secret with owner", "secretName", secretName, "owner", hcpCluster.Name)
	return true, nil
}
