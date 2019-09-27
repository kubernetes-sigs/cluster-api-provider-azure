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
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/pkg/errors"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/availabilityzones"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/disks"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachineextensions"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util"
)

const (
	// DefaultBootstrapTokenTTL default ttl for bootstrap token
	DefaultBootstrapTokenTTL = 10 * time.Minute
)

// azureMachineReconciler are list of services required by cluster actuator, easy to create a fake
type azureMachineReconciler struct {
	machineScope          *scope.MachineScope
	clusterScope          *scope.ClusterScope
	availabilityZonesSvc  azure.GetterService
	networkInterfacesSvc  azure.Service
	virtualMachinesSvc    azure.GetterService
	virtualMachinesExtSvc azure.GetterService
	disksSvc              azure.GetterService
}

// newAzureMachineReconciler populates all the services based on input scope
func newAzureMachineReconciler(machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) *azureMachineReconciler {
	return &azureMachineReconciler{
		machineScope:          machineScope,
		clusterScope:          clusterScope,
		availabilityZonesSvc:  availabilityzones.NewService(clusterScope),
		networkInterfacesSvc:  networkinterfaces.NewService(clusterScope),
		virtualMachinesSvc:    virtualmachines.NewService(clusterScope),
		virtualMachinesExtSvc: virtualmachineextensions.NewService(clusterScope),
		disksSvc:              disks.NewService(clusterScope),
	}
}

// Create creates machine if and only if machine exists, handled by cluster-api
func (r *azureMachineReconciler) Create() (*compute.VirtualMachine, error) {
	nicName := fmt.Sprintf("%s-nic", r.machineScope.Name())
	nicErr := r.createNetworkInterface(nicName)
	if nicErr != nil {
		return nil, errors.Wrapf(nicErr, "failed to create nic %s for machine %s", nicName, r.machineScope.Name())
	}

	vm, vmErr := r.createVirtualMachine(nicName)
	if vmErr != nil {
		return nil, errors.Wrapf(vmErr, "failed to create vm %s ", r.machineScope.Name())
	}

	vmExtSpec := &virtualmachineextensions.Spec{
		Name:       "startupScript",
		VMName:     r.machineScope.Name(),
		ScriptData: *r.machineScope.Machine.Spec.Bootstrap.Data,
	}
	// TODO: handle failures/retries better
	err := r.virtualMachinesExtSvc.Reconcile(r.clusterScope.Context, vmExtSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vm extension")
	}

	return vm, nil
}

// Update updates machine if and only if machine exists, handled by cluster-api
func (r *azureMachineReconciler) Update() error {
	vmSpec := &virtualmachines.Spec{
		Name: r.machineScope.Name(),
	}
	vmInterface, err := r.virtualMachinesSvc.Get(r.clusterScope.Context, vmSpec)
	if err != nil {
		return errors.Wrap(err, "failed to get vm")
	}

	vm, ok := vmInterface.(compute.VirtualMachine)
	if !ok {
		return errors.New("returned incorrect vm interface")
	}

	// We can now compare the various Azure state to the state we were passed.
	// We will check immutable state first, in order to fail quickly before
	// moving on to state that we can mutate.
	if isMachineOutdated(&r.machineScope.AzureMachine.Spec, *converters.SDKToVM(vm)) {
		return errors.New("found attempt to change immutable state")
	}

	// TODO: Uncomment after implementing tagging.
	// Ensure that the tags are correct.
	/*
		_, err = a.ensureTags(computeSvc, machine, scope.MachineStatus.VMID, scope.MachineConfig.AdditionalTags)
		if err != nil {
			return errors.Wrap(err, "failed to ensure tags")
		}
	*/

	return nil
}

// findVM returns a VM if it exists and is running
func (r *azureMachineReconciler) findVM() (*compute.VirtualMachine, error) {
	vm, err := r.VMIfExists()
	if err != nil || vm == nil {
		return vm, err
	}

	if r.machineScope != nil && r.machineScope.AzureMachine != nil && r.machineScope.AzureMachine.Status != nil {
		return nil, nil
	}

	switch *r.machineScope.AzureMachine.Status.VMState {
	case infrav1.VMStateSucceeded:
		klog.Infof("Machine %v is running", r.machineScope.GetVMID())
	case infrav1.VMStateUpdating:
		klog.Infof("Machine %v is updating", r.machineScope.GetVMID())
	case infrav1.VMStateFailed:
		klog.Infof("Machine %v is in failed state", r.machineScope.GetVMID())
		return nil, nil
	case infrav1.VMStateDeleting:
		klog.Infof("Machine %v is deleting", r.machineScope.GetVMID())
		return nil, nil
	default:
		return vm, nil
	}

	return vm, nil
}

