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

package machine

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/availabilityzones"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/config"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/disks"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachineextensions"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachines"
)

const (
	// DefaultBootstrapTokenTTL default ttl for bootstrap token
	DefaultBootstrapTokenTTL = 10 * time.Minute
)

// Reconciler are list of services required by cluster actuator, easy to create a fake
type Reconciler struct {
	scope                 *actuators.MachineScope
	availabilityZonesSvc  azure.GetterService
	networkInterfacesSvc  azure.Service
	virtualMachinesSvc    azure.GetterService
	virtualMachinesExtSvc azure.GetterService
	disksSvc              azure.GetterService
}

// NewReconciler populates all the services based on input scope
func NewReconciler(scope *actuators.MachineScope) *Reconciler {
	return &Reconciler{
		scope:                 scope,
		availabilityZonesSvc:  availabilityzones.NewService(scope.Scope),
		networkInterfacesSvc:  networkinterfaces.NewService(scope.Scope),
		virtualMachinesSvc:    virtualmachines.NewService(scope.Scope),
		virtualMachinesExtSvc: virtualmachineextensions.NewService(scope.Scope),
		disksSvc:              disks.NewService(scope.Scope),
	}
}

// Create creates machine if and only if machine exists, handled by cluster-api
func (r *Reconciler) Create(ctx context.Context, bootstrapToken string) error {
	// TODO: update once machine controllers have a way to indicate a machine has been provisoned. https://github.com/kubernetes-sigs/cluster-api/issues/253
	// Seeing a node cannot be purely relied upon because the provisioned control plane will not be registering with
	// the stack that provisions it.
	if r.scope.Machine.Annotations == nil {
		r.scope.Machine.Annotations = map[string]string{}
	}

	nicName := fmt.Sprintf("%s-nic", r.scope.Machine.Name)
	nicErr := r.createNetworkInterface(ctx, nicName)
	if nicErr != nil {
		return errors.Wrapf(nicErr, "failed to create nic %s for machine %s", nicName, r.scope.Machine.Name)
	}

	vmErr := r.createVirtualMachine(ctx, nicName)
	if vmErr != nil {
		return errors.Wrapf(vmErr, "failed to create vm %s ", r.scope.Machine.Name)
	}

	scriptData, err := config.GetVMStartupScript(r.scope, bootstrapToken)
	if err != nil {
		return errors.Wrapf(err, "failed to get vm startup script")
	}

	vmExtSpec := &virtualmachineextensions.Spec{
		Name:       "startupScript",
		VMName:     r.scope.Machine.Name,
		ScriptData: base64.StdEncoding.EncodeToString([]byte(scriptData)),
	}
	err = r.virtualMachinesExtSvc.Reconcile(ctx, vmExtSpec)
	if err != nil {
		return errors.Wrap(err, "failed to create vm extension")
	}

	// TODO: Store VM ID and VM State
	//r.scope.MachineStatus.VMID = &vm.ID
	//r.scope.MachineStatus.VMState = &vm.State

	r.scope.Machine.Annotations["cluster-api-provider-azure"] = "true"

	return nil
}

