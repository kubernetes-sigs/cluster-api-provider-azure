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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	validSKU = resourceskus.SKU{
		Name: ptr.To("Standard_D2v3"),
		Kind: ptr.To(string(resourceskus.VirtualMachines)),
		Locations: []*string{
			ptr.To("test-location"),
		},
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.VCPUs),
				Value: ptr.To("2"),
			},
			{
				Name:  ptr.To(resourceskus.MemoryGB),
				Value: ptr.To("4"),
			},
		},
	}

	validSKUWithEncryptionAtHost = resourceskus.SKU{
		Name: ptr.To("Standard_D2v3"),
		Kind: ptr.To(string(resourceskus.VirtualMachines)),
		Locations: []*string{
			ptr.To("test-location"),
		},
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.VCPUs),
				Value: ptr.To("2"),
			},
			{
				Name:  ptr.To(resourceskus.MemoryGB),
				Value: ptr.To("4"),
			},
			{
				Name:  ptr.To(resourceskus.EncryptionAtHost),
				Value: ptr.To(string(resourceskus.CapabilitySupported)),
			},
		},
	}

	validSKUWithTrustedLaunchDisabled = resourceskus.SKU{
		Name: ptr.To("Standard_D2v3"),
		Kind: ptr.To(string(resourceskus.VirtualMachines)),
		Locations: []*string{
			ptr.To("test-location"),
		},
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.VCPUs),
				Value: ptr.To("2"),
			},
			{
				Name:  ptr.To(resourceskus.MemoryGB),
				Value: ptr.To("4"),
			},
			{
				Name:  ptr.To(resourceskus.TrustedLaunchDisabled),
				Value: ptr.To(string(resourceskus.CapabilitySupported)),
			},
		},
	}

	validSKUWithConfidentialComputingType = resourceskus.SKU{
		Name: ptr.To("Standard_D2v3"),
		Kind: ptr.To(string(resourceskus.VirtualMachines)),
		Locations: []*string{
			ptr.To("test-location"),
		},
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.VCPUs),
				Value: ptr.To("2"),
			},
			{
				Name:  ptr.To(resourceskus.MemoryGB),
				Value: ptr.To("4"),
			},
			{
				Name:  ptr.To(resourceskus.ConfidentialComputingType),
				Value: ptr.To(string(resourceskus.CapabilitySupported)),
			},
		},
	}

	validSKUWithEphemeralOS = resourceskus.SKU{
		Name: ptr.To("Standard_D2v3"),
		Kind: ptr.To(string(resourceskus.VirtualMachines)),
		Locations: []*string{
			ptr.To("test-location"),
		},
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.VCPUs),
				Value: ptr.To("2"),
			},
			{
				Name:  ptr.To(resourceskus.MemoryGB),
				Value: ptr.To("4"),
			},
			{
				Name:  ptr.To(resourceskus.EphemeralOSDisk),
				Value: ptr.To("True"),
			},
		},
	}

	validSKUWithUltraSSD = resourceskus.SKU{
		Name: ptr.To("Standard_D2v3"),
		Kind: ptr.To(string(resourceskus.VirtualMachines)),
		Locations: []*string{
			ptr.To("test-location"),
		},
		LocationInfo: []*armcompute.ResourceSKULocationInfo{
			{
				Location: ptr.To("test-location"),
				Zones:    []*string{ptr.To("1")},
				ZoneDetails: []*armcompute.ResourceSKUZoneDetails{
					{
						Capabilities: []*armcompute.ResourceSKUCapabilities{
							{
								Name:  ptr.To("UltraSSDAvailable"),
								Value: ptr.To("True"),
							},
						},
						Name: []*string{ptr.To("1")},
					},
				},
			},
		},
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.VCPUs),
				Value: ptr.To("2"),
			},
			{
				Name:  ptr.To(resourceskus.MemoryGB),
				Value: ptr.To("4"),
			},
		},
	}

	invalidCPUSKU = resourceskus.SKU{
		Name: ptr.To("Standard_D2v3"),
		Kind: ptr.To(string(resourceskus.VirtualMachines)),
		Locations: []*string{
			ptr.To("test-location"),
		},
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.VCPUs),
				Value: ptr.To("1"),
			},
			{
				Name:  ptr.To(resourceskus.MemoryGB),
				Value: ptr.To("4"),
			},
		},
	}

	invalidMemSKU = resourceskus.SKU{
		Name: ptr.To("Standard_D2v3"),
		Kind: ptr.To(string(resourceskus.VirtualMachines)),
		Locations: []*string{
			ptr.To("test-location"),
		},
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.VCPUs),
				Value: ptr.To("2"),
			},
			{
				Name:  ptr.To(resourceskus.MemoryGB),
				Value: ptr.To("1"),
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
			existing: armnetwork.VirtualNetwork{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "armnetwork.VirtualNetwork is not an armcompute.VirtualMachine",
		},
		{
			name:     "returns nil if vm already exists",
			spec:     &VMSpec{},
			existing: armcompute.VirtualMachine{},
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				Identity:   infrav1.VMIdentitySystemAssigned,
				SKU:        validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Identity.Type).To(Equal(ptr.To(armcompute.ResourceIdentityTypeSystemAssigned)))
				g.Expect(result.(armcompute.VirtualMachine).Identity.UserAssignedIdentities).To(BeEmpty())
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
				Image:                  &infrav1.Image{ID: ptr.To("fake-image-id")},
				Identity:               infrav1.VMIdentityUserAssigned,
				UserAssignedIdentities: []infrav1.UserAssignedIdentity{{ProviderID: "my-user-id"}},
				SKU:                    validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Identity.Type).To(Equal(ptr.To(armcompute.ResourceIdentityTypeUserAssigned)))
				g.Expect(result.(armcompute.VirtualMachine).Identity.UserAssignedIdentities).To(Equal(map[string]*armcompute.UserAssignedIdentitiesValue{"my-user-id": {}}))
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
				Image:         &infrav1.Image{ID: ptr.To("fake-image-id")},
				SpotVMOptions: &infrav1.SpotVMOptions{},
				SKU:           validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.Priority).To(Equal(ptr.To(armcompute.VirtualMachinePriorityTypesSpot)))
				g.Expect(result.(armcompute.VirtualMachine).Properties.BillingProfile).To(BeNil())
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
				Image:         &infrav1.Image{ID: ptr.To("fake-image-id")},
				SpotVMOptions: &infrav1.SpotVMOptions{EvictionPolicy: &deletePolicy},
				SKU:           validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.Priority).To(Equal(ptr.To(armcompute.VirtualMachinePriorityTypesSpot)))
				g.Expect(result.(armcompute.VirtualMachine).Properties.EvictionPolicy).To(Equal(ptr.To(armcompute.VirtualMachineEvictionPolicyTypesDelete)))
				g.Expect(result.(armcompute.VirtualMachine).Properties.BillingProfile).To(BeNil())
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Windows",
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.OSDisk.OSType).To(Equal(ptr.To(armcompute.OperatingSystemTypesWindows)))
				g.Expect(*result.(armcompute.VirtualMachine).Properties.OSProfile.AdminPassword).Should(HaveLen(123))
				g.Expect(*result.(armcompute.VirtualMachine).Properties.OSProfile.AdminUsername).Should(Equal("capi"))
				g.Expect(*result.(armcompute.VirtualMachine).Properties.OSProfile.WindowsConfiguration.EnableAutomaticUpdates).Should(BeFalse())
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
						DiskEncryptionSet: &infrav1.DiskEncryptionSetParameters{
							ID: "my-diskencryptionset-id",
						},
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.OSDisk.ManagedDisk.DiskEncryptionSet.ID).To(Equal(ptr.To("my-diskencryptionset-id")))
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
				Image:           &infrav1.Image{ID: ptr.To("fake-image-id")},
				SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: ptr.To(true)},
				SKU:             validSKUWithEncryptionAtHost,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(*result.(armcompute.VirtualMachine).Properties.SecurityProfile.EncryptionAtHost).To(BeTrue())
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				SKU:               validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Zones).To(BeNil())
				g.Expect(result.(armcompute.VirtualMachine).Properties.AvailabilitySet.ID).To(Equal(ptr.To("fake-availability-set-id")))
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
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
					},
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option: string(armcompute.DiffDiskOptionsLocal),
					},
				},
				Image: &infrav1.Image{ID: ptr.To("fake-image-id")},
				SKU:   validSKUWithEphemeralOS,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.OSDisk.DiffDiskSettings.Option).To(Equal(ptr.To(armcompute.DiffDiskOptionsLocal)))
			},
			expectedError: "",
		},
		{
			name: "can create a vm with DiffDiskPlacement ResourceDisk",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
					},
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option:    string(armcompute.DiffDiskOptionsLocal),
						Placement: ptr.To(infrav1.DiffDiskPlacementResourceDisk),
					},
				},
				Image: &infrav1.Image{ID: ptr.To("fake-image-id")},
				SKU:   validSKUWithEphemeralOS,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.OSDisk.DiffDiskSettings.Placement).To(Equal(ptr.To(armcompute.DiffDiskPlacementResourceDisk)))
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesTrustedLaunch,
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: ptr.To(true),
						VTpmEnabled:       ptr.To(true),
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(*result.(armcompute.VirtualMachine).Properties.SecurityProfile.UefiSettings.SecureBootEnabled).To(BeTrue())
				g.Expect(*result.(armcompute.VirtualMachine).Properties.SecurityProfile.UefiSettings.VTpmEnabled).To(BeTrue())
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeVMGuestStateOnly,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: ptr.To(false),
						VTpmEnabled:       ptr.To(true),
					},
				},
				SKU: validSKUWithConfidentialComputingType,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.OSDisk.ManagedDisk.SecurityProfile.SecurityEncryptionType).To(Equal(ptr.To(armcompute.SecurityEncryptionTypesVMGuestStateOnly)))
				g.Expect(*result.(armcompute.VirtualMachine).Properties.SecurityProfile.UefiSettings.VTpmEnabled).To(BeTrue())
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeVMGuestStateOnly,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: "",
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: ptr.To(false),
						VTpmEnabled:       ptr.To(true),
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				SecurityProfile:   &infrav1.SecurityProfile{EncryptionAtHost: ptr.To(true)},
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: "",
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: ptr.To(false),
						VTpmEnabled:       ptr.To(true),
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesTrustedLaunch,
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: ptr.To(true),
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesTrustedLaunch,
					UefiSettings: &infrav1.UefiSettings{
						VTpmEnabled: ptr.To(true),
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeDiskWithVMGuestState,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					EncryptionAtHost: ptr.To(true),
					SecurityType:     infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						VTpmEnabled: ptr.To(true),
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeDiskWithVMGuestState,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						SecureBootEnabled: ptr.To(false),
						VTpmEnabled:       ptr.To(true),
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeVMGuestStateOnly,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						VTpmEnabled: ptr.To(false),
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
				Image:             &infrav1.Image{ID: ptr.To("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Linux",
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
						SecurityProfile: &infrav1.VMDiskSecurityProfile{
							SecurityEncryptionType: infrav1.SecurityEncryptionTypeVMGuestStateOnly,
						},
					},
				},
				SecurityProfile: &infrav1.SecurityProfile{
					SecurityType: infrav1.SecurityTypesConfidentialVM,
					UefiSettings: &infrav1.UefiSettings{
						VTpmEnabled: ptr.To(true),
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
					DiskSizeGB: ptr.To[int32](128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
					},
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option: string(armcompute.DiffDiskOptionsLocal),
					},
				},
				Image: &infrav1.Image{ID: ptr.To("fake-image-id")},
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
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
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.ImageReference.Offer).To(Equal(ptr.To("my-offer")))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.ImageReference.Publisher).To(Equal(ptr.To("fake-publisher")))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.ImageReference.SKU).To(Equal(ptr.To("sku-id")))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.ImageReference.Version).To(Equal(ptr.To("1.0")))
				g.Expect(result.(armcompute.VirtualMachine).Plan.Name).To(Equal(ptr.To("sku-id")))
				g.Expect(result.(armcompute.VirtualMachine).Plan.Publisher).To(Equal(ptr.To("fake-publisher")))
				g.Expect(result.(armcompute.VirtualMachine).Plan.Product).To(Equal(ptr.To("my-offer")))
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
						Publisher:      ptr.To("fake-publisher"),
						Offer:          ptr.To("my-offer"),
						SKU:            ptr.To("sku-id"),
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.StorageProfile.ImageReference.ID).To(Equal(ptr.To("/subscriptions/fake-sub-id/resourceGroups/fake-rg/providers/Microsoft.Compute/galleries/fake-gallery/images/fake-name/versions/1.0")))
				g.Expect(result.(armcompute.VirtualMachine).Plan.Name).To(Equal(ptr.To("sku-id")))
				g.Expect(result.(armcompute.VirtualMachine).Plan.Publisher).To(Equal(ptr.To("fake-publisher")))
				g.Expect(result.(armcompute.VirtualMachine).Plan.Product).To(Equal(ptr.To("my-offer")))
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "mydisk",
						DiskSizeGB: 64,
						Lun:        ptr.To[int32](0),
					},
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        ptr.To[int32](1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: string(armcompute.StorageAccountTypesUltraSSDLRS),
						},
					},
					{
						NameSuffix: "myDiskWithManagedDisk",
						DiskSizeGB: 128,
						Lun:        ptr.To[int32](2),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
						},
					},
					{
						NameSuffix: "managedDiskWithEncryption",
						DiskSizeGB: 128,
						Lun:        ptr.To[int32](3),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: string(armcompute.StorageAccountTypesPremiumLRS),
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
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.AdditionalCapabilities.UltraSSDEnabled).To(Equal(ptr.To(true)))
				expectedDataDisks := []*armcompute.DataDisk{
					{
						Lun:          ptr.To[int32](0),
						Name:         ptr.To("my-ultra-ssd-vm_mydisk"),
						CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
						DiskSizeGB:   ptr.To[int32](64),
					},
					{
						Lun:          ptr.To[int32](1),
						Name:         ptr.To("my-ultra-ssd-vm_myDiskWithUltraDisk"),
						CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
						DiskSizeGB:   ptr.To[int32](128),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							StorageAccountType: ptr.To(armcompute.StorageAccountTypesUltraSSDLRS),
						},
					},
					{
						Lun:          ptr.To[int32](2),
						Name:         ptr.To("my-ultra-ssd-vm_myDiskWithManagedDisk"),
						CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
						DiskSizeGB:   ptr.To[int32](128),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							StorageAccountType: ptr.To(armcompute.StorageAccountTypesPremiumLRS),
						},
					},
					{
						Lun:          ptr.To[int32](3),
						Name:         ptr.To("my-ultra-ssd-vm_managedDiskWithEncryption"),
						CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
						DiskSizeGB:   ptr.To[int32](128),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							StorageAccountType: ptr.To(armcompute.StorageAccountTypesPremiumLRS),
							DiskEncryptionSet: &armcompute.DiskEncryptionSetParameters{
								ID: ptr.To("my_id"),
							},
						},
					},
				}
				g.Expect(gomockinternal.DiffEq(expectedDataDisks).Matches(result.(armcompute.VirtualMachine).Properties.StorageProfile.DataDisks)).To(BeTrue(), cmp.Diff(expectedDataDisks, result.(armcompute.VirtualMachine).Properties.StorageProfile.DataDisks))
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        ptr.To[int32](1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: string(armcompute.StorageAccountTypesUltraSSDLRS),
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				AdditionalCapabilities: &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: ptr.To(false),
				},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        ptr.To[int32](1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: string(armcompute.StorageAccountTypesUltraSSDLRS),
						},
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.AdditionalCapabilities.UltraSSDEnabled).To(Equal(ptr.To(false)))
				expectedDataDisks := []*armcompute.DataDisk{
					{
						Lun:          ptr.To[int32](1),
						Name:         ptr.To("my-ultra-ssd-vm_myDiskWithUltraDisk"),
						CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
						DiskSizeGB:   ptr.To[int32](128),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							StorageAccountType: ptr.To(armcompute.StorageAccountTypesUltraSSDLRS),
						},
					},
				}
				g.Expect(gomockinternal.DiffEq(expectedDataDisks).Matches(result.(armcompute.VirtualMachine).Properties.StorageProfile.DataDisks)).To(BeTrue(), cmp.Diff(expectedDataDisks, result.(armcompute.VirtualMachine).Properties.StorageProfile.DataDisks))
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        ptr.To[int32](1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: string(armcompute.StorageAccountTypesUltraSSDLRS),
						},
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.AdditionalCapabilities.UltraSSDEnabled).To(Equal(ptr.To(true)))
				expectedDataDisks := []*armcompute.DataDisk{
					{
						Lun:          ptr.To[int32](1),
						Name:         ptr.To("my-ultra-ssd-vm_myDiskWithUltraDisk"),
						CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
						DiskSizeGB:   ptr.To[int32](128),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							StorageAccountType: ptr.To(armcompute.StorageAccountTypesUltraSSDLRS),
						},
					},
				}
				g.Expect(gomockinternal.DiffEq(expectedDataDisks).Matches(result.(armcompute.VirtualMachine).Properties.StorageProfile.DataDisks)).To(BeTrue(), cmp.Diff(expectedDataDisks, result.(armcompute.VirtualMachine).Properties.StorageProfile.DataDisks))
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				AdditionalCapabilities: &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: ptr.To(true),
				},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        ptr.To[int32](1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: string(armcompute.StorageAccountTypesUltraSSDLRS),
						},
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.AdditionalCapabilities.UltraSSDEnabled).To(Equal(ptr.To(true)))
				expectedDataDisks := []*armcompute.DataDisk{
					{
						Lun:          ptr.To[int32](1),
						Name:         ptr.To("my-ultra-ssd-vm_myDiskWithUltraDisk"),
						CreateOption: ptr.To(armcompute.DiskCreateOptionTypesEmpty),
						DiskSizeGB:   ptr.To[int32](128),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							StorageAccountType: ptr.To(armcompute.StorageAccountTypesUltraSSDLRS),
						},
					},
				}
				g.Expect(gomockinternal.DiffEq(expectedDataDisks).Matches(result.(armcompute.VirtualMachine).Properties.StorageProfile.DataDisks)).To(BeTrue(), cmp.Diff(expectedDataDisks, result.(armcompute.VirtualMachine).Properties.StorageProfile.DataDisks))
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				AdditionalCapabilities: &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: ptr.To(true),
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.AdditionalCapabilities.UltraSSDEnabled).To(Equal(ptr.To(true)))
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				AdditionalCapabilities: &infrav1.AdditionalCapabilities{
					UltraSSDEnabled: ptr.To(false),
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.AdditionalCapabilities.UltraSSDEnabled).To(Equal(ptr.To(false)))
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				DiagnosticsProfile: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.DisabledDiagnosticsStorage,
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.DiagnosticsProfile.BootDiagnostics.Enabled).To(Equal(ptr.To(false)))
				g.Expect(result.(armcompute.VirtualMachine).Properties.DiagnosticsProfile.BootDiagnostics.StorageURI).To(BeNil())
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
				DiagnosticsProfile: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.ManagedDiagnosticsStorage,
					},
				},
				SKU: validSKUWithUltraSSD,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.DiagnosticsProfile.BootDiagnostics.Enabled).To(Equal(ptr.To(true)))
				g.Expect(result.(armcompute.VirtualMachine).Properties.DiagnosticsProfile.BootDiagnostics.StorageURI).To(BeNil())
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
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
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.DiagnosticsProfile.BootDiagnostics.Enabled).To(Equal(ptr.To(true)))
				g.Expect(result.(armcompute.VirtualMachine).Properties.DiagnosticsProfile.BootDiagnostics.StorageURI).To(Equal(ptr.To("aaa")))
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
				Image:      &infrav1.Image{ID: ptr.To("fake-image-id")},
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
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.DiagnosticsProfile.BootDiagnostics.Enabled).To(Equal(ptr.To(true)))
				g.Expect(result.(armcompute.VirtualMachine).Properties.DiagnosticsProfile.BootDiagnostics.StorageURI).To(Equal(ptr.To("aaa")))
			},
			expectedError: "",
		},
		{
			name: "creates a vm and associate it with a capacity reservation group",
			spec: &VMSpec{
				Name:                       "my-vm",
				Role:                       infrav1.Node,
				NICIDs:                     []string{"my-nic"},
				SSHKeyData:                 "fakesshpublickey",
				Size:                       "Standard_D2v3",
				Location:                   "test-location",
				Zone:                       "1",
				Image:                      &infrav1.Image{ID: ptr.To("fake-image-id")},
				CapacityReservationGroupID: "my-crg-id",
				SKU:                        validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.CapacityReservation.CapacityReservationGroup.ID).To(Equal(ptr.To("my-crg-id")))
			},
			expectedError: "",
		},
		{
			name: "creates a vm without capacity reservation group",
			spec: &VMSpec{
				Name:                       "my-vm",
				Role:                       infrav1.Node,
				NICIDs:                     []string{"my-nic"},
				SSHKeyData:                 "fakesshpublickey",
				Size:                       "Standard_D2v3",
				Location:                   "test-location",
				Zone:                       "1",
				Image:                      &infrav1.Image{ID: ptr.To("fake-image-id")},
				CapacityReservationGroupID: "",
				SKU:                        validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.VirtualMachine{}))
				g.Expect(result.(armcompute.VirtualMachine).Properties.CapacityReservation).To(BeNil())
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
