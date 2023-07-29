/*
Copyright 2021 The Kubernetes Authors.

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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	validSKU = resourceskus.SKU{
		Name: pointer.String("Standard_D2v3"),
		Kind: pointer.String(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  pointer.String(resourceskus.VCPUs),
				Value: pointer.String("2"),
			},
			{
				Name:  pointer.String(resourceskus.MemoryGB),
				Value: pointer.String("4"),
			},
		},
	}

	validSKUWithEncryptionAtHost = resourceskus.SKU{
		Name: pointer.String("Standard_D2v3"),
		Kind: pointer.String(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  pointer.String(resourceskus.VCPUs),
				Value: pointer.String("2"),
			},
			{
				Name:  pointer.String(resourceskus.MemoryGB),
				Value: pointer.String("4"),
			},
			{
				Name:  pointer.String(resourceskus.EncryptionAtHost),
				Value: pointer.String(string(resourceskus.CapabilitySupported)),
			},
		},
	}

	validSKUWithTrustedLaunchDisabled = resourceskus.SKU{
		Name: pointer.String("Standard_D2v3"),
		Kind: pointer.String(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  pointer.String(resourceskus.VCPUs),
				Value: pointer.String("2"),
			},
			{
				Name:  pointer.String(resourceskus.MemoryGB),
				Value: pointer.String("4"),
			},
			{
				Name:  pointer.String(resourceskus.TrustedLaunchDisabled),
				Value: pointer.String(string(resourceskus.CapabilitySupported)),
			},
		},
	}

	validSKUWithConfidentialComputingType = resourceskus.SKU{
		Name: pointer.String("Standard_D2v3"),
		Kind: pointer.String(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  pointer.String(resourceskus.VCPUs),
				Value: pointer.String("2"),
			},
			{
				Name:  pointer.String(resourceskus.MemoryGB),
				Value: pointer.String("4"),
			},
			{
				Name:  pointer.String(resourceskus.ConfidentialComputingType),
				Value: pointer.String(string(resourceskus.CapabilitySupported)),
			},
		},
	}

	validSKUWithEphemeralOS = resourceskus.SKU{
		Name: pointer.String("Standard_D2v3"),
		Kind: pointer.String(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  pointer.String(resourceskus.VCPUs),
				Value: pointer.String("2"),
			},
			{
				Name:  pointer.String(resourceskus.MemoryGB),
				Value: pointer.String("4"),
			},
			{
				Name:  pointer.String(resourceskus.EphemeralOSDisk),
				Value: pointer.String("True"),
			},
		},
	}

	validSKUWithUltraSSD = resourceskus.SKU{
		Name: pointer.String("Standard_D2v3"),
		Kind: pointer.String(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		LocationInfo: &[]compute.ResourceSkuLocationInfo{
			{
				Location: pointer.String("test-location"),
				Zones:    &[]string{"1"},
				ZoneDetails: &[]compute.ResourceSkuZoneDetails{
					{
						Capabilities: &[]compute.ResourceSkuCapabilities{
							{
								Name:  pointer.String("UltraSSDAvailable"),
								Value: pointer.String("True"),
							},
						},
						Name: &[]string{"1"},
					},
				},
			},
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  pointer.String(resourceskus.VCPUs),
				Value: pointer.String("2"),
			},
			{
				Name:  pointer.String(resourceskus.MemoryGB),
				Value: pointer.String("4"),
			},
		},
	}

	invalidCPUSKU = resourceskus.SKU{
		Name: pointer.String("Standard_D2v3"),
		Kind: pointer.String(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  pointer.String(resourceskus.VCPUs),
				Value: pointer.String("1"),
			},
			{
				Name:  pointer.String(resourceskus.MemoryGB),
				Value: pointer.String("4"),
			},
		},
	}

	invalidMemSKU = resourceskus.SKU{
		Name: pointer.String("Standard_D2v3"),
		Kind: pointer.String(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  pointer.String(resourceskus.VCPUs),
				Value: pointer.String("2"),
			},
			{
				Name:  pointer.String(resourceskus.MemoryGB),
				Value: pointer.String("1"),
			},
		},
	}

	deletePolicy = infrav1.SpotEvictionPolicyDelete
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *VMSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "fails if existing is not a VirtualMachine",
			spec:     &VMSpec{},
			existing: network.VirtualNetwork{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "network.VirtualNetwork is not a compute.VirtualMachine",
		},
		{
			name:     "returns nil if vm already exists",
			spec:     &VMSpec{},
			existing: compute.VirtualMachine{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name: "fails if vm deleted out of band, should not recreate",
			spec: &VMSpec{
				ProviderID: "fake/vm/id",
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: azure.VMDeletedError{ProviderID: "fake/vm/id"}.Error(),
		},
		{
			name: "can create a vm with system assigned identity ",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				Identity:   infrav1.VMIdentitySystemAssigned,
				SKU:        validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).Identity.Type).To(Equal(compute.ResourceIdentityTypeSystemAssigned))
				g.Expect(result.(compute.VirtualMachine).Identity.UserAssignedIdentities).To(BeEmpty())
			},
			expectedError: "",
		},
		{
			name: "can create a vm with user assigned identity ",
			spec: &VMSpec{
				Name:                   "my-vm",
				Role:                   infrav1.Node,
				NICIDs:                 []string{"my-nic"},
				SSHKeyData:             "fakesshpublickey",
				Size:                   "Standard_D2v3",
				Zone:                   "1",
				Image:                  &infrav1.Image{ID: pointer.String("fake-image-id")},
				Identity:               infrav1.VMIdentityUserAssigned,
				UserAssignedIdentities: []infrav1.UserAssignedIdentity{{ProviderID: "my-user-id"}},
				SKU:                    validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).Identity.Type).To(Equal(compute.ResourceIdentityTypeUserAssigned))
				g.Expect(result.(compute.VirtualMachine).Identity.UserAssignedIdentities).To(Equal(map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue{"my-user-id": {}}))
			},
			expectedError: "",
		},
		{
			name: "can create a spot vm",
			spec: &VMSpec{
				Name:          "my-vm",
				Role:          infrav1.Node,
				NICIDs:        []string{"my-nic"},
				SSHKeyData:    "fakesshpublickey",
				Size:          "Standard_D2v3",
				Zone:          "1",
				Image:         &infrav1.Image{ID: pointer.String("fake-image-id")},
				SpotVMOptions: &infrav1.SpotVMOptions{},
				SKU:           validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).Priority).To(Equal(compute.VirtualMachinePriorityTypesSpot))
				g.Expect(result.(compute.VirtualMachine).BillingProfile).To(BeNil())
			},
			expectedError: "",
		},

		{
			name: "can create a spot vm with evictionPolicy delete",
			spec: &VMSpec{
				Name:          "my-vm",
				Role:          infrav1.Node,
				NICIDs:        []string{"my-nic"},
				SSHKeyData:    "fakesshpublickey",
				Size:          "Standard_D2v3",
				Zone:          "1",
				Image:         &infrav1.Image{ID: pointer.String("fake-image-id")},
				SpotVMOptions: &infrav1.SpotVMOptions{EvictionPolicy: &deletePolicy},
				SKU:           validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).Priority).To(Equal(compute.VirtualMachinePriorityTypesSpot))
				g.Expect(result.(compute.VirtualMachine).EvictionPolicy).To(Equal(compute.VirtualMachineEvictionPolicyTypesDelete))
				g.Expect(result.(compute.VirtualMachine).BillingProfile).To(BeNil())
			},
			expectedError: "",
		},
		{
			name: "can create a windows vm",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Windows",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).VirtualMachineProperties.StorageProfile.OsDisk.OsType).To(Equal(compute.OperatingSystemTypesWindows))
				g.Expect(*result.(compute.VirtualMachine).VirtualMachineProperties.OsProfile.AdminPassword).Should(HaveLen(123))
				g.Expect(*result.(compute.VirtualMachine).VirtualMachineProperties.OsProfile.AdminUsername).Should(Equal("capi"))
				g.Expect(*result.(compute.VirtualMachine).VirtualMachineProperties.OsProfile.WindowsConfiguration.EnableAutomaticUpdates).Should(Equal(false))
			},
			expectedError: "",
		},
		{
			name: "can create a vm with encryption",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
						DiskEncryptionSet: &infrav1.DiskEncryptionSetParameters{
							ID: "my-diskencryptionset-id",
						},
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).VirtualMachineProperties.StorageProfile.OsDisk.ManagedDisk.DiskEncryptionSet.ID).To(Equal(pointer.String("my-diskencryptionset-id")))
			},
			expectedError: "",
		},
		{
			name: "can create a vm with encryption at host",
			spec: &VMSpec{
				Name:            "my-vm",
				Role:            infrav1.Node,
				NICIDs:          []string{"my-nic"},
				SSHKeyData:      "fakesshpublickey",
				Size:            "Standard_D2v3",
				Zone:            "1",
				Image:           &infrav1.Image{ID: pointer.String("fake-image-id")},
				SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: pointer.Bool(true)},
				SKU:             validSKUWithEncryptionAtHost,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(*result.(compute.VirtualMachine).VirtualMachineProperties.SecurityProfile.EncryptionAtHost).To(Equal(true))
			},
			expectedError: "",
		},
		{
			name: "can create a vm and assign it to an availability set",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				SKU:               validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).Zones).To(BeNil())
				g.Expect(result.(compute.VirtualMachine).AvailabilitySet.ID).To(Equal(pointer.String("fake-availability-set-id")))
			},
			expectedError: "",
		},
		{
			name: "can create a vm with EphemeralOSDisk",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option: string(compute.DiffDiskOptionsLocal),
					},
				},
				Image: &infrav1.Image{ID: pointer.String("fake-image-id")},
				SKU:   validSKUWithEphemeralOS,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.OsDisk.DiffDiskSettings.Option).To(Equal(compute.DiffDiskOptionsLocal))
			},
			expectedError: "",
		},
		{
			name: "can create a trusted launch vm",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesTrustedLaunch,
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: pointer.Bool(true),
						VTpmEnabled:       pointer.Bool(true),
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(*result.(compute.VirtualMachine).SecurityProfile.UefiSettings.SecureBootEnabled).To(BeTrue())
				g.Expect(*result.(compute.VirtualMachine).SecurityProfile.UefiSettings.VTpmEnabled).To(BeTrue())
			},
			expectedError: "",
		},
		{
			name: "can create a confidential vm",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeVMGuestStateOnly,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: pointer.Bool(false),
						VTpmEnabled:       pointer.Bool(true),
					},
				},
				SKU: validSKUWithConfidentialComputingType,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.OsDisk.ManagedDisk.SecurityProfile.SecurityEncryptionType).To(Equal(compute.SecurityEncryptionTypesVMGuestStateOnly))
				g.Expect(*result.(compute.VirtualMachine).SecurityProfile.UefiSettings.VTpmEnabled).To(BeTrue())
			},
			expectedError: "",
		},
		{
			name: "creating a confidential vm without the SecurityType set to ConfidentialVM fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeVMGuestStateOnly,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: "",
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: pointer.Bool(false),
						VTpmEnabled:       pointer.Bool(true),
					},
				},
				SKU: validSKUWithConfidentialComputingType,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: securityType should be set to ConfidentialVM when securityEncryptionType is set. Object will not be requeued",
		},
		{
			name: "creating a vm with encryption at host enabled for unsupported VM type fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				SecurityProfile:   &infrav1.SecurityProfile{EncryptionAtHost: pointer.Bool(true)},
				SKU:               validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: encryption at host is not supported for VM type Standard_D2v3. Object will not be requeued",
		},
		{
			name: "creating a trusted launch vm without the SecurityType set to TrustedLaunch fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: "",
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: pointer.Bool(false),
						VTpmEnabled:       pointer.Bool(true),
					},
				},
				SKU: validSKUWithConfidentialComputingType,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: securityType should be set to TrustedLaunch when vTpmEnabled is true. Object will not be requeued",
		},
		{
			name: "creating a trusted launch vm with secure boot enabled on unsupported VM type fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesTrustedLaunch,
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: pointer.Bool(true),
					},
				},
				SKU: validSKUWithTrustedLaunchDisabled,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: secure boot is not supported for VM type Standard_D2v3. Object will not be requeued",
		},
		{
			name: "creating a trusted launch vm with vTPM enabled on unsupported VM type fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesTrustedLaunch,
					UefiSettings: &infrav1.UefiSettings{
						VTpmEnabled: pointer.Bool(true),
					},
				},
				SKU: validSKUWithTrustedLaunchDisabled,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: vTPM is not supported for VM type Standard_D2v3. Object will not be requeued",
		},
		{
			name: "creating a confidential vm with securityTypeEncryption DiskWithVMGuestState and encryption at host enabled fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeDiskWithVMGuestState,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					EncryptionAtHost: pointer.Bool(true),
					SecurityType:     infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						VTpmEnabled: pointer.Bool(true),
					},
				},
				SKU: validSKUWithConfidentialComputingType,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: encryption at host is not supported when securityEncryptionType is set to DiskWithVMGuestState. Object will not be requeued",
		},
		{
			name: "creating a confidential vm with DiskWithVMGuestState encryption type and secure boot disabled fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeDiskWithVMGuestState,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: pointer.Bool(false),
						VTpmEnabled:       pointer.Bool(true),
					},
				},
				SKU: validSKUWithConfidentialComputingType,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: secureBootEnabled should be true when securityEncryptionType is set to DiskWithVMGuestState. Object will not be requeued",
		},
		{
			name: "creating a confidential vm with vTPM disabled fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeVMGuestStateOnly,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						VTpmEnabled: pointer.Bool(false),
					},
				},
				SKU: validSKUWithConfidentialComputingType,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: vTpmEnabled should be true when securityEncryptionType is set. Object will not be requeued",
		},
		{
			name: "creating a confidential vm with unsupported VM type fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: pointer.String("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeVMGuestStateOnly,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						VTpmEnabled: pointer.Bool(true),
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: VM size Standard_D2v3 does not support confidential computing. Select a different VM size or remove the security profile of the OS disk. Object will not be requeued",
		},
		{
			name: "cannot create vm with EphemeralOSDisk if does not support ephemeral os",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: pointer.Int32(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option: string(compute.DiffDiskOptionsLocal),
					},
				},
				Image: &infrav1.Image{ID: pointer.String("fake-image-id")},
				SKU:   validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: VM size Standard_D2v3 does not support ephemeral os. Select a different VM size or disable ephemeral os. Object will not be requeued",
		},
		{
			name: "cannot create vm if vCPU is less than 2",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				SKU:        invalidCPUSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: VM size should be bigger or equal to at least 2 vCPUs. Object will not be requeued",
		},
		{
			name: "cannot create vm if memory is less than 2Gi",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				SKU:        invalidMemSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: VM memory should be bigger or equal to at least 2Gi. Object will not be requeued",
		},
		{
			name: "can create a vm with a marketplace image using a plan",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Image: &infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						ImagePlan: infrav1.ImagePlan{
							Publisher: "fake-publisher",
							Offer:     "my-offer",
							SKU:       "sku-id",
						},
						Version:         "1.0",
						ThirdPartyImage: true,
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.Offer).To(Equal(pointer.String("my-offer")))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.Publisher).To(Equal(pointer.String("fake-publisher")))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.Sku).To(Equal(pointer.String("sku-id")))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.Version).To(Equal(pointer.String("1.0")))
				g.Expect(result.(compute.VirtualMachine).Plan.Name).To(Equal(pointer.String("sku-id")))
				g.Expect(result.(compute.VirtualMachine).Plan.Publisher).To(Equal(pointer.String("fake-publisher")))
				g.Expect(result.(compute.VirtualMachine).Plan.Product).To(Equal(pointer.String("my-offer")))
			},
			expectedError: "",
		},
		{
			name: "can create a vm with a SIG image using a plan",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Image: &infrav1.Image{
					SharedGallery: &infrav1.AzureSharedGalleryImage{
						SubscriptionID: "fake-sub-id",
						ResourceGroup:  "fake-rg",
						Gallery:        "fake-gallery",
						Name:           "fake-name",
						Version:        "1.0",
						Publisher:      pointer.String("fake-publisher"),
						Offer:          pointer.String("my-offer"),
						SKU:            pointer.String("sku-id"),
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.ID).To(Equal(pointer.String("/subscriptions/fake-sub-id/resourceGroups/fake-rg/providers/Microsoft.Compute/galleries/fake-gallery/images/fake-name/versions/1.0")))
				g.Expect(result.(compute.VirtualMachine).Plan.Name).To(Equal(pointer.String("sku-id")))
				g.Expect(result.(compute.VirtualMachine).Plan.Publisher).To(Equal(pointer.String("fake-publisher")))
				g.Expect(result.(compute.VirtualMachine).Plan.Product).To(Equal(pointer.String("my-offer")))
			},
			expectedError: "",
		},
		{
			name: "can create a vm with ultra disk enabled",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "mydisk",
						DiskSizeGB: 64,
						Lun:        pointer.Int32(0),
					},
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        pointer.Int32(1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
					{
						NameSuffix: "myDiskWithManagedDisk",
						DiskSizeGB: 128,
						Lun:        pointer.Int32(2),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
						},
					},
					{
						NameSuffix: "managedDiskWithEncryption",
						DiskSizeGB: 128,
						Lun:        pointer.Int32(3),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
							DiskEncryptionSet: &infrav1.DiskEncryptionSetParameters{
								ID: "my_id",
							},
						},
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).AdditionalCapabilities.UltraSSDEnabled).To(Equal(pointer.Bool(true)))
				expectedDataDisks := &[]compute.DataDisk{
					{
						Lun:          pointer.Int32(0),
						Name:         pointer.String("my-ultra-ssd-vm_mydisk"),
						CreateOption: "Empty",
						DiskSizeGB:   pointer.Int32(64),
					},
					{
						Lun:          pointer.Int32(1),
						Name:         pointer.String("my-ultra-ssd-vm_myDiskWithUltraDisk"),
						CreateOption: "Empty",
						DiskSizeGB:   pointer.Int32(128),
						ManagedDisk: &compute.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
					{
						Lun:          pointer.Int32(2),
						Name:         pointer.String("my-ultra-ssd-vm_myDiskWithManagedDisk"),
						CreateOption: "Empty",
						DiskSizeGB:   pointer.Int32(128),
						ManagedDisk: &compute.ManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
						},
					},
					{
						Lun:          pointer.Int32(3),
						Name:         pointer.String("my-ultra-ssd-vm_managedDiskWithEncryption"),
						CreateOption: "Empty",
						DiskSizeGB:   pointer.Int32(128),
						ManagedDisk: &compute.ManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
							DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
								ID: pointer.String("my_id"),
							},
						},
					},
				}
				g.Expect(gomockinternal.DiffEq(expectedDataDisks).Matches(result.(compute.VirtualMachine).StorageProfile.DataDisks)).To(BeTrue(), cmp.Diff(expectedDataDisks, result.(compute.VirtualMachine).StorageProfile.DataDisks))
			},
			expectedError: "",
		},
		{
			name: "creating vm with ultra disk enabled in unsupported location fails",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        pointer.Int32(1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: VM size Standard_D2v3 does not support ultra disks in location test-location. Select a different VM size or disable ultra disks. Object will not be requeued",
		},
		{
			name: "creates a vm with AdditionalCapabilities.UltraSSDEnabled false, if an ultra disk is specified as data disk but AdditionalCapabilities.UltraSSDEnabled is false",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				AdditionalCapabilities: &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: pointer.Bool(false),
				},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        pointer.Int32(1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).AdditionalCapabilities.UltraSSDEnabled).To(Equal(pointer.Bool(false)))
				expectedDataDisks := &[]compute.DataDisk{
					{
						Lun:          pointer.Int32(1),
						Name:         pointer.String("my-ultra-ssd-vm_myDiskWithUltraDisk"),
						CreateOption: "Empty",
						DiskSizeGB:   pointer.Int32(128),
						ManagedDisk: &compute.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
				}
				g.Expect(gomockinternal.DiffEq(expectedDataDisks).Matches(result.(compute.VirtualMachine).StorageProfile.DataDisks)).To(BeTrue(), cmp.Diff(expectedDataDisks, result.(compute.VirtualMachine).StorageProfile.DataDisks))
			},
			expectedError: "",
		},
		{
			name: "creates a vm with AdditionalCapabilities.UltraSSDEnabled true, if an ultra disk is specified as data disk and no AdditionalCapabilities.UltraSSDEnabled is set",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        pointer.Int32(1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).AdditionalCapabilities.UltraSSDEnabled).To(Equal(pointer.Bool(true)))
				expectedDataDisks := &[]compute.DataDisk{
					{
						Lun:          pointer.Int32(1),
						Name:         pointer.String("my-ultra-ssd-vm_myDiskWithUltraDisk"),
						CreateOption: "Empty",
						DiskSizeGB:   pointer.Int32(128),
						ManagedDisk: &compute.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
				}
				g.Expect(gomockinternal.DiffEq(expectedDataDisks).Matches(result.(compute.VirtualMachine).StorageProfile.DataDisks)).To(BeTrue(), cmp.Diff(expectedDataDisks, result.(compute.VirtualMachine).StorageProfile.DataDisks))
			},
			expectedError: "",
		},
		{
			name: "creates a vm with AdditionalCapabilities.UltraSSDEnabled true, if an ultra disk is specified as data disk and AdditionalCapabilities.UltraSSDEnabled is true",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				AdditionalCapabilities: &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: pointer.Bool(true),
				},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        pointer.Int32(1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).AdditionalCapabilities.UltraSSDEnabled).To(Equal(pointer.Bool(true)))
				expectedDataDisks := &[]compute.DataDisk{
					{
						Lun:          pointer.Int32(1),
						Name:         pointer.String("my-ultra-ssd-vm_myDiskWithUltraDisk"),
						CreateOption: "Empty",
						DiskSizeGB:   pointer.Int32(128),
						ManagedDisk: &compute.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
				}
				g.Expect(gomockinternal.DiffEq(expectedDataDisks).Matches(result.(compute.VirtualMachine).StorageProfile.DataDisks)).To(BeTrue(), cmp.Diff(expectedDataDisks, result.(compute.VirtualMachine).StorageProfile.DataDisks))
			},
			expectedError: "",
		},
		{
			name: "creates a vm with AdditionalCapabilities.UltraSSDEnabled true, if no ultra disk is specified as data disk and AdditionalCapabilities.UltraSSDEnabled is true",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				AdditionalCapabilities: &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: pointer.Bool(true),
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).AdditionalCapabilities.UltraSSDEnabled).To(Equal(pointer.Bool(true)))
			},
			expectedError: "",
		},
		{
			name: "creates a vm with AdditionalCapabilities.UltraSSDEnabled false, if no ultra disk is specified as data disk and AdditionalCapabilities.UltraSSDEnabled is false",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				AdditionalCapabilities: &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: pointer.Bool(false),
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).AdditionalCapabilities.UltraSSDEnabled).To(Equal(pointer.Bool(false)))
			},
			expectedError: "",
		},
		{
			name: "creates a vm with Diagnostics disabled",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				DiagnosticsProfile: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.DisabledDiagnosticsStorage,
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).DiagnosticsProfile.BootDiagnostics.Enabled).To(Equal(pointer.Bool(false)))
				g.Expect(result.(compute.VirtualMachine).DiagnosticsProfile.BootDiagnostics.StorageURI).To(BeNil())
			},
			expectedError: "",
		},
		{
			name: "creates a vm with Managed Diagnostics enabled",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				DiagnosticsProfile: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.ManagedDiagnosticsStorage,
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).DiagnosticsProfile.BootDiagnostics.Enabled).To(Equal(pointer.Bool(true)))
				g.Expect(result.(compute.VirtualMachine).DiagnosticsProfile.BootDiagnostics.StorageURI).To(BeNil())
			},
			expectedError: "",
		},
		{
			name: "creates a vm with User Managed Diagnostics enabled",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				DiagnosticsProfile: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.UserManagedDiagnosticsStorage,
						UserManaged: &infrav1.UserManagedBootDiagnostics{
							StorageAccountURI: "aaa",
						},
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).DiagnosticsProfile.BootDiagnostics.Enabled).To(Equal(pointer.Bool(true)))
				g.Expect(result.(compute.VirtualMachine).DiagnosticsProfile.BootDiagnostics.StorageURI).To(Equal(pointer.String("aaa")))
			},
			expectedError: "",
		},
		{
			name: "creates a vm with User Managed Diagnostics enabled, but missing StorageAccountURI",
			spec: &VMSpec{
				Name:       "my-ultra-ssd-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Location:   "test-location",
				Zone:       "1",
				Image:      &infrav1.Image{ID: pointer.String("fake-image-id")},
				DiagnosticsProfile: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.UserManagedDiagnosticsStorage,
						UserManaged: &infrav1.UserManagedBootDiagnostics{
							StorageAccountURI: "aaa",
						},
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).DiagnosticsProfile.BootDiagnostics.Enabled).To(Equal(pointer.Bool(true)))
				g.Expect(result.(compute.VirtualMachine).DiagnosticsProfile.BootDiagnostics.StorageURI).To(Equal(pointer.String("aaa")))
			},
			expectedError: "",
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, result)
		})
	}
}
