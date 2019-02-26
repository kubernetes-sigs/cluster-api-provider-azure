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
)

// ReconcileNetwork reconciles the network of the given cluster.
func (s *Service) ReconcileNetwork() (err error) {
	klog.V(2).Info("Reconciling network")

	// TODO: Refactor
	// TODO: Fix hardcoded values.
	// Reconcile network security group
	networkSGFuture, err := s.CreateOrUpdateNetworkSecurityGroup(s.scope.ClusterConfig.ResourceGroup, "ClusterAPINSG", s.scope.ClusterConfig.Location)
	if err != nil {
		return fmt.Errorf("error creating or updating network security group: %v", err)
	}
	err = s.WaitForNetworkSGsCreateOrUpdateFuture(*networkSGFuture)
	if err != nil {
		return fmt.Errorf("error waiting for network security group creation or update: %v", err)
	}

	// Reconcile virtual network
	vnetFuture, err := s.CreateOrUpdateVnet(s.scope.ClusterConfig.ResourceGroup, "", s.scope.ClusterConfig.Location)
	if err != nil {
		return fmt.Errorf("error creating or updating virtual network: %v", err)
	}
	err = s.WaitForVnetCreateOrUpdateFuture(*vnetFuture)
	if err != nil {
		return fmt.Errorf("error waiting for virtual network creation or update: %v", err)
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
