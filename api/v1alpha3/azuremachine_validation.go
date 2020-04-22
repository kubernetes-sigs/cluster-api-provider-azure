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

	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateSSHKey validates an SSHKey
func ValidateSSHKey(sshKey string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	decoded, err := base64.StdEncoding.DecodeString(sshKey)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, "the SSH public key is not properly base64 encoded"))
		return allErrs
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(sshKey)); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, sshKey, "the SSH public key is not valid"))
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
}

// ValidateDataDisks validates a list of data disks
func ValidateDataDisks(dataDisks []DataDisk, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// check that all LUNs are unique
	set := make(map[int32]struct{})
	for _, disk := range dataDisks {
		if _, ok := set[disk.Lun]; ok {
			allErrs = append(allErrs, field.Invalid(fldPath, disk, "The LUN must be unique for each data disk attached to a VM"))
		} else {
			set[disk.Lun] = struct{}{}
		}
	}
	return allErrs
}