// Delete reconciles all the services in pre determined order
func (r *azureMachineReconciler) Delete() error {
	vmSpec := &virtualmachines.Spec{
		Name: r.machineScope.Name(),
	}

	err := r.virtualMachinesSvc.Delete(r.clusterScope.Context, vmSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to delete machine")
	}

	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:     fmt.Sprintf("%s-nic", r.machineScope.Name()),
		VnetName: azure.GenerateVnetName(r.clusterScope.Name()),
	}

	err = r.networkInterfacesSvc.Delete(r.clusterScope.Context, networkInterfaceSpec)
	if err != nil {
		return errors.Wrapf(err, "Unable to delete network interface")
	}

	OSDiskSpec := &disks.Spec{
		Name: azure.GenerateOSDiskName(r.machineScope.Name()),
	}
	err = r.disksSvc.Delete(r.clusterScope.Context, OSDiskSpec)
	if err != nil {
		return errors.Wrapf(err, "Failed to delete OS disk of machine %s", r.machineScope.Name())
	}

	return nil
}

// isMachineOutdated checks that no immutable fields have been updated in an
// Update request.
// Returns a bool indicating if an attempt to change immutable state occurred.
//  - true:  An attempt to change immutable state occurred.
//  - false: Immutable state was untouched.
func isMachineOutdated(machineSpec *infrav1.AzureMachineSpec, vm infrav1.VM) bool {
	// VM Size
	if !strings.EqualFold(machineSpec.VMSize, vm.VMSize) {
		return true
	}

	// TODO: Add additional checks for immutable fields

	// No immutable state changes found.
	return false
}

func (r *azureMachineReconciler) VMIfExists() (*compute.VirtualMachine, error) {
	vmSpec := &virtualmachines.Spec{
		Name: r.machineScope.Name(),
	}
	vmInterface, err := r.virtualMachinesSvc.Get(r.clusterScope.Context, vmSpec)

	if err != nil && vmInterface == nil {
		return nil, nil
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get vm")
	}

	vm, ok := vmInterface.(compute.VirtualMachine)
	if !ok {
		return nil, errors.New("returned incorrect vm interface")
	}

	klog.Infof("Found vm for machine %s", r.machineScope.Name())

	vmExtSpec := &virtualmachineextensions.Spec{
		Name:   "startupScript",
		VMName: r.machineScope.Name(),
	}

	vmExt, err := r.virtualMachinesExtSvc.Get(r.clusterScope.Context, vmExtSpec)
	if err != nil && vmExt == nil {
		return nil, nil
	}

	if err != nil {
		return nil, errors.Wrapf(err, "failed to get vm extension")
	}

	return &vm, nil
}

// getVirtualMachineZone gets a random availability zones from available set,
// this will hopefully be an input from upstream machinesets so all the vms are balanced
func (r *azureMachineReconciler) getVirtualMachineZone() (string, error) {
	zonesSpec := &availabilityzones.Spec{
		VMSize: r.machineScope.AzureMachine.Spec.VMSize,
	}
	zonesInterface, err := r.availabilityZonesSvc.Get(r.clusterScope.Context, zonesSpec)
	if err != nil {
		return "", errors.Wrapf(err, "failed to check availability zones for %s in region %s", r.machineScope.AzureMachine.Spec.VMSize, r.clusterScope.AzureCluster.Spec.Location)
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

	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator
	return zones[rand.Intn(len(zones))], nil
}

func (r *azureMachineReconciler) createNetworkInterface(nicName string) error {
	// TODO: do we need this?
	// machineList := &clusterv1.MachineList{}
	// labels := map[string]string{clusterv1.MachineClusterLabelName: r.clusterScope.Name()}
	// if err := r.List(context.TODO(), machineList, client.InNamespace(r.clusterScope.Namespace()), client.MatchingLabels(labels)); err != nil {
	// 	return errors.Wrap(err, "failed to list Machines")
	// }

	// controlPlaneMachines := GetControlPlaneMachines(machineList)

	// var natRule int
	// if len(controlPlaneMachines) == 0 {
	// 	natRule = 0
	// } else {
	// 	natRule = len(controlPlaneMachines) - 1
	// }

	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:     nicName,
		VnetName: azure.GenerateVnetName(r.clusterScope.Name()),
		//NatRule:  natRule,
	}
	switch role := r.machineScope.Role(); role {
	case infrav1.Node:
		networkInterfaceSpec.SubnetName = azure.GenerateNodeSubnetName(r.clusterScope.Name())
	case infrav1.ControlPlane:
		networkInterfaceSpec.SubnetName = azure.GenerateControlPlaneSubnetName(r.clusterScope.Name())
		networkInterfaceSpec.PublicLoadBalancerName = azure.GeneratePublicLBName(r.clusterScope.Name())
		networkInterfaceSpec.InternalLoadBalancerName = azure.GenerateInternalLBName(r.clusterScope.Name())
	default:
		return errors.Errorf("unknown value %s for label `set` on machine %s, skipping machine creation", role, r.machineScope.Name())
	}

	err := r.networkInterfacesSvc.Reconcile(r.clusterScope.Context, networkInterfaceSpec)
	if err != nil {
		return errors.Wrap(err, "unable to create VM network interface")
	}

	return err
}

