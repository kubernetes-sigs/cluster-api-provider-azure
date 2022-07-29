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

package managedmachinepools

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/maps"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "agentpools"

// ManagedMachinePoolScope defines the scope interface for a managed machine pool.
type ManagedMachinePoolScope interface {
	azure.ClusterDescriber

	NodeResourceGroup() string
	NodePoolAnnotations() map[string]string
	NodePoolSpec() azure.AKSNodePoolSpec
	SetNodePoolProviderIDList([]string)
	SetNodePoolReplicas(int32)
	SetNodePoolReady(bool)
}

// Service provides operations on Azure resources.
type Service struct {
	scope ManagedMachinePoolScope
	Client
}

// New creates a new service.
func New(scope ManagedMachinePoolScope) *Service {
	return &Service{
		scope:  scope,
		Client: NewClient(scope),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile idempotently creates or updates a node pool, if possible.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(
		ctx,
		"managedmachinepools.Service.Reconcile",
	)
	defer done()

	nodePoolSpec := s.scope.NodePoolSpec()
	profile := converters.NodePoolToContainerServiceAgentPool(nodePoolSpec)

	existingPool, err := s.Client.Get(ctx, nodePoolSpec.ResourceGroup, nodePoolSpec.Cluster, nodePoolSpec.Name)
	if err != nil && !azure.ResourceNotFound(err) {
		return errors.Wrap(err, "failed to get existing node pool")
	}

	// For updates, we want to pass whatever we find in the existing
	// cluster, normalized to reflect the input we originally provided.
	// AKS will populate defaults and read-only values, which we want
	// to strip/clean to match what we expect.

	customHeaders := maps.FilterByKeyPrefix(s.scope.NodePoolAnnotations(), azure.CustomHeaderPrefix)
	if isCreate := azure.ResourceNotFound(err); isCreate {
		err = s.Client.CreateOrUpdate(ctx, nodePoolSpec.ResourceGroup, nodePoolSpec.Cluster, nodePoolSpec.Name,
			profile, customHeaders)
		if err != nil && azure.ResourceNotFound(err) {
			return azure.WithTransientError(errors.Wrap(err, "node pool dependent resource does not exist yet"), 20*time.Second)
		} else if err != nil {
			return errors.Wrap(err, "failed to create or update node pool")
		}
	} else {
		ps := *existingPool.ManagedClusterAgentPoolProfileProperties.ProvisioningState
		if ps != string(infrav1alpha4.Canceled) && ps != string(infrav1alpha4.Failed) && ps != string(infrav1alpha4.Succeeded) {
			msg := fmt.Sprintf("Unable to update existing node pool in non terminal state. Node pool must be in one of the following provisioning states: canceled, failed, or succeeded. Actual state: %s", ps)
			log.V(2).Info(msg)
			return azure.WithTransientError(errors.New(msg), 20*time.Second)
		}

		// Compare the existing to the updated AKS AgentPoolProfile to determine if we need to update
		existingProfile := containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				Count:               existingPool.Count,
				OrchestratorVersion: existingPool.OrchestratorVersion,
				Mode:                existingPool.Mode,
				EnableAutoScaling:   existingPool.EnableAutoScaling,
				MinCount:            existingPool.MinCount,
				MaxCount:            existingPool.MaxCount,
			},
		}

		normalizedProfile := containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				Count:               profile.Count,
				OrchestratorVersion: profile.OrchestratorVersion,
				Mode:                profile.Mode,
				EnableAutoScaling:   profile.EnableAutoScaling,
				MinCount:            profile.MinCount,
				MaxCount:            profile.MaxCount,
			},
		}

		// Diff and check if we require an update
		diff := cmp.Diff(normalizedProfile, existingProfile)
		if diff != "" {
			log.V(2).Info(fmt.Sprintf("Update required (+new -old):\n%s", diff))
			err = s.Client.CreateOrUpdate(ctx, nodePoolSpec.ResourceGroup, nodePoolSpec.Cluster, nodePoolSpec.Name,
				profile, customHeaders)
			if err != nil {
				return errors.Wrap(err, "failed to create or update node pool")
			}
		} else {
			log.V(2).Info("Normalized and desired node pool matched, no update needed")
		}
	}

	return nil
}

// Delete deletes the virtual network with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(
		ctx,
		"managedmachinepools.Service.Delete",
	)
	defer done()

	nodePoolSpec := s.scope.NodePoolSpec()

	log.V(2).Info(fmt.Sprintf("deleting node pool  %s ", nodePoolSpec.Name))
	err := s.Client.Delete(ctx, nodePoolSpec.ResourceGroup, nodePoolSpec.Cluster, nodePoolSpec.Name)
	if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		return errors.Wrapf(err, "failed to delete node pool %s in resource group %s", nodePoolSpec.Name, nodePoolSpec.ResourceGroup)
	}

	log.V(2).Info(fmt.Sprintf("Successfully deleted node pool %s ", nodePoolSpec.Name))
	return nil
}
