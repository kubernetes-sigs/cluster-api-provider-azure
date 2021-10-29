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

package vmssextensions

import (
	"context"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// VMSSExtensionScope defines the scope interface for a vmss extension service.
type VMSSExtensionScope interface {
	azure.ClusterDescriber
	VMSSExtensionSpecs() []azure.ExtensionSpec
	SetBootstrapConditions(context.Context, string, string) error
}

// Service provides operations on Azure resources.
type Service struct {
	Scope VMSSExtensionScope
	client
}

// New creates a new vm extension service.
func New(scope VMSSExtensionScope) *Service {
	return &Service{
		Scope:  scope,
		client: newClient(scope),
	}
}

// Reconcile creates or updates the VMSS extension.
func (s *Service) Reconcile(ctx context.Context) error {
	_, _, done := tele.StartSpanWithLogger(ctx, "vmssextensions.Service.Reconcile")
	defer done()

	for _, extensionSpec := range s.Scope.VMSSExtensionSpecs() {
		if existing, err := s.client.Get(ctx, s.Scope.ResourceGroup(), extensionSpec.VMName, extensionSpec.Name); err == nil {
			// check the extension status and set the associated conditions.
			if retErr := s.Scope.SetBootstrapConditions(ctx, to.String(existing.ProvisioningState), extensionSpec.Name); retErr != nil {
				return retErr
			}
		} else if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to get vm extension %s on scale set %s", extensionSpec.Name, extensionSpec.VMName)
		}
		//  Nothing else to do here, the extensions are applied to the model as part of the scale set Reconcile.
		continue
	}
	return nil
}

// Delete is a no-op. Extensions will be deleted as part of VMSS deletion.
func (s *Service) Delete(_ context.Context) error {
	return nil
}
