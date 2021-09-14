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

package disks

import (
	"context"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "disks"

// DiskScope defines the scope interface for a disk service.
type DiskScope interface {
	azure.ClusterDescriber
	azure.AsyncStatusUpdater
	DiskSpecs() []azure.ResourceSpecGetter
}

// Service provides operations on Azure resources.
type Service struct {
	Scope DiskScope
	async.Reconciler
}

// New creates a new disks service.
func New(scope DiskScope) *Service {
	client := newClient(scope)
	return &Service{
		Scope:      scope,
		Reconciler: async.New(scope, nil, client),
	}
}

// Reconcile on disk is currently no-op. OS disks should only be deleted and will create with the VM automatically.
func (s *Service) Reconcile(ctx context.Context) error {
	_, _, done := tele.StartSpanWithLogger(ctx, "disks.Service.Reconcile")
	defer done()

	// DisksReadyCondition is set in the VM service.
	return nil
}

// Delete deletes the disk associated with a VM.
func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "disks.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	var result error

	// We go through the list of DiskSpecs to delete each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (ie. error creating) -> operationNotDoneError (ie. creating in progress) -> no error (ie. created)
	for _, diskSpec := range s.Scope.DiskSpecs() {
		if err := s.DeleteResource(ctx, diskSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}
	s.Scope.UpdateDeleteStatus(infrav1.DisksReadyCondition, serviceName, result)
	return result
}
