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

package resourceskus

import (
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/pkg/errors"
)

// SKU is a thin layer over the Azure resource SKU API to better introspect capabilities
type SKU compute.ResourceSku

// ResourceType models available resource types as a set of known string constants.
type ResourceType string

const (
	// VirtualMachines is a convenience constant to filter resource SKUs to only include VMs.
	VirtualMachines ResourceType = "virtualMachines"
	// Disks is a convenience constant to filter resource SKUs to only include disks.
	Disks ResourceType = "disks"
)

// Supported models an enum of possible boolean values for resource support in the Azure API.
type Supported string

const (
	// CapabilitySupported is the value returned by this API from Azure when the capability is supported
	CapabilitySupported Supported = "True"
	// CapabilityUnsupported is the value returned by this API from Azure when the capability is unsupported
	CapabilityUnsupported Supported = "False"
)

const (
	// EphemeralOSDisk identifies the capability for ephemeral os support.
	EphemeralOSDisk = "EphemeralOSDiskSupported"
	// AcceleratedNetworking identifies the capability for accelerated networking support.
	AcceleratedNetworking = "AcceleratedNetworkingEnabled"
	//VCPUs identifies the capability for the number of vCPUS.
	VCPUs = "vCPUs"
	// MemoryGB identifies the capability for memory Size.
	MemoryGB = "MemoryGB"
	// MinimumVCPUS is the minimum vCPUS allowed.
	MinimumVCPUS = 2
	// MinimumMemory is the minimum memory allowed.
	MinimumMemory = 2
)

// HasCapability return true for a capability which can be either
// supported or not. Examples include "EphemeralOSDiskSupported",
// "UltraSSDAvavailable" "EncryptionAtHostSupported",
// "AcceleratedNetworkingEnabled", and "RdmaEnabled"
func (s SKU) HasCapability(name string) bool {
	if s.Capabilities != nil {
		for _, capability := range *s.Capabilities {
			if capability.Name != nil && *capability.Name == name {
				if capability.Value != nil && strings.EqualFold(*capability.Value, string(CapabilitySupported)) {
					return true
				}
			}
		}
	}
	return false
}

// HasCapabilityWithCapacity returns true when the provided resource
// exposes a numeric capability and the maximum value exposed by that
// capability exceeds the value requested by the user. Examples include
// "MaxResourceVolumeMB", "OSVhdSizeMB", "vCPUs",
// "MemoryGB","MaxDataDiskCount", "CombinedTempDiskAndCachedIOPS",
// "CombinedTempDiskAndCachedReadBytesPerSecond",
// "CombinedTempDiskAndCachedWriteBytesPerSecond", "UncachedDiskIOPS",
// and "UncachedDiskBytesPerSecond"
func (s SKU) HasCapabilityWithCapacity(name string, value int64) (bool, error) {
	if s.Capabilities != nil {
		for _, capability := range *s.Capabilities {
			if capability.Name != nil && *capability.Name == name {
				if capability.Value != nil {
					intVal, err := strconv.ParseInt(*capability.Value, 10, 64)
					if err != nil {
						return false, errors.Wrapf(err, "failed to parse string '%s' as int64", *capability.Value)
					}
					if intVal >= value {
						return true, nil
					}
				}
				return false, nil
			}
		}
	}
	return false, nil
}
