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
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

func TestAzureMachineSpec_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machine *AzureMachine
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{machine: createMachineWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{machine: createMachineWithSSHPublicKey("")}

	err := publicKeyExistTest.machine.Spec.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyExistTest.machine.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	err = publicKeyNotExistTest.machine.Spec.SetDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyNotExistTest.machine.Spec.SSHPublicKey).To(Not(BeEmpty()))
}

func TestAzureMachineSpec_SetIdentityDefaults(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machine *AzureMachine
	}

	existingRoleAssignmentName := "42862306-e485-4319-9bf0-35dbc6f6fe9c"
	roleAssignmentExistTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		Identity:           VMIdentitySystemAssigned,
		RoleAssignmentName: existingRoleAssignmentName,
	}}}
	roleAssignmentEmptyTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		Identity:           VMIdentitySystemAssigned,
		RoleAssignmentName: "",
	}}}
	notSystemAssignedTest := test{machine: &AzureMachine{Spec: AzureMachineSpec{
		Identity: VMIdentityUserAssigned,
	}}}

	roleAssignmentExistTest.machine.Spec.SetIdentityDefaults()
	g.Expect(roleAssignmentExistTest.machine.Spec.RoleAssignmentName).To(Equal(existingRoleAssignmentName))

	roleAssignmentEmptyTest.machine.Spec.SetIdentityDefaults()
	g.Expect(roleAssignmentEmptyTest.machine.Spec.RoleAssignmentName).To(Not(BeEmpty()))
	_, err := uuid.Parse(roleAssignmentEmptyTest.machine.Spec.RoleAssignmentName)
	g.Expect(err).To(Not(HaveOccurred()))

	notSystemAssignedTest.machine.Spec.SetIdentityDefaults()
	g.Expect(notSystemAssignedTest.machine.Spec.RoleAssignmentName).To(BeEmpty())
}

func TestAzureMachineSpec_SetDataDisksDefaults(t *testing.T) {
	cases := []struct {
		name   string
		disks  []DataDisk
		output []DataDisk
	}{
		{
			name:   "no disks",
			disks:  []DataDisk{},
			output: []DataDisk{},
		},
		{
			name: "no LUNs specified",
			disks: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					CachingType: "ReadWrite",
				},
			},
			output: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(0),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(1),
					CachingType: "ReadWrite",
				},
			},
		},
		{
			name: "All LUNs specified",
			disks: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(5),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(3),
					CachingType: "ReadWrite",
				},
			},
			output: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(5),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(3),
					CachingType: "ReadWrite",
				},
			},
		},
		{
			name: "Some LUNs missing",
			disks: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(0),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk3",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(1),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk4",
					DiskSizeGB:  30,
					CachingType: "ReadWrite",
				},
			},
			output: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(0),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(2),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk3",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(1),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk4",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(3),
					CachingType: "ReadWrite",
				},
			},
		},
		{
			name: "CachingType unspecified",
			disks: []DataDisk{
				{
					NameSuffix: "testdisk1",
					DiskSizeGB: 30,
					Lun:        pointer.Int32(0),
				},
				{
					NameSuffix: "testdisk2",
					DiskSizeGB: 30,
					Lun:        pointer.Int32(2),
				},
				{
					NameSuffix: "testdisk3",
					DiskSizeGB: 30,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
					Lun: pointer.Int32(3),
				},
			},
			output: []DataDisk{
				{
					NameSuffix:  "testdisk1",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(0),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix:  "testdisk2",
					DiskSizeGB:  30,
					Lun:         pointer.Int32(2),
					CachingType: "ReadWrite",
				},
				{
					NameSuffix: "testdisk3",
					DiskSizeGB: 30,
					Lun:        pointer.Int32(3),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
					CachingType: "None",
				},
			},
		},
	}

	for _, c := range cases {
		tc := c
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			machine := hardcodedAzureMachineWithSSHKey(generateSSHPublicKey(true))
			machine.Spec.DataDisks = tc.disks
			machine.Spec.SetDataDisksDefaults()
			if !reflect.DeepEqual(machine.Spec.DataDisks, tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(machine.Spec.DataDisks, "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestAzureMachineSpec_SetNetworkInterfacesDefaults(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		machine *AzureMachine
		want    *AzureMachine
	}{
		{
			name: "defaulting webhook updates machine with deprecated subnetName field",
			machine: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName: "test-subnet",
				},
			},
			want: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName: "",
					NetworkInterfaces: []NetworkInterface{
						{
							SubnetName:       "test-subnet",
							PrivateIPConfigs: 1,
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook updates machine with deprecated subnetName field and empty NetworkInterfaces slice",
			machine: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName:        "test-subnet",
					NetworkInterfaces: []NetworkInterface{},
				},
			},
			want: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName: "",
					NetworkInterfaces: []NetworkInterface{
						{
							SubnetName:       "test-subnet",
							PrivateIPConfigs: 1,
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook updates machine with deprecated acceleratedNetworking field",
			machine: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName:            "test-subnet",
					AcceleratedNetworking: pointer.Bool(true),
				},
			},
			want: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName:            "",
					AcceleratedNetworking: nil,
					NetworkInterfaces: []NetworkInterface{
						{
							SubnetName:            "test-subnet",
							PrivateIPConfigs:      1,
							AcceleratedNetworking: pointer.Bool(true),
						},
					},
				},
			},
		},
		{
			name: "defaulting webhook does nothing if both new and deprecated subnetName fields are set",
			machine: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName: "test-subnet",
					NetworkInterfaces: []NetworkInterface{{
						SubnetName: "test-subnet",
					}},
				},
			},
			want: &AzureMachine{
				Spec: AzureMachineSpec{
					SubnetName:            "test-subnet",
					AcceleratedNetworking: nil,
					NetworkInterfaces: []NetworkInterface{
						{
							SubnetName: "test-subnet",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.machine.Spec.SetNetworkInterfacesDefaults()
			g.Expect(tc.machine).To(Equal(tc.want))
		})
	}
}

func createMachineWithSSHPublicKey(sshPublicKey string) *AzureMachine {
	machine := hardcodedAzureMachineWithSSHKey(sshPublicKey)
	return machine
}

func createMachineWithUserAssignedIdentities(identitiesList []UserAssignedIdentity) *AzureMachine {
	machine := hardcodedAzureMachineWithSSHKey(generateSSHPublicKey(true))
	machine.Spec.Identity = VMIdentityUserAssigned
	machine.Spec.UserAssignedIdentities = identitiesList
	return machine
}

func hardcodedAzureMachineWithSSHKey(sshPublicKey string) *AzureMachine {
	return &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey: sshPublicKey,
			OSDisk:       generateValidOSDisk(),
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
