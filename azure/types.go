/*
Copyright 2020 The Kubernetes Authors.

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

package azure

import (
	"reflect"

	"github.com/google/go-cmp/cmp"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// RoleAssignmentSpec defines the specification for a Role Assignment.
type RoleAssignmentSpec struct {
	MachineName  string
	Name         string
	ResourceType string
}

// ResourceType defines the type azure resource being reconciled.
// Eg. Virtual Machine, Virtual Machine Scale Sets.
type ResourceType string

const (

	// VirtualMachine ...
	VirtualMachine = "VirtualMachine"

	// VirtualMachineScaleSet ...
	VirtualMachineScaleSet = "VirtualMachineScaleSet"
)

// ScaleSetSpec defines the specification for a Scale Set.
type ScaleSetSpec struct {
	Name                         string
	Size                         string
	Capacity                     int64
	SSHKeyData                   string
	OSDisk                       infrav1.OSDisk
	DataDisks                    []infrav1.DataDisk
	SubnetName                   string
	VNetName                     string
	VNetResourceGroup            string
	PublicLBName                 string
	PublicLBAddressPoolName      string
	AcceleratedNetworking        *bool
	TerminateNotificationTimeout *int
	Identity                     infrav1.VMIdentity
	UserAssignedIdentities       []infrav1.UserAssignedIdentity
	SecurityProfile              *infrav1.SecurityProfile
	SpotVMOptions                *infrav1.SpotVMOptions
	AdditionalCapabilities       *infrav1.AdditionalCapabilities
	FailureDomains               []string
	VMExtensions                 []infrav1.VMExtension
}

// ExtensionSpec defines the specification for a VM or VMSS extension.
type ExtensionSpec struct {
	Name              string
	VMName            string
	Publisher         string
	Version           string
	Settings          map[string]string
	ProtectedSettings map[string]string
}

type (
	// VMSSVM defines a VM in a virtual machine scale set.
	VMSSVM struct {
		ID               string                    `json:"id,omitempty"`
		InstanceID       string                    `json:"instanceID,omitempty"`
		Image            infrav1.Image             `json:"image,omitempty"`
		Name             string                    `json:"name,omitempty"`
		AvailabilityZone string                    `json:"availabilityZone,omitempty"`
		State            infrav1.ProvisioningState `json:"vmState,omitempty"`
	}

	// VMSS defines a virtual machine scale set.
	VMSS struct {
		ID        string                    `json:"id,omitempty"`
		Name      string                    `json:"name,omitempty"`
		Sku       string                    `json:"sku,omitempty"`
		Capacity  int64                     `json:"capacity,omitempty"`
		Zones     []string                  `json:"zones,omitempty"`
		Image     infrav1.Image             `json:"image,omitempty"`
		State     infrav1.ProvisioningState `json:"vmState,omitempty"`
		Identity  infrav1.VMIdentity        `json:"identity,omitempty"`
		Tags      infrav1.Tags              `json:"tags,omitempty"`
		Instances []VMSSVM                  `json:"instances,omitempty"`
	}
)

// HasModelChanges returns true if the spec fields which will mutate the Azure VMSS model are different.
func (vmss VMSS) HasModelChanges(other VMSS) bool {
	equal := cmp.Equal(vmss.Image, other.Image) &&
		cmp.Equal(vmss.Identity, other.Identity) &&
		cmp.Equal(vmss.Zones, other.Zones) &&
		cmp.Equal(vmss.Tags, other.Tags) &&
		cmp.Equal(vmss.Sku, other.Sku)
	return !equal
}

// InstancesByProviderID returns VMSSVMs by ID.
func (vmss VMSS) InstancesByProviderID() map[string]VMSSVM {
	instancesByProviderID := make(map[string]VMSSVM, len(vmss.Instances))
	for _, instance := range vmss.Instances {
		instancesByProviderID[instance.ProviderID()] = instance
	}

	return instancesByProviderID
}

// ProviderID returns the K8s provider ID for the VMSS instance.
func (vm VMSSVM) ProviderID() string {
	return ProviderIDPrefix + vm.ID
}

// HasLatestModelAppliedToAll returns true if all VMSS instance have the latest model applied.
func (vmss VMSS) HasLatestModelAppliedToAll() bool {
	for _, instance := range vmss.Instances {
		if !vmss.HasLatestModelApplied(instance) {
			return false
		}
	}

	return true
}

// HasEnoughLatestModelOrNotMixedModel returns true if VMSS instance have the latest model applied to all or equal to the capacity.
func (vmss VMSS) HasEnoughLatestModelOrNotMixedModel() bool {
	if vmss.HasLatestModelAppliedToAll() {
		return true
	}

	counter := int64(0)
	for _, instance := range vmss.Instances {
		if vmss.HasLatestModelApplied(instance) {
			counter++
		}
	}

	return counter == vmss.Capacity
}

// HasLatestModelApplied returns true if the VMSS instance matches the VMSS image reference.
func (vmss VMSS) HasLatestModelApplied(vm VMSSVM) bool {
	// if the images match, then the VM is of the same model
	return reflect.DeepEqual(vm.Image, vmss.Image)
}
