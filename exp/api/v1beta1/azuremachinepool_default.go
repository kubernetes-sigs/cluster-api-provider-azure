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
	"encoding/base64"

	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/uuid"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilSSH "sigs.k8s.io/cluster-api-provider-azure/util/ssh"
)

// SetDefaultSSHPublicKey sets the default SSHPublicKey for an AzureMachinePool.
func (amp *AzureMachinePool) SetDefaultSSHPublicKey() error {
	if sshKeyData := amp.Spec.Template.SSHPublicKey; sshKeyData == "" {
		_, publicRsaKey, err := utilSSH.GenerateSSHKey()
		if err != nil {
			return err
		}

		amp.Spec.Template.SSHPublicKey = base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}
	return nil
}

// SetIdentityDefaults sets the defaults for VMSS Identity.
func (amp *AzureMachinePool) SetIdentityDefaults() {
	if amp.Spec.Identity == infrav1.VMIdentitySystemAssigned {
		if amp.Spec.RoleAssignmentName == "" {
			amp.Spec.RoleAssignmentName = string(uuid.NewUUID())
		}
	}
}
