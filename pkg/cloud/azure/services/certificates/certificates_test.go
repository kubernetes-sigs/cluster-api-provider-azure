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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func TestCreateOrUpdateCertificates(t *testing.T) {
	scope := actuators.Scope{
		ClusterConfig: &v1alpha1.AzureClusterProviderSpec{},
		ClusterStatus: &v1alpha1.AzureClusterProviderStatus{},
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dummyclustername",
			},
		},
	}

	if err := CreateOrUpdateCertificates(&scope); err != nil {
		t.Errorf("Error creating certificates")
	}

	if !scope.ClusterConfig.CAKeyPair.HasCertAndKey() {
		t.Errorf("Error creating ca keypair")
	}

	if !scope.ClusterConfig.SAKeyPair.HasCertAndKey() {
		t.Errorf("Error creating sa keypair")
	}

	if !scope.ClusterConfig.EtcdCAKeyPair.HasCertAndKey() {
		t.Errorf("Error creating etcd ca keypair")
	}

	if scope.ClusterStatus.CertificateStatus.AdminKubeconfig == "" {
		t.Errorf("Error generating admin kube config")
	}

	if len(scope.ClusterStatus.CertificateStatus.DiscoveryHashes) <= 0 {
		t.Errorf("Error generating discovery hashes")
	}
}
