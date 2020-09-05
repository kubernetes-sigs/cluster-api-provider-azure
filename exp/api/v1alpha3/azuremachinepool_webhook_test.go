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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

var (
	validSSHPublicKey = generateSSHPublicKey(true)
)

func TestAzureMachinePool_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		amp     *AzureMachinePool
		wantErr bool
	}{
		{
			name:    "azuremachinepool with marketplace image - full",
			amp:     createMachinePoolWithtMarketPlaceImage(t, "PUB1234", "OFFER1234", "SKU1234", "1.0.0", to.IntPtr(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - missing publisher",
			amp:     createMachinePoolWithtMarketPlaceImage(t, "", "OFFER1234", "SKU1234", "1.0.0", to.IntPtr(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with shared gallery image - full",
			amp:     createMachinePoolWithSharedImage(t, "SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0", to.IntPtr(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with marketplace image - missing subscription",
			amp:     createMachinePoolWithSharedImage(t, "", "RG123", "NAME123", "GALLERY1", "1.0.0", to.IntPtr(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with image by - with id",
			amp:     createMachinePoolWithImageByID(t, "ID123", to.IntPtr(10)),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with image by - without id",
			amp:     createMachinePoolWithImageByID(t, "", to.IntPtr(10)),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with valid SSHPublicKey",
			amp:     createMachinePoolWithSSHPublicKey(t, validSSHPublicKey),
			wantErr: false,
		},
		{
			name:    "azuremachinepool with invalid SSHPublicKey",
			amp:     createMachinePoolWithSSHPublicKey(t, "invalid ssh key"),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with wrong terminate notification",
			amp:     createMachinePoolWithSharedImage(t, "SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0", to.IntPtr(35)),
			wantErr: true,
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

	tests := []struct {
		name    string
		oldAMP  *AzureMachinePool
		amp     *AzureMachinePool
		wantErr bool
	}{
		{
			name:    "azuremachine with valid SSHPublicKey",
			oldAMP:  createMachinePoolWithSSHPublicKey(t, ""),
			amp:     createMachinePoolWithSSHPublicKey(t, validSSHPublicKey),
			wantErr: false,
		},
		{
			name:    "azuremachine with invalid SSHPublicKey",
			oldAMP:  createMachinePoolWithSSHPublicKey(t, ""),
			amp:     createMachinePoolWithSSHPublicKey(t, "invalid ssh key"),
			wantErr: true,
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
	publicKeyExistTest := test{amp: createMachinePoolWithSSHPublicKey(t, existingPublicKey)}
	publicKeyNotExistTest := test{amp: createMachinePoolWithSSHPublicKey(t, "")}

	publicKeyExistTest.amp.Default()
	g.Expect(publicKeyExistTest.amp.Spec.Template.SSHPublicKey).To(Equal(existingPublicKey))

	publicKeyNotExistTest.amp.Default()
	g.Expect(publicKeyNotExistTest.amp.Spec.Template.SSHPublicKey).NotTo((BeEmpty()))
}

func createMachinePoolWithtMarketPlaceImage(t *testing.T, publisher, offer, sku, version string, terminateNotificationTimeout *int) *AzureMachinePool {
	image := &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			Publisher: publisher,
			Offer:     offer,
			SKU:       sku,
			Version:   version,
		},
	}

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachineTemplate{
				Image:                        image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
			},
		},
	}
}

func createMachinePoolWithSharedImage(t *testing.T, subscriptionID, resourceGroup, name, gallery, version string, terminateNotificationTimeout *int) *AzureMachinePool {
	image := &infrav1.Image{
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
			Template: AzureMachineTemplate{
				Image:                        image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
			},
		},
	}
}

func createMachinePoolWithImageByID(t *testing.T, imageID string, terminateNotificationTimeout *int) *AzureMachinePool {
	image := &infrav1.Image{
		ID: &imageID,
	}

	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachineTemplate{
				Image:                        image,
				SSHPublicKey:                 validSSHPublicKey,
				TerminateNotificationTimeout: terminateNotificationTimeout,
			},
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
