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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"

	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
)

var (
	validSSHPublicKey = generateSSHPublicKey(true)
)

func TestAzureMachinePool_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	var (
		zero = intstr.FromInt(0)
		one  = intstr.FromInt(1)
	)

	tests := []struct {
		name    string
		amp     *AzureMachinePool
		wantErr bool
	}{
		{
			name:    "azuremachinepool with marketplace image - full",
			amp:     createMachinePoolWithtMarketPlaceImage("PUB1234", "OFFER1234", "SKU1234", "1.0.0", to.IntPtr(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - missing publisher",
			amp:     createMachinePoolWithtMarketPlaceImage("", "OFFER1234", "SKU1234", "1.0.0", to.IntPtr(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with shared gallery image - full",
			amp:     createMachinePoolWithSharedImage("SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0", to.IntPtr(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - missing subscription",
			amp:     createMachinePoolWithSharedImage("", "RG123", "NAME123", "GALLERY1", "1.0.0", to.IntPtr(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with image by - with id",
			amp:     createMachinePoolWithImageByID("ID123", to.IntPtr(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with image by - without id",
			amp:     createMachinePoolWithImageByID("", to.IntPtr(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with valid SSHPublicKey",
			amp:     createMachinePoolWithSSHPublicKey(validSSHPublicKey),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with invalid SSHPublicKey",
			amp:     createMachinePoolWithSSHPublicKey("invalid ssh key"),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with wrong terminate notification",
			amp:     createMachinePoolWithSharedImage("SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0", to.IntPtr(35)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with system assigned identity",
			amp:     createMachinePoolWithSystemAssignedIdentity(string(uuid.NewUUID())),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with system assigned identity, but invalid role",
			amp:     createMachinePoolWithSystemAssignedIdentity("not_a_uuid"),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with user assigned identity",
			amp:     createMachinePoolWithUserAssignedIdentity([]string{"azure:://id1", "azure:://id2"}),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with user assigned identity, but without any provider ids",
			amp:     createMachinePoolWithUserAssignedIdentity([]string{}),
			wantErr: true,
		},
		{
			name: "azuremachinepool with invalid MaxSurge and MaxUnavailable rolling upgrade configuration",
			amp: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{
				Type: RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &zero,
				},
			}),
			wantErr: true,
		},
		{
			name: "azuremachinepool with valid MaxSurge and MaxUnavailable rolling upgrade configuration",
			amp: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{
				Type: RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &one,
				},
			}),
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.amp.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachinePool_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	var (
		zero = intstr.FromInt(0)
		one  = intstr.FromInt(1)
	)

	tests := []struct {
		name    string
		oldAMP  *AzureMachinePool
		amp     *AzureMachinePool
		wantErr bool
	}{
		{
			name:    "azuremachinepool with valid SSHPublicKey",
			oldAMP:  createMachinePoolWithSSHPublicKey(""),
			amp:     createMachinePoolWithSSHPublicKey(validSSHPublicKey),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with invalid SSHPublicKey",
			oldAMP:  createMachinePoolWithSSHPublicKey(""),
			amp:     createMachinePoolWithSSHPublicKey("invalid ssh key"),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with system-assigned identity, and role unchanged",
			oldAMP:  createMachinePoolWithSystemAssignedIdentity("30a757d8-fcf0-4c8b-acf0-9253a7e093ea"),
			amp:     createMachinePoolWithSystemAssignedIdentity("30a757d8-fcf0-4c8b-acf0-9253a7e093ea"),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with system-assigned identity, and role changed",
			oldAMP:  createMachinePoolWithSystemAssignedIdentity(string(uuid.NewUUID())),
			amp:     createMachinePoolWithSystemAssignedIdentity(string(uuid.NewUUID())),
			wantErr: true,
		},
		{
			name:   "azuremachinepool with invalid MaxSurge and MaxUnavailable rolling upgrade configuration",
			oldAMP: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{}),
			amp: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{
				Type: RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &zero,
				},
			}),
			wantErr: true,
		},
		{
			name:   "azuremachinepool with valid MaxSurge and MaxUnavailable rolling upgrade configuration",
			oldAMP: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{}),
			amp: createMachinePoolWithStrategy(AzureMachinePoolDeploymentStrategy{
				Type: RollingUpdateAzureMachinePoolDeploymentStrategyType,
				RollingUpdate: &MachineRollingUpdateDeployment{
					MaxSurge:       &zero,
					MaxUnavailable: &one,
				},
			}),
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.amp.ValidateUpdate(tc.oldAMP)
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
		amp *AzureMachinePool
	}

	existingPublicKey := validSSHPublicKey
	publicKeyExistTest := test{amp: createMachinePoolWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{amp: createMachinePoolWithSSHPublicKey("")}

	publicKeyExistTest.amp.Default()
	g.Expect(publicKeyExistTest.amp.Spec.Template.SSHPublicKey).To(Equal(existingPublicKey))

	publicKeyNotExistTest.amp.Default()
	g.Expect(publicKeyNotExistTest.amp.Spec.Template.SSHPublicKey).NotTo(BeEmpty())
}

func createMachinePoolWithtMarketPlaceImage(publisher, offer, sku, version string, terminateNotificationTimeout *int) *AzureMachinePool {
	image := infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			Publisher: publisher,
			Offer:     offer,
			SKU:       sku,
			Version:   version,
		},
	}

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
			},
		},
	}
}

func createMachinePoolWithSharedImage(subscriptionID, resourceGroup, name, gallery, version string, terminateNotificationTimeout *int) *AzureMachinePool {
	image := infrav1.Image{
		SharedGallery: &infrav1.AzureSharedGalleryImage{
			SubscriptionID: subscriptionID,
			ResourceGroup:  resourceGroup,
			Name:           name,
			Gallery:        gallery,
			Version:        version,
		},
	}

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
			},
		},
	}
}

func createMachinePoolWithImageByID(imageID string, terminateNotificationTimeout *int) *AzureMachinePool {
	image := infrav1.Image{
		ID: &imageID,
	}

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Image:                        &image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
			},
		},
	}
}

func createMachinePoolWithSystemAssignedIdentity(role string) *AzureMachinePool {
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Identity:           infrav1.VMIdentitySystemAssigned,
			RoleAssignmentName: role,
		},
	}
}

func createMachinePoolWithUserAssignedIdentity(providerIds []string) *AzureMachinePool {
	userAssignedIdentities := make([]infrav1.UserAssignedIdentity, len(providerIds))

	for _, providerID := range providerIds {
		userAssignedIdentities = append(userAssignedIdentities, infrav1.UserAssignedIdentity{
			ProviderID: providerID,
		})
	}

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Identity:               infrav1.VMIdentityUserAssigned,
			UserAssignedIdentities: userAssignedIdentities,
		},
	}
}

func generateSSHPublicKey(b64Enconded bool) string {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicRsaKey, _ := ssh.NewPublicKey(&privateKey.PublicKey)
	if b64Enconded {
		return base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}
	return string(ssh.MarshalAuthorizedKey(publicRsaKey))
}

func createMachinePoolWithStrategy(strategy AzureMachinePoolDeploymentStrategy) *AzureMachinePool {
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Strategy: strategy,
		},
	}
}
