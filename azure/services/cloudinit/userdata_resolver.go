/*
Copyright 2022 The Kubernetes Authors.

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

package cloudinit

import (
	"encoding/base64"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/data"
)

// SecureUserDataResolver is used to create bootstrap data for secure bootstrapping.
type SecureUserDataResolver struct {
	Data        []byte
	Identity    string
	ClusterName string
	MachineName string
	Boundary    string
}

// ResolveUserData returns a script that fetches cloud init data as secrets from azure vault.
// This script will be added as a boot hook in cloud init. During initialization, it fetches the actual cloud init data (that does kubeadm init/join)
// from azure key vault, writes it to a file, and restarts cloudinit.
func (s SecureUserDataResolver) ResolveUserData() (string, error) {
	vaultName := azure.GenerateVaultName(s.ClusterName)
	secretPrefix := azure.GenerateBootstrapSecretName(s.MachineName)
	chunks := data.SplitIntoChunks(s.Data, azure.DefaultChunkSize)
	userData, err := GenerateInitDocument(s.Identity, vaultName, secretPrefix, len(chunks), secretFetchScript, s.Boundary)
	if err != nil {
		return "", err
	}
	compressedUserData, err := data.GzipBytes(userData)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(compressedUserData), nil
}

// SimpleUserDataResolver is used to create bootstrap data for normal(insecure) bootstrapping.
type SimpleUserDataResolver struct {
	Data string
}

// ResolveUserData return the bootstrap data as is.
func (s SimpleUserDataResolver) ResolveUserData() (string, error) {
	return s.Data, nil
}
