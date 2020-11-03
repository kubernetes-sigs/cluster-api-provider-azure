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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// DiskScope defines the scope interface for a disk service.
type DiskScope interface {
	logr.Logger
	azure.ClusterDescriber
	DiskSpecs() []azure.DiskSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope DiskScope
	client
}

// New creates a new disks service.
func New(scope DiskScope) *Service {
	return &Service{
		Scope:  scope,
		client: newClient(scope),
	}
}

// Reconcile on disk is currently no-op. OS disks should only be deleted and will create with the VM automatically.
func (s *Service) Reconcile(ctx context.Context) error {
	_, span := tele.Tracer().Start(ctx, "disks.Service.Reconcile")
	defer span.End()

	return nil
}

// Delete deletes the disk associated with a VM.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "disks.Service.Delete")
	defer span.End()

	for _, diskSpec := range s.Scope.DiskSpecs() {
		s.Scope.V(2).Info("deleting disk", "disk", diskSpec.Name)
		err := s.client.Delete(ctx, s.Scope.ResourceGroup(), diskSpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete disk %s in resource group %s", diskSpec.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("successfully deleted disk", "disk", diskSpec.Name)
	}
	return nil
}
