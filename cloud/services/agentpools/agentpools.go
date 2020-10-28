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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/klog"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Spec contains properties to create a agent pool.
type Spec struct {
	Name          string
	ResourceGroup string
	Cluster       string
	Version       *string
	SKU           string
	Replicas      int32
	OSDiskSizeGB  int32
	VnetSubnetID  string
}

// Reconcile idempotently creates or updates a agent pool, if possible.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	ctx, span := tele.Tracer().Start(ctx, "agentpools.Service.Reconcile")
	defer span.End()

	agentPoolSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid agent pool specification")
	}

	profile := containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			VMSize:              containerservice.VMSizeTypes(agentPoolSpec.SKU),
			OsDiskSizeGB:        &agentPoolSpec.OSDiskSizeGB,
			Count:               &agentPoolSpec.Replicas,
			Type:                containerservice.VirtualMachineScaleSets,
			OrchestratorVersion: agentPoolSpec.Version,
			VnetSubnetID:        &agentPoolSpec.VnetSubnetID,
		},
	}

	existingPool, err := s.Client.Get(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name)
	if err != nil && !azure.ResourceNotFound(err) {
		return errors.Wrapf(err, "failed to get existing agent pool")
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
		if ps != "Canceled" && ps != "Failed" && ps != "Succeeded" {
			klog.V(2).Infof("Unable to update existing agent pool in non terminal state.  Agent pool must be in one of the following provisioning states: canceled, failed, or succeeded")
			return nil
		}

		// Normalize individual agent pools to diff in case we need to update
		existingProfile := containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				VMSize:              existingPool.ManagedClusterAgentPoolProfileProperties.VMSize,
				OsDiskSizeGB:        existingPool.ManagedClusterAgentPoolProfileProperties.OsDiskSizeGB,
				Count:               existingPool.ManagedClusterAgentPoolProfileProperties.Count,
				Type:                containerservice.VirtualMachineScaleSets,
				OrchestratorVersion: existingPool.ManagedClusterAgentPoolProfileProperties.OrchestratorVersion,
				VnetSubnetID:        existingPool.ManagedClusterAgentPoolProfileProperties.VnetSubnetID,
			},
		}

		// Diff and check if we require an update
		diff := cmp.Diff(profile, existingProfile)
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
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	ctx, span := tele.Tracer().Start(ctx, "agentpools.Service.Delete")
	defer span.End()

	agentPoolSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid agent pool specification")
	}

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
