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
	"fmt"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

	fakeSubscriptionID := uuid.New().String()
	fakeClusterName := "testcluster"
	fakeRoleDefinitionID := "testroledefinitionid"
	fakeScope := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", fakeSubscriptionID, fakeClusterName)
	existingRoleAssignmentName := "42862306-e485-4319-9bf0-35dbc6f6fe9c"
	roleAssignmentExistTest := test{machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity: infrav1.VMIdentitySystemAssigned,
		SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
			Name: existingRoleAssignmentName,
		},
	}}}
	notSystemAssignedTest := test{machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity: infrav1.VMIdentityUserAssigned,
	}}}
	systemAssignedIdentityRoleExistTest := test{machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity: infrav1.VMIdentitySystemAssigned,
		SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
			DefinitionID: fakeRoleDefinitionID,
			Scope:        fakeScope,
		},
	}}}
	deprecatedRoleAssignmentNameTest := test{machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity:           infrav1.VMIdentitySystemAssigned,
		RoleAssignmentName: existingRoleAssignmentName,
	}}}
	emptyTest := test{machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity:                   infrav1.VMIdentitySystemAssigned,
		SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{},
	}}}

	bothRoleAssignmentNamesPopulatedTest := test{machinePool: &AzureMachinePool{Spec: AzureMachinePoolSpec{
		Identity:           infrav1.VMIdentitySystemAssigned,
		RoleAssignmentName: existingRoleAssignmentName,
		SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
			Name: existingRoleAssignmentName,
		},
	}}}

	bothRoleAssignmentNamesPopulatedTest.machinePool.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(bothRoleAssignmentNamesPopulatedTest.machinePool.Spec.RoleAssignmentName).To(Equal(existingRoleAssignmentName))
	g.Expect(bothRoleAssignmentNamesPopulatedTest.machinePool.Spec.SystemAssignedIdentityRole.Name).To(Equal(existingRoleAssignmentName))

	roleAssignmentExistTest.machinePool.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(roleAssignmentExistTest.machinePool.Spec.SystemAssignedIdentityRole.Name).To(Equal(existingRoleAssignmentName))

	notSystemAssignedTest.machinePool.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(notSystemAssignedTest.machinePool.Spec.SystemAssignedIdentityRole).To(BeNil())

	systemAssignedIdentityRoleExistTest.machinePool.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(systemAssignedIdentityRoleExistTest.machinePool.Spec.SystemAssignedIdentityRole.Scope).To(Equal(fakeScope))
	g.Expect(systemAssignedIdentityRoleExistTest.machinePool.Spec.SystemAssignedIdentityRole.DefinitionID).To(Equal(fakeRoleDefinitionID))

	deprecatedRoleAssignmentNameTest.machinePool.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(deprecatedRoleAssignmentNameTest.machinePool.Spec.SystemAssignedIdentityRole.Name).To(Equal(existingRoleAssignmentName))
	g.Expect(deprecatedRoleAssignmentNameTest.machinePool.Spec.RoleAssignmentName).To(BeEmpty())

	emptyTest.machinePool.SetIdentityDefaults(fakeSubscriptionID)
	g.Expect(emptyTest.machinePool.Spec.SystemAssignedIdentityRole.Name).To(Not(BeEmpty()))
	_, err := uuid.Parse(emptyTest.machinePool.Spec.SystemAssignedIdentityRole.Name)
	g.Expect(err).To(Not(HaveOccurred()))
	g.Expect(emptyTest.machinePool.Spec.SystemAssignedIdentityRole).To(Not(BeNil()))
	g.Expect(emptyTest.machinePool.Spec.SystemAssignedIdentityRole.Scope).To(Equal(fmt.Sprintf("/subscriptions/%s/", fakeSubscriptionID)))
	g.Expect(emptyTest.machinePool.Spec.SystemAssignedIdentityRole.DefinitionID).To(Equal(fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", fakeSubscriptionID, infrav1.ContributorRoleID)))
}

func TestAzureMachinePool_SetDiagnosticsDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machinePool *AzureMachinePool
	}

	bootDiagnosticsDefault := &infrav1.BootDiagnostics{
		StorageAccountType: infrav1.ManagedDiagnosticsStorage,
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

	// Test that when no diagnostics are specified, the defaults are set correctly
	nilBootDiagnostics := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				Diagnostics: &infrav1.Diagnostics{},
			},
		},
	}}

	nilBootDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(nilBootDiagnostics.machinePool.Spec.Template.Diagnostics.Boot).To(Equal(bootDiagnosticsDefault))

	managedStorageDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(managedStorageDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.ManagedDiagnosticsStorage))

	disabledStorageDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(disabledStorageDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.DisabledDiagnosticsStorage))

	userManagedDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(userManagedDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.UserManagedDiagnosticsStorage))

	nilDiagnostics.machinePool.SetDiagnosticsDefaults()
	g.Expect(nilDiagnostics.machinePool.Spec.Template.Diagnostics.Boot.StorageAccountType).To(Equal(infrav1.ManagedDiagnosticsStorage))
}

func TestAzureMachinePool_SetSpotEvictionPolicyDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machinePool *AzureMachinePool
	}

	// test to Ensure the the default policy is set to Deallocate if EvictionPolicy is nil
	defaultEvictionPolicy := infrav1.SpotEvictionPolicyDeallocate
	nilDiffDiskSettingsPolicy := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				SpotVMOptions: &infrav1.SpotVMOptions{
					EvictionPolicy: nil,
				},
			},
		},
	}}
	nilDiffDiskSettingsPolicy.machinePool.SetSpotEvictionPolicyDefaults()
	g.Expect(nilDiffDiskSettingsPolicy.machinePool.Spec.Template.SpotVMOptions.EvictionPolicy).To(Equal(&defaultEvictionPolicy))

	// test to Ensure the the default policy is set to Delete if diffDiskSettings option is set to "Local"
	expectedEvictionPolicy := infrav1.SpotEvictionPolicyDelete
	diffDiskSettingsPolicy := test{machinePool: &AzureMachinePool{
		Spec: AzureMachinePoolSpec{
			Template: AzureMachinePoolMachineTemplate{
				SpotVMOptions: &infrav1.SpotVMOptions{},
				OSDisk: infrav1.OSDisk{
					DiffDiskSettings: &infrav1.DiffDiskSettings{
						Option: "Local",
					},
				},
			},
		},
	}}
	diffDiskSettingsPolicy.machinePool.SetSpotEvictionPolicyDefaults()
	g.Expect(diffDiskSettingsPolicy.machinePool.Spec.Template.SpotVMOptions.EvictionPolicy).To(Equal(&expectedEvictionPolicy))
}

func TestAzureMachinePool_SetNetworkInterfacesDefaults(t *testing.T) {
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
						AcceleratedNetworking: ptr.To(true),
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
								AcceleratedNetworking: ptr.To(true),
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
			g := NewWithT(t)
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
		ObjectMeta: metav1.ObjectMeta{
			Name: "testmachinepool",
		},
	}
}
