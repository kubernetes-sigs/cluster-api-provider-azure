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

package virtualmachineextensions

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Spec input specification for Get/CreateOrUpdate/Delete calls
type Spec struct {
	Name       string
	VMName     string
	ScriptData string
}

// Get provides information about a virtual machine extension.
func (s *Service) Get(ctx context.Context, spec interface{}) (interface{}, error) {
	vmExtSpec, ok := spec.(*Spec)
	if !ok {
		return compute.VirtualMachineExtension{}, errors.New("invalid vm extension specification")
	}
	vmExt, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), vmExtSpec.VMName, vmExtSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "vm extension %s not found", vmExtSpec.Name)
	} else if err != nil {
		return vmExt, err
	}
	return vmExt, nil
}

// Reconcile gets/creates/updates a virtual machine extension.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	vmExtSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid vm extension specification")
	}

	klog.V(2).Infof("creating vm extension %s ", vmExtSpec.Name)

	err := s.Client.CreateOrUpdate(
		ctx,
		s.Scope.ResourceGroup(),
		vmExtSpec.VMName,
		vmExtSpec.Name,
		compute.VirtualMachineExtension{
			Name:     to.StringPtr(vmExtSpec.Name),
			Location: to.StringPtr(s.Scope.Location()),
			VirtualMachineExtensionProperties: &compute.VirtualMachineExtensionProperties{
				Type:                    to.StringPtr("CustomScript"),
				TypeHandlerVersion:      to.StringPtr("2.0"),
				AutoUpgradeMinorVersion: to.BoolPtr(true),
				Settings:                map[string]bool{"skipDos2Unix": true},
				Publisher:               to.StringPtr("Microsoft.Azure.Extensions"),
				ProtectedSettings:       map[string]string{"script": vmExtSpec.ScriptData},
			},
		})
	if err != nil {
		return errors.Wrapf(err, "cannot create vm extension")
	}

	klog.V(2).Infof("successfully created vm extension %s ", vmExtSpec.Name)
	return nil
}

// Delete deletes the virtual machine extension with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	vmExtSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid vm extension specification")
	}
	klog.V(2).Infof("deleting vm extension %s ", vmExtSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), vmExtSpec.VMName, vmExtSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete vm extension %s in resource group %s", vmExtSpec.Name, s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("successfully deleted vm %s ", vmExtSpec.Name)
	return nil
}
