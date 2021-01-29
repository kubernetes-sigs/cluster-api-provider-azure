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

package v1alpha3

import (
	"fmt"

	"github.com/google/go-cmp/cmp"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

type (
	// VMSSVM defines a VM in a virtual machine scale set.
	VMSSVM struct {
		ID                 string          `json:"id,omitempty"`
		InstanceID         string          `json:"instanceID,omitempty"`
		Name               string          `json:"name,omitempty"`
		AvailabilityZone   string          `json:"availabilityZone,omitempty"`
		State              infrav1.VMState `json:"vmState,omitempty"`
		LatestModelApplied bool            `json:"latestModelApplied,omitempty"`
	}

	// VMSS defines a virtual machine scale set.
	VMSS struct {
		ID        string             `json:"id,omitempty"`
		Name      string             `json:"name,omitempty"`
		Sku       string             `json:"sku,omitempty"`
		Capacity  int64              `json:"capacity,omitempty"`
		Zones     []string           `json:"zones,omitempty"`
		Image     infrav1.Image      `json:"image,omitempty"`
		State     infrav1.VMState    `json:"vmState,omitempty"`
		Identity  infrav1.VMIdentity `json:"identity,omitempty"`
		Tags      infrav1.Tags       `json:"tags,omitempty"`
		Instances []VMSSVM           `json:"instances,omitempty"`
	}
)

// HasModelChanges returns true if the spec fields which will mutate the Azure VMSS model are different.
func (vmss VMSS) HasModelChanges(other VMSS) bool {
	equal := cmp.Equal(vmss.Image, other.Image) &&
		cmp.Equal(vmss.Identity, other.Identity) &&
		cmp.Equal(vmss.Sku, other.Sku) &&
		cmp.Equal(vmss.Zones, other.Zones)
	return !equal
}

// ReadyAndNotRunningLatestModel returns VMSSVMs that are ready and not running the latest model
func (vmss VMSS) ReadyAndNotRunningLatestModel() []VMSSVM {
	var instances []VMSSVM
	for _, instance := range vmss.Instances {
		if !instance.LatestModelApplied && instance.State == infrav1.VMStateSucceeded {
			instances = append(instances, instance)
		}
	}

	return instances
}

// ReadyAndRunningLatestModel returns VMSSVMs running the latest model and are ready
func (vmss VMSS) ReadyAndRunningLatestModel() []VMSSVM {
	var instances []VMSSVM
	for _, instance := range vmss.Instances {
		if instance.LatestModelApplied && instance.State == infrav1.VMStateSucceeded {
			instances = append(instances, instance)
		}
	}

	return instances
}

// ReadyInstances returns VMSSVMs that are ready
func (vmss VMSS) ReadyInstances() []VMSSVM {
	var instances []VMSSVM
	for _, instance := range vmss.Instances {
		if instance.State == infrav1.VMStateSucceeded {
			instances = append(instances, instance)
		}
	}

	return instances
}

// InstancesByProviderID returns VMSSVMs by ID
func (vmss VMSS) InstancesByProviderID() map[string]VMSSVM {
	instancesByProviderID := make(map[string]VMSSVM, len(vmss.Instances))
	for _, instance := range vmss.Instances {
		instancesByProviderID[instance.ProviderID()] = instance
	}

	return instancesByProviderID
}

// ProviderID returns the K8s provider ID for the VMSS instance
func (vm VMSSVM) ProviderID() string {
	return fmt.Sprintf("azure://%s", vm.ID)
}
