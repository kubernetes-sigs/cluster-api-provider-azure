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

package agentpools

import (
	"context"
	"fmt"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/preview/containerservice/mgmt/2022-03-02-preview/containerservice"
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
	azure.AsyncStatusUpdater

	NodeResourceGroup() string
	AgentPoolAnnotations() map[string]string
	AgentPoolSpec() azure.AgentPoolSpec
	SetAgentPoolProviderIDList([]string)
	SetAgentPoolReplicas(int32)
	SetAgentPoolReady(bool)
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

// Reconcile idempotently creates or updates a agent pool, if possible.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(
		ctx,
		"agentpools.Service.Reconcile",
	)
	defer done()

	agentPoolSpec := s.scope.AgentPoolSpec()
	profile := converters.AgentPoolToContainerServiceAgentPool(agentPoolSpec)

	if agentPoolSpec.KubeletConfig != nil {
		profile.KubeletConfig = (*containerservice.KubeletConfig)(agentPoolSpec.KubeletConfig)
	}

	profile.Tags = agentPoolSpec.AdditionalTags

	existingPool, err := s.Client.Get(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name)
	if err != nil && !azure.ResourceNotFound(err) {
		s.scope.UpdatePutStatus(clusterv1.ReadyCondition, serviceName, err)
		return errors.Wrap(err, "failed to get existing agent pool")
	}

	// For updates, we want to pass whatever we find in the existing
	// cluster, normalized to reflect the input we originally provided.
	// AKS will populate defaults and read-only values, which we want
	// to strip/clean to match what we expect.

	customHeaders := maps.FilterByKeyPrefix(s.scope.AgentPoolAnnotations(), azure.CustomHeaderPrefix)
	if isCreate := azure.ResourceNotFound(err); isCreate {
		err = s.Client.CreateOrUpdate(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name,
			profile, customHeaders)
		s.scope.UpdatePutStatus(clusterv1.ReadyCondition, serviceName, err)
		if err != nil && azure.ResourceNotFound(err) {
			return azure.WithTransientError(errors.Wrap(err, "agent pool dependent resource does not exist yet"), 20*time.Second)
		} else if err != nil {
			return errors.Wrap(err, "failed to create or update agent pool")
		}
	} else {
		ps := *existingPool.ManagedClusterAgentPoolProfileProperties.ProvisioningState
		if ps != string(infrav1alpha4.Canceled) && ps != string(infrav1alpha4.Failed) && ps != string(infrav1alpha4.Succeeded) {
			msg := fmt.Sprintf("Unable to update existing agent pool in non terminal state. Agent pool must be in one of the following provisioning states: canceled, failed, or succeeded. Actual state: %s", ps)
			log.V(2).Info(msg)
			retErr := azure.WithTransientError(errors.New(msg), 20*time.Second)
			s.scope.UpdatePatchStatus(clusterv1.ReadyCondition, serviceName, retErr)
			return retErr
		}

		// When tags are removed, the change will be ignored if only set to nil.
		if existingPool.Tags != nil && len(existingPool.Tags) > 0 && profile.Tags == nil {
			profile.Tags = map[string]*string{}
			log.V(2).Info("Remove additional tags from agent pool")
		}

		// Normalize individual agent pools to diff in case we need to update
		existingProfile := containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				Count:               existingPool.Count,
				OrchestratorVersion: existingPool.OrchestratorVersion,
				Mode:                existingPool.Mode,
				EnableAutoScaling:   existingPool.EnableAutoScaling,
				MinCount:            existingPool.MinCount,
				MaxCount:            existingPool.MaxCount,
				NodeLabels:          existingPool.NodeLabels,
				Tags:                existingPool.Tags,
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
				NodeLabels:          profile.NodeLabels,
				Tags:                profile.Tags,
			},
		}

		// Auto scaler enabled, no need to reconcile on node pool count
		if profile.MinCount != nil && profile.MaxCount != nil {
			normalizedProfile.Count = existingPool.Count
		}

		// Diff and check if we require an update
		diff := cmp.Diff(normalizedProfile, existingProfile)
		if diff != "" {
			log.V(2).Info(fmt.Sprintf("Update required (+new -old):\n%s", diff))
			err = s.Client.CreateOrUpdate(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name,
				profile, customHeaders)
			s.scope.UpdatePatchStatus(clusterv1.ReadyCondition, serviceName, err)
			if err != nil {
				return errors.Wrap(err, "failed to create or update agent pool")
			}
		} else {
			s.scope.UpdatePatchStatus(clusterv1.ReadyCondition, serviceName, nil)
			log.V(2).Info("Normalized and desired agent pool matched, no update needed")
		}
	}

	return nil
}

// Delete deletes the virtual network with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(
		ctx,
		"agentpools.Service.Delete",
	)
	defer done()

	agentPoolSpec := s.scope.AgentPoolSpec()

	log.V(2).Info(fmt.Sprintf("deleting agent pool  %s ", agentPoolSpec.Name))
	err := s.Client.Delete(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name)
	s.scope.UpdateDeleteStatus(clusterv1.ReadyCondition, serviceName, err)
	if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		return errors.Wrapf(err, "failed to delete agent pool %s in resource group %s", agentPoolSpec.Name, agentPoolSpec.ResourceGroup)
	}

	log.V(2).Info(fmt.Sprintf("Successfully deleted agent pool %s ", agentPoolSpec.Name))
	return nil
}
