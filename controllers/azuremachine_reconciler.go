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

package controllers

import (
	"encoding/base64"
	"time"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/availabilityzones"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/disks"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
)

const (
	// DefaultBootstrapTokenTTL default ttl for bootstrap token
	DefaultBootstrapTokenTTL = 10 * time.Minute
)

// azureMachineService are list of services required by cluster actuator, easy to create a fake
type azureMachineService struct {
	machineScope         *scope.MachineScope
	clusterScope         *scope.ClusterScope
	availabilityZonesSvc azure.GetterService
	networkInterfacesSvc azure.Service
	virtualMachinesSvc   azure.GetterService
	disksSvc             azure.GetterService
}

// newAzureMachineService populates all the services based on input scope
func newAzureMachineService(machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) *azureMachineService {
	return &azureMachineService{
		machineScope:         machineScope,
		clusterScope:         clusterScope,
		availabilityZonesSvc: availabilityzones.NewService(clusterScope),
		networkInterfacesSvc: networkinterfaces.NewService(clusterScope, machineScope),
		virtualMachinesSvc:   virtualmachines.NewService(clusterScope, machineScope),
		disksSvc:             disks.NewService(clusterScope),
	}
}

// Create creates machine if and only if machine exists, handled by cluster-api
func (s *azureMachineService) Create() (*infrav1.VM, error) {
	nicName := azure.GenerateNICName(s.machineScope.Name())
	nicErr := s.reconcileNetworkInterface(nicName)
	if nicErr != nil {
		return nil, errors.Wrapf(nicErr, "failed to create NIC %s for machine %s", nicName, s.machineScope.Name())
	}

	vm, vmErr := s.createVirtualMachine(nicName)
	if vmErr != nil {
		return nil, errors.Wrapf(vmErr, "failed to create VM %s ", s.machineScope.Name())
	}

	return vm, nil
}

// Delete reconciles all the services in pre determined order
func (s *azureMachineService) Delete() error {
	vmSpec := &virtualmachines.Spec{
		Name: s.machineScope.Name(),
	}

	err := s.virtualMachinesSvc.Delete(s.clusterScope.Context, vmSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to delete machine")
	}

	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:        azure.GenerateNICName(s.machineScope.Name()),
		VnetName:    s.clusterScope.Vnet().Name,
		MachineRole: s.machineScope.Role(),
	}

	if s.machineScope.Role() == infrav1.ControlPlane {
		networkInterfaceSpec.PublicLoadBalancerName = azure.GeneratePublicLBName(s.clusterScope.Name())
	} else if s.machineScope.Role() == infrav1.Node {
		networkInterfaceSpec.PublicLoadBalancerName = s.clusterScope.Name()
	}

	err = s.networkInterfacesSvc.Delete(s.clusterScope.Context, networkInterfaceSpec)
	if err != nil {
		return errors.Wrapf(err, "Unable to delete network interface")
	}

	OSDiskSpec := &disks.Spec{
		Name: azure.GenerateOSDiskName(s.machineScope.Name()),
	}
	err = s.disksSvc.Delete(s.clusterScope.Context, OSDiskSpec)
	if err != nil {
		return errors.Wrapf(err, "Failed to delete OS disk of machine %s", s.machineScope.Name())
	}

	return nil
}

func (s *azureMachineService) VMIfExists(id *string) (*infrav1.VM, error) {
	if id == nil {
		s.clusterScope.Info("VM does not have an ID")
		return nil, nil
	}

	vmSpec := &virtualmachines.Spec{
		Name: s.machineScope.Name(),
	}
	vmInterface, err := s.virtualMachinesSvc.Get(s.clusterScope.Context, vmSpec)

	if err != nil && vmInterface == nil {
		return nil, nil
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get VM")
	}

	vm, ok := vmInterface.(*infrav1.VM)
	if !ok {
		return nil, errors.New("returned incorrect vm interface")
	}

	klog.V(2).Infof("Found VM for AzureMachine %s", s.machineScope.Name())

	return vm, nil
}

