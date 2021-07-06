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
	"testing"

	. "github.com/onsi/gomega"
)

func TestAzureCluster_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		cluster *AzureCluster
		wantErr bool
	}{
		{
			name: "azurecluster with pre-existing vnet - valid spec",
			cluster: func() *AzureCluster {
				return createValidCluster()
			}(),
			wantErr: false,
		},
		{
			name: "azurecluster without pre-existing vnet - valid spec",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""
				return cluster
			}(),
			wantErr: false,
		},
		{
			name: "azurecluster with pre-existing vnet - lack control plane subnet",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[1:]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with pre-existing vnet - lack node subnet",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[:1]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with pre-existing vnet - invalid resourcegroup name",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = "invalid-rg-name###"
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with pre-existing vnet - invalid subnet name",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = append(cluster.Spec.NetworkSpec.Subnets,
					SubnetSpec{Name: "invalid-subnet-name###", Role: "random-role"})
				return cluster
			}(),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cluster.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureCluster_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name       string
		oldCluster *AzureCluster
		cluster    *AzureCluster
		wantErr    bool
	}{
		{
			name: "azurecluster with pre-existing vnet - valid spec",
			cluster: func() *AzureCluster {
				return createValidCluster()
			}(),
			wantErr: false,
		},
		{
			name: "azurecluster without pre-existing vnet - valid spec",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""
				return cluster
			}(),
			wantErr: false,
		},
		{
			name: "azurecluster with pre-existing vnet - lack control plane subnet",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[1:]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with pre-existing vnet - lack node subnet",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[:1]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with pre-existing vnet - invalid resourcegroup name",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = "invalid-name###"
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with pre-existing vnet - invalid subnet name",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = append(cluster.Spec.NetworkSpec.Subnets,
					SubnetSpec{Name: "invalid-name###", Role: "random-role"})
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster resource group is immutable",
			oldCluster: &AzureCluster{
				Spec: AzureClusterSpec{
					ResourceGroup: "demoResourceGroup",
				},
			},
			cluster: &AzureCluster{
				Spec: AzureClusterSpec{
					ResourceGroup: "demoResourceGroup-2",
				},
			},
			wantErr: true,
		},
		{
			name: "azurecluster subscription ID is immutable",
			oldCluster: &AzureCluster{
				Spec: AzureClusterSpec{
					SubscriptionID: "212ec1q8",
				},
			},
			cluster: &AzureCluster{
				Spec: AzureClusterSpec{
					SubscriptionID: "212ec1q9",
				},
			},
			wantErr: true,
		},
		{
			name: "azurecluster location is immutable",
			oldCluster: &AzureCluster{
				Spec: AzureClusterSpec{
					Location: "North Europe",
				},
			},
			cluster: &AzureCluster{
				Spec: AzureClusterSpec{
					Location: "West Europe",
				},
			},
			wantErr: true,
		},
		{
			name: "azurecluster azureEnvironment is immutable",
			oldCluster: &AzureCluster{
				Spec: AzureClusterSpec{
					AzureEnvironment: "AzureGermanCloud",
				},
			},
			cluster: &AzureCluster{
				Spec: AzureClusterSpec{
					AzureEnvironment: "AzureChinaCloud",
				},
			},
			wantErr: true,
		},
		{
			name: "azurecluster azureEnvironment is immutable",
			oldCluster: &AzureCluster{
				Spec: AzureClusterSpec{
					AzureEnvironment: "AzureGermanCloud",
				},
			},
			cluster: &AzureCluster{
				Spec: AzureClusterSpec{
					AzureEnvironment: "AzureChinaCloud",
				},
			},
			wantErr: true,
		},
		{
			name: "control plane outbound lb is immutable",
			oldCluster: &AzureCluster{
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						ControlPlaneOutboundLB: &LoadBalancerSpec{Name: "cp-lb"},
					},
				},
			},
			cluster: &AzureCluster{
				Spec: AzureClusterSpec{
					NetworkSpec: NetworkSpec{
						ControlPlaneOutboundLB: &LoadBalancerSpec{Name: "cp-lb-new"},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cluster.ValidateUpdate(tc.oldCluster)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
