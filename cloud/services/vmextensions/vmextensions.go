/*
Copyright 2021 The Kubernetes Authors.

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

package vmextensions

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// VMExtensionScope defines the scope interface for a vm extension service.
type VMExtensionScope interface {
	logr.Logger
	azure.ClusterDescriber
	VMExtensionSpecs() []azure.VMExtensionSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope VMExtensionScope
	client
}

// New creates a new vm extension service.
func New(scope VMExtensionScope) *Service {
	return &Service{
		Scope:  scope,
		client: newClient(scope),
	}
}

// Reconcile creates or updates the VM extension.
func (s *Service) Reconcile(ctx context.Context) error {
	_, span := tele.Tracer().Start(ctx, "vmextensions.Service.Reconcile")
	defer span.End()

	for _, extensionSpec := range s.Scope.VMExtensionSpecs() {
		s.Scope.V(2).Info("creating VM extension", "vm extension", extensionSpec.Name)
		err := s.client.CreateOrUpdate(
			ctx,
			s.Scope.ResourceGroup(),
			extensionSpec.VMName,
			extensionSpec.Name,
			compute.VirtualMachineExtension{
				VirtualMachineExtensionProperties: &compute.VirtualMachineExtensionProperties{
					Publisher:          to.StringPtr(extensionSpec.Publisher),
					Type:               to.StringPtr(extensionSpec.Name),
					TypeHandlerVersion: to.StringPtr(extensionSpec.Version),
					Settings:           nil,
					ProtectedSettings:  nil,
				},
				Location: to.StringPtr(s.Scope.Location()),
			},
		)
		if err != nil {
			return errors.Wrapf(err, "failed to create VM extension %s on VM %s in resource group %s", extensionSpec.Name, extensionSpec.VMName, s.Scope.ResourceGroup())
		}
		s.Scope.V(2).Info("successfully created VM extension", "vm extension", extensionSpec.Name)
	}
	return nil
}

// Delete is a no-op. Extensions will be deleted as part of VM deletion.
func (s *Service) Delete(ctx context.Context) error {
	return nil
}
