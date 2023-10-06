/*
Copyright 2023 The Kubernetes Authors.

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

package scalesets

import (
	"context"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
)

var (
	defaultSpec, defaultVMSS                                                           = getDefaultVMSS()
	windowsSpec, windowsVMSS                                                           = getDefaultWindowsVMSS()
	acceleratedNetworkingSpec, acceleratedNetworkingVMSS                               = getAcceleratedNetworkingVMSS()
	customSubnetSpec, customSubnetVMSS                                                 = getCustomSubnetVMSS()
	customNetworkingSpec, customNetworkingVMSS                                         = getCustomNetworkingVMSS()
	spotVMSpec, spotVMVMSS                                                             = getSpotVMVMSS()
	ephemeralSpec, ephemeralVMSS                                                       = getEPHVMSSS()
	evictionSpec, evictionVMSS                                                         = getEvictionPolicyVMSS()
	maxPriceSpec, maxPriceVMSS                                                         = getMaxPriceVMSS()
	encryptionSpec, encryptionVMSS                                                     = getEncryptionVMSS()
	userIdentitySpec, userIdentityVMSS                                                 = getUserIdentityVMSS()
	hostEncryptionSpec, hostEncryptionVMSS                                             = getHostEncryptionVMSS()
	hostEncryptionUnsupportedSpec                                                      = getHostEncryptionUnsupportedSpec()
	ephemeralReadSpec, ephemeralReadVMSS                                               = getEphemeralReadOnlyVMSS()
	defaultExistingSpec, defaultExistingVMSS, defaultExistingVMSSClone                 = getExistingDefaultVMSS()
	userManagedStorageAccountDiagnosticsSpec, userManagedStorageAccountDiagnosticsVMSS = getUserManagedAndStorageAcccountDiagnosticsVMSS()
	managedDiagnosticsSpec, managedDiagnoisticsVMSS                                    = getManagedDiagnosticsVMSS()
	disabledDiagnosticsSpec, disabledDiagnosticsVMSS                                   = getDisabledDiagnosticsVMSS()
	nilDiagnosticsProfileSpec, nilDiagnosticsProfileVMSS                               = getNilDiagnosticsProfileVMSS()
)

func getDefaultVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})

	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}

	return spec, vmss
}

func getDefaultWindowsVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newWindowsVMSSSpec()
	// Do we want this here?
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	vmss := newDefaultWindowsVMSS()
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}

	return spec, vmss
}

func getAcceleratedNetworkingVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.Size = "VM_SIZE_AN"
	spec.AcceleratedNetworking = ptr.To(true)
	spec.NetworkInterfaces[0].AcceleratedNetworking = ptr.To(true)
	vmss := newDefaultVMSS("VM_SIZE_AN")
	vmss.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations[0].Properties.EnableAcceleratedNetworking = ptr.To(true)

	return spec, vmss
}

func getCustomSubnetVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.Size = "VM_SIZE_AN"
	spec.AcceleratedNetworking = ptr.To(true)
	spec.NetworkInterfaces = []infrav1.NetworkInterface{
		{
			SubnetName:       "somesubnet",
			PrivateIPConfigs: 1, // defaulter sets this to one
		},
	}
	customSubnetVMSS := newDefaultVMSS("VM_SIZE_AN")
	netConfigs := customSubnetVMSS.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations
	netConfigs[0].Name = ptr.To("my-vmss-nic-0")
	netConfigs[0].Properties.EnableIPForwarding = ptr.To(true)
	netConfigs[0].Properties.EnableAcceleratedNetworking = ptr.To(true)
	nic1IPConfigs := netConfigs[0].Properties.IPConfigurations
	nic1IPConfigs[0].Name = ptr.To("ipConfig0")
	nic1IPConfigs[0].Properties.PrivateIPAddressVersion = ptr.To(armcompute.IPVersionIPv4)
	nic1IPConfigs[0].Properties.Subnet = &armcompute.APIEntityReference{
		ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/somesubnet"),
	}
	netConfigs[0].Properties.EnableAcceleratedNetworking = ptr.To(true)
	netConfigs[0].Properties.Primary = ptr.To(true)

	return spec, customSubnetVMSS
}

func getCustomNetworkingVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.NetworkInterfaces = []infrav1.NetworkInterface{
		{
			SubnetName:            "my-subnet",
			PrivateIPConfigs:      1,
			AcceleratedNetworking: ptr.To(true),
		},
		{
			SubnetName:            "subnet2",
			PrivateIPConfigs:      2,
			AcceleratedNetworking: ptr.To(true),
		},
	}
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
	netConfigs := vmss.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations
	netConfigs[0].Name = ptr.To("my-vmss-nic-0")
	netConfigs[0].Properties.EnableIPForwarding = ptr.To(true)
	nic1IPConfigs := netConfigs[0].Properties.IPConfigurations
	nic1IPConfigs[0].Name = ptr.To("ipConfig0")
	nic1IPConfigs[0].Properties.PrivateIPAddressVersion = ptr.To(armcompute.IPVersionIPv4)
	netConfigs[0].Properties.EnableAcceleratedNetworking = ptr.To(true)
	netConfigs[0].Properties.Primary = ptr.To(true)
	vmssIPConfigs := []armcompute.VirtualMachineScaleSetIPConfiguration{
		{
			Name: ptr.To("ipConfig0"),
			Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
				Primary:                 ptr.To(true),
				PrivateIPAddressVersion: ptr.To(armcompute.IPVersionIPv4),
				Subnet: &armcompute.APIEntityReference{
					ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/subnet2"),
				},
			},
		},
		{
			Name: ptr.To("ipConfig1"),
			Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
				PrivateIPAddressVersion: ptr.To(armcompute.IPVersionIPv4),
				Subnet: &armcompute.APIEntityReference{
					ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/subnet2"),
				},
			},
		},
	}
	netConfigs = append(netConfigs, &armcompute.VirtualMachineScaleSetNetworkConfiguration{
		Name: ptr.To("my-vmss-nic-1"),
		Properties: &armcompute.VirtualMachineScaleSetNetworkConfigurationProperties{
			EnableAcceleratedNetworking: ptr.To(true),
			IPConfigurations:            azure.PtrSlice(&vmssIPConfigs),
			EnableIPForwarding:          ptr.To(true),
		},
	})
	vmss.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations = netConfigs

	return spec, vmss
}

func getSpotVMVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	spec.SpotVMOptions = &infrav1.SpotVMOptions{}
	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
	vmss.Properties.VirtualMachineProfile.Priority = ptr.To(armcompute.VirtualMachinePriorityTypesSpot)

	return spec, vmss
}

func getEPHVMSSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.Size = vmSizeEPH
	spec.SKU = resourceskus.SKU{
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.EphemeralOSDisk),
				Value: ptr.To("True"),
			},
		},
	}
	spec.SpotVMOptions = &infrav1.SpotVMOptions{}
	spec.OSDisk.DiffDiskSettings = &infrav1.DiffDiskSettings{
		Option: string(armcompute.DiffDiskOptionsLocal),
	}
	vmss := newDefaultVMSS(vmSizeEPH)
	vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings = &armcompute.DiffDiskSettings{
		Option: ptr.To(armcompute.DiffDiskOptionsLocal),
	}
	vmss.Properties.VirtualMachineProfile.Priority = ptr.To(armcompute.VirtualMachinePriorityTypesSpot)

	return spec, vmss
}

func getEvictionPolicyVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.Size = vmSizeEPH
	deletePolicy := infrav1.SpotEvictionPolicyDelete
	spec.SpotVMOptions = &infrav1.SpotVMOptions{EvictionPolicy: &deletePolicy}
	vmss := newDefaultVMSS(vmSizeEPH)
	vmss.Properties.VirtualMachineProfile.Priority = ptr.To(armcompute.VirtualMachinePriorityTypesSpot)
	vmss.Properties.VirtualMachineProfile.EvictionPolicy = ptr.To(armcompute.VirtualMachineEvictionPolicyTypesDelete)

	return spec, vmss
}

func getMaxPriceVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	maxPrice := resource.MustParse("0.001")
	spec.SpotVMOptions = &infrav1.SpotVMOptions{
		MaxPrice: &maxPrice,
	}
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.VirtualMachineProfile.Priority = ptr.To(armcompute.VirtualMachinePriorityTypesSpot)
	vmss.Properties.VirtualMachineProfile.BillingProfile = &armcompute.BillingProfile{
		MaxPrice: ptr.To[float64](0.001),
	}
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}

	return spec, vmss
}

func getEncryptionVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.OSDisk.ManagedDisk.DiskEncryptionSet = &infrav1.DiskEncryptionSetParameters{
		ID: "my-diskencryptionset-id",
	}
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
	osdisk := vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk
	osdisk.ManagedDisk = &armcompute.VirtualMachineScaleSetManagedDiskParameters{
		StorageAccountType: ptr.To(armcompute.StorageAccountTypesPremiumLRS),
		DiskEncryptionSet: &armcompute.DiskEncryptionSetParameters{
			ID: ptr.To("my-diskencryptionset-id"),
		},
	}

	return spec, vmss
}

func getUserIdentityVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	spec.Identity = infrav1.VMIdentityUserAssigned
	spec.UserAssignedIdentities = []infrav1.UserAssignedIdentity{
		{
			ProviderID: "azure:///subscriptions/123/resourcegroups/456/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1",
		},
	}
	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
	vmss.Identity = &armcompute.VirtualMachineScaleSetIdentity{
		Type: ptr.To(armcompute.ResourceIdentityTypeUserAssigned),
		UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
			"/subscriptions/123/resourcegroups/456/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1": {},
		},
	}

	return spec, vmss
}

func getHostEncryptionVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.Size = "VM_SIZE_EAH"
	spec.SecurityProfile = &infrav1.SecurityProfile{EncryptionAtHost: ptr.To(true)}
	spec.SKU = resourceskus.SKU{
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.EncryptionAtHost),
				Value: ptr.To("True"),
			},
		},
	}
	vmss := newDefaultVMSS("VM_SIZE_EAH")
	vmss.Properties.VirtualMachineProfile.SecurityProfile = &armcompute.SecurityProfile{
		EncryptionAtHost: ptr.To(true),
	}
	vmss.SKU.Name = ptr.To(spec.Size)

	return spec, vmss
}

func getHostEncryptionUnsupportedSpec() ScaleSetSpec {
	spec, _ := getHostEncryptionVMSS()
	spec.SKU = resourceskus.SKU{}
	return spec
}

func getEphemeralReadOnlyVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.Size = "VM_SIZE_EPH"
	spec.OSDisk.DiffDiskSettings = &infrav1.DiffDiskSettings{
		Option: "Local",
	}
	spec.OSDisk.CachingType = "ReadOnly"
	spec.SKU = resourceskus.SKU{
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.EphemeralOSDisk),
				Value: ptr.To("True"),
			},
		},
	}

	vmss := newDefaultVMSS("VM_SIZE_EPH")
	vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings = &armcompute.DiffDiskSettings{
		Option: ptr.To(armcompute.DiffDiskOptionsLocal),
	}
	vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.Caching = ptr.To(armcompute.CachingTypesReadOnly)

	return spec, vmss
}

func getExistingDefaultVMSS() (s ScaleSetSpec, existing armcompute.VirtualMachineScaleSet, result armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.Capacity = 2
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	spec.MaxSurge = 1

	spec.VMImage = &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			ImagePlan: infrav1.ImagePlan{
				Publisher: "fake-publisher",
				Offer:     "my-offer",
				SKU:       "sku-id",
			},
			Version: "2.0",
		},
	}

	existingVMSS := newDefaultExistingVMSS("VM_SIZE")
	existingVMSS.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
	existingVMSS.SKU.Capacity = ptr.To[int64](2)
	existingVMSS.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}

	clone := newDefaultExistingVMSS("VM_SIZE")
	clone.SKU.Capacity = ptr.To[int64](3)
	clone.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
	clone.Properties.VirtualMachineProfile.NetworkProfile = nil

	clone.Properties.VirtualMachineProfile.StorageProfile.ImageReference.Version = ptr.To("2.0")
	clone.Properties.VirtualMachineProfile.NetworkProfile = nil

	return spec, existingVMSS, clone
}

func getUserManagedAndStorageAcccountDiagnosticsVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	storageURI := "https://fakeurl"
	spec := newDefaultVMSSSpec()
	spec.DiagnosticsProfile = &infrav1.Diagnostics{
		Boot: &infrav1.BootDiagnostics{
			StorageAccountType: infrav1.UserManagedDiagnosticsStorage,
			UserManaged: &infrav1.UserManagedBootDiagnostics{
				StorageAccountURI: storageURI,
			},
		},
	}
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})

	spec.VMSSInstances = newDefaultInstances()
	spec.MaxSurge = 1

	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.VirtualMachineProfile.DiagnosticsProfile = &armcompute.DiagnosticsProfile{BootDiagnostics: &armcompute.BootDiagnostics{
		Enabled:    ptr.To(true),
		StorageURI: &storageURI,
	}}
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}

	return spec, vmss
}

func getManagedDiagnosticsVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.DiagnosticsProfile = &infrav1.Diagnostics{
		Boot: &infrav1.BootDiagnostics{
			StorageAccountType: infrav1.ManagedDiagnosticsStorage,
		},
	}
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	spec.VMSSInstances = newDefaultInstances()

	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.VirtualMachineProfile.DiagnosticsProfile = &armcompute.DiagnosticsProfile{BootDiagnostics: &armcompute.BootDiagnostics{
		Enabled: ptr.To(true),
	}}
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}

	return spec, vmss
}

func getDisabledDiagnosticsVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.DiagnosticsProfile = &infrav1.Diagnostics{
		Boot: &infrav1.BootDiagnostics{
			StorageAccountType: infrav1.DisabledDiagnosticsStorage,
		},
	}
	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	spec.VMSSInstances = newDefaultInstances()

	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.VirtualMachineProfile.DiagnosticsProfile = &armcompute.DiagnosticsProfile{BootDiagnostics: &armcompute.BootDiagnostics{
		Enabled: ptr.To(false),
	}}
	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}

	return spec, vmss
}

func getNilDiagnosticsProfileVMSS() (ScaleSetSpec, armcompute.VirtualMachineScaleSet) {
	spec := newDefaultVMSSSpec()
	spec.DiagnosticsProfile = nil

	spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
		NameSuffix: "my_disk_with_ultra_disks",
		DiskSizeGB: 128,
		Lun:        ptr.To[int32](3),
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "UltraSSD_LRS",
		},
	})
	spec.VMSSInstances = newDefaultInstances()

	vmss := newDefaultVMSS("VM_SIZE")
	vmss.Properties.VirtualMachineProfile.DiagnosticsProfile = nil

	vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}

	return spec, vmss
}

func TestScaleSetParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          ScaleSetSpec
		existing      interface{}
		expected      interface{}
		expectedError string
	}{
		{
			name:          "get parameters for a vmss",
			spec:          defaultSpec,
			existing:      nil,
			expected:      defaultVMSS,
			expectedError: "",
		},
		{
			name:          "get parameters for a windows vmss",
			spec:          windowsSpec,
			existing:      nil,
			expected:      windowsVMSS,
			expectedError: "",
		},
		{
			name:          "windows vmss up to date",
			spec:          windowsSpec,
			existing:      windowsVMSS,
			expected:      nil,
			expectedError: "",
		},
		{
			name:          "accelerated networking vmss",
			spec:          acceleratedNetworkingSpec,
			existing:      nil,
			expected:      acceleratedNetworkingVMSS,
			expectedError: "",
		},
		{
			name:          "custom subnet vmss",
			spec:          customSubnetSpec,
			existing:      nil,
			expected:      customSubnetVMSS,
			expectedError: "",
		},
		{
			name:          "custom networking vmss",
			spec:          customNetworkingSpec,
			existing:      nil,
			expected:      customNetworkingVMSS,
			expectedError: "",
		},
		{
			name:          "spot vm vmss",
			spec:          spotVMSpec,
			existing:      nil,
			expected:      spotVMVMSS,
			expectedError: "",
		},
		{
			name:          "spot vm and ephemeral disk vmss",
			spec:          ephemeralSpec,
			existing:      nil,
			expected:      ephemeralVMSS,
			expectedError: "",
		},
		{
			name:          "spot vm and eviction policy vmss",
			spec:          evictionSpec,
			existing:      nil,
			expected:      evictionVMSS,
			expectedError: "",
		},
		{
			name:          "spot vm and max price vmss",
			spec:          maxPriceSpec,
			existing:      nil,
			expected:      maxPriceVMSS,
			expectedError: "",
		},
		{
			name:          "eviction policy vmss",
			spec:          evictionSpec,
			existing:      nil,
			expected:      evictionVMSS,
			expectedError: "",
		},
		{
			name:          "encryption vmss",
			spec:          encryptionSpec,
			existing:      nil,
			expected:      encryptionVMSS,
			expectedError: "",
		},
		{
			name:          "user assigned identity vmss",
			spec:          userIdentitySpec,
			existing:      nil,
			expected:      userIdentityVMSS,
			expectedError: "",
		},
		{
			name:          "host encryption vmss",
			spec:          hostEncryptionSpec,
			existing:      nil,
			expected:      hostEncryptionVMSS,
			expectedError: "",
		},
		{
			name:          "host encryption unsupported vmss",
			spec:          hostEncryptionUnsupportedSpec,
			existing:      nil,
			expected:      nil,
			expectedError: "reconcile error that cannot be recovered occurred: encryption at host is not supported for VM type VM_SIZE_EAH. Object will not be requeued",
		},
		{
			name:          "ephemeral os disk read only vmss",
			spec:          ephemeralReadSpec,
			existing:      nil,
			expected:      ephemeralReadVMSS,
			expectedError: "",
		},
		{
			name:          "update for existing vmss",
			spec:          defaultExistingSpec,
			existing:      defaultExistingVMSS,
			expected:      defaultExistingVMSSClone,
			expectedError: "",
		},
		{
			name:          "vm with diagnostics set to User Managed and StorageAccountURI set",
			spec:          userManagedStorageAccountDiagnosticsSpec,
			existing:      nil,
			expected:      userManagedStorageAccountDiagnosticsVMSS,
			expectedError: "",
		},
		{
			name:          "vm with diagnostics set to Managed",
			spec:          managedDiagnosticsSpec,
			existing:      nil,
			expected:      managedDiagnoisticsVMSS,
			expectedError: "",
		},
		{
			name:          "vm with diagnostics set to Disabled",
			spec:          disabledDiagnosticsSpec,
			existing:      nil,
			expected:      disabledDiagnosticsVMSS,
			expectedError: "",
		},
		{
			name:          "vm with DiagnosticsProfile set to nil, do not panic",
			spec:          nilDiagnosticsProfileSpec,
			existing:      nil,
			expected:      nilDiagnosticsProfileVMSS,
			expectedError: "",
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			param, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				if tc.expected == nil {
					g.Expect(param).To(BeNil())
				} else {
					result, ok := param.(armcompute.VirtualMachineScaleSet)
					if !ok {
						t.Fatalf("expected type VirtualMachineScaleSet, got %T", param)
					}
					result.Properties.VirtualMachineProfile.OSProfile.AdminPassword = nil // Override this field as it's randomly generated. We can't set anything in tc.expected to match it.

					if !reflect.DeepEqual(tc.expected, result) {
						t.Errorf("Diff between actual result and expected result:\n%s", cmp.Diff(result, tc.expected))
					}
				}
			}
		})
	}
}
