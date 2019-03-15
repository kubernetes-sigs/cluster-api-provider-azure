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

package cluster

import (
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/certificates"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/groups"
)

// Reconciler are list of services required by cluster actuator, easy to create a fake
type Reconciler struct {
	scope           *actuators.Scope
	groupsSvc       services.Service
	certificatesSvc services.Service
}

// NewReconciler populates all the services based on input scope
func NewReconciler(scope *actuators.Scope) *Reconciler {
	return &Reconciler{
		scope:           scope,
		groupsSvc:       groups.NewService(scope),
		certificatesSvc: certificates.NewService(scope),
	}
}

// Reconcile reconciles all the services in pre determined order
func (s *Reconciler) Reconcile() error {
	// Store cert material in spec.
	if err := s.certificatesSvc.CreateOrUpdate(s.scope.Context); err != nil {
		return errors.Wrapf(err, "failed to CreateOrUpdate certificates for cluster %s", s.scope.Cluster.Name)
	}

	if err := s.groupsSvc.CreateOrUpdate(s.scope.Context); err != nil {
		return errors.Wrapf(err, "failed to CreateOrUpdate resource group for cluster %s", s.scope.Cluster.Name)
	}
	return nil
}

// Delete reconciles all the services in pre determined order
func (s *Reconciler) Delete() error {
	if err := s.groupsSvc.Delete(s.scope.Context); err != nil {
		if services.ResourceNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "failed to Delete resource group for cluster %s", s.scope.Cluster.Name)
	}
	return nil
}
