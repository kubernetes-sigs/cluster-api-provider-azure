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

// Package hcpopenshiftnodepools provides ASO-based HCP OpenShift node pool management.
package hcpopenshiftnodepools

import (
	"context"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiconditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	v1beta2 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "hcpopenshiftnodepools"

// Service provides ASO-based operations on HCP OpenShift node pools.
type Service struct {
	Scope  *scope.AROMachinePoolScope
	client client.Client
}

// New creates a new ASO-based HCP OpenShift node pool service.
func New(aroScope *scope.AROMachinePoolScope) (*Service, error) {
	return &Service{
		Scope:  aroScope,
		client: aroScope.Client,
	}, nil
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile creates or updates the HcpOpenShiftClustersNodePool ASO resource.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftnodepools.Service.Reconcile")
	defer done()

	log.V(4).Info("reconciling HcpOpenShiftClustersNodePool with ASO")

	// Build the HcpOpenShiftClustersNodePool spec from the scope
	nodePool, err := s.buildHcpOpenShiftNodePool(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to build HcpOpenShiftClustersNodePool")
	}

	log.V(4).Info("applying HcpOpenShiftClustersNodePool ASO resource", "name", nodePool.Name)

	// Apply the HcpOpenShiftClustersNodePool using server-side apply
	err = s.client.Patch(ctx, nodePool, client.Apply, client.FieldOwner("capz-manager"), client.ForceOwnership)
	if err != nil {
		return errors.Wrap(err, "failed to apply HcpOpenShiftClustersNodePool")
	}

	// Fetch the applied resource to get status
	appliedNodePool := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePool{}
	if err := s.client.Get(ctx, client.ObjectKeyFromObject(nodePool), appliedNodePool); err != nil {
		return errors.Wrap(err, "failed to get applied HcpOpenShiftClustersNodePool")
	}

	// Get both Azure provisioning state and ASO Ready condition for complete status view
	var provisioningState asoredhatopenshiftv1.ProvisioningState_STATUS
	if appliedNodePool.Status.Properties != nil && appliedNodePool.Status.Properties.ProvisioningState != nil {
		provisioningState = *appliedNodePool.Status.Properties.ProvisioningState
	}
	readyCondition := findCondition(appliedNodePool.Status.Conditions, conditions.ConditionTypeReady)

	// Log combined status for visibility
	if readyCondition != nil {
		log.V(4).Info("HcpOpenShiftClustersNodePool status",
			"azureProvisioningState", provisioningState,
			"asoConditionStatus", readyCondition.Status,
			"asoConditionReason", readyCondition.Reason,
			"asoConditionMessage", readyCondition.Message)
	} else {
		log.V(4).Info("HcpOpenShiftClustersNodePool status", "azureProvisioningState", provisioningState)
	}

	// Mirror the HcpOpenShiftClustersNodePool Ready condition to AROMachinePool
	s.setNodePoolReadyCondition(readyCondition, provisioningState)

	// Set the provisioning state in the AROMachinePool status
	if provisioningState != "" {
		s.Scope.SetAgentPoolProvisioningState(string(provisioningState))
	}

	// Check Azure provisioning state first (authoritative for Azure resource status)
	if provisioningState != "" {
		// Azure reports provisioning failed
		if provisioningState == asoredhatopenshiftv1.ProvisioningState_STATUS_Failed {
			return errors.Errorf("HcpOpenShiftClustersNodePool provisioning failed in Azure")
		}

		// Azure provisioning not yet complete
		if provisioningState != asoredhatopenshiftv1.ProvisioningState_STATUS_Succeeded {
			log.V(4).Info("HcpOpenShiftClustersNodePool Azure provisioning in progress", "state", provisioningState)
			return azure.WithTransientError(
				errors.Errorf("node pool Azure provisioning state: %s", provisioningState),
				30)
		}

		// Azure provisioning succeeded, now check ASO Ready condition
		if readyCondition != nil {
			// ASO operator reports error
			if readyCondition.Status == metav1.ConditionFalse && readyCondition.Severity == conditions.ConditionSeverityError {
				return errors.Errorf("HcpOpenShiftClustersNodePool ASO reconciliation failed: %s", readyCondition.Message)
			}

			// ASO still reconciling (even though Azure provisioning succeeded)
			if readyCondition.Status == metav1.ConditionFalse {
				log.V(4).Info("HcpOpenShiftClustersNodePool ASO reconciliation in progress",
					"reason", readyCondition.Reason)
				return azure.WithTransientError(
					errors.Errorf("node pool ASO reconciliation: %s - %s", readyCondition.Reason, readyCondition.Message),
					15)
			}
		}

		// Update version in scope
		if appliedNodePool.Status.Properties != nil && appliedNodePool.Status.Properties.Version != nil && appliedNodePool.Status.Properties.Version.Id != nil {
			s.Scope.InfraMachinePool.Status.Version = *appliedNodePool.Status.Properties.Version.Id
		}
	} else {
		// No Azure provisioning state yet - node pool just created
		log.V(4).Info("HcpOpenShiftClustersNodePool Azure provisioning state not yet available, will requeue")
		return azure.WithTransientError(errors.New("node pool Azure status not yet available"), 15)
	}

	log.V(4).Info("successfully reconciled HcpOpenShiftClustersNodePool")
	return nil
}

// Delete deletes the HcpOpenShiftClustersNodePool ASO resource.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftnodepools.Service.Delete")
	defer done()

	log.V(4).Info("deleting HcpOpenShiftClustersNodePool ASO resource")

	nodePool := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.getNodePoolName(),
			Namespace: s.Scope.InfraMachinePool.Namespace,
		},
	}

	// Try to get the resource first to check if it exists
	err := s.client.Get(ctx, client.ObjectKeyFromObject(nodePool), nodePool)
	if apierrors.IsNotFound(err) {
		log.V(4).Info("HcpOpenShiftClustersNodePool already deleted")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to get HcpOpenShiftClustersNodePool for deletion")
	}

	// Mirror the HcpOpenShiftClustersNodePool Ready condition to AROMachinePool during deletion
	var provisioningState asoredhatopenshiftv1.ProvisioningState_STATUS
	if nodePool.Status.Properties != nil && nodePool.Status.Properties.ProvisioningState != nil {
		provisioningState = *nodePool.Status.Properties.ProvisioningState
	}
	readyCondition := findCondition(nodePool.Status.Conditions, conditions.ConditionTypeReady)
	s.setNodePoolReadyCondition(readyCondition, provisioningState)

	// If the resource exists and doesn't have a deletion timestamp, delete it
	if nodePool.DeletionTimestamp.IsZero() {
		log.V(4).Info("initiating HcpOpenShiftClustersNodePool deletion")
		if err := s.client.Delete(ctx, nodePool); err != nil {
			return errors.Wrap(err, "failed to delete HcpOpenShiftClustersNodePool")
		}
	}

	// Resource is being deleted, wait for it to be fully removed
	log.V(4).Info("waiting for HcpOpenShiftClustersNodePool deletion to complete")
	return azure.WithTransientError(errors.New("HcpOpenShiftClustersNodePool deletion in progress"), 120) // 2 minutes
}

