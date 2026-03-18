/*
Copyright 2020 The Kubernetes Authors.

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
	"strings"
	"time"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	asoredhatopenshiftv1api2025 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20251223preview"
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/mutators"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	// aroMachinePoolService contains the services required by the cluster controller.
	aroMachinePoolService struct {
		scope                 *scope.AROMachinePoolScope
		kubeclient            client.Client
		cluster               *clusterv1.Cluster
		tracker               controllers.ClusterTracker
		newResourceReconciler func(*infrav1exp.AROMachinePool, []*unstructured.Unstructured) resourceReconciler
	}
)

// newAROMachinePoolService populates all the services based on input scope.
func newAROMachinePoolService(scope *scope.AROMachinePoolScope, cluster *clusterv1.Cluster, tracker controllers.ClusterTracker, _ time.Duration) (*aroMachinePoolService, error) {
	return &aroMachinePoolService{
		scope:      scope,
		kubeclient: scope.Client,
		cluster:    cluster,
		tracker:    tracker,
		newResourceReconciler: func(machinePool *infrav1exp.AROMachinePool, resources []*unstructured.Unstructured) resourceReconciler {
			return controllers.NewResourceReconciler(scope.Client, resources, machinePool)
		},
	}, nil
}

// Reconcile reconciles all the services in a predetermined order.
func (s *aroMachinePoolService) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroMachinePoolService.Reconcile")
	defer done()

	log.Info("reconciling ARO machine pool")

	// Resources mode is the only supported mode
	return s.reconcileResources(ctx)
}

// Pause pauses all components making up the machine pool.
func (s *aroMachinePoolService) Pause(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroMachinePoolService.Pause")
	defer done()

	log.Info("pausing ARO machine pool")

	// Resources mode: pause ASO resources
	return s.pauseResources(ctx)
}

// Delete reconciles all the services in a predetermined order.
func (s *aroMachinePoolService) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroMachinePoolService.Delete")
	defer done()

	log.Info("deleting ARO machine pool")

	// Resources mode is the only supported mode
	return s.deleteResources(ctx)
}

// reconcileResources handles reconciliation when spec.resources is specified.
func (s *aroMachinePoolService) reconcileResources(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroMachinePoolService.reconcileResources")
	defer done()

	log.V(4).Info("Reconciling AROMachinePool using resources mode")

	// Get HCP cluster name from the control plane
	// This is needed to set owner references for the node pool
	hcpClusterName := s.getHcpClusterName()

	// Apply mutators to set defaults and owner references
	resources, err := mutators.ApplyMutators(
		ctx,
		s.scope.InfraMachinePool.Spec.Resources,
		mutators.SetHcpOpenShiftNodePoolDefaults(s.kubeclient, s.scope.InfraMachinePool, hcpClusterName),
	)
	if err != nil {
		return errors.Wrap(err, "failed to apply mutators")
	}

	// Use the ResourceReconciler to apply resources
	resourceReconciler := s.newResourceReconciler(s.scope.InfraMachinePool, resources)

	if err := resourceReconciler.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile ASO resources")
	}

	// Find HcpOpenShiftClustersNodePool to extract status information
	var nodePoolName string
	for _, resource := range resources {
		if (resource.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group ||
			resource.GroupVersionKind().Group == asoredhatopenshiftv1api2025.GroupVersion.Group) &&
			resource.GroupVersionKind().Kind == "HcpOpenShiftClustersNodePool" {
			nodePoolName = resource.GetName()
			break
		}
	}

	if nodePoolName == "" {
		return errors.New("no HcpOpenShiftClustersNodePool found in resources")
	}

	// Get the HcpOpenShiftClustersNodePool to extract status (try both API versions)
	var statusID *string
	var version *string
	var provisioningState string
	var replicas *int
	var statusConditions []asoconditions.Condition

	// Try v1api20240610preview first
	nodePoolV1 := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePool{}
	err = s.kubeclient.Get(ctx, client.ObjectKey{
		Namespace: s.scope.InfraMachinePool.Namespace,
		Name:      nodePoolName,
	}, nodePoolV1)

	if err == nil {
		// Found v1api20240610preview version
		statusID = nodePoolV1.Status.Id
		statusConditions = nodePoolV1.Status.Conditions
		if nodePoolV1.Status.Properties != nil {
			if nodePoolV1.Status.Properties.Version != nil {
				version = nodePoolV1.Status.Properties.Version.Id
			}
			if nodePoolV1.Status.Properties.ProvisioningState != nil {
				provisioningState = string(*nodePoolV1.Status.Properties.ProvisioningState)
			}
			replicas = nodePoolV1.Status.Properties.Replicas
		}
	} else if client.IgnoreNotFound(err) == nil {
		// Not found, try v1api20251223preview
		nodePoolV2 := &asoredhatopenshiftv1api2025.HcpOpenShiftClustersNodePool{}
		err = s.kubeclient.Get(ctx, client.ObjectKey{
			Namespace: s.scope.InfraMachinePool.Namespace,
			Name:      nodePoolName,
		}, nodePoolV2)

		if err == nil {
			// Found v1api20251223preview version
			statusID = nodePoolV2.Status.Id
			statusConditions = nodePoolV2.Status.Conditions
			if nodePoolV2.Status.Properties != nil {
				if nodePoolV2.Status.Properties.Version != nil {
					version = nodePoolV2.Status.Properties.Version.Id
				}
				if nodePoolV2.Status.Properties.ProvisioningState != nil {
					provisioningState = string(*nodePoolV2.Status.Properties.ProvisioningState)
				}
				replicas = nodePoolV2.Status.Properties.Replicas
			}
		} else {
			return errors.Wrap(err, "failed to get HcpOpenShiftNodePool")
		}
	} else {
		return errors.Wrap(err, "failed to get HcpOpenShiftNodePool")
	}

	// Extract status information from HcpOpenShiftNodePool
	if statusID != nil {
		s.scope.InfraMachinePool.Status.ID = *statusID
	}

	if version != nil {
		s.scope.InfraMachinePool.Status.Version = *version
	}

	if provisioningState != "" {
		s.scope.InfraMachinePool.Status.ProvisioningState = provisioningState
	}

	// Set replicas from node pool status
	// For HCP node pools with autoscaling, the status doesn't include replicas count
	// In that case, use the CAPI MachinePool replicas as the source of truth
	if replicas != nil {
		s.scope.InfraMachinePool.Status.Replicas = int32(*replicas)
	} else if s.scope.MachinePool.Spec.Replicas != nil {
		s.scope.InfraMachinePool.Status.Replicas = *s.scope.MachinePool.Spec.Replicas
	}

	// Populate providerIDList from workload cluster nodes
	// This is required by CAPI to match MachinePool replicas to actual nodes
	// Get client for the workload cluster (not management cluster)
	clusterClient, err := s.tracker.GetClient(ctx, util.ObjectKey(s.cluster))
	if err != nil {
		log.V(4).Info("failed to get workload cluster client", "error", err)
		// Don't fail reconciliation if we can't get cluster client yet - control plane may not be ready
	} else {
		// List all nodes from the workload cluster
		// Note: We list all nodes and filter by node pool name pattern rather than using labels
		// This is because ARO HCP node labels may vary
		nodes := &corev1.NodeList{}
		err = clusterClient.List(ctx, nodes)
		if err != nil {
			log.V(4).Info("failed to list nodes in workload cluster", "error", err)
			// Don't fail reconciliation if we can't list nodes yet
		} else {
			// Filter nodes by providerID pattern matching this node pool
			// ARO HCP node names contain the node pool name: <cluster-name>-<nodepool-name>-<random-suffix>
			nodePoolNamePattern := s.scope.ClusterName() + "-" + nodePoolName + "-"
			var providerIDs []string
			for _, node := range nodes.Items {
				if node.Spec.ProviderID == "" {
					// The node will receive a provider ID soon
					log.V(4).Info("node does not have providerID yet", "nodeName", node.Name)
					continue
				}
				// Check if the node name matches the expected pattern for this node pool
				if strings.Contains(node.Name, nodePoolNamePattern) {
					providerIDs = append(providerIDs, node.Spec.ProviderID)
				}
			}

			// Set the provider IDs on the AROMachinePool
			s.scope.SetAgentPoolProviderIDList(providerIDs)

			// Update replicas count based on actual nodes if we successfully listed them
			// This ensures the replica count matches the actual number of nodes
			currentReplicas := int32(len(providerIDs))
			if currentReplicas > 0 {
				s.scope.InfraMachinePool.Status.Replicas = currentReplicas
			}

			log.V(4).Info("populated providerIDList from workload cluster nodes",
				"nodePoolNamePattern", nodePoolNamePattern,
				"providerIDCount", len(providerIDs))
		}
	}

	// Mark as ready and set condition based on HcpOpenShiftClustersNodePool status
	ready := false
	var readyCondition *asoconditions.Condition
	for i, condition := range statusConditions {
		if condition.Type == asoconditions.ConditionTypeReady {
			readyCondition = &statusConditions[i]
			if condition.Status == metav1.ConditionTrue {
				ready = true
			}
			break
		}
	}

	// Set the NodePoolReady condition based on the HcpOpenShiftClustersNodePool status
	if ready {
		conditions.Set(s.scope.InfraMachinePool, metav1.Condition{
			Type:   string(infrav1exp.NodePoolReadyCondition),
			Status: metav1.ConditionTrue,
			Reason: "Succeeded",
		})
	} else {
		// Extract error details from HcpOpenShiftClustersNodePool's Ready condition
		reason := "Provisioning"
		message := "HcpOpenShiftClustersNodePool is not yet ready"

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

		conditions.Set(s.scope.InfraMachinePool, metav1.Condition{
			Type:    string(infrav1exp.NodePoolReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		})
	}

	s.scope.SetAgentPoolReady(ready)

	// Set the top-level AROMachinePoolReadyCondition based on overall status
	// This is the condition that clusterctl uses to display machine pool status
	if ready {
		conditions.Set(s.scope.InfraMachinePool, metav1.Condition{
			Type:   string(infrav1exp.AROMachinePoolReadyCondition),
			Status: metav1.ConditionTrue,
			Reason: "Succeeded",
		})
	} else {
		// Extract error details from NodePoolReady condition to propagate to top-level condition
		reason := "Provisioning"
		message := "ARO machine pool is not yet ready"

		if readyCondition != nil {
			if readyCondition.Reason != "" {
				reason = readyCondition.Reason
			}
			if readyCondition.Message != "" {
				message = readyCondition.Message
			}

			if readyCondition.Severity == asoconditions.ConditionSeverityError || readyCondition.Severity == asoconditions.ConditionSeverityWarning {
				message = fmt.Sprintf("[%s] %s", readyCondition.Severity, message)
			}
		}

		conditions.Set(s.scope.InfraMachinePool, metav1.Condition{
			Type:    string(infrav1exp.AROMachinePoolReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		})
	}

	// Check if all resources are ready
	allResourcesReady := true
	for _, status := range s.scope.InfraMachinePool.Status.Resources {
		if !status.Ready {
			allResourcesReady = false
			log.V(4).Info("waiting for resource to be ready", "resource", status.Resource.Name)
			break
		}
	}

	// Set initialization provisioned status for CAPI contract
	// Infrastructure is provisioned when all resources are ready
	if allResourcesReady && len(s.scope.InfraMachinePool.Status.Resources) > 0 {
		s.scope.InfraMachinePool.Status.Initialization = &infrav1exp.AROMachinePoolInitializationStatus{
			Provisioned: true,
		}
	} else if s.scope.InfraMachinePool.Status.Initialization == nil {
		s.scope.InfraMachinePool.Status.Initialization = &infrav1exp.AROMachinePoolInitializationStatus{
			Provisioned: false,
		}
	}

	// Return early if resources aren't ready to allow continued reconciliation
	if !allResourcesReady {
		return nil
	}

	log.V(4).Info("successfully reconciled AROMachinePool using resources mode")
	return nil
}

// pauseResources handles pausing when using resources mode.
func (s *aroMachinePoolService) pauseResources(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroMachinePoolService.pauseResources")
	defer done()

	log.V(4).Info("Pausing AROMachinePool using resources mode")

	// Apply mutators to get the resources
	resources, err := mutators.ToUnstructured(ctx, s.scope.InfraMachinePool.Spec.Resources)
	if err != nil {
		return errors.Wrap(err, "failed to convert resources to unstructured")
	}

	// Use the ResourceReconciler to pause resources
	resourceReconciler := s.newResourceReconciler(s.scope.InfraMachinePool, resources)

	if err := resourceReconciler.Pause(ctx); err != nil {
		return errors.Wrap(err, "failed to pause ASO resources")
	}

	return nil
}

// deleteResources handles deletion when spec.resources is specified.
func (s *aroMachinePoolService) deleteResources(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroMachinePoolService.deleteResources")
	defer done()

	log.V(4).Info("Deleting AROMachinePool using resources mode")

	// Use the ResourceReconciler to delete resources
	// Pass nil for resources to indicate all should be deleted
	resourceReconciler := s.newResourceReconciler(s.scope.InfraMachinePool, nil)

	if err := resourceReconciler.Delete(ctx); err != nil {
		return errors.Wrap(err, "failed to delete ASO resources")
	}

	// Check if there are still resources being deleted
	// The ResourceReconciler updates the status with resources that are still deleting
	for _, status := range s.scope.InfraMachinePool.Status.Resources {
		if !status.Ready {
			log.V(4).Info("waiting for resource to be deleted", "resource", status.Resource.Name)
			return azure.WithTransientError(errors.New("waiting for resources to be deleted"), 15*time.Second)
		}
	}

	return nil
}

// getHcpClusterName retrieves the HCP cluster name from the control plane.
func (s *aroMachinePoolService) getHcpClusterName() string {
	// The HCP cluster name should match the CAPI cluster name
	// This is consistent with how AROControlPlane names its HcpOpenShiftCluster
	return s.scope.ClusterName()
}
