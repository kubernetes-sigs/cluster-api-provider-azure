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

package converters

import (
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest/to"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
)

// SDKToVMSS converts an Azure SDK VirtualMachineScaleSet to the AzureMachinePool type.
func SDKToVMSS(sdkvmss compute.VirtualMachineScaleSet, sdkinstances []compute.VirtualMachineScaleSetVM) *infrav1exp.VMSS {
	vmss := &infrav1exp.VMSS{
		ID:    to.String(sdkvmss.ID),
		Name:  to.String(sdkvmss.Name),
		State: infrav1.VMState(to.String(sdkvmss.ProvisioningState)),
	}

	if sdkvmss.Sku != nil {
		vmss.Sku = to.String(sdkvmss.Sku.Name)
		vmss.Capacity = to.Int64(sdkvmss.Sku.Capacity)
	}

	if sdkvmss.Zones != nil && len(*sdkvmss.Zones) > 0 {
		vmss.Zones = to.StringSlice(sdkvmss.Zones)
	}

	if len(sdkvmss.Tags) > 0 {
		vmss.Tags = MapToTags(sdkvmss.Tags)
	}

	if len(sdkinstances) > 0 {
		vmss.Instances = make([]infrav1exp.VMSSVM, len(sdkinstances))
		for i, vm := range sdkinstances {
			vmss.Instances[i] = *SDKToVMSSVM(vm)
		}
	}

	if sdkvmss.VirtualMachineProfile != nil &&
		sdkvmss.VirtualMachineProfile.StorageProfile != nil &&
		sdkvmss.VirtualMachineProfile.StorageProfile.ImageReference != nil {

		imageRef := sdkvmss.VirtualMachineProfile.StorageProfile.ImageReference
		vmss.Image = infrav1.Image{
			ID: imageRef.ID,
			Marketplace: &infrav1.AzureMarketplaceImage{
				Publisher:       to.String(imageRef.Publisher),
				Offer:           to.String(imageRef.Offer),
				SKU:             to.String(imageRef.Sku),
				Version:         to.String(imageRef.Version),
				ThirdPartyImage: false,
			},
		}
	}

	return vmss
}

// SDKToVMSSVM converts an Azure SDK VirtualMachineScaleSetVM into an infrav1exp.VMSSVM
func SDKToVMSSVM(sdkInstance compute.VirtualMachineScaleSetVM) *infrav1exp.VMSSVM {
	instance := infrav1exp.VMSSVM{
		ID:                 to.String(sdkInstance.ID),
		InstanceID:         to.String(sdkInstance.InstanceID),
		LatestModelApplied: true,
	}

	if sdkInstance.VirtualMachineScaleSetVMProperties == nil {
		return &instance
	}

	instance.State = infrav1.VMStateCreating
	if sdkInstance.ProvisioningState != nil {
		instance.State = infrav1.VMState(to.String(sdkInstance.ProvisioningState))
	}

	if sdkInstance.OsProfile != nil && sdkInstance.OsProfile.ComputerName != nil {
		instance.Name = *sdkInstance.OsProfile.ComputerName
	}

	if sdkInstance.LatestModelApplied != nil {
		instance.LatestModelApplied = *sdkInstance.LatestModelApplied
	}

	if sdkInstance.Zones != nil && len(*sdkInstance.Zones) > 0 {
		instance.AvailabilityZone = to.StringSlice(sdkInstance.Zones)[0]
	}

	return &instance
}
