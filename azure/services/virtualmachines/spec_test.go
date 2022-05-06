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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	validSKU = resourceskus.SKU{
		Name: to.StringPtr("Standard_D2v3"),
		Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  to.StringPtr(resourceskus.VCPUs),
				Value: to.StringPtr("2"),
			},
			{
				Name:  to.StringPtr(resourceskus.MemoryGB),
				Value: to.StringPtr("4"),
			},
		},
	}

	validSKUWithEncryptionAtHost = resourceskus.SKU{
		Name: to.StringPtr("Standard_D2v3"),
		Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  to.StringPtr(resourceskus.VCPUs),
				Value: to.StringPtr("2"),
			},
			{
				Name:  to.StringPtr(resourceskus.MemoryGB),
				Value: to.StringPtr("4"),
			},
			{
				Name:  to.StringPtr(resourceskus.EncryptionAtHost),
				Value: to.StringPtr(string(resourceskus.CapabilitySupported)),
			},
		},
	}

	validSKUWithEphemeralOS = resourceskus.SKU{
		Name: to.StringPtr("Standard_D2v3"),
		Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  to.StringPtr(resourceskus.VCPUs),
				Value: to.StringPtr("2"),
			},
			{
				Name:  to.StringPtr(resourceskus.MemoryGB),
				Value: to.StringPtr("4"),
			},
			{
				Name:  to.StringPtr(resourceskus.EphemeralOSDisk),
				Value: to.StringPtr("True"),
			},
		},
	}

	validSKUWithUltraSSD = resourceskus.SKU{
		Name: to.StringPtr("Standard_D2v3"),
		Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		LocationInfo: &[]compute.ResourceSkuLocationInfo{
			{
				Location: to.StringPtr("test-location"),
				Zones:    &[]string{"1"},
				ZoneDetails: &[]compute.ResourceSkuZoneDetails{
					{
						Capabilities: &[]compute.ResourceSkuCapabilities{
							{
								Name:  to.StringPtr("UltraSSDAvailable"),
								Value: to.StringPtr("True"),
							},
						},
						Name: &[]string{"1"},
					},
				},
			},
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  to.StringPtr(resourceskus.VCPUs),
				Value: to.StringPtr("2"),
			},
			{
				Name:  to.StringPtr(resourceskus.MemoryGB),
				Value: to.StringPtr("4"),
			},
		},
	}

	invalidCPUSKU = resourceskus.SKU{
		Name: to.StringPtr("Standard_D2v3"),
		Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  to.StringPtr(resourceskus.VCPUs),
				Value: to.StringPtr("1"),
			},
			{
				Name:  to.StringPtr(resourceskus.MemoryGB),
				Value: to.StringPtr("4"),
			},
		},
	}

	invalidMemSKU = resourceskus.SKU{
		Name: to.StringPtr("Standard_D2v3"),
		Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"test-location",
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  to.StringPtr(resourceskus.VCPUs),
				Value: to.StringPtr("2"),
			},
			{
				Name:  to.StringPtr(resourceskus.MemoryGB),
				Value: to.StringPtr("1"),
			},
		},
	}
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
				Image:      &infrav1.Image{ID: to.StringPtr("fake-image-id")},
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
				Image:                  &infrav1.Image{ID: to.StringPtr("fake-image-id")},
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
				Image:         &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				SpotVMOptions: &infrav1.SpotVMOptions{},
				SKU:           validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).Priority).To(Equal(compute.VirtualMachinePriorityTypesSpot))
				g.Expect(result.(compute.VirtualMachine).EvictionPolicy).To(Equal(compute.VirtualMachineEvictionPolicyTypesDeallocate))
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
				Image:      &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				OSDisk: infrav1.OSDisk{
					OSType:     "Windows",
					DiskSizeGB: to.Int32Ptr(128),
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
				Image:      &infrav1.Image{ID: to.StringPtr("fake-image-id")},
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
				g.Expect(result.(compute.VirtualMachine).VirtualMachineProperties.StorageProfile.OsDisk.ManagedDisk.DiskEncryptionSet.ID).To(Equal(to.StringPtr("my-diskencryptionset-id")))
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
				Image:           &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: to.BoolPtr(true)},
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
				Image:             &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				SKU:               validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).Zones).To(BeNil())
				g.Expect(result.(compute.VirtualMachine).AvailabilitySet.ID).To(Equal(to.StringPtr("fake-availability-set-id")))
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
					DiskSizeGB: to.Int32Ptr(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option: string(compute.DiffDiskOptionsLocal),
					},
				},
				Image: &infrav1.Image{ID: to.StringPtr("fake-image-id")},
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
			name: "creating a vm with encryption at host enabled for unsupported VM type fails",
			spec: &VMSpec{
				Name:              "my-vm",
				Role:              infrav1.Node,
				NICIDs:            []string{"my-nic"},
				SSHKeyData:        "fakesshpublickey",
				Size:              "Standard_D2v3",
				AvailabilitySetID: "fake-availability-set-id",
				Zone:              "",
				Image:             &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				SecurityProfile:   &infrav1.SecurityProfile{EncryptionAtHost: to.BoolPtr(true)},
				SKU:               validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: encryption at host is not supported for VM type Standard_D2v3. Object will not be requeued",
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
					DiskSizeGB: to.Int32Ptr(128),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option: string(compute.DiffDiskOptionsLocal),
					},
				},
				Image: &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				SKU:   validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: vm size Standard_D2v3 does not support ephemeral os. select a different vm size or disable ephemeral os. Object will not be requeued",
		},
		{
			name: "cannot create vm if vCPU is less than 2",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Image:      &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				SKU:        invalidCPUSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: vm size should be bigger or equal to at least 2 vCPUs. Object will not be requeued",
		},
		{
			name: "cannot create vm if memory is less than 2Gi",
			spec: &VMSpec{
				Name:       "my-vm",
				Role:       infrav1.Node,
				NICIDs:     []string{"my-nic"},
				SSHKeyData: "fakesshpublickey",
				Size:       "Standard_D2v3",
				Image:      &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				SKU:        invalidMemSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "reconcile error that cannot be recovered occurred: vm memory should be bigger or equal to at least 2Gi. Object will not be requeued",
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
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.Offer).To(Equal(to.StringPtr("my-offer")))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.Publisher).To(Equal(to.StringPtr("fake-publisher")))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.Sku).To(Equal(to.StringPtr("sku-id")))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.Version).To(Equal(to.StringPtr("1.0")))
				g.Expect(result.(compute.VirtualMachine).Plan.Name).To(Equal(to.StringPtr("sku-id")))
				g.Expect(result.(compute.VirtualMachine).Plan.Publisher).To(Equal(to.StringPtr("fake-publisher")))
				g.Expect(result.(compute.VirtualMachine).Plan.Product).To(Equal(to.StringPtr("my-offer")))
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
						Publisher:      to.StringPtr("fake-publisher"),
						Offer:          to.StringPtr("my-offer"),
						SKU:            to.StringPtr("sku-id"),
					},
				},
				SKU: validSKU,
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(compute.VirtualMachine{}))
				g.Expect(result.(compute.VirtualMachine).StorageProfile.ImageReference.ID).To(Equal(to.StringPtr("/subscriptions/fake-sub-id/resourceGroups/fake-rg/providers/Microsoft.Compute/galleries/fake-gallery/images/fake-name/versions/1.0")))
				g.Expect(result.(compute.VirtualMachine).Plan.Name).To(Equal(to.StringPtr("sku-id")))
				g.Expect(result.(compute.VirtualMachine).Plan.Publisher).To(Equal(to.StringPtr("fake-publisher")))
				g.Expect(result.(compute.VirtualMachine).Plan.Product).To(Equal(to.StringPtr("my-offer")))
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
				Image:      &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "mydisk",
						DiskSizeGB: 64,
						Lun:        to.Int32Ptr(0),
					},
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        to.Int32Ptr(1),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
					{
						NameSuffix: "myDiskWithManagedDisk",
						DiskSizeGB: 128,
						Lun:        to.Int32Ptr(2),
						ManagedDisk: &infrav1.ManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
						},
					},
					{
						NameSuffix: "managedDiskWithEncryption",
						DiskSizeGB: 128,
						Lun:        to.Int32Ptr(3),
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
				g.Expect(result.(compute.VirtualMachine).AdditionalCapabilities.UltraSSDEnabled).To(Equal(to.BoolPtr(true)))
				expectedDataDisks := &[]compute.DataDisk{
					{
						Lun:          to.Int32Ptr(0),
						Name:         to.StringPtr("my-ultra-ssd-vm_mydisk"),
						CreateOption: "Empty",
						DiskSizeGB:   to.Int32Ptr(64),
					},
					{
						Lun:          to.Int32Ptr(1),
						Name:         to.StringPtr("my-ultra-ssd-vm_myDiskWithUltraDisk"),
						CreateOption: "Empty",
						DiskSizeGB:   to.Int32Ptr(128),
						ManagedDisk: &compute.ManagedDiskParameters{
							StorageAccountType: "UltraSSD_LRS",
						},
					},
					{
						Lun:          to.Int32Ptr(2),
						Name:         to.StringPtr("my-ultra-ssd-vm_myDiskWithManagedDisk"),
						CreateOption: "Empty",
						DiskSizeGB:   to.Int32Ptr(128),
						ManagedDisk: &compute.ManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
						},
					},
					{
						Lun:          to.Int32Ptr(3),
						Name:         to.StringPtr("my-ultra-ssd-vm_managedDiskWithEncryption"),
						CreateOption: "Empty",
						DiskSizeGB:   to.Int32Ptr(128),
						ManagedDisk: &compute.ManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
							DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
								ID: to.StringPtr("my_id"),
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
				Image:      &infrav1.Image{ID: to.StringPtr("fake-image-id")},
				DataDisks: []infrav1.DataDisk{
					{
						NameSuffix: "myDiskWithUltraDisk",
						DiskSizeGB: 128,
						Lun:        to.Int32Ptr(1),
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
			expectedError: "reconcile error that cannot be recovered occurred: vm size Standard_D2v3 does not support ultra disks in location test-location. select a different vm size or disable ultra disks. Object will not be requeued",
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(tc.existing)
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
