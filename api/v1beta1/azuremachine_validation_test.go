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

	"github.com/google/uuid"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/Azure/go-autorest/autorest/to"

	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestAzureMachine_ValidateSSHKey(t *testing.T) {
	g := NewWithT(t)

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
			err := ValidateSSHKey(tc.sshKey, field.NewPath("sshPublicKey"))
			if tc.wantErr {
				g.Expect(err).ToNot(HaveLen(0))
			} else {
				g.Expect(err).To(HaveLen(0))
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
	g := NewWithT(t)

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
				DiskSizeGB:  to.Int32Ptr(30),
				CachingType: "None",
				OSType:      "blah",
				DiffDiskSettings: &DiffDiskSettings{
					Option: string(compute.DiffDiskOptionsLocal),
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
				DiskSizeGB:  to.Int32Ptr(30),
				CachingType: "None",
				OSType:      "blah",
				DiffDiskSettings: &DiffDiskSettings{
					Option: string(compute.DiffDiskOptionsLocal),
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
			err := ValidateOSDisk(test.osDisk, field.NewPath("osDisk"))
			if test.wantErr {
				g.Expect(err).NotTo(HaveLen(0))
			} else {
				g.Expect(err).To(HaveLen(0))
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
			DiskSizeGB: to.Int32Ptr(0),
			OSType:     "blah",
		},
		{
			DiskSizeGB: to.Int32Ptr(-10),
			OSType:     "blah",
		},
		{
			DiskSizeGB: to.Int32Ptr(2050),
			OSType:     "blah",
		},
		{
			DiskSizeGB: to.Int32Ptr(20),
			OSType:     "",
		},
		{
			DiskSizeGB:  to.Int32Ptr(30),
			OSType:      "blah",
			ManagedDisk: &ManagedDiskParameters{},
		},
		{
			DiskSizeGB: to.Int32Ptr(30),
			OSType:     "blah",
			ManagedDisk: &ManagedDiskParameters{
				StorageAccountType: "",
			},
		},
		{
			DiskSizeGB: to.Int32Ptr(30),
			OSType:     "blah",
			ManagedDisk: &ManagedDiskParameters{
				StorageAccountType: "invalid_type",
			},
		},
		{
			DiskSizeGB: to.Int32Ptr(30),
			OSType:     "blah",
			ManagedDisk: &ManagedDiskParameters{
				StorageAccountType: "Premium_LRS",
			},
			DiffDiskSettings: &DiffDiskSettings{
				Option: string(compute.DiffDiskOptionsLocal),
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
		DiskSizeGB: to.Int32Ptr(30),
		OSType:     "Linux",
		ManagedDisk: &ManagedDiskParameters{
			StorageAccountType: "Premium_LRS",
		},
		CachingType: string(compute.PossibleCachingTypesValues()[0]),
	}
}

func createOSDiskWithCacheType(cacheType string) OSDisk {
	osDisk := generateValidOSDisk()
	osDisk.CachingType = cacheType
	return osDisk
}

func TestAzureMachine_ValidateDataDisks(t *testing.T) {
	g := NewWithT(t)

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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix:  "my_other_disk",
					DiskSizeGB:  64,
					Lun:         to.Int32Ptr(1),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix:  "disk",
					DiskSizeGB:  64,
					Lun:         to.Int32Ptr(1),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix:  "my_other_disk",
					DiskSizeGB:  64,
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_disk_2",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					Lun:         to.Int32Ptr(1),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateDataDisks(test.disks, field.NewPath("dataDisks"))
			if test.wantErr {
				g.Expect(err).NotTo(HaveLen(0))
			} else {
				g.Expect(err).To(HaveLen(0))
			}
		})
	}
}

func TestAzureMachine_ValidateSystemAssignedIdentity(t *testing.T) {
	g := NewWithT(t)

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
			err := ValidateSystemAssignedIdentity(tc.Identity, tc.old, tc.roleAssignmentName, field.NewPath("sshPublicKey"))
			if tc.wantErr {
				g.Expect(err).ToNot(HaveLen(0))
			} else {
				g.Expect(err).To(HaveLen(0))
			}
		})
	}
}

func TestAzureMachine_ValidateDataDisksUpdate(t *testing.T) {
	g := NewWithT(t)

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
					Lun:        to.Int32Ptr(0),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_other_disk",
					DiskSizeGB: 64,
					Lun:        to.Int32Ptr(1),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix: "my_disk",
					DiskSizeGB: 64,
					Lun:        to.Int32Ptr(0),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_other_disk",
					DiskSizeGB: 64,
					Lun:        to.Int32Ptr(1),
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 128,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun: to.Int32Ptr(0),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix:  "my_disk_1",
					DiskSizeGB:  128,
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_disk_2",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					Lun:         to.Int32Ptr(2),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
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
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
				{
					NameSuffix: "my_disk_2",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Premium_LRS",
					},
					Lun:         to.Int32Ptr(2),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
			},
			oldDisks: []DataDisk{
				{
					NameSuffix: "my_disk_1",
					DiskSizeGB: 64,
					ManagedDisk: &ManagedDiskParameters{
						StorageAccountType: "Standard_LRS",
					},
					Lun:         to.Int32Ptr(0),
					CachingType: string(compute.PossibleCachingTypesValues()[0]),
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateDataDisksUpdate(test.oldDisks, test.disks, field.NewPath("dataDisks"))
			if test.wantErr {
				g.Expect(err).NotTo(HaveLen(0))
			} else {
				g.Expect(err).To(HaveLen(0))
			}
		})
	}
}
