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

// Package hcpopenshiftclustersexternalauth provides ASO-based HCP OpenShift cluster external authentication management.
package hcpopenshiftclustersexternalauth

import (
	"context"
	"fmt"
	"strconv"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav1beta2 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "hcpopenshiftclustersexternalauth"

// Service provides ASO-based operations on HCP OpenShift cluster external authentication.
type Service struct {
	Scope  *scope.AROControlPlaneScope
	client client.Client
}

// New creates a new ASO-based HCP OpenShift cluster external auth service.
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

// Reconcile creates or updates the HcpOpenShiftClustersExternalAuth ASO resources.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftclustersexternalauth.Service.Reconcile")
	defer done()

	// Skip if external auth is not enabled
	if !s.Scope.ControlPlane.Spec.EnableExternalAuthProviders {
		log.V(4).Info("external auth providers not enabled, skipping reconciliation")
		return nil
	}

	// Check if at least one machine pool is ready before configuring external auth
	// Azure requires ready node pools for console authentication configuration
	hasReadyMachinePool, err := s.hasReadyMachinePool(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check machine pool readiness")
	}

	if !hasReadyMachinePool {
		log.V(4).Info("waiting for at least one machine pool to be ready before configuring external auth")
		capiconditions.Set(s.Scope.ControlPlane, metav1.Condition{
			Type:    string(cplane.ExternalAuthReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "WaitingForMachinePools",
			Message: "waiting for at least one machine pool to be ready",
		})
		return azure.WithTransientError(
			errors.New("external authentication requires at least one ready machine pool"),
			30)
	}

	log.V(4).Info("reconciling HcpOpenShiftClustersExternalAuth with ASO")

	// Reconcile each external auth provider
	for _, provider := range s.Scope.ControlPlane.Spec.ExternalAuthProviders {
		externalAuth, err := s.buildHcpOpenShiftClustersExternalAuth(ctx, provider)
		if err != nil {
			return errors.Wrapf(err, "failed to build HcpOpenShiftClustersExternalAuth for provider %s", provider.Name)
		}

		log.V(4).Info("applying HcpOpenShiftClustersExternalAuth ASO resource", "name", externalAuth.Name, "provider", provider.Name)

		// Apply the HcpOpenShiftClustersExternalAuth using server-side apply
		err = s.client.Patch(ctx, externalAuth, client.Apply, client.FieldOwner("capz-manager"), client.ForceOwnership)
		if err != nil {
			return errors.Wrapf(err, "failed to apply HcpOpenShiftClustersExternalAuth for provider %s", provider.Name)
		}

		// Fetch the applied resource to check ASO status
		appliedAuth := &asoredhatopenshiftv1.HcpOpenShiftClustersExternalAuth{}
		if err := s.client.Get(ctx, client.ObjectKeyFromObject(externalAuth), appliedAuth); err != nil {
			return errors.Wrapf(err, "failed to get applied HcpOpenShiftClustersExternalAuth for provider %s", provider.Name)
		}

		// Check if ASO resource is ready
		readyCondition := findCondition(appliedAuth.Status.Conditions, asoconditions.ConditionTypeReady)
		if readyCondition == nil || readyCondition.Status != metav1.ConditionTrue {
			log.V(4).Info("waiting for HcpOpenShiftClustersExternalAuth to be ready", "provider", provider.Name)
			capiconditions.Set(s.Scope.ControlPlane, metav1.Condition{
				Type:    string(cplane.ExternalAuthReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  "WaitingForExternalAuth",
				Message: "waiting for HcpOpenShiftClustersExternalAuth to be ready in Azure",
			})
			return azure.WithTransientError(
				errors.Errorf("HcpOpenShiftClustersExternalAuth %s not yet ready", provider.Name),
				15)
		}
	}

	// Verify that console URL appeared in HcpOpenShiftCluster after external auth was configured
	// Console URL is required for ExternalAuthReady to be True
	if consoleURL := s.Scope.ControlPlane.Status.ConsoleURL; consoleURL == "" {
		log.V(4).Info("waiting for console URL to appear after external auth configuration")

		// Trigger ASO to sync the HcpOpenShiftCluster status from Azure
		// This is needed because ASO's default sync period is 1 hour, but the console URL
		// is populated in Azure by the external auth configuration, not by a Kubernetes change.
		// By updating a label on the HcpOpenShiftCluster, we force ASO to reconcile and
		// fetch the updated status from Azure, which should include the console URL.
		if err := s.triggerHcpClusterSync(ctx); err != nil {
			log.V(4).Info("failed to trigger HcpOpenShiftCluster sync, will retry", "error", err)
			// Don't fail - just log and continue waiting
		}

		capiconditions.Set(s.Scope.ControlPlane, metav1.Condition{
			Type:    string(cplane.ExternalAuthReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "WaitingForConsoleURL",
			Message: "waiting for console URL to appear in HcpOpenShiftCluster status",
		})
		return azure.WithTransientError(
			errors.New("console URL not yet available after external auth configuration"),
			15)
	}

	log.V(4).Info("external auth configured successfully with console URL available")
	capiconditions.Set(s.Scope.ControlPlane, metav1.Condition{
		Type:   string(cplane.ExternalAuthReadyCondition),
		Status: metav1.ConditionTrue,
		Reason: "Succeeded",
	})
	return nil
}

// findCondition finds a condition by type in the ASO conditions list.
func findCondition(conditionsList []asoconditions.Condition, conditionType asoconditions.ConditionType) *asoconditions.Condition {
	for i := range conditionsList {
		if conditionsList[i].Type == conditionType {
			return &conditionsList[i]
		}
	}
	return nil
}

// Delete removes the HcpOpenShiftClustersExternalAuth ASO resources.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftclustersexternalauth.Service.Delete")
	defer done()

	log.V(4).Info("deleting HcpOpenShiftClustersExternalAuth resources")

	// Delete each external auth provider
	for _, provider := range s.Scope.ControlPlane.Spec.ExternalAuthProviders {
		externalAuth := &asoredhatopenshiftv1.HcpOpenShiftClustersExternalAuth{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", s.Scope.ClusterName(), provider.Name),
				Namespace: s.Scope.ControlPlane.Namespace,
			},
		}

		err := s.client.Delete(ctx, externalAuth)
		if err != nil && client.IgnoreNotFound(err) != nil {
			return errors.Wrapf(err, "failed to delete HcpOpenShiftClustersExternalAuth for provider %s", provider.Name)
		}
	}

	return nil
}

// IsManaged returns true if external auth is enabled.
func (s *Service) IsManaged(_ context.Context) (bool, error) {
	return s.Scope.ControlPlane.Spec.EnableExternalAuthProviders, nil
}

// Pause is a no-op for external auth.
func (s *Service) Pause(_ context.Context) error {
	return nil
}

// buildHcpOpenShiftClustersExternalAuth creates the ASO HcpOpenShiftClustersExternalAuth resource from CAPI types.
//
//nolint:unparam // error return kept for future extensibility (e.g., fetching CA bundles from ConfigMap)
func (s *Service) buildHcpOpenShiftClustersExternalAuth(_ context.Context, provider cplane.ExternalAuthProvider) (*asoredhatopenshiftv1.HcpOpenShiftClustersExternalAuth, error) {
	// Build the external auth properties
	properties := &asoredhatopenshiftv1.ExternalAuthProperties{
		Issuer: &asoredhatopenshiftv1.TokenIssuerProfile{
			Url:       ptr.To(provider.Issuer.URL),
			Audiences: []string{},
		},
	}

	// Convert audiences
	for _, aud := range provider.Issuer.Audiences {
		properties.Issuer.Audiences = append(properties.Issuer.Audiences, string(aud))
	}

	// Set CA if provided
	if provider.Issuer.CertificateAuthority != nil {
		// TODO: Fetch the CA bundle from the referenced configmap
		// For now, we'll leave it nil as it's optional
		properties.Issuer.Ca = nil
	}

	// Convert claim mappings
	if provider.ClaimMappings != nil {
		properties.Claim = &asoredhatopenshiftv1.ExternalAuthClaimProfile{
			Mappings: &asoredhatopenshiftv1.TokenClaimMappingsProfile{},
		}

		// Username mapping
		if provider.ClaimMappings.Username != nil {
			usernameProfile := &asoredhatopenshiftv1.UsernameClaimProfile{
				Claim: ptr.To(provider.ClaimMappings.Username.Claim),
			}

			// Map prefix policy
			switch provider.ClaimMappings.Username.PrefixPolicy {
			case cplane.NoPrefix:
				usernameProfile.PrefixPolicy = ptr.To(asoredhatopenshiftv1.UsernameClaimPrefixPolicy_NoPrefix)
			case cplane.Prefix:
				usernameProfile.PrefixPolicy = ptr.To(asoredhatopenshiftv1.UsernameClaimPrefixPolicy_Prefix)
				usernameProfile.Prefix = provider.ClaimMappings.Username.Prefix
			default:
				usernameProfile.PrefixPolicy = ptr.To(asoredhatopenshiftv1.UsernameClaimPrefixPolicy_None)
			}

			properties.Claim.Mappings.Username = usernameProfile
		}

		// Groups mapping
		if provider.ClaimMappings.Groups != nil {
			properties.Claim.Mappings.Groups = &asoredhatopenshiftv1.GroupClaimProfile{
				Claim:  ptr.To(provider.ClaimMappings.Groups.Claim),
				Prefix: ptr.To(provider.ClaimMappings.Groups.Prefix),
			}
		}

		// Validation rules
		if len(provider.ClaimValidationRules) > 0 {
			properties.Claim.ValidationRules = []asoredhatopenshiftv1.TokenClaimValidationRule{}
			for _, rule := range provider.ClaimValidationRules {
				properties.Claim.ValidationRules = append(properties.Claim.ValidationRules, asoredhatopenshiftv1.TokenClaimValidationRule{
					Type: ptr.To(asoredhatopenshiftv1.TokenClaimValidationRule_Type_RequiredClaim),
					RequiredClaim: &asoredhatopenshiftv1.TokenRequiredClaim{
						Claim:         ptr.To(rule.RequiredClaim.Claim),
						RequiredValue: ptr.To(rule.RequiredClaim.RequiredValue),
					},
				})
			}
		}
	}

	// Convert OIDC clients
	if len(provider.OIDCClients) > 0 {
		properties.Clients = []asoredhatopenshiftv1.ExternalAuthClientProfile{}
		for _, oidcClient := range provider.OIDCClients {
			clientProfile := asoredhatopenshiftv1.ExternalAuthClientProfile{
				ClientId: ptr.To(oidcClient.ClientID),
				Component: &asoredhatopenshiftv1.ExternalAuthClientComponentProfile{
					Name:                ptr.To(oidcClient.ComponentName),
					AuthClientNamespace: ptr.To(oidcClient.ComponentNamespace),
				},
				Type: ptr.To(asoredhatopenshiftv1.ExternalAuthClientType_Confidential),
			}

			if len(oidcClient.ExtraScopes) > 0 {
				clientProfile.ExtraScopes = oidcClient.ExtraScopes
			}

			properties.Clients = append(properties.Clients, clientProfile)
		}
	}

	// Create the HcpOpenShiftClustersExternalAuth resource
	externalAuth := &asoredhatopenshiftv1.HcpOpenShiftClustersExternalAuth{
		TypeMeta: metav1.TypeMeta{
			APIVersion: asoredhatopenshiftv1.GroupVersion.String(),
			Kind:       "HcpOpenShiftClustersExternalAuth",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", s.Scope.ClusterName(), provider.Name),
			Namespace: s.Scope.ControlPlane.Namespace,
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": s.Scope.ClusterName(),
			},
		},
		Spec: asoredhatopenshiftv1.HcpOpenShiftClustersExternalAuth_Spec{
			AzureName: provider.Name,
			Owner: &genruntime.KnownResourceReference{
				Name: s.Scope.ClusterName(),
			},
			Properties: properties,
		},
	}

	return externalAuth, nil
}

