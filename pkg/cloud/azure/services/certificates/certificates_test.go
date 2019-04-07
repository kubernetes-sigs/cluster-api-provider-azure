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
	"context"
	"encoding/base64"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func TestReconcileCertificates(t *testing.T) {
	scope := actuators.Scope{
		ClusterConfig: &v1alpha1.AzureClusterProviderSpec{
			Location: "eastus",
		},
		ClusterStatus: &v1alpha1.AzureClusterProviderStatus{},
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dummyclustername",
			},
		},
	}
	scope.Network().APIServerIP.DNSName = "dummydnsname"

	certsSvc := NewService(&scope)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := certsSvc.Reconcile(ctx, nil); err != nil {
		t.Errorf("Error creating certificates: %v", err)
	}

	caKeyPair := scope.ClusterConfig.CAKeyPair

	if !scope.ClusterConfig.CAKeyPair.HasCertAndKey() {
		t.Errorf("Error creating ca keypair")
	}

	if !scope.ClusterConfig.SAKeyPair.HasCertAndKey() {
		t.Errorf("Error creating sa keypair")
	}

	if !scope.ClusterConfig.EtcdCAKeyPair.HasCertAndKey() {
		t.Errorf("Error creating etcd ca keypair")
	}

	if scope.ClusterConfig.AdminKubeconfig == "" {
		t.Errorf("Error generating admin kube config")
	}

	if len(scope.ClusterConfig.DiscoveryHashes) <= 0 {
		t.Errorf("Error generating discovery hashes")
	}

	if err := certsSvc.Reconcile(ctx, nil); err != nil {
		t.Errorf("Error creating certificates: %v", err)
	}

	if !reflect.DeepEqual(scope.ClusterConfig.CAKeyPair, caKeyPair) {
		t.Errorf("Expected ca key pair not be regenerated")
	}
}

func TestCreateBastionSSHKeys(t *testing.T) {
	type args struct {
		clusterConfig *v1alpha1.AzureClusterProviderSpec
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "create ssh key-pairs",
			args: args{
				clusterConfig: &v1alpha1.AzureClusterProviderSpec{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateBastionSSHKeys(tt.args.clusterConfig); (err != nil) != tt.wantErr {
				t.Errorf("CreateBastionSSHKeys() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				b, _ := base64.StdEncoding.DecodeString(tt.args.clusterConfig.SSHPublicKey)
				if len(b) <= 0 {
					t.Errorf("ssh public key can't be empty")
				}

				b, _ = base64.StdEncoding.DecodeString(tt.args.clusterConfig.SSHPrivateKey)
				if len(b) <= 0 {
					t.Errorf("ssh private key can't be empty")
				}
			}
		})
	}
}
