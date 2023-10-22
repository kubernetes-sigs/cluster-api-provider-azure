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
	"testing"

	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestAzureCluster_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		cluster *AzureCluster
		wantErr bool
	}{
		{
			name:    "azurecluster with pre-existing vnet - valid spec",
			cluster: createValidCluster(),
			wantErr: false,
		},
		{
			name: "azurecluster with pre-existing control plane endpoint - valid spec",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
					Host: "apiserver.example.com",
					Port: 8443,
				}
				return cluster
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
					SubnetSpec{SubnetClassSpec: SubnetClassSpec{Name: "invalid-subnet-name###", Role: "random-role"}})
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with ExtendedLocation and false EdgeZone feature flag",
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.ExtendedLocation = &ExtendedLocationSpec{
					Name: "rr4",
					Type: "EdgeZone",
				}
				return cluster
			}(),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := tc.cluster.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureCluster_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name       string
		oldCluster *AzureCluster
		cluster    *AzureCluster
		wantErr    bool
	}{
		{
			name: "azurecluster with pre-existing control plane endpoint - valid spec",
			oldCluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
					Host: "apiserver.example.com",
					Port: 8443,
				}
				return cluster
			}(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
					Host: "apiserver.example.io",
					Port: 6443,
				}
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "azurecluster with no control plane endpoint - valid spec",
			oldCluster: createValidCluster(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
					Host: "apiserver.example.com",
					Port: 8443,
				}
				return cluster
			}(),
			wantErr: false,
		},
		{
			name:       "azurecluster with pre-existing vnet - valid spec",
			oldCluster: createValidCluster(),
			cluster:    createValidCluster(),
			wantErr:    false,
		},
		{
			name: "azurecluster without pre-existing vnet - valid spec",
			oldCluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""
				return cluster
			}(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""
				return cluster
			}(),
			wantErr: false,
		},
		{
			name:       "azurecluster with pre-existing vnet - lack control plane subnet",
			oldCluster: createValidCluster(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[1:]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "azurecluster with pre-existing vnet - lack node subnet",
			oldCluster: createValidCluster(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[:1]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "azurecluster with pre-existing vnet - invalid resourcegroup name",
			oldCluster: createValidCluster(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = "invalid-name###"
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "azurecluster with pre-existing vnet - invalid subnet name",
			oldCluster: createValidCluster(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets = append(cluster.Spec.NetworkSpec.Subnets,
					SubnetSpec{SubnetClassSpec: SubnetClassSpec{Name: "invalid-name###", Role: "random-role"}})
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
					AzureClusterClassSpec: AzureClusterClassSpec{
						SubscriptionID: "212ec1q8",
					},
				},
			},
			cluster: &AzureCluster{
				Spec: AzureClusterSpec{
					AzureClusterClassSpec: AzureClusterClassSpec{
						SubscriptionID: "212ec1q9",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "azurecluster location is immutable",
			oldCluster: &AzureCluster{
				Spec: AzureClusterSpec{
					AzureClusterClassSpec: AzureClusterClassSpec{
						Location: "North Europe",
					},
				},
			},
			cluster: &AzureCluster{
				Spec: AzureClusterSpec{
					AzureClusterClassSpec: AzureClusterClassSpec{
						Location: "West Europe",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "azurecluster azureEnvironment is immutable",
			oldCluster: &AzureCluster{
				Spec: AzureClusterSpec{
					AzureClusterClassSpec: AzureClusterClassSpec{
						AzureEnvironment: "AzureGermanCloud",
					},
				},
			},
			cluster: &AzureCluster{
				Spec: AzureClusterSpec{
					AzureClusterClassSpec: AzureClusterClassSpec{
						AzureEnvironment: "AzureChinaCloud",
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "azurecluster azureEnvironment default mismatch",
			oldCluster: createValidCluster(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.AzureEnvironment = "AzurePublicCloud"
				return cluster
			}(),
			wantErr: false,
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
		{
			name: "natGateway name is immutable",
			oldCluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets[0].NatGateway.Name = "cluster-test-node-natgw-0"
				return cluster
			}(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets[0].NatGateway.Name = "cluster-test-node-natgw-1"
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "natGateway name can be empty before AzureCluster is updated",
			oldCluster: createValidCluster(),
			cluster: func() *AzureCluster {
				cluster := createValidCluster()
				cluster.Spec.NetworkSpec.Subnets[0].NatGateway.Name = "cluster-test-node-natgw"
				return cluster
			}(),
			wantErr: false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			_, err := tc.cluster.ValidateUpdate(tc.oldCluster)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
