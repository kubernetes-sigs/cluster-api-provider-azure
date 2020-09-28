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

	utilSSH "sigs.k8s.io/cluster-api-provider-azure/util/ssh"
)

// SetDefaultSSHPublicKey sets the default SSHPublicKey for an AzureManagedControlPlane
func (r *AzureManagedControlPlane) SetDefaultSSHPublicKey() error {
	sshKeyData := r.Spec.SSHPublicKey
	if sshKeyData == "" {
		_, publicRsaKey, err := utilSSH.GenerateSSHKey()
		if err != nil {
			return err
		}

		r.Spec.SSHPublicKey = base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}

	return nil
}