// getVirtualMachineZone gets a random availability zones from available set,
// this will hopefully be an input from upstream machinesets so all the vms are balanced
func (s *azureMachineService) getVirtualMachineZone() (string, error) {
	vmName := s.machineScope.AzureMachine.Name
	vmSize := s.machineScope.AzureMachine.Spec.VMSize
	location := s.machineScope.AzureMachine.Spec.Location

	zonesSpec := &availabilityzones.Spec{
		VMSize: to.StringPtr(vmSize),
	}
	zonesInterface, err := s.availabilityZonesSvc.Get(s.clusterScope.Context, zonesSpec)
	if err != nil {
		return "", errors.Wrapf(err, "failed to check availability zones for %s in region %s", vmSize, location)
	}
	if zonesInterface == nil {
		// if its nil, probably means no zones found
		return "", nil
	}
	zones, ok := zonesInterface.([]string)
	if !ok {
		return "", errors.New("availability zones Get returned invalid interface")
	}

	if len(zones) <= 0 {
		return "", nil
	}

	zone := s.machineScope.AvailabilityZone()
	var selectedZone string

	// DEPRECATED: to support old clients
	if zone == "" && s.machineScope.AzureMachine.Spec.AvailabilityZone.ID != nil {
		zone = *s.machineScope.AzureMachine.Spec.AvailabilityZone.ID
	}

	if zone != "" {
		for _, allowedZone := range zones {
			if allowedZone == zone {
				selectedZone = zone
				break
			}
		}
	} else {
		klog.Infof("Selecting first available AZ as no availability zone was set or user-provided availability zone is not supported for VM size %s in location %s", vmSize, location)
		selectedZone = zones[0]
	}

	klog.Infof("Selected availability zone %s for %s", selectedZone, vmName)

	return selectedZone, nil
}

func (s *azureMachineService) reconcileNetworkInterface(nicName string) error {
	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:        nicName,
		VnetName:    s.clusterScope.Vnet().Name,
		MachineRole: s.machineScope.Role(),
	}

	if s.machineScope.AzureMachine.Spec.AllocatePublicIP == true {
		networkInterfaceSpec.PublicIPName = azure.GenerateNodePublicIPName(nicName)
	}

	networkInterfaceSpec.AcceleratedNetworking = s.machineScope.AzureMachine.Spec.AcceleratedNetworking

	switch role := s.machineScope.Role(); role {
	case infrav1.Node:
		networkInterfaceSpec.SubnetName = s.clusterScope.NodeSubnet().Name
		networkInterfaceSpec.PublicLoadBalancerName = s.clusterScope.Name()
	case infrav1.ControlPlane:
		networkInterfaceSpec.SubnetName = s.clusterScope.ControlPlaneSubnet().Name
		networkInterfaceSpec.PublicLoadBalancerName = azure.GeneratePublicLBName(s.clusterScope.Name())
		networkInterfaceSpec.InternalLoadBalancerName = azure.GenerateInternalLBName(s.clusterScope.Name())
	default:
		return errors.Errorf("unknown value %s for label `set` on machine %s, skipping machine creation", role, s.machineScope.Name())
	}

	err := s.networkInterfacesSvc.Reconcile(s.clusterScope.Context, networkInterfaceSpec)
	if err != nil {
		return errors.Wrap(err, "unable to create VM network interface")
	}

	return err
}