func (r *azureMachineReconciler) createVirtualMachine(nicName string) (*compute.VirtualMachine, error) {
	var vm compute.VirtualMachine
	decoded, err := base64.StdEncoding.DecodeString(r.machineScope.AzureMachine.Spec.SSHPublicKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode ssh public key")
	}

	vmSpec := &virtualmachines.Spec{
		Name: r.machineScope.Name(),
	}

	vmInterface, err := r.virtualMachinesSvc.Get(r.clusterScope.Context, vmSpec)
	if err != nil && vmInterface == nil {
		var vmZone string
		var zoneErr error

		vmZone = r.machineScope.AzureMachine.Spec.AvailabilityZone

		if vmZone == "" {
			vmZone, zoneErr = r.getVirtualMachineZone()
			if zoneErr != nil {
				return nil, errors.Wrap(zoneErr, "failed to get availability zone")
			}
			klog.Info("No availability zone set, selecting random availability zone:", vmZone)
		}

		vmSpec = &virtualmachines.Spec{
			Name:       r.machineScope.Name(),
			NICName:    nicName,
			SSHKeyData: string(decoded),
			Size:       r.machineScope.AzureMachine.Spec.VMSize,
			OSDisk:     r.machineScope.AzureMachine.Spec.OSDisk,
			Image:      r.machineScope.AzureMachine.Spec.Image,
			// Zone:       vmZone,
		}

		err = r.virtualMachinesSvc.Reconcile(r.clusterScope.Context, vmSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create or get machine")
		}
		// r.scope.Machine.Annotations["availability-zone"] = vmZone
	} else if err != nil {
		return nil, errors.Wrap(err, "failed to get vm")
	} else {
		vm, ok := vmInterface.(compute.VirtualMachine)
		if !ok {
			return nil, errors.New("returned incorrect vm interface")
		}
		if vm.ProvisioningState == nil {
			return nil, errors.Errorf("vm %s is nil provisioning state, reconcile", r.machineScope.Name())
		}

		if *vm.ProvisioningState == "Failed" {
			// If VM failed provisioning, delete it so it can be recreated
			err = r.virtualMachinesSvc.Delete(r.clusterScope.Context, vmSpec)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to delete machine")
			}
			return nil, errors.Errorf("vm %s is deleted, retry creating in next reconcile", r.machineScope.Name())
		} else if *vm.ProvisioningState != "Succeeded" {
			return nil, errors.Errorf("vm %s is still in provisioningstate %s, reconcile", r.machineScope.Name(), *vm.ProvisioningState)
		}
	}

	return &vm, err
}

// GetControlPlaneMachines retrieves all non-deleted control plane nodes from a MachineList
func GetControlPlaneMachines(machineList *clusterv1.MachineList) []*clusterv1.Machine {
	var cpm []*clusterv1.Machine
	for _, m := range machineList.Items {
		if util.IsControlPlaneMachine(&m) {
			cpm = append(cpm, m.DeepCopy())
		}
	}
	return cpm
}

func (r *azureMachineReconciler) GetOrCreate() (*compute.VirtualMachine, error) {
	vm, err := r.findVM()
	if err != nil {
		return nil, err
	}

	if vm == nil {
		// Create a new AzureMachine VM if we couldn't find a running VM.
		vm, err = r.Create()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create AzureMachine VM")
		}
	}

	return vm, nil
}
