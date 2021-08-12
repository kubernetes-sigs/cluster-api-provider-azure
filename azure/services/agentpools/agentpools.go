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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// ManagedMachinePoolScope defines the scope interface for a managed machine pool.
type ManagedMachinePoolScope interface {
	logr.Logger
	azure.ClusterDescriber

	NodeResourceGroup() string
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

// Reconcile idempotently creates or updates a agent pool, if possible.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "agentpools.Service.Reconcile")
	defer span.End()

	agentPoolSpec := s.scope.AgentPoolSpec()

	profile := containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			VMSize:              &agentPoolSpec.SKU,
			OsType:              containerservice.OSTypeLinux,
			OsDiskSizeGB:        &agentPoolSpec.OSDiskSizeGB,
			Count:               &agentPoolSpec.Replicas,
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			OrchestratorVersion: agentPoolSpec.Version,
			VnetSubnetID:        &agentPoolSpec.VnetSubnetID,
			Mode:                containerservice.AgentPoolMode(agentPoolSpec.Mode),
		},
	}

	existingPool, err := s.Client.Get(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name)
	if err != nil && !azure.ResourceNotFound(err) {
		return errors.Wrap(err, "failed to get existing agent pool")
	}

	// For updates, we want to pass whatever we find in the existing
	// cluster, normalized to reflect the input we originally provided.
	// AKS will populate defaults and read-only values, which we want
	// to strip/clean to match what we expect.
	isCreate := azure.ResourceNotFound(err)
	if isCreate {
		err = s.Client.CreateOrUpdate(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name, profile)
		if err != nil {
			return errors.Wrap(err, "failed to create or update agent pool")
		}
	} else {
		ps := *existingPool.ManagedClusterAgentPoolProfileProperties.ProvisioningState
		if ps != string(infrav1alpha4.Canceled) && ps != string(infrav1alpha4.Failed) && ps != string(infrav1alpha4.Succeeded) {
			msg := fmt.Sprintf("Unable to update existing agent pool in non terminal state. Agent pool must be in one of the following provisioning states: canceled, failed, or succeeded. Actual state: %s", ps)
			klog.V(2).Infof(msg)
			return errors.New(msg)
		}

		// Normalize individual agent pools to diff in case we need to update
		existingProfile := containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				Count:               existingPool.Count,
				OrchestratorVersion: existingPool.OrchestratorVersion,
				Mode:                existingPool.Mode,
			},
		}

		normalizedProfile := containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				Count:               profile.Count,
				OrchestratorVersion: profile.OrchestratorVersion,
				Mode:                profile.Mode,
			},
		}

		// Diff and check if we require an update
		diff := cmp.Diff(existingProfile, normalizedProfile)
		if diff != "" {
			klog.V(2).Infof("Update required (+new -old):\n%s", diff)
			err = s.Client.CreateOrUpdate(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name, profile)
			if err != nil {
				return errors.Wrap(err, "failed to create or update agent pool")
			}
		} else {
			klog.V(2).Infof("Normalized and desired agent pool matched, no update needed")
		}
	}

	return nil
}

// Delete deletes the virtual network with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "agentpools.Service.Delete")
	defer span.End()

	agentPoolSpec := s.scope.AgentPoolSpec()

	klog.V(2).Infof("deleting agent pool  %s ", agentPoolSpec.Name)
	err := s.Client.Delete(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name)
	if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		return errors.Wrapf(err, "failed to delete agent pool %s in resource group %s", agentPoolSpec.Name, agentPoolSpec.ResourceGroup)
	}

	klog.V(2).Infof("Successfully deleted agent pool %s ", agentPoolSpec.Name)
	return nil
}