// IsManaged returns true if the HcpOpenShiftClustersNodePool is managed by this service.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	// ASO resources are always managed if they exist
	nodePool := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePool{}
	err := s.client.Get(ctx, client.ObjectKey{
		Name:      s.getNodePoolName(),
		Namespace: s.Scope.InfraMachinePool.Namespace,
	}, nodePool)

	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, "failed to check if HcpOpenShiftClustersNodePool exists")
	}

	return true, nil
}

// Pause pauses the HcpOpenShiftClustersNodePool reconciliation.
func (s *Service) Pause(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftnodepools.Service.Pause")
	defer done()

	log.V(4).Info("pausing HcpOpenShiftClustersNodePool reconciliation")

	nodePool := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePool{}
	err := s.client.Get(ctx, client.ObjectKey{
		Name:      s.getNodePoolName(),
		Namespace: s.Scope.InfraMachinePool.Namespace,
	}, nodePool)

	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to get HcpOpenShiftClustersNodePool")
	}

	// Pause ASO reconciliation by setting the reconcile-policy annotation
	if nodePool.Annotations == nil {
		nodePool.Annotations = make(map[string]string)
	}
	nodePool.Annotations["serviceoperator.azure.com/reconcile-policy"] = "skip"

	if err := s.client.Update(ctx, nodePool); err != nil {
		return errors.Wrap(err, "failed to pause HcpOpenShiftClustersNodePool")
	}

	log.V(4).Info("successfully paused HcpOpenShiftClustersNodePool")
	return nil
}

