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

func TestAzureManagedControlPlane_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		r *AzureManagedControlPlane
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{r: createAzureManagedControlPlaneWithSSHPublicKey(t, existingPublicKey)}
	publicKeyNotExistTest := test{r: createAzureManagedControlPlaneWithSSHPublicKey(t, "")}

	err := publicKeyExistTest.r.setDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyExistTest.r.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	err = publicKeyNotExistTest.r.setDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyNotExistTest.r.Spec.SSHPublicKey).NotTo(BeEmpty())
}

func createAzureManagedControlPlaneWithSSHPublicKey(t *testing.T, sshPublicKey string) *AzureManagedControlPlane {
	return hardcodedAzureManagedControlPlaneWithSSHKey(sshPublicKey)
}

func hardcodedAzureManagedControlPlaneWithSSHKey(sshPublicKey string) *AzureManagedControlPlane {
	return &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			SSHPublicKey: sshPublicKey,
		},
	}
}
