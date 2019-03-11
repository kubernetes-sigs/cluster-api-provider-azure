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

package network

import (
	"fmt"

	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
)

// ReconcileNetwork reconciles the network of the given cluster.
func (s *Service) ReconcileNetwork() (err error) {
	klog.V(2).Info("Reconciling network")

	// TODO: Refactor
	// TODO: Fix hardcoded values.
	// Reconcile network security group
	_, err = s.CreateOrUpdateNetworkSecurityGroup(s.scope.ClusterConfig.ResourceGroup, SecurityGroupDefaultName)
	if err != nil {
		return fmt.Errorf("error creating or updating network security group: %v", err)
	}

	// Reconcile virtual network
	vnet, err := s.CreateOrUpdateVnet(s.scope.ClusterConfig.ResourceGroup, "")
	if err != nil {
		return fmt.Errorf("error creating or updating virtual network: %v", err)
	}

	s.scope.Vnet().ID = *vnet.ID
	s.scope.Vnet().Name = *vnet.Name

	// TODO: This should reconcile the subnet list. Right now, it only appends.
	azsubnets := *vnet.Subnets
	for _, azsubnet := range azsubnets {
		s.scope.ClusterConfig.NetworkSpec.Subnets = append(
			// TODO: Complete SubnetSpec struct
			s.scope.ClusterConfig.NetworkSpec.Subnets,
			&v1alpha1.SubnetSpec{
				ID:   *azsubnet.ID,
				Name: *azsubnet.Name,
			},
		)
	}

	klog.V(2).Info("Reconcile network completed successfully")
	return nil
}

// TODO: Implement method
/*
// DeleteNetwork deletes the network of the given cluster.
func (s *Service) DeleteNetwork() (err error) {
	klog.V(2).Info("Deleting network")

	klog.V(2).Info("Delete network completed successfully")
	return nil
}
*/
