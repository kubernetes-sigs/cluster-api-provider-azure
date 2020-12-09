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
	"testing"

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
		machine    *AzureMachine
		wantErr    bool
	}{
		{
			name:       "azuremachine with valid SSHPublicKey",
			oldMachine: createMachineWithSSHPublicKey(t, ""),
			machine:    createMachineWithSSHPublicKey(t, validSSHPublicKey),
			wantErr:    false,
		},
		{
			name:       "azuremachine without SSHPublicKey",
			oldMachine: createMachineWithSSHPublicKey(t, ""),
			machine:    createMachineWithSSHPublicKey(t, ""),
			wantErr:    true,
		},
		{
			name:       "azuremachine with invalid SSHPublicKey",
			oldMachine: createMachineWithSSHPublicKey(t, ""),
			machine:    createMachineWithSSHPublicKey(t, "invalid ssh key"),
			wantErr:    true,
		},
		{
			name:       "azuremachine with user assigned identities",
			oldMachine: createMachineWithUserAssignedIdentities(t, []UserAssignedIdentity{{ProviderID: "azure:///123"}}),
			machine:    createMachineWithUserAssignedIdentities(t, []UserAssignedIdentity{{ProviderID: "azure:///123"}, {ProviderID: "azure:///456"}}),
			wantErr:    false,
		},
		{
			name:       "azuremachine with empty user assigned identities",
			oldMachine: createMachineWithUserAssignedIdentities(t, []UserAssignedIdentity{{ProviderID: "azure:///123"}}),
			machine:    createMachineWithUserAssignedIdentities(t, []UserAssignedIdentity{}),
			wantErr:    true,
		},
		{
			name:       "azuremachine with valid osDisk cache type",
			oldMachine: createMachineWithOsDiskCacheType(t, string(compute.PossibleCachingTypesValues()[0])),
			machine:    createMachineWithOsDiskCacheType(t, string(compute.PossibleCachingTypesValues()[1])),
			wantErr:    false,
		},
		{
			name:       "azuremachine with invalid osDisk cache type",
			oldMachine: createMachineWithOsDiskCacheType(t, string(compute.PossibleCachingTypesValues()[0])),
			machine:    createMachineWithOsDiskCacheType(t, "invalid_cache_type"),
			wantErr:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.machine.ValidateUpdate(tc.oldMachine)
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
