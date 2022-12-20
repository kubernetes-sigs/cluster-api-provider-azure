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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestAzureManagedControlPlane_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		m *AzureManagedControlPlane
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{m: createAzureManagedControlPlaneWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{m: createAzureManagedControlPlaneWithSSHPublicKey("")}

	err := publicKeyExistTest.m.setDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyExistTest.m.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	err = publicKeyNotExistTest.m.setDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyNotExistTest.m.Spec.SSHPublicKey).NotTo(BeEmpty())
}

func createAzureManagedControlPlaneWithSSHPublicKey(sshPublicKey string) *AzureManagedControlPlane {
	return hardcodedAzureManagedControlPlaneWithSSHKey(sshPublicKey)
}

func hardcodedAzureManagedControlPlaneWithSSHKey(sshPublicKey string) *AzureManagedControlPlane {
	return &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			SSHPublicKey: sshPublicKey,
		},
	}
}
