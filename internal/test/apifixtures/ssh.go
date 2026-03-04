/*
Copyright 2026 The Kubernetes Authors.

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

package apifixtures

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"

	"golang.org/x/crypto/ssh"
)

// GenerateSSHPublicKey generates an RSA SSH public key, optionally base64-encoded.
func GenerateSSHPublicKey(b64Enconded bool) string {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicRsaKey, _ := ssh.NewPublicKey(&privateKey.PublicKey)
	if b64Enconded {
		return base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}
	return string(ssh.MarshalAuthorizedKey(publicRsaKey))
}