// buildHcpOpenShiftNodePool builds the HcpOpenShiftClustersNodePool ASO resource from the scope.
func (s *Service) buildHcpOpenShiftNodePool(ctx context.Context) (*asoredhatopenshiftv1.HcpOpenShiftClustersNodePool, error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "hcpopenshiftnodepools.Service.buildHcpOpenShiftNodePool")
	defer done()

	// Get the basic node pool information
	nodePoolName := s.getNodePoolName()
	namespace := s.Scope.InfraMachinePool.Namespace

	// Create the HcpOpenShiftClustersNodePool resource
	nodePool := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: asoredhatopenshiftv1.GroupVersion.Identifier(),
			Kind:       "HcpOpenShiftClustersNodePool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodePoolName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: s.Scope.InfraMachinePool.APIVersion,
					Kind:       s.Scope.InfraMachinePool.Kind,
					Name:       s.Scope.InfraMachinePool.Name,
					UID:        s.Scope.InfraMachinePool.UID,
					Controller: ptr.To(true),
				},
			},
		},
		Spec: asoredhatopenshiftv1.HcpOpenShiftClustersNodePool_Spec{
			AzureName: nodePoolName,
			Location:  ptr.To(s.Scope.ControlPlane.Spec.Platform.Location),
			Tags:      s.Scope.InfraMachinePool.Spec.AdditionalTags,
		},
	}

	// Set the owner reference to the HcpOpenShiftCluster
	clusterName := s.Scope.ControlPlane.Spec.AroClusterName
	nodePool.Spec.Owner = &genruntime.KnownResourceReference{
		Name: clusterName,
	}

	// Build properties from scope
	nodePool.Spec.Properties = s.Scope.HcpOpenShiftNodePoolProperties()

	return nodePool, nil
}

// getNodePoolName returns the node pool name for the HcpOpenShiftClustersNodePool resource.
func (s *Service) getNodePoolName() string {
	return s.Scope.InfraMachinePool.Spec.NodePoolName
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

// setNodePoolReadyCondition mirrors the HcpOpenShiftClustersNodePool Ready condition to the AROMachinePool.
func (s *Service) setNodePoolReadyCondition(readyCondition *conditions.Condition, provisioningState asoredhatopenshiftv1.ProvisioningState_STATUS) {
	aroMachinePool := s.Scope.InfraMachinePool

	// If we don't have a Ready condition yet, set condition based on provisioning state
	if readyCondition == nil {
		if provisioningState == "" {
			// Node pool just created, provisioning not started
			capiconditions.MarkFalse(
				aroMachinePool,
				v1beta2.AROMachinePoolReadyCondition,
				"Provisioning",
				clusterv1.ConditionSeverityInfo,
				"HcpOpenShiftClustersNodePool provisioning starting",
			)
		} else {
			// Have provisioning state but no Ready condition yet
			capiconditions.MarkFalse(
				aroMachinePool,
				v1beta2.AROMachinePoolReadyCondition,
				string(provisioningState),
				clusterv1.ConditionSeverityInfo,
				"HcpOpenShiftClustersNodePool provisioning state: %s", provisioningState,
			)
		}
		return
	}

	// Mirror the ASO Ready condition
	switch readyCondition.Status {
	case metav1.ConditionTrue:
		// ASO reports node pool is ready
		capiconditions.MarkTrue(aroMachinePool, v1beta2.AROMachinePoolReadyCondition)

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
			aroMachinePool,
			v1beta2.AROMachinePoolReadyCondition,
			readyCondition.Reason,
			severity,
			"%s",
			readyCondition.Message,
		)

	case metav1.ConditionUnknown:
		// ASO reports unknown state
		capiconditions.MarkUnknown(
			aroMachinePool,
			v1beta2.AROMachinePoolReadyCondition,
			readyCondition.Reason,
			"%s",
			readyCondition.Message,
		)
	}
}
