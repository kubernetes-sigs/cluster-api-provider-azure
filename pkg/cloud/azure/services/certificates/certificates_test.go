/*
Copyright 2019 The Kubernetes Authors.

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

package certificates

import (
	"testing"

	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
)

func TestCreateOrUpdateCertificates(t *testing.T) {
	cfg := v1alpha1.AzureClusterProviderSpec{}

	if err := CreateOrUpdateCertificates(&cfg, "dummyclustername"); err != nil {
		t.Fatalf("Error creating certificates")
		return
	}

	if !cfg.CAKeyPair.HasCertAndKey() {
		t.Fatalf("Error creating ca keypair")
		return
	}

	if !cfg.SAKeyPair.HasCertAndKey() {
		t.Fatalf("Error creating sa keypair")
		return
	}

	if !cfg.EtcdCAKeyPair.HasCertAndKey() {
		t.Fatalf("Error creating etcd ca keypair")
		return
	}

	if cfg.AdminKubeconfig == "" {
		t.Fatalf("Error generating admin kube config")
		return
	}

	if len(cfg.DiscoveryHashes) <= 0 {
		t.Fatalf("Error generating discovery hashes")
		return
	}
}
