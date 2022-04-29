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

package virtualmachines

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "virtualmachine"

// VMScope defines the scope interface for a virtual machines service.
type VMScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	VMSpec() azure.ResourceSpecGetter
	SetAnnotation(string, string)
	SetProviderID(string)
	SetAddresses([]corev1.NodeAddress)
	SetVMState(infrav1.ProvisioningState)
}

// Service provides operations on Azure resources.
type Service struct {
	Scope VMScope
	async.Reconciler
	interfacesGetter async.Getter
	publicIPsClient  publicips.Client
}

// New creates a new service.
func New(scope VMScope) *Service {
	Client := NewClient(scope)
	return &Service{
		Scope:            scope,
		interfacesGetter: networkinterfaces.NewClient(scope),
		publicIPsClient:  publicips.NewClient(scope),
		Reconciler:       async.New(scope, Client, Client),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile gets/creates/updates a virtual machine.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	vmSpec := s.Scope.VMSpec()
	if vmSpec == nil {
		return nil
	}

	result, err := s.CreateResource(ctx, vmSpec, serviceName)
	s.Scope.UpdatePutStatus(infrav1.VMRunningCondition, serviceName, err)
	// Set the DiskReady condition here since the disk gets created with the VM.
	s.Scope.UpdatePutStatus(infrav1.DisksReadyCondition, serviceName, err)
	if err == nil && result != nil {
		vm, ok := result.(compute.VirtualMachine)
		if !ok {
			return errors.Errorf("%T is not a compute.VirtualMachine", result)
		}
		infraVM, err := converters.SDKToVM(vm)
		if err != nil {
			return err
		}
		s.Scope.SetProviderID(azure.ProviderIDPrefix + infraVM.ID)
		s.Scope.SetAnnotation("cluster-api-provider-azure", "true")

		// Discover addresses for NICs associated with the VM
		addresses, err := s.getAddresses(ctx, vm, vmSpec.ResourceGroupName())
		if err != nil {
			return errors.Wrap(err, "failed to fetch VM addresses")
		}
		s.Scope.SetAddresses(addresses)
		s.Scope.SetVMState(infraVM.State)
	}
	return err
}

// Delete deletes the virtual machine with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	vmSpec := s.Scope.VMSpec()
	if vmSpec == nil {
		return nil
	}

	err := s.DeleteResource(ctx, vmSpec, serviceName)
	if err != nil {
		s.Scope.SetVMState(infrav1.Deleting)
	} else {
		s.Scope.SetVMState(infrav1.Deleted)
	}
	s.Scope.UpdateDeleteStatus(infrav1.VMRunningCondition, serviceName, err)
	return err
}

func (s *Service) getAddresses(ctx context.Context, vm compute.VirtualMachine, rgName string) ([]corev1.NodeAddress, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.Service.getAddresses")
	defer done()

	addresses := []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalDNS,
			Address: to.String(vm.Name),
		},
	}
	if vm.NetworkProfile.NetworkInterfaces == nil {
		return addresses, nil
	}
	for _, nicRef := range *vm.NetworkProfile.NetworkInterfaces {
		// The full ID includes the name at the very end. Split the string and pull the last element
		// Ex: /subscriptions/$SUB/resourceGroups/$RG/providers/Microsoft.Network/networkInterfaces/$NICNAME
		// We'll check to see if ID is nil and bail early if we don't have it
		if nicRef.ID == nil {
			continue
		}
		nicName := getResourceNameByID(to.String(nicRef.ID))

		// Fetch nic and append its addresses
		existingNic, err := s.interfacesGetter.Get(ctx, &networkinterfaces.NICSpec{
			Name:          nicName,
			ResourceGroup: rgName,
		})
		if err != nil {
			return addresses, err
		}

		nic, ok := existingNic.(network.Interface)
		if !ok {
			return nil, errors.Errorf("%T is not a network.Interface", existingNic)
		}

		if nic.IPConfigurations == nil {
			continue
		}
		for _, ipConfig := range *nic.IPConfigurations {
			if ipConfig.PrivateIPAddress != nil {
				addresses = append(addresses,
					corev1.NodeAddress{
						Type:    corev1.NodeInternalIP,
						Address: to.String(ipConfig.PrivateIPAddress),
					},
				)
			}

			if ipConfig.PublicIPAddress == nil {
				continue
			}
			// ID is the only field populated in PublicIPAddress sub-resource.
			// Thus, we have to go fetch the publicIP with the name.
			publicIPName := getResourceNameByID(to.String(ipConfig.PublicIPAddress.ID))
			publicNodeAddress, err := s.getPublicIPAddress(ctx, publicIPName, rgName)
			if err != nil {
				return addresses, err
			}
			addresses = append(addresses, publicNodeAddress)
		}
	}

	return addresses, nil
}

// getPublicIPAddress will fetch a public ip address resource by name and return a nodeaddresss representation.
func (s *Service) getPublicIPAddress(ctx context.Context, publicIPAddressName string, rgName string) (corev1.NodeAddress, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachines.Service.getPublicIPAddress")
	defer done()

	retAddress := corev1.NodeAddress{}
	publicIP, err := s.publicIPsClient.Get(ctx, rgName, publicIPAddressName)
	if err != nil {
		return retAddress, err
	}
	retAddress.Type = corev1.NodeExternalIP
	retAddress.Address = to.String(publicIP.IPAddress)

	return retAddress, nil
}

// getResourceNameById takes a resource ID like
// `/subscriptions/$SUB/resourceGroups/$RG/providers/Microsoft.Network/networkInterfaces/$NICNAME`
// and parses out the string after the last slash.
func getResourceNameByID(resourceID string) string {
	explodedResourceID := strings.Split(resourceID, "/")
	resourceName := explodedResourceID[len(explodedResourceID)-1]
	return resourceName
}

// IsManaged returns always returns true as CAPZ does not support BYO VM.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}
