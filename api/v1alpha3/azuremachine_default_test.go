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

	. "github.com/onsi/gomega"
)

func TestAzureMachine_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machine *AzureMachine
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{machine: createMachineWithSSHPublicKey(t, existingPublicKey)}
	publicKeyNotExistTest := test{machine: createMachineWithSSHPublicKey(t, "")}

	err := publicKeyExistTest.machine.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyExistTest.machine.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	err = publicKeyNotExistTest.machine.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyNotExistTest.machine.Spec.SSHPublicKey).To(Not(BeEmpty()))
}

func createMachineWithSSHPublicKey(t *testing.T, sshPublicKey string) *AzureMachine {
	return &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey: sshPublicKey,
			Image: &Image{
				SharedGallery: &AzureSharedGalleryImage{
					SubscriptionID: "SUB123",
					ResourceGroup:  "RG123",
					Name:           "NAME123",
					Gallery:        "GALLERY1",
					Version:        "1.0.0",
				},
			},
		},
	}
}

func createMachineWithUserAssignedIdentities(t *testing.T, identitiesList []UserAssignedIdentity) *AzureMachine {
	return &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey: generateSSHPublicKey(),
			Image: &Image{
				SharedGallery: &AzureSharedGalleryImage{
					SubscriptionID: "SUB123",
					ResourceGroup:  "RG123",
					Name:           "NAME123",
					Gallery:        "GALLERY1",
					Version:        "1.0.0",
				},
			},
			Identity:               VMIdentityUserAssigned,
			UserAssignedIdentities: identitiesList,
		},
	}
}
