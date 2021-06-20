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

package v1alpha4

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	. "github.com/onsi/gomega"
)

var (
	validSSHPublicKey = generateSSHPublicKey(true)
	validOSDisk       = generateValidOSDisk()
)

func TestAzureMachine_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		machine *AzureMachine
		wantErr bool
	}{
		{
			name:    "azuremachine with marketplace image - full",
			machine: createMachineWithtMarketPlaceImage(t, "PUB1234", "OFFER1234", "SKU1234", "1.0.0"),
			wantErr: false,
		},
		{
			name:    "azuremachine with marketplace image - missing publisher",
			machine: createMachineWithtMarketPlaceImage(t, "", "OFFER1234", "SKU1234", "1.0.0"),
			wantErr: true,
		},
		{
			name:    "azuremachine with shared gallery image - full",
			machine: createMachineWithSharedImage(t, "SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0"),
			wantErr: false,
		},
		{
			name:    "azuremachine with marketplace image - missing subscription",
			machine: createMachineWithSharedImage(t, "", "RG123", "NAME123", "GALLERY1", "1.0.0"),
			wantErr: true,
		},
		{
			name:    "azuremachine with image by - with id",
			machine: createMachineWithImageByID(t, "ID123"),
			wantErr: false,
		},
		{
			name:    "azuremachine with image by - without id",
			machine: createMachineWithImageByID(t, ""),
			wantErr: true,
		},
		{
			name:    "azuremachine with valid SSHPublicKey",
			machine: createMachineWithSSHPublicKey(t, validSSHPublicKey),
			wantErr: false,
		},
		{
			name:    "azuremachine without SSHPublicKey",
			machine: createMachineWithSSHPublicKey(t, ""),
			wantErr: true,
		},
		{
			name:    "azuremachine with invalid SSHPublicKey",
			machine: createMachineWithSSHPublicKey(t, "invalid ssh key"),
			wantErr: true,
		},
		{
			name:    "azuremachine with list of user-assigned identities",
			machine: createMachineWithUserAssignedIdentities(t, []UserAssignedIdentity{{ProviderID: "azure:///123"}, {ProviderID: "azure:///456"}}),
			wantErr: false,
		},
		{
			name:    "azuremachine with empty list of user-assigned identities",
			machine: createMachineWithUserAssignedIdentities(t, []UserAssignedIdentity{}),
			wantErr: true,
		},
		{
			name:    "azuremachine with valid osDisk cache type",
			machine: createMachineWithOsDiskCacheType(t, string(compute.PossibleCachingTypesValues()[1])),
			wantErr: false,
		},
		{
			name:    "azuremachine with invalid osDisk cache type",
			machine: createMachineWithOsDiskCacheType(t, "invalid_cache_type"),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.machine.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachine_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name       string
		oldMachine *AzureMachine
		newMachine *AzureMachine
		wantErr    bool
	}{
		{
			name: "invalidTest: azuremachine.spec.image is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Image: &Image{
						ID: pointer.String("imageID-1"),
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Image: &Image{
						ID: pointer.String("imageID-2"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.image is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Image: &Image{
						ID: pointer.String("imageID-1"),
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Image: &Image{
						ID: pointer.String("imageID-1"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.Identity is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Identity: VMIdentityUserAssigned,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Identity: VMIdentityNone,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.Identity is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Identity: VMIdentityNone,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Identity: VMIdentityNone,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.UserAssignedIdentities is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					UserAssignedIdentities: []UserAssignedIdentity{
						{ProviderID: "providerID-1"},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					UserAssignedIdentities: []UserAssignedIdentity{
						{ProviderID: "providerID-2"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.UserAssignedIdentities is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					UserAssignedIdentities: []UserAssignedIdentity{
						{ProviderID: "providerID-1"},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					UserAssignedIdentities: []UserAssignedIdentity{
						{ProviderID: "providerID-1"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.RoleAssignmentName is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					RoleAssignmentName: "role",
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					RoleAssignmentName: "not-role",
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.RoleAssignmentName is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					RoleAssignmentName: "role",
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					RoleAssignmentName: "role",
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.OSDisk is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					OSDisk: OSDisk{
						OSType: "osType-0",
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					OSDisk: OSDisk{
						OSType: "osType-1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.OSDisk is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					OSDisk: OSDisk{
						OSType: "osType-1",
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					OSDisk: OSDisk{
						OSType: "osType-1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.DataDisks is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					DataDisks: []DataDisk{
						{
							DiskSizeGB: 128,
						},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					DataDisks: []DataDisk{
						{
							DiskSizeGB: 64,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.DataDisks is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					DataDisks: []DataDisk{
						{
							DiskSizeGB: 128,
						},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					DataDisks: []DataDisk{
						{
							DiskSizeGB: 128,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.SSHPublicKey is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SSHPublicKey: "validKey",
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SSHPublicKey: "invalidKey",
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SSHPublicKey is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SSHPublicKey: "validKey",
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SSHPublicKey: "validKey",
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.AllocatePublicIP is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AllocatePublicIP: true,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AllocatePublicIP: false,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.AllocatePublicIP is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AllocatePublicIP: true,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AllocatePublicIP: true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.EnableIPForwarding is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					EnableIPForwarding: true,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					EnableIPForwarding: false,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.EnableIPForwarding is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					EnableIPForwarding: true,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					EnableIPForwarding: true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.AcceleratedNetworking is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: pointer.Bool(true),
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: pointer.Bool(false),
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.AcceleratedNetworking is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: pointer.Bool(true),
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: pointer.Bool(true),
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.SpotVMOptions is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SpotVMOptions: &SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-0"},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SpotVMOptions: &SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-1"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SpotVMOptions is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SpotVMOptions: &SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-1"},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SpotVMOptions: &SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-1"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.SecurityProfile is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SecurityProfile: &SecurityProfile{EncryptionAtHost: pointer.Bool(true)},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SecurityProfile: &SecurityProfile{EncryptionAtHost: pointer.Bool(false)},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SecurityProfile is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SecurityProfile: &SecurityProfile{EncryptionAtHost: pointer.Bool(true)},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SecurityProfile: &SecurityProfile{EncryptionAtHost: pointer.Bool(true)},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.newMachine.ValidateUpdate(tc.oldMachine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachine_Default(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machine *AzureMachine
	}

	existingPublicKey := validSSHPublicKey
	publicKeyExistTest := test{machine: createMachineWithSSHPublicKey(t, existingPublicKey)}
	publicKeyNotExistTest := test{machine: createMachineWithSSHPublicKey(t, "")}

	publicKeyExistTest.machine.Default()
	g.Expect(publicKeyExistTest.machine.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	publicKeyNotExistTest.machine.Default()
	g.Expect(publicKeyNotExistTest.machine.Spec.SSHPublicKey).To(Not(BeEmpty()))

	cacheTypeNotSpecifiedTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{OSDisk: OSDisk{CachingType: ""}}}}
	cacheTypeNotSpecifiedTest.machine.Default()
	g.Expect(cacheTypeNotSpecifiedTest.machine.Spec.OSDisk.CachingType).To(Equal("None"))

	for _, possibleCachingType := range compute.PossibleCachingTypesValues() {
		cacheTypeSpecifiedTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{OSDisk: OSDisk{CachingType: string(possibleCachingType)}}}}
		cacheTypeSpecifiedTest.machine.Default()
		g.Expect(cacheTypeSpecifiedTest.machine.Spec.OSDisk.CachingType).To(Equal(string(possibleCachingType)))
	}
}

func createMachineWithSharedImage(t *testing.T, subscriptionID, resourceGroup, name, gallery, version string) *AzureMachine {
	image := &Image{
		SharedGallery: &AzureSharedGalleryImage{
			SubscriptionID: subscriptionID,
			ResourceGroup:  resourceGroup,
			Name:           name,
			Gallery:        gallery,
			Version:        version,
		},
	}

	return &AzureMachine{
		Spec: AzureMachineSpec{
			Image:        image,
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
}

func createMachineWithtMarketPlaceImage(t *testing.T, publisher, offer, sku, version string) *AzureMachine {
	image := &Image{
		Marketplace: &AzureMarketplaceImage{
			Publisher: publisher,
			Offer:     offer,
			SKU:       sku,
			Version:   version,
		},
	}

	return &AzureMachine{
		Spec: AzureMachineSpec{
			Image:        image,
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
}

func createMachineWithImageByID(t *testing.T, imageID string) *AzureMachine {
	image := &Image{
		ID: &imageID,
	}

	return &AzureMachine{
		Spec: AzureMachineSpec{
			Image:        image,
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
}

func createMachineWithOsDiskCacheType(t *testing.T, cacheType string) *AzureMachine {
	machine := &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
	machine.Spec.OSDisk.CachingType = cacheType
	return machine
}
