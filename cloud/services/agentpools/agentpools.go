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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/klog"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
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
}

// Get fetches a agent pool from Azure.
func (s *Service) Get(ctx context.Context, spec interface{}) (interface{}, error) {
	agentPoolSpec, ok := spec.(*Spec)
	if !ok {
		return containerservice.AgentPool{}, errors.New("expected agent pool specification")
	}
	return s.Client.Get(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name)
}

// Reconcile idempotently creates or updates a agent pool, if possible.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	agentPoolSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("expected agent pool specification")
	}

	profile := containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			VMSize:              containerservice.VMSizeTypes(agentPoolSpec.SKU),
			OsDiskSizeGB:        &agentPoolSpec.OSDiskSizeGB,
			Count:               &agentPoolSpec.Replicas,
			Type:                containerservice.VirtualMachineScaleSets,
			OrchestratorVersion: agentPoolSpec.Version,
		},
	}

	existingSpec, err := s.Get(ctx, spec)
	existingPool, ok := existingSpec.(containerservice.AgentPool)
	if !ok {
		return errors.New("expected agent pool specification")
	}

	if err == nil {
		existingProfile := containerservice.AgentPool{
			ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
				VMSize:              existingPool.ManagedClusterAgentPoolProfileProperties.VMSize,
				OsDiskSizeGB:        existingPool.ManagedClusterAgentPoolProfileProperties.OsDiskSizeGB,
				Count:               existingPool.ManagedClusterAgentPoolProfileProperties.Count,
				Type:                containerservice.VirtualMachineScaleSets,
				OrchestratorVersion: existingPool.ManagedClusterAgentPoolProfileProperties.OrchestratorVersion,
			},
		}

		diff := cmp.Diff(profile, existingProfile)
		if diff != "" {
			klog.V(2).Infof("update required (+new -old):\n%s", diff)
			err = s.Client.CreateOrUpdate(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name, profile)
			if err != nil {
				return fmt.Errorf("failed to create or update agent pool, %#+v", err)
			}
		} else {
			klog.V(2).Infof("normalized and desired managed cluster matched, no update needed")
		}
	} else if azure.ResourceNotFound(err) {
		err = s.Client.CreateOrUpdate(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name, profile)
		if err != nil {
			return fmt.Errorf("failed to create or update agent pool, %#+v", err)
		}
	}

	return nil
}

// Delete deletes the virtual network with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	agentPoolSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("expected agent pool specification")
	}

	klog.V(2).Infof("deleting agent pool  %s ", agentPoolSpec.Name)
	err := s.Client.Delete(ctx, agentPoolSpec.ResourceGroup, agentPoolSpec.Cluster, agentPoolSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete agent pool %s in resource group %s", agentPoolSpec.Name, agentPoolSpec.ResourceGroup)
	}

	klog.V(2).Infof("successfully deleted agent pool %s ", agentPoolSpec.Name)
	return nil
}
