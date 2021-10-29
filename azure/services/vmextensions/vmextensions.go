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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// VMExtensionScope defines the scope interface for a vm extension service.
type VMExtensionScope interface {
	azure.ClusterDescriber
	VMExtensionSpecs() []azure.ExtensionSpec
	SetBootstrapConditions(context.Context, string, string) error
}

// Service provides operations on Azure resources.
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
	ctx, log, done := tele.StartSpanWithLogger(ctx, "vmextensions.Service.Reconcile")
	defer done()

	for _, extensionSpec := range s.Scope.VMExtensionSpecs() {
		if existing, err := s.client.Get(ctx, s.Scope.ResourceGroup(), extensionSpec.VMName, extensionSpec.Name); err == nil {
			// check the extension status and set the associated conditions.
			if retErr := s.Scope.SetBootstrapConditions(ctx, to.String(existing.ProvisioningState), extensionSpec.Name); retErr != nil {
				return retErr
			}
			// if the extension already exists, do not update it.
			continue
		} else if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to get vm extension %s on vm %s", extensionSpec.Name, extensionSpec.VMName)
		}

		log.V(2).Info("creating VM extension", "vm extension", extensionSpec.Name)
		err := s.client.CreateOrUpdateAsync(
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
					ProtectedSettings:  extensionSpec.ProtectedSettings,
				},
				Location: to.StringPtr(s.Scope.Location()),
			},
		)
		if err != nil {
			return errors.Wrapf(err, "failed to create VM extension %s on VM %s in resource group %s", extensionSpec.Name, extensionSpec.VMName, s.Scope.ResourceGroup())
		}
		log.V(2).Info("successfully created VM extension", "vm extension", extensionSpec.Name)
	}
	return nil
}

// Delete is a no-op. Extensions will be deleted as part of VM deletion.
func (s *Service) Delete(_ context.Context) error {
	return nil
}
