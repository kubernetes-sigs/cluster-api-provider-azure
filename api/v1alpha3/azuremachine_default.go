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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// SetDefaultSSHPublicKey sets the default SSHPublicKey for an AzureMachine
func (m *AzureMachine) SetDefaultSSHPublicKey() error {
	sshKeyData := m.Spec.SSHPublicKey
	if sshKeyData == "" {
		privateKey, perr := rsa.GenerateKey(rand.Reader, 2048)
		if perr != nil {
			return errors.Wrap(perr, "Failed to generate private key")
		}

		publicRsaKey, perr := ssh.NewPublicKey(&privateKey.PublicKey)
		if perr != nil {
			return errors.Wrap(perr, "Failed to generate public key")
		}
		m.Spec.SSHPublicKey = base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}

	return nil
}

// SetDefaultsDataDisks sets the data disk defaults for an AzureMachine
func (m *AzureMachine) SetDefaultsDataDisks() error {
	// set := make(map[int32]struct{})
	// // populate all the existing values in the set
	// for _, disk := range m.Spec.DataDisks {
	// 	if disk.Lun != nil {
	// 		set[*disk.Lun] = struct{}{}
	// 	}
	// }
	// for _, disk := range m.Spec.DataDisks {
	// 	if disk.Lun != nil {
	// 		set[*disk.Lun] = struct{}{}
	// 	}
	// }
	return nil
}
