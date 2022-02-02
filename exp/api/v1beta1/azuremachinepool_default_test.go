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

package v1beta1

import (
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestAzureMachinePool_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		amp *AzureMachinePool
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{amp: createMachinePoolWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{amp: createMachinePoolWithSSHPublicKey("")}

	err := publicKeyExistTest.amp.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyExistTest.amp.Spec.Template.SSHPublicKey).To(Equal(existingPublicKey))

	err = publicKeyNotExistTest.amp.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyNotExistTest.amp.Spec.Template.SSHPublicKey).NotTo(BeEmpty())
}

func TestAzureMachinePool_SetIdentityDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machinePool *AzureMachinePool
	}

	existingRoleAssignmentName := "42862306-e485-4319-9bf0-35dbc6f6fe9c"
	roleAssignmentExistTest := test{machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity:           infrav1.VMIdentitySystemAssigned,
		RoleAssignmentName: existingRoleAssignmentName,
	}}}
	roleAssignmentEmptyTest := test{machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity:           infrav1.VMIdentitySystemAssigned,
		RoleAssignmentName: "",
	}}}
	notSystemAssignedTest := test{machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity: infrav1.VMIdentityUserAssigned,
	}}}

	roleAssignmentExistTest.machinePool.SetIdentityDefaults()
	g.Expect(roleAssignmentExistTest.machinePool.Spec.RoleAssignmentName).To(Equal(existingRoleAssignmentName))

	roleAssignmentEmptyTest.machinePool.SetIdentityDefaults()
	g.Expect(roleAssignmentEmptyTest.machinePool.Spec.RoleAssignmentName).To(Not(BeEmpty()))
	_, err := uuid.Parse(roleAssignmentEmptyTest.machinePool.Spec.RoleAssignmentName)
	g.Expect(err).To(Not(HaveOccurred()))

	notSystemAssignedTest.machinePool.SetIdentityDefaults()
	g.Expect(notSystemAssignedTest.machinePool.Spec.RoleAssignmentName).To(BeEmpty())
}

func createMachinePoolWithSSHPublicKey(sshPublicKey string) *AzureMachinePool {
	return hardcodedAzureMachinePoolWithSSHKey(sshPublicKey)
}

func hardcodedAzureMachinePoolWithSSHKey(sshPublicKey string) *AzureMachinePool {
	return &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				SSHPublicKey: sshPublicKey,
			},
		},
	}
}
