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

package v1alpha4

import (
	"encoding/base64"

	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/uuid"

	utilSSH "sigs.k8s.io/cluster-api-provider-azure/util/ssh"
)

// SetDefaultSSHPublicKey sets the default SSHPublicKey for an AzureMachine.
func (m *AzureMachine) SetDefaultSSHPublicKey() error {
	sshKeyData := m.Spec.SSHPublicKey
	if sshKeyData == "" {
		_, publicRsaKey, err := utilSSH.GenerateSSHKey()
		if err != nil {
			return err
		}

		m.Spec.SSHPublicKey = base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}

	return nil
}

// SetDefaultCachingType sets the default cache type for an AzureMachine.
func (m *AzureMachine) SetDefaultCachingType() error {
	if m.Spec.OSDisk.CachingType == "" {
		m.Spec.OSDisk.CachingType = "None"
	}
	return nil
}

// SetDataDisksDefaults sets the data disk defaults for an AzureMachine.
func (m *AzureMachine) SetDataDisksDefaults() {
	set := make(map[int32]struct{})
	// populate all the existing values in the set
	for _, disk := range m.Spec.DataDisks {
		if disk.Lun != nil {
			set[*disk.Lun] = struct{}{}
		}
	}
	// Look for unique values for unassigned LUNs
	for i, disk := range m.Spec.DataDisks {
		if disk.Lun == nil {
			for l := range m.Spec.DataDisks {
				lun := int32(l)
				if _, ok := set[lun]; !ok {
					m.Spec.DataDisks[i].Lun = &lun
					set[lun] = struct{}{}
					break
				}
			}
		}
		if disk.CachingType == "" {
			m.Spec.DataDisks[i].CachingType = "ReadWrite"
		}
	}
}

// SetIdentityDefaults sets the defaults for VM Identity.
func (m *AzureMachine) SetIdentityDefaults() {
	if m.Spec.Identity == VMIdentitySystemAssigned {
		if m.Spec.RoleAssignmentName == "" {
			m.Spec.RoleAssignmentName = string(uuid.NewUUID())
		}
	}
}
