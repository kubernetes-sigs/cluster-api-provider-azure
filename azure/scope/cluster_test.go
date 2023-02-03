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
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/bastionhosts"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/natgateways"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func specToString(spec azure.ResourceSpecGetter) string {
	var sb strings.Builder
	sb.WriteString("{ ")
	sb.WriteString(fmt.Sprintf("%+v ", spec))
	sb.WriteString("}")
	return sb.String()
}

func specArrayToString(specs []azure.ResourceSpecGetter) string {
	var sb strings.Builder
	sb.WriteString("[\n")
	for _, spec := range specs {
		sb.WriteString(fmt.Sprintf("\t%+v\n", specToString(spec)))
	}
	sb.WriteString("]")

	return sb.String()
}

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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: fakeSubscriptionID,
					},
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: infrav1.LoadBalancerSpec{
							FrontendIPs: []infrav1.FrontendIP{
								{
									PublicIP: &infrav1.PublicIPSpec{
										DNSName: "my-cluster-apiserver.example.com",
									},
								},
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: fakeSubscriptionID,
					},
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: infrav1.LoadBalancerSpec{
							FrontendIPs: []infrav1.FrontendIP{
								{
									PublicIP: &infrav1.PublicIPSpec{
										DNSName: "my-cluster-apiserver.capz.io",
									},
								},
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Public,
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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: fakeSubscriptionID,
					},
					NetworkSpec: infrav1.NetworkSpec{
						NetworkClassSpec: infrav1.NetworkClassSpec{
							PrivateDNSZoneName: "example.private",
						},
						APIServerLB: infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
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
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "123",
			},
			NetworkSpec: infrav1.NetworkSpec{
				Subnets: infrav1.Subnets{
					{
						Name: "node",
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Role: infrav1.SubnetNode,
						},
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

func TestPublicIPSpecs(t *testing.T) {
	tests := []struct {
		name                 string
		azureCluster         *infrav1.AzureCluster
		expectedPublicIPSpec []azure.PublicIPSpec
	}{
		{
			name: "Azure cluster with internal type LB and nil frontend IP count",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Cluster",
							Name:       "my-cluster",
						},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
			expectedPublicIPSpec: nil,
		},
		{
			name: "Azure cluster with internal type LB and 0 frontend IP count",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Cluster",
							Name:       "my-cluster",
						},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
			expectedPublicIPSpec: nil,
		},
		{
			name: "Azure cluster with internal type apiserver LB and 1 frontend IP count",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Cluster",
							Name:       "my-cluster",
						},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							FrontendIPsCount:      to.Int32Ptr(1),
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						APIServerLB: infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
			expectedPublicIPSpec: []azure.PublicIPSpec{
				{
					Name:    "pip-my-cluster-controlplane-outbound",
					DNSName: "",
					IsIPv6:  false,
				},
			},
		},
		{
			name: "Azure cluster with internal type apiserver LB and many frontend IP count",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Cluster",
							Name:       "my-cluster",
						},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							FrontendIPsCount:      to.Int32Ptr(3),
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						APIServerLB: infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
			expectedPublicIPSpec: []azure.PublicIPSpec{
				{
					Name:    "pip-my-cluster-controlplane-outbound-1",
					DNSName: "",
					IsIPv6:  false,
				},
				{
					Name:    "pip-my-cluster-controlplane-outbound-2",
					DNSName: "",
					IsIPv6:  false,
				},
				{
					Name:    "pip-my-cluster-controlplane-outbound-3",
					DNSName: "",
					IsIPv6:  false,
				},
			},
		},
		{
			name: "Azure cluster with public type apiserver LB",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Cluster",
							Name:       "my-cluster",
						},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						APIServerLB: infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
							FrontendIPs: []infrav1.FrontendIP{
								{
									PublicIP: &infrav1.PublicIPSpec{
										Name:    "40.60.89.22",
										DNSName: "fake-dns",
									},
								},
							},
						},
					},
				},
			},
			expectedPublicIPSpec: []azure.PublicIPSpec{
				{
					Name:    "40.60.89.22",
					DNSName: "fake-dns",
					IsIPv6:  false,
				},
			},
		},
		{
			name: "Azure cluster with public type apiserver LB and public node outbound lb",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Cluster",
							Name:       "my-cluster",
						},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						APIServerLB: infrav1.LoadBalancerSpec{
							FrontendIPs: []infrav1.FrontendIP{
								{
									PublicIP: &infrav1.PublicIPSpec{
										Name:    "40.60.89.22",
										DNSName: "fake-dns",
									},
								},
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
					},
				},
			},
			expectedPublicIPSpec: []azure.PublicIPSpec{
				{
					Name:    "40.60.89.22",
					DNSName: "fake-dns",
					IsIPv6:  false,
				},
			},
		},
		{
			name: "Azure cluster with public type apiserver LB and public node outbound lb, NAT gateways and bastions",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Cluster",
							Name:       "my-cluster",
						},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							PublicIP: infrav1.PublicIPSpec{
								Name:    "fake-bastion-public-ip",
								DNSName: "fake-bastion-dns-name",
							},
						},
					},
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							infrav1.SubnetSpec{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
								},
								NatGateway: infrav1.NatGateway{
									NatGatewayIP: infrav1.PublicIPSpec{
										Name:    "fake-public-ip",
										DNSName: "fake-dns-name",
									},
								},
							},
						},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						APIServerLB: infrav1.LoadBalancerSpec{
							FrontendIPs: []infrav1.FrontendIP{
								{
									PublicIP: &infrav1.PublicIPSpec{
										Name:    "40.60.89.22",
										DNSName: "fake-dns",
									},
								},
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
					},
				},
			},
			expectedPublicIPSpec: []azure.PublicIPSpec{
				{
					Name:    "40.60.89.22",
					DNSName: "fake-dns",
					IsIPv6:  false,
				},
				{
					Name:    "fake-bastion-public-ip",
					DNSName: "fake-bastion-dns-name",
					IsIPv6:  false,
				},
			},
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
					Name:      tc.azureCluster.Name,
					Namespace: "default",
				},
			}
			cluster.Default()

			initObjects := []runtime.Object{cluster, tc.azureCluster}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

			clusterScope, err := NewClusterScope(context.TODO(), ClusterScopeParams{
				AzureClients: AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Cluster:      cluster,
				AzureCluster: tc.azureCluster,
				Client:       fakeClient,
			})
			g.Expect(err).NotTo(HaveOccurred())
			got := clusterScope.PublicIPSpecs()
			g.Expect(tc.expectedPublicIPSpec).Should(Equal(got))
		})
	}
}

