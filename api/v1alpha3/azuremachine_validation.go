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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateSSHKey validates an SSHKey
func ValidateSSHKey(sshKey string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	decoded, err := base64.StdEncoding.DecodeString(sshKey)
	if err != nil {
		allErrs = append(allErrs, field.Required(fldPath, "the SSH public key is not properly base64 encoded"))
		return allErrs
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey(decoded); err != nil {
		allErrs = append(allErrs, field.Required(fldPath, "the SSH public key is not valid"))
		return allErrs
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
