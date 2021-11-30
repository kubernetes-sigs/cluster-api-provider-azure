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

package scope

import (
	"context"
	"testing"

	"github.com/Azure/go-autorest/autorest"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAPIServerHost(t *testing.T) {
	fakeSubscriptionID := "123"

	tests := []struct {
		name         string
		azureCluster infrav1.AzureCluster
		want         string
	}{
		{
			name: "public apiserver lb (user-defined dns)",
			azureCluster: infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					SubscriptionID: fakeSubscriptionID,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: infrav1.LoadBalancerSpec{
							Type: infrav1.Public,
							FrontendIPs: []infrav1.FrontendIP{
								{
									PublicIP: &infrav1.PublicIPSpec{
										DNSName: "my-cluster-apiserver.example.com",
									},
								},
							},
						},
					},
				},
			},
			want: "my-cluster-apiserver.example.com",
		},
		{
			name: "private apiserver lb (default private dns zone)",
			azureCluster: infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					SubscriptionID: fakeSubscriptionID,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: infrav1.LoadBalancerSpec{
							Type: infrav1.Public,
							FrontendIPs: []infrav1.FrontendIP{
								{
									PublicIP: &infrav1.PublicIPSpec{
										DNSName: "my-cluster-apiserver.capz.io",
									},
								},
							},
						},
					},
				},
			},
			want: "my-cluster-apiserver.capz.io",
		},
		{
			name: "private apiserver (user-defined private dns zone)",
			azureCluster: infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					SubscriptionID: fakeSubscriptionID,
					NetworkSpec: infrav1.NetworkSpec{
						PrivateDNSZoneName: "example.private",
						APIServerLB: infrav1.LoadBalancerSpec{
							Type: infrav1.Internal,
						},
					},
				},
			},
			want: "apiserver.example.private",
		},
	}

	for _, tc := range tests {
		g := NewWithT(t)
		scheme := runtime.NewScheme()
		_ = clusterv1.AddToScheme(scheme)
		_ = infrav1.AddToScheme(scheme)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-cluster",
				Namespace: "default",
			},
		}
		cluster.Default()

		tc.azureCluster.ObjectMeta = metav1.ObjectMeta{
			Name: cluster.Name,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "Cluster",
					Name:       "my-cluster",
				},
			},
		}
		tc.azureCluster.Default()

		initObjects := []runtime.Object{cluster, &tc.azureCluster}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

		clusterScope, err := NewClusterScope(context.TODO(), ClusterScopeParams{
			AzureClients: AzureClients{
				Authorizer: autorest.NullAuthorizer{},
			},
			Cluster:      cluster,
			AzureCluster: &tc.azureCluster,
			Client:       fakeClient,
		})
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(clusterScope.APIServerHost()).Should(Equal(tc.want))
	}
}

func TestGettingSecurityRules(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "default",
		},
	}
	cluster.Default()

	azureCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-azure-cluster",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "Cluster",
					Name:       "my-cluster",
				},
			},
		},
		Spec: infrav1.AzureClusterSpec{
			SubscriptionID: "123",
			NetworkSpec: infrav1.NetworkSpec{
				Subnets: infrav1.Subnets{
					{
						Name: "node",
						Role: infrav1.SubnetNode,
					},
				},
			},
		},
	}
	azureCluster.Default()

	initObjects := []runtime.Object{cluster, azureCluster}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	clusterScope, err := NewClusterScope(context.TODO(), ClusterScopeParams{
		AzureClients: AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Cluster:      cluster,
		AzureCluster: azureCluster,
		Client:       fakeClient,
	})
	g.Expect(err).NotTo(HaveOccurred())

	clusterScope.SetControlPlaneSecurityRules()

	subnet, err := clusterScope.AzureCluster.Spec.NetworkSpec.GetControlPlaneSubnet()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(len(subnet.SecurityGroup.SecurityRules)).To(Equal(2))
}

func TestOutboundLBName(t *testing.T) {
	tests := []struct {
		clusterName            string
		name                   string
		role                   string
		apiServerLB            *infrav1.LoadBalancerSpec
		controlPlaneOutboundLB *infrav1.LoadBalancerSpec
		nodeOutboundLB         *infrav1.LoadBalancerSpec
		expected               string
	}{
		{
			clusterName: "my-cluster",
			name:        "public cluster node outbound lb",
			role:        "node",
			expected:    "my-cluster",
		},
		{
			clusterName: "my-cluster",
			name:        "public cluster control plane outbound lb",
			role:        "control-plane",
			expected:    "my-cluster-public-lb",
		},
		{
			clusterName:    "my-cluster",
			name:           "private cluster with node outbound lb",
			role:           "node",
			nodeOutboundLB: &infrav1.LoadBalancerSpec{},
			apiServerLB:    &infrav1.LoadBalancerSpec{Type: "Internal"},
			expected:       "my-cluster",
		},
		{
			clusterName: "my-cluster",
			name:        "private cluster without node outbound lb",
			role:        "node",
			apiServerLB: &infrav1.LoadBalancerSpec{Type: "Internal"},
			expected:    "",
		},
		{
			clusterName:            "my-cluster",
			name:                   "private cluster with control plane outbound lb",
			role:                   "control-plane",
			controlPlaneOutboundLB: &infrav1.LoadBalancerSpec{Name: "cp-outbound-lb"},
			apiServerLB:            &infrav1.LoadBalancerSpec{Type: "Internal"},
			expected:               "cp-outbound-lb",
		},
		{
			clusterName: "my-cluster",
			name:        "private cluster without control plane outbound lb",
			role:        "control-plane",
			apiServerLB: &infrav1.LoadBalancerSpec{Type: "Internal"},
			expected:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			_ = infrav1.AddToScheme(scheme)
			_ = clusterv1.AddToScheme(scheme)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.clusterName,
					Namespace: "default",
				},
			}
			cluster.Default()

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Cluster",
							Name:       "my-cluster",
						},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					SubscriptionID: "123",
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								Name: "node",
								Role: infrav1.SubnetNode,
							},
						},
					},
				},
			}

			if tc.apiServerLB != nil {
				azureCluster.Spec.NetworkSpec.APIServerLB = *tc.apiServerLB
			}

			if tc.controlPlaneOutboundLB != nil {
				azureCluster.Spec.NetworkSpec.ControlPlaneOutboundLB = tc.controlPlaneOutboundLB
			}

			if tc.nodeOutboundLB != nil {
				azureCluster.Spec.NetworkSpec.NodeOutboundLB = tc.nodeOutboundLB
			}

			azureCluster.Default()

			initObjects := []runtime.Object{cluster, azureCluster}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

			clusterScope, err := NewClusterScope(context.TODO(), ClusterScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Cluster:      cluster,
				AzureCluster: azureCluster,
				Client:       fakeClient,
			})
			g.Expect(err).NotTo(HaveOccurred())
			got := clusterScope.OutboundLBName(tc.role)
			g.Expect(tc.expected).Should(Equal(got))
		})
	}
}
