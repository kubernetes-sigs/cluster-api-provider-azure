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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

func TestAzureMachine_ValidateSSHKey(t *testing.T) {
	tests := []struct {
		name    string
		sshKey  string
		wantErr bool
	}{
		{
			name:    "valid ssh key",
			sshKey:  generateSSHPublicKey(true),
			wantErr: false,
		},
		{
			name:    "invalid ssh key",
			sshKey:  "invalid ssh key",
			wantErr: true,
		},
		{
			name:    "ssh key not base64 encoded",
			sshKey:  generateSSHPublicKey(false),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateSSHKey(tc.sshKey, field.NewPath("sshPublicKey"))
			if tc.wantErr {
				g.Expect(err).NotTo(BeEmpty())
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
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

type osDiskTestInput struct {
	name    string
	wantErr bool
	osDisk  OSDisk
}

func TestAzureMachine_ValidateOSDisk(t *testing.T) {
	testcases := []osDiskTestInput{
		{
			name:    "valid os disk spec",
			wantErr: false,
			osDisk:  generateValidOSDisk(),
		},
		{
			name:    "invalid os disk cache type",
			wantErr: true,
			osDisk:  createOSDiskWithCacheType("invalid_cache_type"),
		},
		{
			name:    "valid ephemeral os disk spec",
			wantErr: false,
			osDisk: OSDisk{
				DiskSizeGB:  ptr.To[int32](30),
				CachingType: "None",
				OSType:      "blah",
				DiffDiskSettings: &DiffDiskSettings{
					Option: string(armcompute.DiffDiskOptionsLocal),
				},
				ManagedDisk: &ManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
				},
			},
		},
		{
			name:    "byoc encryption with ephemeral os disk spec",
			wantErr: true,
			osDisk: OSDisk{
				DiskSizeGB:  ptr.To[int32](30),
				CachingType: "None",
				OSType:      "blah",
				DiffDiskSettings: &DiffDiskSettings{
					Option: string(armcompute.DiffDiskOptionsLocal),
				},
				ManagedDisk: &ManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
					DiskEncryptionSet: &DiskEncryptionSetParameters{
						ID: "disk-encryption-set",
					},
				},
			},
		},
	}
	testcases = append(testcases, generateNegativeTestCases()...)

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateOSDisk(test.osDisk, field.NewPath("osDisk"))
			if test.wantErr {
				g.Expect(err).NotTo(BeEmpty())
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func generateNegativeTestCases() []osDiskTestInput {
	inputs := []osDiskTestInput{}
	testCaseName := "invalid os disk spec"

	invalidDiskSpecs := []OSDisk{
		{},
		{
			DiskSizeGB: ptr.To[int32](0),
			OSType:     "blah",
		},
		{
			DiskSizeGB: ptr.To[int32](-10),
			OSType:     "blah",
		},
		{
			DiskSizeGB: ptr.To[int32](2050),
			OSType:     "blah",
		},
		{
			DiskSizeGB: ptr.To[int32](20),
			OSType:     "",
		},
		{
			DiskSizeGB:  ptr.To[int32](30),
			OSType:      "blah",
			ManagedDisk: &ManagedDiskParameters{},
		},
		{
			DiskSizeGB: ptr.To[int32](30),
			OSType:     "blah",
			ManagedDisk: &ManagedDiskParameters{
				StorageAccountType: "",
			},
		},
		{
			DiskSizeGB: ptr.To[int32](30),
			OSType:     "blah",
			ManagedDisk: &ManagedDiskParameters{
				StorageAccountType: "invalid_type",
			},
		},
		{
			DiskSizeGB: ptr.To[int32](30),
			OSType:     "blah",
			ManagedDisk: &ManagedDiskParameters{
				StorageAccountType: "Premium_LRS",
			},
			DiffDiskSettings: &DiffDiskSettings{
				Option: string(armcompute.DiffDiskOptionsLocal),
			},
		},
	}

	for i, input := range invalidDiskSpecs {
		inputs = append(inputs, osDiskTestInput{
			name:    fmt.Sprintf("%s-%d", testCaseName, i),
			wantErr: true,
			osDisk:  input,
		})
	}

	return inputs
}

func generateValidOSDisk() OSDisk {
	return OSDisk{
		DiskSizeGB: ptr.To[int32](30),
		OSType:     LinuxOS,
		ManagedDisk: &ManagedDiskParameters{
			StorageAccountType: "Premium_LRS",
		},
		CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
	}
}

func createOSDiskWithCacheType(cacheType string) OSDisk {
	osDisk := generateValidOSDisk()
	osDisk.CachingType = cacheType
	return osDisk
}

func TestAzureMachine_ValidateDataDisks(t *testing.T) {
	testcases := []struct {
		name    string
		disks   []DataDisk
		wantErr bool
	}{
		{
			name:    "valid nil data disks",
			disks:   nil,
			wantErr: false,
		},
		{
			name:    "valid empty data disks",
			disks:   []DataDisk{},
			wantErr: false,
		},
		{
			name: "valid disks",
			disks: []DataDisk{
				{
					NameSuffix:  "my_disk",
					DiskSizeGB:  64,
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix:  "my_other_disk",
					DiskSizeGB:  64,
					Lun:         ptr.To[int32](1),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate names",
			disks: []DataDisk{
				{
					NameSuffix:  "disk",
					DiskSizeGB:  64,
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix:  "disk",
					DiskSizeGB:  64,
					Lun:         ptr.To[int32](1),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate LUNs",
			disks: []DataDisk{
				{
					NameSuffix:  "my_disk",
					DiskSizeGB:  64,
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix:  "my_other_disk",
					DiskSizeGB:  64,
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid disk size",
			disks: []DataDisk{
				{
					NameSuffix:  "my_disk",
					DiskSizeGB:  0,
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
		{
			name: "empty name",
			disks: []DataDisk{
				{
					NameSuffix:  "",
					DiskSizeGB:  0,
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid disk cachingType",
			disks: []DataDisk{
				{
					NameSuffix:  "my_disk",
					DiskSizeGB:  64,
					Lun:         ptr.To[int32](0),
					CachingType: "invalidCacheType",
				},
			},
			wantErr: true,
		},
		{
			name: "valid disk cachingType",
			disks: []DataDisk{
				{
					NameSuffix:  "my_disk",
					DiskSizeGB:  64,
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: false,
		},
		{
			name: "valid managed disk storage account type",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_disk_2",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					Lun:         ptr.To[int32](1),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid managed disk storage account type",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "invalid storage account",
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
		{
			name: "valid combination of managed disk storage account type UltraSSD_LRS and cachingType None",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesUltraSSDLRS),
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.CachingTypesNone),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid combination of managed disk storage account type UltraSSD_LRS and cachingType ReadWrite",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesUltraSSDLRS),
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.CachingTypesReadWrite),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid combination of managed disk storage account type UltraSSD_LRS and cachingType ReadOnly",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: string(armcompute.StorageAccountTypesUltraSSDLRS),
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.CachingTypesReadOnly),
				},
			},
			wantErr: true,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateDataDisks(test.disks, field.NewPath("dataDisks"))
			if test.wantErr {
				g.Expect(err).NotTo(BeEmpty())
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestAzureMachine_ValidateSystemAssignedIdentity(t *testing.T) {
	tests := []struct {
		name               string
		roleAssignmentName string
		old                string
		Identity           VMIdentity
		wantErr            bool
	}{
		{
			name:               "valid UUID",
			roleAssignmentName: uuid.New().String(),
			Identity:           VMIdentitySystemAssigned,
			wantErr:            false,
		},
		{
			name:               "wrong Identity type",
			roleAssignmentName: uuid.New().String(),
			Identity:           VMIdentityNone,
			wantErr:            true,
		},
		{
			name:               "not a valid UUID",
			roleAssignmentName: "notaguid",
			Identity:           VMIdentitySystemAssigned,
			wantErr:            true,
		},
		{
			name:               "empty",
			roleAssignmentName: "",
			Identity:           VMIdentitySystemAssigned,
			wantErr:            true,
		},
		{
			name:               "changed",
			roleAssignmentName: uuid.New().String(),
			old:                uuid.New().String(),
			Identity:           VMIdentitySystemAssigned,
			wantErr:            true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateSystemAssignedIdentity(tc.Identity, tc.old, tc.roleAssignmentName, field.NewPath("sshPublicKey"))
			if tc.wantErr {
				g.Expect(err).NotTo(BeEmpty())
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestAzureMachine_ValidateSystemAssignedIdentityRole(t *testing.T) {
	tests := []struct {
		name               string
		Identity           VMIdentity
		roleAssignmentName string
		role               *SystemAssignedIdentityRole
		wantErr            bool
	}{
		{
			name:     "valid role",
			Identity: VMIdentitySystemAssigned,
			role: &SystemAssignedIdentityRole{
				Name:         uuid.New().String(),
				Scope:        "fake-scope",
				DefinitionID: "fake-definition-id",
			},
		},
		{
			name:               "valid role using deprecated role assignment name",
			Identity:           VMIdentitySystemAssigned,
			roleAssignmentName: uuid.New().String(),
			role: &SystemAssignedIdentityRole{
				Scope:        "fake-scope",
				DefinitionID: "fake-definition-id",
			},
		},
		{
			name:               "set both system assigned identity role and role assignment name",
			Identity:           VMIdentitySystemAssigned,
			roleAssignmentName: uuid.New().String(),
			role: &SystemAssignedIdentityRole{
				Name:         uuid.New().String(),
				Scope:        "fake-scope",
				DefinitionID: "fake-definition-id",
			},
			wantErr: true,
		},
		{
			name:     "wrong Identity type",
			Identity: VMIdentityUserAssigned,
			role: &SystemAssignedIdentityRole{
				Name:         uuid.New().String(),
				Scope:        "fake-scope",
				DefinitionID: "fake-definition-id",
			},
			wantErr: true,
		},
		{
			name:     "missing scope",
			Identity: VMIdentitySystemAssigned,
			role: &SystemAssignedIdentityRole{
				Name:         uuid.New().String(),
				DefinitionID: "fake-definition-id",
			},
			wantErr: true,
		},
		{
			name:     "missing definition id",
			Identity: VMIdentitySystemAssigned,
			role: &SystemAssignedIdentityRole{
				Name:  uuid.New().String(),
				Scope: "fake-scope",
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateSystemAssignedIdentityRole(tc.Identity, tc.roleAssignmentName, tc.role, field.NewPath("systemAssignedIdentityRole"))
			if tc.wantErr {
				g.Expect(err).NotTo(BeEmpty())
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestAzureMachine_ValidateUserAssignedIdentity(t *testing.T) {
	tests := []struct {
		name       string
		idType     VMIdentity
		identities []UserAssignedIdentity
		wantErr    bool
	}{
		{
			name:       "empty identity list",
			idType:     VMIdentityUserAssigned,
			identities: []UserAssignedIdentity{},
			wantErr:    true,
		},
		{
			name:   "invalid: providerID must start with slash",
			idType: VMIdentityUserAssigned,
			identities: []UserAssignedIdentity{
				{
					ProviderID: "subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-20202-control-plane-7w265",
				},
			},
			wantErr: true,
		},
		{
			name:   "invalid: providerID must start with subscriptions or providers",
			idType: VMIdentityUserAssigned,
			identities: []UserAssignedIdentity{
				{
					ProviderID: "azure:///prescriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-20202-control-plane-7w265",
				},
			},
			wantErr: true,
		},
		{
			name:   "valid",
			idType: VMIdentityUserAssigned,
			identities: []UserAssignedIdentity{
				{
					ProviderID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-20202-control-plane-7w265",
				},
			},
			wantErr: false,
		},
		{
			name:   "valid with provider prefix",
			idType: VMIdentityUserAssigned,
			identities: []UserAssignedIdentity{
				{
					ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-20202-control-plane-7w265",
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			errs := ValidateUserAssignedIdentity(tc.idType, tc.identities, field.NewPath("userAssignedIdentities"))
			if tc.wantErr {
				g.Expect(errs).NotTo(BeEmpty())
			} else {
				g.Expect(errs).To(BeEmpty())
			}
		})
	}
}

func TestAzureMachine_ValidateDataDisksUpdate(t *testing.T) {
	tests := []struct {
		name     string
		disks    []DataDisk
		oldDisks []DataDisk
		wantErr  bool
	}{
		{
			name:     "valid nil data disks",
			disks:    nil,
			oldDisks: nil,
			wantErr:  false,
		},
		{
			name:     "valid empty data disks",
			disks:    []DataDisk{},
			oldDisks: []DataDisk{},
			wantErr:  false,
		},
		{
			name: "valid data disk updates",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk",
					DiskSizeGB: 64,
					Lun:        ptr.To[int32](0),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_other_disk",
					DiskSizeGB: 64,
					Lun:        ptr.To[int32](1),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix: "my_disk",
					DiskSizeGB: 64,
					Lun:        ptr.To[int32](0),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_other_disk",
					DiskSizeGB: 64,
					Lun:        ptr.To[int32](1),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: false,
		},
		{
			name: "cannot update data disk fields after machine creation",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 128,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
		{
			name: "validate updates to optional fields",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 128,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					Lun: ptr.To[int32](0),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix:  "my_disk_1",
					DiskSizeGB:  128,
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
		{
			name: "data disks cannot be added after machine creation",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_disk_2",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					Lun:         ptr.To[int32](2),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
		{
			name: "data disks cannot be removed after machine creation",
			disks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_disk_2",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					Lun:         ptr.To[int32](2),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					Lun:         ptr.To[int32](0),
					CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateDataDisksUpdate(test.oldDisks, test.disks, field.NewPath("dataDisks"))
			if test.wantErr {
				g.Expect(err).NotTo(BeEmpty())
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestAzureMachine_ValidateNetwork(t *testing.T) {
	tests := []struct {
		name                  string
		subnetName            string
		acceleratedNetworking *bool
		networkInterfaces     []NetworkInterface
		wantErr               bool
	}{
		{
			name:                  "valid config with deprecated network fields",
			subnetName:            "subnet1",
			acceleratedNetworking: ptr.To(true),
			networkInterfaces:     nil,
			wantErr:               false,
		},
		{
			name:                  "valid config with networkInterfaces fields",
			subnetName:            "",
			acceleratedNetworking: nil,
			networkInterfaces: []NetworkInterface{{
				SubnetName:            "subnet1",
				AcceleratedNetworking: ptr.To(true),
				PrivateIPConfigs:      1,
			}},
			wantErr: false,
		},
		{
			name:                  "valid config with multiple networkInterfaces",
			subnetName:            "",
			acceleratedNetworking: nil,
			networkInterfaces: []NetworkInterface{
				{
					SubnetName:            "subnet1",
					AcceleratedNetworking: ptr.To(true),
					PrivateIPConfigs:      1,
				},
				{
					SubnetName:            "subnet2",
					AcceleratedNetworking: ptr.To(true),
					PrivateIPConfigs:      30,
				},
			},
			wantErr: false,
		},
		{
			name:                  "invalid config using both deprecated subnetName and networkInterfaces",
			subnetName:            "subnet1",
			acceleratedNetworking: nil,
			networkInterfaces: []NetworkInterface{{
				SubnetName:            "subnet1",
				AcceleratedNetworking: nil,
				PrivateIPConfigs:      1,
			}},
			wantErr: true,
		},
		{
			name:                  "invalid config using both deprecated acceleratedNetworking and networkInterfaces",
			subnetName:            "",
			acceleratedNetworking: ptr.To(true),
			networkInterfaces: []NetworkInterface{{
				SubnetName:            "subnet1",
				AcceleratedNetworking: ptr.To(true),
				PrivateIPConfigs:      1,
			}},
			wantErr: true,
		},
		{
			name:                  "invalid config setting privateIPConfigs to less than 1",
			subnetName:            "",
			acceleratedNetworking: nil,
			networkInterfaces: []NetworkInterface{{
				SubnetName:            "subnet1",
				AcceleratedNetworking: ptr.To(true),
				PrivateIPConfigs:      0,
			}},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateNetwork(test.subnetName, test.acceleratedNetworking, test.networkInterfaces, field.NewPath("networkInterfaces"))
			if test.wantErr {
				g.Expect(err).ToNot(BeEmpty())
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}

func TestAzureMachine_ValidateConfidentialCompute(t *testing.T) {
	tests := []struct {
		name            string
		managedDisk     *ManagedDiskParameters
		securityProfile *SecurityProfile
		wantErr         bool
	}{
		{
			name: "valid configuration without confidential compute",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: "",
				},
			},
			securityProfile: nil,
			wantErr:         false,
		},
		{
			name: "valid configuration without confidential compute and host encryption enabled",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: "",
				},
			},
			securityProfile: &SecurityProfile{
				EncryptionAtHost: ptr.To(true),
			},
			wantErr: false,
		},
		{
			name: "valid configuration with VMGuestStateOnly encryption and secure boot disabled",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: SecurityEncryptionTypeVMGuestStateOnly,
				},
			},
			securityProfile: &SecurityProfile{
				SecurityType: SecurityTypesConfidentialVM,
				UefiSettings: &UefiSettings{
					VTpmEnabled:       ptr.To(true),
					SecureBootEnabled: ptr.To(false),
				},
			},
			wantErr: false,
		},
		{
			name: "valid configuration with VMGuestStateOnly encryption and secure boot enabled",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: SecurityEncryptionTypeVMGuestStateOnly,
				},
			},
			securityProfile: &SecurityProfile{
				SecurityType: SecurityTypesConfidentialVM,
				UefiSettings: &UefiSettings{
					VTpmEnabled:       ptr.To(true),
					SecureBootEnabled: ptr.To(true),
				},
			},
			wantErr: false,
		},
		{
			name: "valid configuration with VMGuestStateOnly encryption and EncryptionAtHost enabled",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: SecurityEncryptionTypeVMGuestStateOnly,
				},
			},
			securityProfile: &SecurityProfile{
				EncryptionAtHost: ptr.To(true),
				SecurityType:     SecurityTypesConfidentialVM,
				UefiSettings: &UefiSettings{
					VTpmEnabled: ptr.To(true),
				},
			},
			wantErr: false,
		},
		{
			name: "valid configuration with DiskWithVMGuestState encryption",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: SecurityEncryptionTypeDiskWithVMGuestState,
				},
			},
			securityProfile: &SecurityProfile{
				SecurityType: SecurityTypesConfidentialVM,
				UefiSettings: &UefiSettings{
					SecureBootEnabled: ptr.To(true),
					VTpmEnabled:       ptr.To(true),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid configuration with DiskWithVMGuestState encryption and EncryptionAtHost enabled",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: SecurityEncryptionTypeDiskWithVMGuestState,
				},
			},
			securityProfile: &SecurityProfile{
				EncryptionAtHost: ptr.To(true),
			},
			wantErr: true,
		},
		{
			name: "invalid configuration with DiskWithVMGuestState encryption and vTPM disabled",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: SecurityEncryptionTypeDiskWithVMGuestState,
				},
			},
			securityProfile: &SecurityProfile{
				SecurityType: SecurityTypesConfidentialVM,
				UefiSettings: &UefiSettings{
					VTpmEnabled:       ptr.To(false),
					SecureBootEnabled: ptr.To(false),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid configuration with DiskWithVMGuestState encryption and secure boot disabled",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: SecurityEncryptionTypeDiskWithVMGuestState,
				},
			},
			securityProfile: &SecurityProfile{
				SecurityType: SecurityTypesConfidentialVM,
				UefiSettings: &UefiSettings{
					VTpmEnabled:       ptr.To(true),
					SecureBootEnabled: ptr.To(false),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid configuration with DiskWithVMGuestState encryption and SecurityType not set to ConfidentialVM",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: SecurityEncryptionTypeDiskWithVMGuestState,
				},
			},
			securityProfile: &SecurityProfile{
				UefiSettings: &UefiSettings{
					VTpmEnabled:       ptr.To(true),
					SecureBootEnabled: ptr.To(true),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid configuration with VMGuestStateOnly encryption and SecurityType not set to ConfidentialVM",
			managedDisk: &ManagedDiskParameters{
				SecurityProfile: &VMDiskSecurityProfile{
					SecurityEncryptionType: SecurityEncryptionTypeVMGuestStateOnly,
				},
			},
			securityProfile: &SecurityProfile{
				UefiSettings: &UefiSettings{
					VTpmEnabled:       ptr.To(true),
					SecureBootEnabled: ptr.To(true),
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateConfidentialCompute(tc.managedDisk, tc.securityProfile, field.NewPath("securityProfile"))
			if tc.wantErr {
				g.Expect(err).NotTo(BeEmpty())
			} else {
				g.Expect(err).To(BeEmpty())
			}
		})
	}
}