// Update updates machine if and only if machine exists, handled by cluster-api
func (r *Reconciler) Update(ctx context.Context) error {
	vmSpec := &virtualmachines.Spec{
		Name: r.scope.Machine.Name,
	}
	vmInterface, err := r.virtualMachinesSvc.Get(ctx, vmSpec)
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
	if isMachineOutdated(r.scope.MachineConfig, converters.SDKToVM(vm)) {
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

// Exists checks if machine exists
func (r *Reconciler) Exists(ctx context.Context) (bool, error) {
	exists, err := r.isVMExists(ctx)
	if err != nil {
		return false, err
	} else if !exists {
		return false, nil
	}

	switch *r.scope.MachineStatus.VMState {
	case v1alpha1.VMStateSucceeded:
		klog.Infof("Machine %v is running", *r.scope.MachineStatus.VMID)
	case v1alpha1.VMStateUpdating:
		klog.Infof("Machine %v is updating", *r.scope.MachineStatus.VMID)
	default:
		return false, nil
	}

	return true, nil
}

// Delete reconciles all the services in pre determined order
func (r *Reconciler) Delete(ctx context.Context) error {
	vmSpec := &virtualmachines.Spec{
		Name: r.scope.Machine.Name,
	}

	err := r.virtualMachinesSvc.Delete(ctx, vmSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to delete machine")
	}

	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:     fmt.Sprintf("%s-nic", r.scope.Machine.Name),
		VnetName: azure.GenerateVnetName(r.scope.Cluster.Name),
	}

	err = r.networkInterfacesSvc.Delete(ctx, networkInterfaceSpec)
	if err != nil {
		return errors.Wrapf(err, "Unable to delete network interface")
	}

	OSDiskSpec := &disks.Spec{
		Name: azure.GenerateOSDiskName(r.scope.Machine.Name),
	}
	err = r.disksSvc.Delete(ctx, OSDiskSpec)
	if err != nil {
		return errors.Wrapf(err, "Failed to delete OS disk of machine %s", r.scope.Machine.Name)
	}

	return nil
}

// isMachineOutdated checks that no immutable fields have been updated in an
// Update request.
// Returns a bool indicating if an attempt to change immutable state occurred.
//  - true:  An attempt to change immutable state occurred.
//  - false: Immutable state was untouched.
func isMachineOutdated(machineSpec *v1alpha1.AzureMachineProviderSpec, vm *v1alpha1.VM) bool {
	// VM Size
	if !strings.EqualFold(machineSpec.VMSize, vm.VMSize) {
		return true
	}

	// TODO: Add additional checks for immutable fields

	// No immutable state changes found.
	return false
}

func (r *Reconciler) isVMExists(ctx context.Context) (bool, error) {
	vmSpec := &virtualmachines.Spec{
		Name: r.scope.Name(),
	}
	vmInterface, err := r.virtualMachinesSvc.Get(ctx, vmSpec)

	if err != nil && vmInterface == nil {
		return false, nil
	}

	if err != nil {
		return false, errors.Wrap(err, "Failed to get vm")
	}

	vm, ok := vmInterface.(compute.VirtualMachine)
	if !ok {
		return false, errors.New("returned incorrect vm interface")
	}

	klog.Infof("Found vm for machine %s", r.scope.Name())

	vmExtSpec := &virtualmachineextensions.Spec{
		Name:   "startupScript",
		VMName: r.scope.Name(),
	}

	vmExt, err := r.virtualMachinesExtSvc.Get(ctx, vmExtSpec)
	if err != nil && vmExt == nil {
		return false, nil
	}

	if err != nil {
		return false, errors.Wrapf(err, "failed to get vm extension")
	}

	vmState := v1alpha1.VMState(*vm.ProvisioningState)

	r.scope.MachineStatus.VMID = vm.ID
	r.scope.MachineStatus.VMState = &vmState

	if r.scope.Machine.Spec.ProviderID == nil || *r.scope.Machine.Spec.ProviderID == "" {
		providerID := fmt.Sprintf("azure:////%s", *r.scope.MachineStatus.VMID)
		r.scope.Machine.Spec.ProviderID = &providerID
	}

	return true, nil
}

// getVirtualMachineZone gets a random availability zones from available set,
// this will hopefully be an input from upstream machinesets so all the vms are balanced
func (r *Reconciler) getVirtualMachineZone(ctx context.Context) (string, error) {
	zonesSpec := &availabilityzones.Spec{
		VMSize: r.scope.MachineConfig.VMSize,
	}
	zonesInterface, err := r.availabilityZonesSvc.Get(ctx, zonesSpec)
	if err != nil {
		return "", errors.Wrapf(err, "failed to check availability zones for %s in region %s", r.scope.MachineConfig.VMSize, r.scope.ClusterConfig.Location)
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

func (r *Reconciler) createNetworkInterface(ctx context.Context, nicName string) error {
	machineList, err := r.scope.MachineClient.List(actuators.ListOptionsForCluster(r.scope.Cluster.Name))
	if err != nil {
		return err
	}

	controlPlaneMachines := GetControlPlaneMachines(machineList)

	var natRule int
	if len(controlPlaneMachines) == 0 {
		natRule = 0
	} else {
		natRule = len(controlPlaneMachines) - 1
	}

	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:     nicName,
		VnetName: azure.GenerateVnetName(r.scope.Cluster.Name),
		NatRule:  natRule,
	}
	switch set := r.scope.Machine.ObjectMeta.Labels["set"]; set {
	case v1alpha1.Node:
		networkInterfaceSpec.SubnetName = azure.GenerateNodeSubnetName(r.scope.Cluster.Name)
	case v1alpha1.ControlPlane:
		networkInterfaceSpec.SubnetName = azure.GenerateControlPlaneSubnetName(r.scope.Cluster.Name)
		networkInterfaceSpec.PublicLoadBalancerName = azure.GeneratePublicLBName(r.scope.Cluster.Name)
		networkInterfaceSpec.InternalLoadBalancerName = azure.GenerateInternalLBName(r.scope.Cluster.Name)
	default:
		return errors.Errorf("unknown value %s for label `set` on machine %s, skipping machine creation", set, r.scope.Machine.Name)
	}

	err = r.networkInterfacesSvc.Reconcile(ctx, networkInterfaceSpec)
	if err != nil {
		return errors.Wrap(err, "unable to create VM network interface")
	}

	return err
}

func (r *Reconciler) createVirtualMachine(ctx context.Context, nicName string) error {
	decoded, err := base64.StdEncoding.DecodeString(r.scope.MachineConfig.SSHPublicKey)
	if err != nil {
		return errors.Wrapf(err, "failed to decode ssh public key")
	}

	vmSpec := &virtualmachines.Spec{
		Name: r.scope.Machine.Name,
	}

	vmInterface, err := r.virtualMachinesSvc.Get(ctx, vmSpec)
	if err != nil && vmInterface == nil {
		var vmZone string
		var zoneErr error

		vmZone = r.scope.MachineConfig.AvailabilityZone

		if r.scope.MachineConfig.AvailabilityZone == "" {
			vmZone, zoneErr = r.getVirtualMachineZone(ctx)
			if zoneErr != nil {
				return errors.Wrap(zoneErr, "failed to get availability zone")
			}
			klog.Info("No availability zone set, selecting random availability zone:", vmZone)
		}

		vmSpec = &virtualmachines.Spec{
			Name:       r.scope.Machine.Name,
			NICName:    nicName,
			SSHKeyData: string(decoded),
			Size:       r.scope.MachineConfig.VMSize,
			OSDisk:     r.scope.MachineConfig.OSDisk,
			Image:      r.scope.MachineConfig.Image,
			Zone:       vmZone,
		}

		err = r.virtualMachinesSvc.Reconcile(ctx, vmSpec)
		if err != nil {
			return errors.Wrapf(err, "failed to create or get machine")
		}
		r.scope.Machine.Annotations["availability-zone"] = vmZone
	} else if err != nil {
		return errors.Wrap(err, "failed to get vm")
	} else {
		vm, ok := vmInterface.(compute.VirtualMachine)
		if !ok {
			return errors.New("returned incorrect vm interface")
		}
		if vm.ProvisioningState == nil {
			return errors.Errorf("vm %s is nil provisioning state, reconcile", r.scope.Machine.Name)
		}

		if *vm.ProvisioningState == "Failed" {
			// If VM failed provisioning, delete it so it can be recreated
			err = r.virtualMachinesSvc.Delete(ctx, vmSpec)
			if err != nil {
				return errors.Wrapf(err, "failed to delete machine")
			}
			return errors.Errorf("vm %s is deleted, retry creating in next reconcile", r.scope.Machine.Name)
		} else if *vm.ProvisioningState != "Succeeded" {
			return errors.Errorf("vm %s is still in provisioningstate %s, reconcile", r.scope.Machine.Name, *vm.ProvisioningState)
		}
	}

	return err
}