func TestRouteTableSpecs(t *testing.T) {
	tests := []struct {
		name         string
		clusterScope ClusterScope
		want         []azure.ResourceSpecGetter
	}{
		{
			name: "returns nil if no subnets are specified",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: nil,
		},
		{
			name: "returns specified route tables if present",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "my-rg",
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{
								{
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-1",
										Name: "fake-route-table-1",
									},
								},
								{
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-2",
										Name: "fake-route-table-2",
									},
								},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: []azure.ResourceSpecGetter{
				&routetables.RouteTableSpec{
					Name:          "fake-route-table-1",
					ResourceGroup: "my-rg",
					Location:      "centralIndia",
				},
				&routetables.RouteTableSpec{
					Name:          "fake-route-table-2",
					ResourceGroup: "my-rg",
					Location:      "centralIndia",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.clusterScope.RouteTableSpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RouteTableSpecs() = %s, want %s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func TestNatGatewaySpecs(t *testing.T) {
	tests := []struct {
		name         string
		clusterScope ClusterScope
		want         []azure.ResourceSpecGetter
	}{
		{
			name: "returns nil if no subnets are specified",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: nil,
		},
		{
			name: "returns specified node NAT gateway if present",
			clusterScope: ClusterScope{
				AzureClients: AzureClients{
					EnvironmentSettings: auth.EnvironmentSettings{
						Values: map[string]string{
							auth.SubscriptionID: "123",
						},
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "my-rg",
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{
								{
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role: infrav1.SubnetNode,
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-1",
										Name: "fake-route-table-1",
									},
									NatGateway: infrav1.NatGateway{
										NatGatewayIP: infrav1.PublicIPSpec{
											Name: "44.78.67.90",
										},
										NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
											Name: "fake-nat-gateway-1",
										},
									},
								},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: []azure.ResourceSpecGetter{
				&natgateways.NatGatewaySpec{
					Name:           "fake-nat-gateway-1",
					ResourceGroup:  "my-rg",
					Location:       "centralIndia",
					SubscriptionID: "123",
					NatGatewayIP: infrav1.PublicIPSpec{
						Name: "44.78.67.90",
					},
				},
			},
		},
		{
			name: "returns specified node NAT gateway if present and ignores duplicate",
			clusterScope: ClusterScope{
				AzureClients: AzureClients{
					EnvironmentSettings: auth.EnvironmentSettings{
						Values: map[string]string{
							auth.SubscriptionID: "123",
						},
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "my-rg",
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{
								{
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role: infrav1.SubnetNode,
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-1",
										Name: "fake-route-table-1",
									},
									NatGateway: infrav1.NatGateway{
										NatGatewayIP: infrav1.PublicIPSpec{
											Name: "44.78.67.90",
										},
										NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
											Name: "fake-nat-gateway-1",
										},
									},
								},
								// Duplicate Entry
								{
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role: infrav1.SubnetNode,
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-1",
										Name: "fake-route-table-1",
									},
									NatGateway: infrav1.NatGateway{
										NatGatewayIP: infrav1.PublicIPSpec{
											Name: "44.78.67.90",
										},
										NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
											Name: "fake-nat-gateway-1",
										},
									},
								},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: []azure.ResourceSpecGetter{
				&natgateways.NatGatewaySpec{
					Name:           "fake-nat-gateway-1",
					ResourceGroup:  "my-rg",
					Location:       "centralIndia",
					SubscriptionID: "123",
					NatGatewayIP: infrav1.PublicIPSpec{
						Name: "44.78.67.90",
					},
				},
			},
		},
		{
			name: "returns specified node NAT gateway if present and ignores control plane nat gateway",
			clusterScope: ClusterScope{
				AzureClients: AzureClients{
					EnvironmentSettings: auth.EnvironmentSettings{
						Values: map[string]string{
							auth.SubscriptionID: "123",
						},
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "my-rg",
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{
								{
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role: infrav1.SubnetNode,
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-1",
										Name: "fake-route-table-1",
									},
									NatGateway: infrav1.NatGateway{
										NatGatewayIP: infrav1.PublicIPSpec{
											Name: "44.78.67.90",
										},
										NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
											Name: "fake-nat-gateway-1",
										},
									},
								},
								{
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role: infrav1.SubnetControlPlane,
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-2",
										Name: "fake-route-table-2",
									},
									NatGateway: infrav1.NatGateway{
										NatGatewayIP: infrav1.PublicIPSpec{
											Name: "44.78.67.91",
										},
										NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
											Name: "fake-nat-gateway-2",
										},
									},
								},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: []azure.ResourceSpecGetter{
				&natgateways.NatGatewaySpec{
					Name:           "fake-nat-gateway-1",
					ResourceGroup:  "my-rg",
					Location:       "centralIndia",
					SubscriptionID: "123",
					NatGatewayIP: infrav1.PublicIPSpec{
						Name: "44.78.67.90",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.clusterScope.NatGatewaySpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NatGatewaySpecs() = %s, want %s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func TestNSGSpecs(t *testing.T) {
	tests := []struct {
		name         string
		clusterScope ClusterScope
		want         []azure.ResourceSpecGetter
	}{
		{
			name: "returns empty if no subnets are specified",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{},
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{},
		},
		{
			name: "returns specified security groups if present",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "my-rg",
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{
								{
									SecurityGroup: infrav1.SecurityGroup{
										Name: "fake-security-group-1",
										SecurityGroupClass: infrav1.SecurityGroupClass{
											SecurityRules: infrav1.SecurityRules{
												{
													Name: "fake-rule-1",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: []azure.ResourceSpecGetter{
				&securitygroups.NSGSpec{
					Name: "fake-security-group-1",
					SecurityRules: infrav1.SecurityRules{
						{
							Name: "fake-rule-1",
						},
					},
					ResourceGroup: "my-rg",
					Location:      "centralIndia",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.clusterScope.NSGSpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RouteTableSpecs() = %s, want %s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func TestSubnetSpecs(t *testing.T) {
	tests := []struct {
		name         string
		clusterScope ClusterScope
		want         []azure.ResourceSpecGetter
	}{
		{
			name: "returns empty if no subnets are specified",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: []azure.ResourceSpecGetter{},
		},
		{
			name: "returns specified subnet spec",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
				},
				AzureClients: AzureClients{
					EnvironmentSettings: auth.EnvironmentSettings{
						Values: map[string]string{
							auth.SubscriptionID: "123",
						},
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "my-rg",
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ID:            "fake-vnet-id-1",
								Name:          "fake-vnet-1",
								ResourceGroup: "my-rg-vnet",
							},
							Subnets: infrav1.Subnets{
								{
									Name: "fake-subnet-1",
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetNode,
										CIDRBlocks: []string{"192.168.1.1/16"},
									},
									NatGateway: infrav1.NatGateway{
										NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
											Name: "fake-natgateway-1",
										},
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-1",
										Name: "fake-route-table-1",
									},
									SecurityGroup: infrav1.SecurityGroup{
										Name: "fake-security-group-1",
										SecurityGroupClass: infrav1.SecurityGroupClass{
											SecurityRules: infrav1.SecurityRules{
												{
													Name: "fake-rule-1",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: []azure.ResourceSpecGetter{
				&subnets.SubnetSpec{
					Name:              "fake-subnet-1",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					CIDRs:             []string{"192.168.1.1/16"},
					VNetName:          "fake-vnet-1",
					VNetResourceGroup: "my-rg-vnet",
					IsVNetManaged:     false,
					RouteTableName:    "fake-route-table-1",
					SecurityGroupName: "fake-security-group-1",
					Role:              infrav1.SubnetNode,
					NatGatewayName:    "fake-natgateway-1",
				},
			},
		},

		{
			name: "returns specified subnet spec and bastion spec if enabled",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
				},
				AzureClients: AzureClients{
					EnvironmentSettings: auth.EnvironmentSettings{
						Values: map[string]string{
							auth.SubscriptionID: "123",
						},
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						BastionSpec: infrav1.BastionSpec{
							AzureBastion: &infrav1.AzureBastion{
								Name: "fake-azure-bastion",
								Subnet: infrav1.SubnetSpec{
									Name: "fake-bastion-subnet-1",
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetBastion,
										CIDRBlocks: []string{"172.122.1.1./16"},
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-bastion-route-table-id-1",
										Name: "fake-bastion-route-table-1",
									},
									SecurityGroup: infrav1.SecurityGroup{
										Name: "fake-bastion-security-group-1",
										SecurityGroupClass: infrav1.SecurityGroupClass{
											SecurityRules: infrav1.SecurityRules{
												{
													Name: "fake-rule-1",
												},
											},
										},
									},
								},
							},
						},
						ResourceGroup: "my-rg",
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ID:            "fake-vnet-id-1",
								Name:          "fake-vnet-1",
								ResourceGroup: "my-rg-vnet",
							},
							Subnets: infrav1.Subnets{
								{
									Name: "fake-subnet-1",
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetNode,
										CIDRBlocks: []string{"192.168.1.1/16"},
									},
									NatGateway: infrav1.NatGateway{
										NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
											Name: "fake-natgateway-1",
										},
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-1",
										Name: "fake-route-table-1",
									},
									SecurityGroup: infrav1.SecurityGroup{
										Name: "fake-security-group-1",
										SecurityGroupClass: infrav1.SecurityGroupClass{
											SecurityRules: infrav1.SecurityRules{
												{
													Name: "fake-rule-1",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: []azure.ResourceSpecGetter{
				&subnets.SubnetSpec{
					Name:              "fake-subnet-1",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					CIDRs:             []string{"192.168.1.1/16"},
					VNetName:          "fake-vnet-1",
					VNetResourceGroup: "my-rg-vnet",
					IsVNetManaged:     false,
					RouteTableName:    "fake-route-table-1",
					SecurityGroupName: "fake-security-group-1",
					Role:              infrav1.SubnetNode,
					NatGatewayName:    "fake-natgateway-1",
				},
				&subnets.SubnetSpec{
					Name:              "fake-bastion-subnet-1",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					CIDRs:             []string{"172.122.1.1./16"},
					VNetName:          "fake-vnet-1",
					VNetResourceGroup: "my-rg-vnet",
					IsVNetManaged:     false,
					SecurityGroupName: "fake-bastion-security-group-1",
					RouteTableName:    "fake-bastion-route-table-1",
					Role:              infrav1.SubnetBastion,
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.clusterScope.SubnetSpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SubnetSpecs() = \n%s, want \n%s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func TestIsVnetManaged(t *testing.T) {
	tests := []struct {
		name         string
		clusterScope ClusterScope
		want         bool
	}{
		{
			name: "VNET ID is empty",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ID: "",
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: true,
		},
		{
			name: "Wrong tags",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ID: "my-id",
								VnetClassSpec: infrav1.VnetClassSpec{Tags: map[string]string{
									"key": "value",
								}},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: false,
		},
		{
			name: "Has owning tags",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ID: "my-id",
								VnetClassSpec: infrav1.VnetClassSpec{Tags: map[string]string{
									"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
								}},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: true,
		},
		{
			name: "Has cached value of false",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{},
				},
				cache: &ClusterCache{
					isVnetManaged: to.BoolPtr(false),
				},
			},
			want: false,
		},
		{
			name: "Has cached value of true",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{},
				},
				cache: &ClusterCache{
					isVnetManaged: to.BoolPtr(true),
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.clusterScope.IsVnetManaged()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IsVnetManaged() = \n%t, want \n%t", got, tt.want)
			}
			if to.Bool(tt.clusterScope.cache.isVnetManaged) != got {
				t.Errorf("IsVnetManaged() = \n%t, cache = \n%t", got, to.Bool(tt.clusterScope.cache.isVnetManaged))
			}
		})
	}
}

func TestAzureBastionSpec(t *testing.T) {
	tests := []struct {
		name         string
		clusterScope ClusterScope
		want         azure.ResourceSpecGetter
	}{
		{
			name: "returns nil if no subnets are specified",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "returns bastion spec if enabled",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
				},
				AzureClients: AzureClients{
					EnvironmentSettings: auth.EnvironmentSettings{
						Values: map[string]string{
							auth.SubscriptionID: "123",
						},
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						BastionSpec: infrav1.BastionSpec{
							AzureBastion: &infrav1.AzureBastion{
								Name: "fake-azure-bastion-1",
								Subnet: infrav1.SubnetSpec{
									Name: "fake-bastion-subnet-1",
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetBastion,
										CIDRBlocks: []string{"172.122.1.1./16"},
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-bastion-route-table-id-1",
										Name: "fake-bastion-route-table-1",
									},
									SecurityGroup: infrav1.SecurityGroup{
										Name: "fake-bastion-security-group-1",
										SecurityGroupClass: infrav1.SecurityGroupClass{
											SecurityRules: infrav1.SecurityRules{
												{
													Name: "fake-rule-1",
												},
											},
										},
									},
								},
								PublicIP: infrav1.PublicIPSpec{
									Name: "fake-public-ip-1",
								},
							},
						},
						ResourceGroup: "my-rg",
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ID:            "fake-vnet-id-1",
								Name:          "fake-vnet-1",
								ResourceGroup: "my-rg-vnet",
							},
							Subnets: infrav1.Subnets{
								{
									Name: "fake-subnet-1",
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetNode,
										CIDRBlocks: []string{"192.168.1.1/16"},
									},
									NatGateway: infrav1.NatGateway{
										NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
											Name: "fake-natgateway-1",
										},
									},
									RouteTable: infrav1.RouteTable{
										ID:   "fake-route-table-id-1",
										Name: "fake-route-table-1",
									},
									SecurityGroup: infrav1.SecurityGroup{
										Name: "fake-security-group-1",
										SecurityGroupClass: infrav1.SecurityGroupClass{
											SecurityRules: infrav1.SecurityRules{
												{
													Name: "fake-rule-1",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: &bastionhosts.AzureBastionSpec{
				Name:          "fake-azure-bastion-1",
				ResourceGroup: "my-rg",
				Location:      "centralIndia",
				ClusterName:   "my-cluster",
				SubnetID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/"+
					"virtualNetworks/%s/subnets/%s", "123", "my-rg", "fake-vnet-1", "fake-bastion-subnet-1"),
				PublicIPID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/"+
					"publicIPAddresses/%s", "123", "my-rg", "fake-public-ip-1"),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.clusterScope.AzureBastionSpec(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AzureBastionSpec() = \n%s, want \n%s", specToString(got), specToString(tt.want))
			}
		})
	}
}

func TestSubnet(t *testing.T) {
	tests := []struct {
		clusterName             string
		subnetName              string
		azureClusterNetworkSpec infrav1.NetworkSpec
		expectSubnet            infrav1.SubnetSpec
	}{
		{
			clusterName:             "my-cluster-1",
			subnetName:              "subnet-1",
			azureClusterNetworkSpec: infrav1.NetworkSpec{},
			expectSubnet:            infrav1.SubnetSpec{},
		},
		{
			clusterName: "my-cluster-1",
			subnetName:  "subnet-1",
			azureClusterNetworkSpec: infrav1.NetworkSpec{
				Subnets: infrav1.Subnets{
					infrav1.SubnetSpec{
						ID:   "subnet-1-id",
						Name: "subnet-1",
					},
					infrav1.SubnetSpec{
						ID:   "subnet-1-id",
						Name: "subnet-2",
					},
					infrav1.SubnetSpec{
						ID:   "subnet-2-id",
						Name: "subnet-3",
					},
				},
			},
			expectSubnet: infrav1.SubnetSpec{
				ID:   "subnet-1-id",
				Name: "subnet-1",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.clusterName, func(t *testing.T) {
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
					NetworkSpec: tc.azureClusterNetworkSpec,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
				},
			}

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
			got := clusterScope.Subnet(tc.subnetName)
			g.Expect(tc.expectSubnet).Should(Equal(got))
		})
	}
}

func TestControlPlaneRouteTable(t *testing.T) {
	tests := []struct {
		clusterName             string
		azureClusterNetworkSpec infrav1.NetworkSpec
		expectRouteTable        infrav1.RouteTable
	}{
		{
			clusterName:             "my-cluster-1",
			azureClusterNetworkSpec: infrav1.NetworkSpec{},
			expectRouteTable:        infrav1.RouteTable{},
		},
		{
			clusterName: "my-cluster-2",
			azureClusterNetworkSpec: infrav1.NetworkSpec{
				Subnets: infrav1.Subnets{
					infrav1.SubnetSpec{
						RouteTable: infrav1.RouteTable{
							ID:   "fake-id-1",
							Name: "route-tb-1",
						},
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Role: infrav1.SubnetNode,
						},
					},
					infrav1.SubnetSpec{
						RouteTable: infrav1.RouteTable{
							ID:   "fake-id-2",
							Name: "route-tb-2",
						},
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Role: infrav1.SubnetControlPlane,
						},
					},
					infrav1.SubnetSpec{
						RouteTable: infrav1.RouteTable{
							ID:   "fake-id-3",
							Name: "route-tb-3",
						},
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Role: infrav1.SubnetBastion,
						},
					},
				},
			},
			expectRouteTable: infrav1.RouteTable{
				ID:   "fake-id-2",
				Name: "route-tb-2",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.clusterName, func(t *testing.T) {
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
					NetworkSpec: tc.azureClusterNetworkSpec,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
				},
			}

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
			got := clusterScope.ControlPlaneRouteTable()
			g.Expect(tc.expectRouteTable).Should(Equal(got))
		})
	}
}

func TestGetPrivateDNSZoneName(t *testing.T) {
	tests := []struct {
		clusterName              string
		azureClusterNetworkSpec  infrav1.NetworkSpec
		expectPrivateDNSZoneName string
	}{
		{
			clusterName: "my-cluster-1",
			azureClusterNetworkSpec: infrav1.NetworkSpec{
				NetworkClassSpec: infrav1.NetworkClassSpec{
					PrivateDNSZoneName: "fake-privateDNSZoneName",
				},
			},
			expectPrivateDNSZoneName: "fake-privateDNSZoneName",
		},
		{
			clusterName:              "my-cluster-2",
			expectPrivateDNSZoneName: "my-cluster-2.capz.io",
		},
	}
	for _, tc := range tests {
		t.Run(tc.clusterName, func(t *testing.T) {
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
					NetworkSpec: tc.azureClusterNetworkSpec,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
				},
			}

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
			got := clusterScope.GetPrivateDNSZoneName()
			g.Expect(tc.expectPrivateDNSZoneName).Should(Equal(got))
		})
	}
}

func TestAPIServerLBPoolName(t *testing.T) {
	tests := []struct {
		lbName           string
		clusterName      string
		expectLBpoolName string
	}{
		{
			lbName:           "fake-lb-1",
			clusterName:      "my-cluster-1",
			expectLBpoolName: "fake-lb-1-backendPool",
		},
		{
			lbName:           "fake-lb-2",
			clusterName:      "my-cluster-2",
			expectLBpoolName: "fake-lb-2-backendPool",
		},
	}
	for _, tc := range tests {
		t.Run(tc.lbName, func(t *testing.T) {
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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
				},
			}

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
			got := clusterScope.APIServerLBPoolName(tc.lbName)
			g.Expect(tc.expectLBpoolName).Should(Equal(got))
		})
	}
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
			apiServerLB: &infrav1.LoadBalancerSpec{
				LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
					Type: "Internal",
				}},
			expected: "my-cluster",
		},
		{
			clusterName: "my-cluster",
			name:        "private cluster without node outbound lb",
			role:        "node",
			apiServerLB: &infrav1.LoadBalancerSpec{
				LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
					Type: "Internal",
				}},
			expected: "",
		},
		{
			clusterName:            "my-cluster",
			name:                   "private cluster with control plane outbound lb",
			role:                   "control-plane",
			controlPlaneOutboundLB: &infrav1.LoadBalancerSpec{Name: "cp-outbound-lb"},
			apiServerLB: &infrav1.LoadBalancerSpec{
				LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
					Type: "Internal",
				}},
			expected: "cp-outbound-lb",
		},
		{
			clusterName: "my-cluster",
			name:        "private cluster without control plane outbound lb",
			role:        "control-plane",
			apiServerLB: &infrav1.LoadBalancerSpec{
				LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
					Type: "Internal",
				}},
			expected: "",
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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								Name: "node",
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
								},
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

func TestOutboundPoolName(t *testing.T) {
	tests := []struct {
		name                   string
		clusterName            string
		loadBalancerName       string
		expectOutboundPoolName string
	}{
		{
			name:                   "Empty loadBalancerName",
			clusterName:            "my-cluster",
			loadBalancerName:       "",
			expectOutboundPoolName: "",
		},
		{
			name:                   "Non empty loadBalancerName",
			clusterName:            "my-cluster",
			loadBalancerName:       "my-loadbalancer",
			expectOutboundPoolName: "my-loadbalancer-outboundBackendPool",
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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
				},
			}

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
			got := clusterScope.OutboundPoolName(tc.loadBalancerName)
			g.Expect(tc.expectOutboundPoolName).Should(Equal(got))
		})
	}
}

func TestGenerateFQDN(t *testing.T) {
	tests := []struct {
		clusterName    string
		ipName         string
		subscriptionID string
		resourceGroup  string
		location       string
		expectFQDN     string
	}{
		{
			clusterName:    "my-cluster",
			ipName:         "172.123.45.78",
			subscriptionID: "123",
			resourceGroup:  "my-rg",
			location:       "eastus",
		},
		{
			clusterName:    "my-cluster-1",
			ipName:         "172.123.45.79",
			subscriptionID: "567",
			resourceGroup:  "my-rg-1",
			location:       "westus",
		},
		{
			clusterName:    "my-cluster-2",
			ipName:         "172.123.45.80",
			subscriptionID: "183",
			resourceGroup:  "my-rg-2",
			location:       "centralasia",
		},
	}
	for _, tc := range tests {
		t.Run(tc.clusterName, func(t *testing.T) {
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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						Location:       tc.location,
					},
					ResourceGroup: tc.resourceGroup,
				},
			}

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
			got := clusterScope.GenerateFQDN(tc.ipName)
			g.Expect(got).Should(ContainSubstring(tc.clusterName))
			g.Expect(got).Should(ContainSubstring(tc.location))
		})
	}
}

func TestAdditionalTags(t *testing.T) {
	tests := []struct {
		name                       string
		clusterName                string
		azureClusterAdditionalTags infrav1.Tags
		expectTags                 infrav1.Tags
	}{
		{
			name:        "Nil tags",
			clusterName: "my-cluster",
			expectTags:  infrav1.Tags{},
		},

		{
			name:        "Single tag present in azure cluster spec",
			clusterName: "my-cluster",
			azureClusterAdditionalTags: infrav1.Tags{
				"fake-id-1": "fake-value-1",
			},
			expectTags: infrav1.Tags{
				"fake-id-1": "fake-value-1",
			},
		},
		{
			name:        "Multiple tags present in azure cluster spec",
			clusterName: "my-cluster",
			azureClusterAdditionalTags: infrav1.Tags{
				"fake-id-1": "fake-value-1",
				"fake-id-2": "fake-value-2",
				"fake-id-3": "fake-value-3",
			},
			expectTags: infrav1.Tags{
				"fake-id-1": "fake-value-1",
				"fake-id-2": "fake-value-2",
				"fake-id-3": "fake-value-3",
			},
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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						AdditionalTags: tc.azureClusterAdditionalTags,
					},
				},
			}

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
			got := clusterScope.AdditionalTags()
			g.Expect(tc.expectTags).Should(Equal(got))
		})
	}
}

func TestAPIServerPort(t *testing.T) {
	tests := []struct {
		name                string
		clusterName         string
		clusterNetowrk      *clusterv1.ClusterNetwork
		expectAPIServerPort int32
	}{
		{
			name:                "Nil cluster network",
			clusterName:         "my-cluster",
			expectAPIServerPort: 6443,
		},

		{
			name:                "Non nil cluster network but nil apiserverport",
			clusterName:         "my-cluster",
			clusterNetowrk:      &clusterv1.ClusterNetwork{},
			expectAPIServerPort: 6443,
		},
		{
			name:        "Non nil cluster network and non nil apiserverport",
			clusterName: "my-cluster",
			clusterNetowrk: &clusterv1.ClusterNetwork{
				APIServerPort: to.Int32Ptr(7000),
			},
			expectAPIServerPort: 7000,
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
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: tc.clusterNetowrk,
				},
			}
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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
				},
			}

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
			got := clusterScope.APIServerPort()
			g.Expect(tc.expectAPIServerPort).Should(Equal(got))
		})
	}
}

func TestFailureDomains(t *testing.T) {
	tests := []struct {
		name                 string
		expectFailureDomains []string
		clusterName          string
		azureClusterStatus   infrav1.AzureClusterStatus
	}{
		{
			name:                 "Empty azure cluster status",
			expectFailureDomains: []string{},
			clusterName:          "my-cluster",
		},

		{
			name:                 "Single failure domain present in azure cluster status",
			expectFailureDomains: []string{"failure-domain-id"},
			clusterName:          "my-cluster",
			azureClusterStatus: infrav1.AzureClusterStatus{
				FailureDomains: map[string]clusterv1.FailureDomainSpec{
					"failure-domain-id": {},
				},
			},
		},
		{
			name:                 "Mutiple failure domains present in azure cluster status",
			expectFailureDomains: []string{"failure-domain-id-1", "failure-domain-id-2", "failure-domain-id-3"},
			clusterName:          "my-cluster",
			azureClusterStatus: infrav1.AzureClusterStatus{
				FailureDomains: map[string]clusterv1.FailureDomainSpec{
					"failure-domain-id-1": {},
					"failure-domain-id-2": {},
					"failure-domain-id-3": {},
				},
			},
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
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
				},
				Status: tc.azureClusterStatus,
			}

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
			got := clusterScope.FailureDomains()
			g.Expect(tc.expectFailureDomains).Should(ConsistOf(got))
		})
	}
}