func (s *azureMachineService) createVirtualMachine(nicName string) (*infrav1.VM, error) {
	var vm *infrav1.VM
	decoded, err := base64.StdEncoding.DecodeString(s.machineScope.AzureMachine.Spec.SSHPublicKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode ssh public key")
	}

	vmSpec := &virtualmachines.Spec{
		Name: s.machineScope.Name(),
	}

	vmInterface, err := s.virtualMachinesSvc.Get(s.clusterScope.Context, vmSpec)
	if err != nil && vmInterface == nil {
		var vmZone string

		azSupported := s.isAvailabilityZoneSupported()

		if azSupported {
			useAZ := true

			if s.machineScope.AzureMachine.Spec.AvailabilityZone.Enabled != nil {
				useAZ = *s.machineScope.AzureMachine.Spec.AvailabilityZone.Enabled
			}

			if useAZ {
				var zoneErr error
				vmZone, zoneErr = s.getVirtualMachineZone()
				if zoneErr != nil {
					return nil, errors.Wrap(zoneErr, "failed to get availability zone")
				}
			}
		}

		image, err := getVMImage(s.machineScope)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get VM image")
		}

		bootstrapData, err := s.machineScope.GetBootstrapData()
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve bootstrap data")
		}

		vmSpec = &virtualmachines.Spec{
			Name:                   s.machineScope.Name(),
			NICName:                nicName,
			SSHKeyData:             string(decoded),
			Size:                   s.machineScope.AzureMachine.Spec.VMSize,
			OSDisk:                 s.machineScope.AzureMachine.Spec.OSDisk,
			Image:                  image,
			CustomData:             bootstrapData,
			Zone:                   vmZone,
			Identity:               s.machineScope.AzureMachine.Spec.Identity,
			UserAssignedIdentities: s.machineScope.AzureMachine.Spec.UserAssignedIdentities,
		}

		err = s.virtualMachinesSvc.Reconcile(s.clusterScope.Context, vmSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create or get machine")
		}
	} else if err != nil {
		return nil, errors.Wrap(err, "failed to get vm")
	}

	newVM, err := s.virtualMachinesSvc.Get(s.clusterScope.Context, vmSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vm")
	}

	vm, ok := newVM.(*infrav1.VM)
	if !ok {
		return nil, errors.New("returned incorrect vm interface")
	}
	if vm.State == "" {
		return nil, errors.Errorf("vm %s is nil provisioning state, reconcile", s.machineScope.Name())
	}

	if vm.State == infrav1.VMStateFailed {
		// If VM failed provisioning, delete it so it can be recreated
		err = s.virtualMachinesSvc.Delete(s.clusterScope.Context, vmSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to delete machine")
		}
		return nil, errors.Errorf("vm %s is deleted, retry creating in next reconcile", s.machineScope.Name())
	} else if vm.State != infrav1.VMStateSucceeded {
		return nil, errors.Errorf("vm %s is still in provisioningstate %s, reconcile", s.machineScope.Name(), vm.State)
	}

	return vm, nil
}

// GetControlPlaneMachines retrieves all non-deleted control plane nodes from a MachineList
func GetControlPlaneMachines(machineList *clusterv1.MachineList) []*clusterv1.Machine {
	var cpm []*clusterv1.Machine
	for _, m := range machineList.Items {
		m := m
		if util.IsControlPlaneMachine(&m) {
			cpm = append(cpm, m.DeepCopy())
		}
	}
	return cpm
}

// isAvailabilityZoneSupported determines if Availability Zones are supported in a selected location
// based on SupportedAvailabilityZoneLocations. Returns true if supported.
func (s *azureMachineService) isAvailabilityZoneSupported() bool {
	azSupported := false

	for _, supportedLocation := range azure.SupportedAvailabilityZoneLocations {
		if s.machineScope.Location() == supportedLocation {
			azSupported = true

			return azSupported
		}
	}

	s.machineScope.V(2).Info("Availability Zones are not supported in the selected location", "location", s.machineScope.Location())
	return azSupported
}

// Pick image from the machine configuration, or use a default one.
func getVMImage(scope *scope.MachineScope) (*infrav1.Image, error) {
	// Use custom Marketplace image, Image ID or a Shared Image Gallery image if provided
	if scope.AzureMachine.Spec.Image != nil {
		return scope.AzureMachine.Spec.Image, nil
	}
	scope.Info("No image specified for machine, using default", "machine", scope.AzureMachine.GetName())
	return azure.GetDefaultUbuntuImage(to.String(scope.Machine.Spec.Version))
}
