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
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateSSHKey validates an SSHKey
func ValidateSSHKey(sshKey string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	decoded, err := base64.StdEncoding.DecodeString(sshKey)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, sshKey, "the SSH public key is not properly base64 encoded"))
		return allErrs
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey(decoded); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, sshKey, "the SSH public key is not valid"))
		return allErrs
	}

	return allErrs
}

// ValidateSystemAssignedIdentity validates the system-assigned identities list.
func ValidateSystemAssignedIdentity(identityType VMIdentity, old, new string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if identityType == VMIdentitySystemAssigned {
		if _, err := uuid.Parse(new); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, new, "Role assignment name must be a valid GUID. It is optional and will be auto-generated when not specified."))
		}
		if old != "" && old != new {
			allErrs = append(allErrs, field.Invalid(fldPath, new, "Role assignment name should not be modified after AzureMachine creation."))
		}
	} else if len(new) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath, "Role assignment name should only be set when using system assigned identity."))
	}

	return allErrs
}

// ValidateUserAssignedIdentity validates the user-assigned identities list
func ValidateUserAssignedIdentity(identityType VMIdentity, userAssignedIdenteties []UserAssignedIdentity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if identityType == VMIdentityUserAssigned && len(userAssignedIdenteties) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must be specified for the 'UserAssigned' identity type"))
	}
	return allErrs
}

// ValidateDataDisks validates a list of data disks
func ValidateDataDisks(dataDisks []DataDisk, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	lunSet := make(map[int32]struct{})
	nameSet := make(map[string]struct{})
	for _, disk := range dataDisks {
		// validate that the disk size is between 4 and 32767.
		if disk.DiskSizeGB < 4 || disk.DiskSizeGB > 32767 {
			allErrs = append(allErrs, field.Invalid(fieldPath.Child("DiskSizeGB"), "", "the disk size should be a value between 4 and 32767"))
		}

		// validate that all names are unique
		if disk.NameSuffix == "" {
			allErrs = append(allErrs, field.Required(fieldPath.Child("NameSuffix"), "the name suffix cannot be empty"))
		}
		if _, ok := nameSet[disk.NameSuffix]; ok {
			allErrs = append(allErrs, field.Duplicate(fieldPath, disk.NameSuffix))
		} else {
			nameSet[disk.NameSuffix] = struct{}{}
		}

		// validate that all LUNs are unique and between 0 and 63.
		if disk.Lun == nil {
			allErrs = append(allErrs, field.Required(fieldPath, "LUN should not be nil"))
		} else if *disk.Lun < 0 || *disk.Lun > 63 {
			allErrs = append(allErrs, field.Invalid(fieldPath, disk.Lun, "logical unit number must be between 0 and 63"))
		} else if _, ok := lunSet[*disk.Lun]; ok {
			allErrs = append(allErrs, field.Duplicate(fieldPath, disk.Lun))
		} else {
			lunSet[*disk.Lun] = struct{}{}
		}

		// validate cachingType
		allErrs = append(allErrs, validateCachingType(disk.CachingType, fieldPath)...)
	}
	return allErrs
}

// ValidateOSDisk validates the OSDisk spec
func ValidateOSDisk(osDisk OSDisk, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if osDisk.DiskSizeGB <= 0 || osDisk.DiskSizeGB > 2048 {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("DiskSizeGB"), "", "the Disk size should be a value between 1 and 2048"))
	}

	if osDisk.OSType == "" {
		allErrs = append(allErrs, field.Required(fieldPath.Child("OSType"), "the OS type cannot be empty"))
	}

	allErrs = append(allErrs, validateStorageAccountType(osDisk.ManagedDisk.StorageAccountType, fieldPath)...)

	allErrs = append(allErrs, validateCachingType(osDisk.CachingType, fieldPath)...)

	if errs := ValidateManagedDisk(osDisk.ManagedDisk, osDisk.ManagedDisk, fieldPath.Child("managedDisk")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if err := validateDiffDiskSetings(osDisk.DiffDiskSettings, fieldPath.Child("diffDiskSettings")); err != nil {
		allErrs = append(allErrs, err)
	}

	if osDisk.DiffDiskSettings != nil && osDisk.DiffDiskSettings.Option == string(compute.Local) && osDisk.ManagedDisk.StorageAccountType != "Standard_LRS" {
		allErrs = append(allErrs, field.Invalid(
			fieldPath.Child("managedDisks").Child("storageAccountType"),
			osDisk.ManagedDisk.StorageAccountType,
			"storageAccountType must be Standard_LRS when diffDiskSettings.option is 'Local'",
		))
	}

	if osDisk.DiffDiskSettings != nil && osDisk.DiffDiskSettings.Option == string(compute.Local) && osDisk.ManagedDisk.DiskEncryptionSet != nil {
		allErrs = append(allErrs, field.Invalid(
			fieldPath.Child("managedDisks").Child("diskEncryptionSet"),
			osDisk.ManagedDisk.DiskEncryptionSet.ID,
			"diskEncryptionSet is not supported when diffDiskSettings.option is 'Local'",
		))
	}

	return allErrs
}

// ValidateManagedDisk validates updates to the ManagedDisk field.
func ValidateManagedDisk(old, new ManagedDisk, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if old.StorageAccountType != new.StorageAccountType {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("storageAccountType"), new, "changing storage account type after machine creation is not allowed"))
	}

	return allErrs
}

func validateDiffDiskSetings(d *DiffDiskSettings, fldPath *field.Path) *field.Error {
	if d != nil {
		if d.Option != string(compute.Local) {
			msg := "changing ephemeral os settings after machine creation is not allowed"
			return field.Invalid(fldPath.Child("option"), d, msg)
		}
	}
	return nil
}

func validateDiffDiskSettingsUpdate(old, new *DiffDiskSettings, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	fldPath := fieldPath.Child("diffDiskSettings")

	if old == nil && new != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, new, "enabling ephemeral os after machine creation is not allowed"))
		return allErrs
	}
	if old != nil && new == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, new, "disabling ephemeral os after machine creation is not allowed"))
		return allErrs
	}

	if old != nil && new != nil {
		if old.Option != new.Option {
			msg := "changing ephemeral os settings after machine creation is not allowed"
			return append(allErrs, field.Invalid(fldPath.Child("option"), new, msg))
		}
	}

	return allErrs
}

func validateStorageAccountType(storageAccountType string, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	storageAccTypeChildPath := fieldPath.Child("ManagedDisk").Child("StorageAccountType")

	if storageAccountType == "" {
		allErrs = append(allErrs, field.Required(storageAccTypeChildPath, "the Storage Account Type for Managed Disk cannot be empty"))
		return allErrs
	}

	for _, possibleStorageAccountType := range compute.PossibleDiskStorageAccountTypesValues() {
		if string(possibleStorageAccountType) == storageAccountType {
			return allErrs
		}
	}
	allErrs = append(allErrs, field.Invalid(storageAccTypeChildPath, "", fmt.Sprintf("allowed values are %v", compute.PossibleDiskStorageAccountTypesValues())))
	return allErrs
}

func validateCachingType(cachingType string, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	cachingTypeChildPath := fieldPath.Child("CachingType")

	for _, possibleCachingType := range compute.PossibleCachingTypesValues() {
		if string(possibleCachingType) == cachingType {
			return allErrs
		}
	}

	allErrs = append(allErrs, field.Invalid(cachingTypeChildPath, cachingType, fmt.Sprintf("allowed values are %v", compute.PossibleCachingTypesValues())))
	return allErrs
}