// hasReadyMachinePool checks if at least one AROMachinePool is ready.
// Azure requires at least one ready node pool before external authentication can be configured.
func (s *Service) hasReadyMachinePool(ctx context.Context) (bool, error) {
	// List all AROMachinePools for this cluster
	machinePools := &infrav1beta2.AROMachinePoolList{}
	if err := s.client.List(ctx, machinePools,
		client.InNamespace(s.Scope.ControlPlane.Namespace),
		client.MatchingLabels{
			clusterv1beta1.ClusterNameLabel: s.Scope.ClusterName(),
		}); err != nil {
		return false, errors.Wrap(err, "failed to list AROMachinePools")
	}

	// Check if any machine pool is ready
	for i := range machinePools.Items {
		mp := &machinePools.Items[i]
		if mp.Status.Ready {
			return true, nil
		}
	}

	return false, nil
}

// triggerHcpClusterSync forces ASO to reconcile the HcpOpenShiftCluster by updating a tag in the spec.
// This is necessary when external auth configuration adds the console URL in Azure, but ASO
// hasn't synced the updated status yet (default sync period is 1 hour).
//
// We use a spec.tags field with an incrementing counter. Changing the spec triggers ASO to
// reconcile and send a PUT/PATCH to Azure, which causes ASO to fetch fresh status including
// the console URL. This is more reliable than metadata changes which don't trigger reconciliation.
func (s *Service) triggerHcpClusterSync(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftclustersexternalauth.Service.triggerHcpClusterSync")
	defer done()

	// Get the HcpOpenShiftCluster resource
	hcpCluster := &asoredhatopenshiftv1.HcpOpenShiftCluster{}
	if err := s.client.Get(ctx, client.ObjectKey{
		Name:      s.Scope.ClusterName(),
		Namespace: s.Scope.ControlPlane.Namespace,
	}, hcpCluster); err != nil {
		return errors.Wrap(err, "failed to get HcpOpenShiftCluster")
	}

	// Use a tag in spec.tags with a counter that increments on each trigger
	// Changing spec.tags triggers ASO to reconcile and send a PUT to Azure,
	// which causes ASO to fetch updated status including console URL
	const syncTriggerTag = "aro-sync-console-url-ver"

	// Initialize tags map if it doesn't exist
	if hcpCluster.Spec.Tags == nil {
		hcpCluster.Spec.Tags = make(map[string]string)
	}

	// Get current counter value (defaults to 0 if tag doesn't exist)
	counter := 0
	if counterStr, exists := hcpCluster.Spec.Tags[syncTriggerTag]; exists {
		// Parse existing counter value
		if parsed, err := strconv.Atoi(counterStr); err == nil {
			counter = parsed
		}
	}

	// Increment counter and update tag
	counter++
	hcpCluster.Spec.Tags[syncTriggerTag] = fmt.Sprintf("%d", counter)

	log.V(4).Info("triggering ASO reconciliation for HcpOpenShiftCluster by updating spec.tags to sync console URL from Azure",
		"syncCounter", counter)

	if err := s.client.Update(ctx, hcpCluster); err != nil {
		return errors.Wrap(err, "failed to update HcpOpenShiftCluster spec.tags with sync trigger")
	}

	log.V(4).Info("successfully triggered HcpOpenShiftCluster sync via spec change", "syncCounter", counter)
	return nil
}
