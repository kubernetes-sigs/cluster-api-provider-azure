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
	"k8s.io/utils/pointer"
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

func TestAzureMachinePool_SetDiagnosticsDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machinePool *AzureMachinePool
	}

	managedStorageDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.ManagedDiagnosticsStorage,
					},
				},
			},
		},
	}}

	disabledStorageDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.DisabledDiagnosticsStorage,
					},
				},
			},
		},
	}}

	userManagedDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.UserManagedDiagnosticsStorage,
						UserManaged: &infrav1.UserManagedBootDiagnostics{
							StorageAccountURI: "https://fakeurl",
						},
					},
				},
			},
		},
	}}

	nilDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: nil,
			},
		},
	}}

	managedStorageDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(managedStorageDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.ManagedDiagnosticsStorage))

	disabledStorageDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(disabledStorageDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.DisabledDiagnosticsStorage))

	userManagedDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(userManagedDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.UserManagedDiagnosticsStorage))

	nilDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(nilDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.ManagedDiagnosticsStorage))
}

func TestAzureMachinePool_SetNetworkInterfacesDefaults(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name        string
		machinePool *AzureMachinePool
		want        *AzureMachinePool
	}{
		{
			name: "defaulting webhook updates MachinePool with deprecated subnetName field",
			machinePool: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
						SubnetName: "test-subnet",
					},
				},
			},
			want: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
						SubnetName: "",
						NetworkInterfaces: []infrav1.NetworkInterface{
							{
								SubnetName:       "test-subnet",
								PrivateIPConfigs: 1,
							},
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook updates MachinePool with deprecated acceleratedNetworking field",
			machinePool: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
						SubnetName:            "test-subnet",
						AcceleratedNetworking: pointer.Bool(true),
					},
				},
			},
			want: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
						SubnetName:            "",
						AcceleratedNetworking: nil,
						NetworkInterfaces: []infrav1.NetworkInterface{
							{
								SubnetName:            "test-subnet",
								PrivateIPConfigs:      1,
								AcceleratedNetworking: pointer.Bool(true),
							},
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook does nothing if both new and deprecated subnetName fields are set",
			machinePool: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
						SubnetName: "test-subnet",
						NetworkInterfaces: []infrav1.NetworkInterface{{
							SubnetName: "test-subnet",
						}},
					},
				},
			},
			want: &AzureMachinePool{
				Spec: AzureMachinePoolSpec{
					Template: AzureMachinePoolMachineTemplate{
						SubnetName:            "test-subnet",
						AcceleratedNetworking: nil,
						NetworkInterfaces: []infrav1.NetworkInterface{
							{
								SubnetName: "test-subnet",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.machinePool.SetNetworkInterfacesDefaults()
			g.Expect(tc.machinePool).To(Equal(tc.want))
		})
	}
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
