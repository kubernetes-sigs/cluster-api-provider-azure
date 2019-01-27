/*
Copyright 2018 The Kubernetes Authors.

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

package compute

import (
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
)

// RunCommand executes a command on the VM.
func (s *Service) RunCommand(resoureGroup string, name string, cmd string) (compute.VirtualMachinesRunCommandFuture, error) {
	cmdInput := compute.RunCommandInput{
		CommandID: to.StringPtr("RunShellScript"),
		Script:    to.StringSlicePtr([]string{cmd}),
	}
	return s.VirtualMachinesClient.RunCommand(s.ctx, resoureGroup, name, cmdInput)
}

// VMIfExists returns the reference to the VM object if it exists.
func (s *Service) VMIfExists(resourceGroup string, name string) (*compute.VirtualMachine, error) {
	vm, err := s.VirtualMachinesClient.Get(s.ctx, resourceGroup, name, "")
	if err != nil {
		if aerr, ok := err.(autorest.DetailedError); ok {
			if aerr.StatusCode.(int) == 404 {
				return nil, nil
			}
		}
		return nil, err
	}
	return &vm, nil
}

// DeleteVM deletes the virtual machine.
func (s *Service) DeleteVM(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error) {
	return s.VirtualMachinesClient.Delete(s.ctx, resourceGroup, name)
}

// WaitForVMRunCommandFuture returns when the RunCommand operation completes.
func (s *Service) WaitForVMRunCommandFuture(future compute.VirtualMachinesRunCommandFuture) error {
	return future.Future.WaitForCompletionRef(s.ctx, s.VirtualMachinesClient.Client)
}

// WaitForVMDeletionFuture returns when the DeleteVM operation completes.
func (s *Service) WaitForVMDeletionFuture(future compute.VirtualMachinesDeleteFuture) error {
	return future.Future.WaitForCompletionRef(s.ctx, s.VirtualMachinesClient.Client)
}
