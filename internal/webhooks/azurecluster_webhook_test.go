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

package webhooks

import (
	"testing"

	. "github.com/onsi/gomega"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	apifixtures "sigs.k8s.io/cluster-api-provider-azure/internal/test/apifixtures"
)

func TestAzureCluster_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		cluster *infrav1.AzureCluster
		wantErr bool
	}{
		{
			name:    "azurecluster with pre-existing vnet - valid spec",
			cluster: apifixtures.CreateValidCluster(),
			wantErr: false,
		},
		{
			name: "azurecluster with pre-existing control plane endpoint - valid spec",
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.ControlPlaneEndpoint = clusterv1beta1.APIEndpoint{
					Host: "apiserver.example.com",
					Port: 8443,
				}
				return cluster
			}(),
			wantErr: false,
		},
		{
			name: "azurecluster without pre-existing vnet - valid spec",
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""
				return cluster
			}(),
			wantErr: false,
		},
		{
			name: "azurecluster with pre-existing vnet - lack control plane subnet",
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[1:]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with pre-existing vnet - lack node subnet",
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[:1]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with pre-existing vnet - invalid resourcegroup name",
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = "invalid-rg-name###"
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with pre-existing vnet - invalid subnet name",
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Subnets = append(cluster.Spec.NetworkSpec.Subnets,
					infrav1.SubnetSpec{SubnetClassSpec: infrav1.SubnetClassSpec{Name: "invalid-subnet-name###", Role: "random-role"}})
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster with ExtendedLocation and false EdgeZone feature flag",
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.ExtendedLocation = &infrav1.ExtendedLocationSpec{
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
			_, err := (&AzureClusterWebhook{}).ValidateCreate(t.Context(), tc.cluster)
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
		oldCluster *infrav1.AzureCluster
		cluster    *infrav1.AzureCluster
		wantErr    bool
	}{
		{
			name: "azurecluster with pre-existing control plane endpoint - valid spec",
			oldCluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.ControlPlaneEndpoint = clusterv1beta1.APIEndpoint{
					Host: "apiserver.example.com",
					Port: 8443,
				}
				return cluster
			}(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.ControlPlaneEndpoint = clusterv1beta1.APIEndpoint{
					Host: "apiserver.example.io",
					Port: 6443,
				}
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "azurecluster with no control plane endpoint - valid spec",
			oldCluster: apifixtures.CreateValidCluster(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.ControlPlaneEndpoint = clusterv1beta1.APIEndpoint{
					Host: "apiserver.example.com",
					Port: 8443,
				}
				return cluster
			}(),
			wantErr: false,
		},
		{
			name:       "azurecluster with pre-existing vnet - valid spec",
			oldCluster: apifixtures.CreateValidCluster(),
			cluster:    apifixtures.CreateValidCluster(),
			wantErr:    false,
		},
		{
			name: "azurecluster without pre-existing vnet - valid spec",
			oldCluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""
				return cluster
			}(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""
				return cluster
			}(),
			wantErr: false,
		},
		{
			name:       "azurecluster with pre-existing vnet - lack control plane subnet",
			oldCluster: apifixtures.CreateValidCluster(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[1:]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "azurecluster with pre-existing vnet - lack node subnet",
			oldCluster: apifixtures.CreateValidCluster(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Subnets = cluster.Spec.NetworkSpec.Subnets[:1]
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "azurecluster with pre-existing vnet - invalid resourcegroup name",
			oldCluster: apifixtures.CreateValidCluster(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Vnet.ResourceGroup = "invalid-name###"
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "azurecluster with pre-existing vnet - invalid subnet name",
			oldCluster: apifixtures.CreateValidCluster(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Subnets = append(cluster.Spec.NetworkSpec.Subnets,
					infrav1.SubnetSpec{SubnetClassSpec: infrav1.SubnetClassSpec{Name: "invalid-name###", Role: "random-role"}})
				return cluster
			}(),
			wantErr: true,
		},
		{
			name: "azurecluster resource group is immutable",
			oldCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "demoResourceGroup",
				},
			},
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "demoResourceGroup-2",
				},
			},
			wantErr: true,
		},
		{
			name: "azurecluster subscription ID is immutable",
			oldCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "212ec1q8",
					},
				},
			},
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "212ec1q9",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "azurecluster location is immutable",
			oldCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: "North Europe",
					},
				},
			},
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: "West Europe",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "azurecluster azureEnvironment is immutable",
			oldCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AzureEnvironment: "AzureGermanCloud",
					},
				},
			},
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AzureEnvironment: "AzureChinaCloud",
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "azurecluster azureEnvironment default mismatch",
			oldCluster: apifixtures.CreateValidCluster(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.AzureEnvironment = "AzurePublicCloud"
				return cluster
			}(),
			wantErr: false,
		},
		{
			name: "control plane outbound lb is immutable",
			oldCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{Name: "cp-lb"},
					},
				},
			},
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{Name: "cp-lb-new"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "natGateway name is immutable",
			oldCluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Subnets[0].NatGateway.Name = "cluster-test-node-natgw-0"
				return cluster
			}(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Subnets[0].NatGateway.Name = "cluster-test-node-natgw-1"
				return cluster
			}(),
			wantErr: true,
		},
		{
			name:       "natGateway name can be empty before AzureCluster is updated",
			oldCluster: apifixtures.CreateValidCluster(),
			cluster: func() *infrav1.AzureCluster {
				cluster := apifixtures.CreateValidCluster()
				cluster.Spec.NetworkSpec.Subnets[0].NatGateway.Name = "cluster-test-node-natgw"
				return cluster
			}(),
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			_, err := (&AzureClusterWebhook{}).ValidateUpdate(t.Context(), tc.oldCluster, tc.cluster)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
