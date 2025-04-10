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

	asonetworkv1api20201101 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	asonetworkv1api20220701 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/component-base/featuregate"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/bastionhosts"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/natgateways"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/privateendpoints"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vnetpeerings"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
)

const fakeClientID = "fake-client-id"
const fakeTenantID = "fake-tenant-id"
const fakeSubscriptionID = "123"

func specToString(spec any) string {
	var sb strings.Builder
	sb.WriteString("{ ")
	sb.WriteString(fmt.Sprintf("%+v ", spec))
	sb.WriteString("}")
	return sb.String()
}

func specArrayToString[T any](specs []T) string {
	var sb strings.Builder
	sb.WriteString("[\n")
	for _, spec := range specs {
		sb.WriteString(fmt.Sprintf("\t%+v\n", specToString(spec)))
	}
	sb.WriteString("]")

	return sb.String()
}

func TestNewClusterScope(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "default",
		},
	}
	azureCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster.Name,
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: fakeSubscriptionID,
				IdentityRef: &corev1.ObjectReference{
					Kind: infrav1.AzureClusterIdentityKind,
				},
			},
		},
	}
	azureCluster.Default()

	fakeIdentity := &infrav1.AzureClusterIdentity{
		Spec: infrav1.AzureClusterIdentitySpec{
			Type:     infrav1.ServicePrincipal,
			ClientID: fakeClientID,
			TenantID: fakeTenantID,
		},
	}
	fakeSecret := &corev1.Secret{Data: map[string][]byte{"clientSecret": []byte("fooSecret")}}

	initObjects := []runtime.Object{cluster, azureCluster, fakeIdentity, fakeSecret}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	_, err := NewClusterScope(context.TODO(), ClusterScopeParams{
		Cluster:         cluster,
		AzureCluster:    azureCluster,
		Client:          fakeClient,
		CredentialCache: azure.NewCredentialCache(),
	})
	g.Expect(err).NotTo(HaveOccurred())
}

func TestAPIServerHost(t *testing.T) {
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
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
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
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
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
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						NetworkClassSpec: infrav1.NetworkClassSpec{
							PrivateDNSZoneName: "example.private",
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
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

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-cluster",
				Namespace: "default",
			},
		}

		tc.azureCluster.ObjectMeta = metav1.ObjectMeta{
			Name: cluster.Name,
		}
		tc.azureCluster.Default()

		clusterScope := &ClusterScope{
			Cluster:      cluster,
			AzureCluster: &tc.azureCluster,
		}

		g.Expect(clusterScope.APIServerHost()).Should(Equal(tc.want))
	}
}

func TestGettingSecurityRules(t *testing.T) {
	tests := []struct {
		name              string
		cluster           *clusterv1.Cluster
		azureCluster      *infrav1.AzureCluster
		expectedRuleCount int
	}{
		{
			name: "default control plane subnet with no rules should have 2 security rules defaulted",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "default",
				},
			},
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-azure-cluster",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node",
								},
							},
						},
					},
				},
			},
			expectedRuleCount: 2,
		},
		{
			name: "additional rules are preserved",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "default",
				},
			},
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-azure-cluster",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SecurityGroup: infrav1.SecurityGroup{
									SecurityGroupClass: infrav1.SecurityGroupClass{
										SecurityRules: []infrav1.SecurityRule{{
											Name:             "allow_9345",
											Description:      "Allow port 9345",
											Priority:         2200,
											Protocol:         infrav1.SecurityGroupProtocolTCP,
											Direction:        infrav1.SecurityRuleDirectionInbound,
											Source:           ptr.To("*"),
											SourcePorts:      ptr.To("*"),
											Destination:      ptr.To("*"),
											DestinationPorts: ptr.To("9345"),
											Action:           infrav1.SecurityRuleActionAllow,
										}},
									},
								},
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: string(infrav1.SubnetControlPlane),
								},
							},
						},
					},
				},
			},
			expectedRuleCount: 3,
		},
		{
			name: "override rules are accepted",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "default",
				},
			},
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-azure-cluster",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SecurityGroup: infrav1.SecurityGroup{
									SecurityGroupClass: infrav1.SecurityGroupClass{
										SecurityRules: []infrav1.SecurityRule{{
											Name:             "deny_ssh",
											Description:      "Deny SSH",
											Priority:         2200,
											Protocol:         infrav1.SecurityGroupProtocolTCP,
											Direction:        infrav1.SecurityRuleDirectionInbound,
											Source:           ptr.To("*"),
											SourcePorts:      ptr.To("*"),
											Destination:      ptr.To("*"),
											DestinationPorts: ptr.To("22"),
											Action:           infrav1.SecurityRuleActionDeny,
										}},
									},
								},
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetControlPlane,
									Name: string(infrav1.SubnetControlPlane),
								},
							},
						},
					},
				},
			},
			expectedRuleCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tt.azureCluster.Default()

			clusterScope := &ClusterScope{
				Cluster:      tt.cluster,
				AzureCluster: tt.azureCluster,
			}
			clusterScope.SetControlPlaneSecurityRules()

			subnet, err := clusterScope.AzureCluster.Spec.NetworkSpec.GetControlPlaneSubnet()
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(subnet.SecurityGroup.SecurityRules).To(HaveLen(tt.expectedRuleCount))
		})
	}
}

func TestPublicIPSpecs(t *testing.T) {
	tests := []struct {
		name                 string
		azureCluster         *infrav1.AzureCluster
		expectedPublicIPSpec []azure.ResourceSpecGetter
	}{
		{
			name: "Azure cluster with internal type LB and nil frontend IP count",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Status: infrav1.AzureClusterStatus{
					FailureDomains: map[string]clusterv1.FailureDomainSpec{
						"failure-domain-id-1": {},
						"failure-domain-id-2": {},
						"failure-domain-id-3": {},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "my-rg",
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: "centralIndia",
						AdditionalTags: infrav1.Tags{
							"Name": "my-publicip-ipv6",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
						},
					},
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
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
				},
				Status: infrav1.AzureClusterStatus{
					FailureDomains: map[string]clusterv1.FailureDomainSpec{
						"failure-domain-id-1": {},
						"failure-domain-id-2": {},
						"failure-domain-id-3": {},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup:       "my-rg",
					ControlPlaneEnabled: true,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: "centralIndia",
						AdditionalTags: infrav1.Tags{
							"Name": "my-publicip-ipv6",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
						},
					},
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							FrontendIPsCount: ptr.To[int32](0),
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
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
				},
				Status: infrav1.AzureClusterStatus{
					FailureDomains: map[string]clusterv1.FailureDomainSpec{
						"failure-domain-id-1": {},
						"failure-domain-id-2": {},
						"failure-domain-id-3": {},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup:       "my-rg",
					ControlPlaneEnabled: true,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: "centralIndia",
						AdditionalTags: infrav1.Tags{
							"Name": "my-publicip-ipv6",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
						},
					},
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							FrontendIPsCount: ptr.To[int32](1),
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "my-frontend-ip",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "pip-my-cluster-controlplane-outbound",
									},
								},
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
			expectedPublicIPSpec: []azure.ResourceSpecGetter{
				&publicips.PublicIPSpec{
					Name:           "pip-my-cluster-controlplane-outbound",
					ResourceGroup:  "my-rg",
					DNSName:        "",
					IsIPv6:         false,
					ClusterName:    "my-cluster",
					Location:       "centralIndia",
					FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
					AdditionalTags: infrav1.Tags{
						"Name": "my-publicip-ipv6",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
			},
		},
		{
			name: "Azure cluster with internal type apiserver LB and many frontend IP count",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Status: infrav1.AzureClusterStatus{
					FailureDomains: map[string]clusterv1.FailureDomainSpec{
						"failure-domain-id-1": {},
						"failure-domain-id-2": {},
						"failure-domain-id-3": {},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup:       "my-rg",
					ControlPlaneEnabled: true,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: "centralIndia",
						AdditionalTags: infrav1.Tags{
							"Name": "my-publicip-ipv6",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
						},
					},
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							FrontendIPsCount: ptr.To[int32](3),
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "my-frontend-ip-1",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "pip-my-cluster-controlplane-outbound-1",
									},
								},
								{
									Name: "my-frontend-ip-2",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "pip-my-cluster-controlplane-outbound-2",
									},
								},
								{
									Name: "my-frontend-ip-3",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "pip-my-cluster-controlplane-outbound-3",
									},
								},
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type: infrav1.Internal,
							},
						},
					},
				},
			},
			expectedPublicIPSpec: []azure.ResourceSpecGetter{
				&publicips.PublicIPSpec{
					Name:           "pip-my-cluster-controlplane-outbound-1",
					ResourceGroup:  "my-rg",
					DNSName:        "",
					IsIPv6:         false,
					ClusterName:    "my-cluster",
					Location:       "centralIndia",
					FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
					AdditionalTags: infrav1.Tags{
						"Name": "my-publicip-ipv6",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
				&publicips.PublicIPSpec{
					Name:           "pip-my-cluster-controlplane-outbound-2",
					ResourceGroup:  "my-rg",
					DNSName:        "",
					IsIPv6:         false,
					ClusterName:    "my-cluster",
					Location:       "centralIndia",
					FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
					AdditionalTags: infrav1.Tags{
						"Name": "my-publicip-ipv6",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
				&publicips.PublicIPSpec{
					Name:           "pip-my-cluster-controlplane-outbound-3",
					ResourceGroup:  "my-rg",
					DNSName:        "",
					IsIPv6:         false,
					ClusterName:    "my-cluster",
					Location:       "centralIndia",
					FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
					AdditionalTags: infrav1.Tags{
						"Name": "my-publicip-ipv6",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
			},
		},
		{
			name: "Azure cluster with public type apiserver LB",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Status: infrav1.AzureClusterStatus{
					FailureDomains: map[string]clusterv1.FailureDomainSpec{
						"failure-domain-id-1": {},
						"failure-domain-id-2": {},
						"failure-domain-id-3": {},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup:       "my-rg",
					ControlPlaneEnabled: true,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: "centralIndia",
						AdditionalTags: infrav1.Tags{
							"Name": "my-publicip-ipv6",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
						},
					},
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
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
			expectedPublicIPSpec: []azure.ResourceSpecGetter{
				&publicips.PublicIPSpec{
					Name:           "40.60.89.22",
					ResourceGroup:  "my-rg",
					DNSName:        "fake-dns",
					IsIPv6:         false,
					ClusterName:    "my-cluster",
					Location:       "centralIndia",
					FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
					AdditionalTags: infrav1.Tags{
						"Name": "my-publicip-ipv6",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
			},
		},
		{
			name: "Azure cluster with public type apiserver LB and public node outbound lb",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Status: infrav1.AzureClusterStatus{
					FailureDomains: map[string]clusterv1.FailureDomainSpec{
						"failure-domain-id-1": {},
						"failure-domain-id-2": {},
						"failure-domain-id-3": {},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup:       "my-rg",
					ControlPlaneEnabled: true,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: "centralIndia",
						AdditionalTags: infrav1.Tags{
							"Name": "my-publicip-ipv6",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
						},
					},
					NetworkSpec: infrav1.NetworkSpec{
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
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
			expectedPublicIPSpec: []azure.ResourceSpecGetter{
				&publicips.PublicIPSpec{
					Name:           "40.60.89.22",
					ResourceGroup:  "my-rg",
					DNSName:        "fake-dns",
					IsIPv6:         false,
					ClusterName:    "my-cluster",
					Location:       "centralIndia",
					FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
					AdditionalTags: infrav1.Tags{
						"Name": "my-publicip-ipv6",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
			},
		},
		{
			name: "Azure cluster with public type apiserver LB and public node outbound lb, NAT gateways and bastions",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Status: infrav1.AzureClusterStatus{
					FailureDomains: map[string]clusterv1.FailureDomainSpec{
						"failure-domain-id-1": {},
						"failure-domain-id-2": {},
						"failure-domain-id-3": {},
					},
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup:       "my-rg",
					ControlPlaneEnabled: true,
					BastionSpec: infrav1.BastionSpec{
						AzureBastion: &infrav1.AzureBastion{
							PublicIP: infrav1.PublicIPSpec{
								Name:    "fake-bastion-public-ip",
								DNSName: "fake-bastion-dns-name",
							},
						},
					},
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: "centralIndia",
						AdditionalTags: infrav1.Tags{
							"Name": "my-publicip-ipv6",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
						},
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
						APIServerLB: &infrav1.LoadBalancerSpec{
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
			expectedPublicIPSpec: []azure.ResourceSpecGetter{
				&publicips.PublicIPSpec{
					Name:           "40.60.89.22",
					ResourceGroup:  "my-rg",
					DNSName:        "fake-dns",
					IsIPv6:         false,
					ClusterName:    "my-cluster",
					Location:       "centralIndia",
					FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
					AdditionalTags: infrav1.Tags{
						"Name": "my-publicip-ipv6",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
				&publicips.PublicIPSpec{
					Name:           "fake-bastion-public-ip",
					ResourceGroup:  "my-rg",
					DNSName:        "fake-bastion-dns-name",
					IsIPv6:         false,
					ClusterName:    "my-cluster",
					Location:       "centralIndia",
					FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
					AdditionalTags: infrav1.Tags{
						"Name": "my-publicip-ipv6",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.azureCluster.Name,
					Namespace: "default",
				},
			}
			clusterScope := &ClusterScope{
				Cluster:      cluster,
				AzureCluster: tc.azureCluster,
			}

			if got := clusterScope.PublicIPSpecs(); !reflect.DeepEqual(got, tc.expectedPublicIPSpec) {
				t.Errorf("PublicIPSpecs() diff between expected result and actual result (%v): %s", got, cmp.Diff(tc.expectedPublicIPSpec, got))
			}
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
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ResourceGroup: "my-rg",
							},
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
					Name:           "fake-route-table-1",
					ResourceGroup:  "my-rg",
					Location:       "centralIndia",
					ClusterName:    "my-cluster",
					AdditionalTags: make(infrav1.Tags),
				},
				&routetables.RouteTableSpec{
					Name:           "fake-route-table-2",
					ResourceGroup:  "my-rg",
					Location:       "centralIndia",
					ClusterName:    "my-cluster",
					AdditionalTags: make(infrav1.Tags),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.clusterScope.RouteTableSpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RouteTableSpecs() = %s, want %s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func TestNatGatewaySpecs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = asonetworkv1api20201101.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	tests := []struct {
		name         string
		clusterScope ClusterScope
		vnet         asonetworkv1api20201101.VirtualNetwork
		want         []azure.ASOResourceSpecGetter[*asonetworkv1api20220701.NatGateway]
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
							Vnet: infrav1.VnetSpec{
								Name: "fake-vnet-1",
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			vnet: asonetworkv1api20201101.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vnet-1",
				},
				Status: asonetworkv1api20201101.VirtualNetwork_STATUS{
					Tags: map[string]string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
			},
			want: []azure.ASOResourceSpecGetter[*asonetworkv1api20220701.NatGateway]{
				&natgateways.NatGatewaySpec{
					Name:           "fake-nat-gateway-1",
					ResourceGroup:  "my-rg",
					Location:       "centralIndia",
					SubscriptionID: "123",
					ClusterName:    "my-cluster",
					NatGatewayIP: infrav1.PublicIPSpec{
						Name: "44.78.67.90",
					},
					AdditionalTags: make(infrav1.Tags),
					IsVnetManaged:  true,
				},
			},
		},
		{
			name: "returns specified node NAT gateway if present and ignores duplicate",
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
							Vnet: infrav1.VnetSpec{
								Name: "fake-vnet-1",
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			vnet: asonetworkv1api20201101.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vnet-1",
				},
				Status: asonetworkv1api20201101.VirtualNetwork_STATUS{
					Tags: map[string]string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
			},
			want: []azure.ASOResourceSpecGetter[*asonetworkv1api20220701.NatGateway]{
				&natgateways.NatGatewaySpec{
					Name:           "fake-nat-gateway-1",
					ResourceGroup:  "my-rg",
					Location:       "centralIndia",
					SubscriptionID: "123",
					ClusterName:    "my-cluster",
					NatGatewayIP: infrav1.PublicIPSpec{
						Name: "44.78.67.90",
					},
					AdditionalTags: make(infrav1.Tags),
					IsVnetManaged:  true,
				},
			},
		},
		{
			name: "returns specified node NAT gateway if present and ignores control plane nat gateway",
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
							Vnet: infrav1.VnetSpec{
								Name: "fake-vnet-1",
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			vnet: asonetworkv1api20201101.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vnet-1",
				},
				Status: asonetworkv1api20201101.VirtualNetwork_STATUS{
					Tags: map[string]string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
			},
			want: []azure.ASOResourceSpecGetter[*asonetworkv1api20220701.NatGateway]{
				&natgateways.NatGatewaySpec{
					Name:           "fake-nat-gateway-1",
					ResourceGroup:  "my-rg",
					Location:       "centralIndia",
					SubscriptionID: "123",
					ClusterName:    "my-cluster",
					NatGatewayIP: infrav1.PublicIPSpec{
						Name: "44.78.67.90",
					},
					AdditionalTags: make(infrav1.Tags),
					IsVnetManaged:  true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fakeIdentity := &infrav1.AzureClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-identity",
					Namespace: "default",
				},
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			}
			fakeSecret := &corev1.Secret{Data: map[string][]byte{"clientSecret": []byte("fooSecret")}}

			initObjects := []runtime.Object{&tt.vnet, fakeIdentity, fakeSecret}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()
			tt.clusterScope.Client = fakeClient

			if got := tt.clusterScope.NatGatewaySpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NatGatewaySpecs() = %s, want %s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func TestSetNatGatewayIDInSubnets(t *testing.T) {
	tests := []struct {
		name          string
		clusterScope  ClusterScope
		asoNatgateway *asonetworkv1api20220701.NatGateway
	}{
		{
			name: "sets nat gateway id in the matching subnet",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: infrav1.Subnets{
								{
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Name: "fake-subnet-1",
									},
									NatGateway: infrav1.NatGateway{
										NatGatewayClassSpec: infrav1.NatGatewayClassSpec{
											Name: "fake-nat-gateway-1",
										},
									},
								},
								{
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Name: "fake-subnet-2",
									},
									NatGateway: infrav1.NatGateway{
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
			asoNatgateway: &asonetworkv1api20220701.NatGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-nat-gateway-1",
				},
				Status: asonetworkv1api20220701.NatGateway_STATUS{
					Id: ptr.To("dummy-id-1"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			tt.clusterScope.SetNatGatewayIDInSubnets(tt.asoNatgateway.Name, *tt.asoNatgateway.Status.Id)
			for _, subnet := range tt.clusterScope.AzureCluster.Spec.NetworkSpec.Subnets {
				if subnet.NatGateway.Name == tt.asoNatgateway.Name {
					g.Expect(subnet.NatGateway.ID).To(Equal(*tt.asoNatgateway.Status.Id))
				} else {
					g.Expect(subnet.NatGateway.ID).To(Equal(""))
				}
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
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							Location: "centralIndia",
						},
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ResourceGroup: "my-rg",
							},
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
					ResourceGroup:            "my-rg",
					Location:                 "centralIndia",
					ClusterName:              "my-cluster",
					AdditionalTags:           make(infrav1.Tags),
					LastAppliedSecurityRules: map[string]interface{}{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.clusterScope.NSGSpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RouteTableSpecs() = %s, want %s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func TestSubnetSpecs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = asonetworkv1api20201101.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	tests := []struct {
		name         string
		clusterScope ClusterScope
		vnet         asonetworkv1api20201101.VirtualNetwork
		want         []azure.ASOResourceSpecGetter[*asonetworkv1api20201101.VirtualNetworksSubnet]
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
			want: []azure.ASOResourceSpecGetter[*asonetworkv1api20201101.VirtualNetworksSubnet]{},
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
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetNode,
										CIDRBlocks: []string{"192.168.1.1/16"},
										Name:       "fake-subnet-1",
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
			vnet: asonetworkv1api20201101.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vnet-1",
				},
			},
			want: []azure.ASOResourceSpecGetter[*asonetworkv1api20201101.VirtualNetworksSubnet]{
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
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetBastion,
										CIDRBlocks: []string{"172.122.1.1./16"},
										Name:       "fake-bastion-subnet-1",
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
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetNode,
										CIDRBlocks: []string{"192.168.1.1/16"},
										Name:       "fake-subnet-1",
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
			vnet: asonetworkv1api20201101.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vnet-1",
				},
			},
			want: []azure.ASOResourceSpecGetter[*asonetworkv1api20201101.VirtualNetworksSubnet]{
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
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fakeIdentity := &infrav1.AzureClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-identity",
					Namespace: "default",
				},
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			}
			fakeSecret := &corev1.Secret{Data: map[string][]byte{"clientSecret": []byte("fooSecret")}}

			initObjects := []runtime.Object{&tt.vnet, fakeIdentity, fakeSecret}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()
			tt.clusterScope.Client = fakeClient

			if got := tt.clusterScope.SubnetSpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SubnetSpecs() = \n%s, want \n%s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func TestIsVnetManaged(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = asonetworkv1api20201101.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	tests := []struct {
		name         string
		clusterScope ClusterScope
		vnet         asonetworkv1api20201101.VirtualNetwork
		want         bool
	}{
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
								Name: "fake-vnet-1",
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			vnet: asonetworkv1api20201101.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vnet-1",
				},
				Status: asonetworkv1api20201101.VirtualNetwork_STATUS{
					Tags: map[string]string{
						"key": "value",
					},
				},
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
								Name: "fake-vnet-1",
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			vnet: asonetworkv1api20201101.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vnet-1",
				},
				Status: asonetworkv1api20201101.VirtualNetwork_STATUS{
					Tags: map[string]string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
					},
				},
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
					isVnetManaged: ptr.To(false),
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
					isVnetManaged: ptr.To(true),
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fakeIdentity := &infrav1.AzureClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-identity",
					Namespace: "default",
				},
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			}
			fakeSecret := &corev1.Secret{Data: map[string][]byte{"clientSecret": []byte("fooSecret")}}

			initObjects := []runtime.Object{&tt.vnet, fakeIdentity, fakeSecret}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()
			tt.clusterScope.Client = fakeClient

			got := tt.clusterScope.IsVnetManaged()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IsVnetManaged() = \n%t, want \n%t", got, tt.want)
			}
			if ptr.Deref(tt.clusterScope.cache.isVnetManaged, false) != got {
				t.Errorf("IsVnetManaged() = \n%t, cache = \n%t", got, ptr.Deref(tt.clusterScope.cache.isVnetManaged, false))
			}
		})
	}
}

func TestAzureBastionSpec(t *testing.T) {
	tests := []struct {
		name         string
		clusterScope ClusterScope
		want         azure.ASOResourceSpecGetter[*asonetworkv1api20220701.BastionHost]
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
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetBastion,
										CIDRBlocks: []string{"172.122.1.1./16"},
										Name:       "fake-bastion-subnet-1",
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
									SubnetClassSpec: infrav1.SubnetClassSpec{
										Role:       infrav1.SubnetNode,
										CIDRBlocks: []string{"192.168.1.1/16"},
										Name:       "fake-subnet-1",
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
					"virtualNetworks/%s/subnets/%s", "123", "my-rg-vnet", "fake-vnet-1", "fake-bastion-subnet-1"),
				PublicIPID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/"+
					"publicIPAddresses/%s", "123", "my-rg", "fake-public-ip-1"),
			},
		},
	}

	for _, tt := range tests {
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
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Name: "subnet-1",
						},
						ID: "subnet-1-id",
					},
					infrav1.SubnetSpec{
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Name: "subnet-2",
						},
						ID: "subnet-1-id",
					},
					infrav1.SubnetSpec{
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Name: "subnet-3",
						},
						ID: "subnet-2-id",
					},
				},
			},
			expectSubnet: infrav1.SubnetSpec{
				SubnetClassSpec: infrav1.SubnetClassSpec{
					Name: "subnet-1",
				},
				ID: "subnet-1-id",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.clusterName, func(t *testing.T) {
			g := NewWithT(t)

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: tc.azureClusterNetworkSpec,
				},
			}

			clusterScope := &ClusterScope{
				AzureCluster: azureCluster,
			}
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

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: tc.azureClusterNetworkSpec,
				},
			}

			clusterScope := &ClusterScope{
				AzureCluster: azureCluster,
			}
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

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.clusterName,
					Namespace: "default",
				},
			}
			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: tc.azureClusterNetworkSpec,
				},
			}

			clusterScope := &ClusterScope{
				Cluster:      cluster,
				AzureCluster: azureCluster,
			}
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

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: tc.lbName,
						},
					},
				},
			}

			clusterScope := &ClusterScope{
				AzureCluster: azureCluster,
			}
			clusterScope.AzureCluster.SetBackendPoolNameDefault()
			got := clusterScope.APIServerLBPoolName()
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
			expected:    "",
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

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					ControlPlaneEnabled: true,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Name: "node",
									Role: infrav1.SubnetNode,
								},
							},
						},
					},
				},
			}

			if tc.apiServerLB != nil {
				azureCluster.Spec.NetworkSpec.APIServerLB = tc.apiServerLB
			}

			if tc.controlPlaneOutboundLB != nil {
				azureCluster.Spec.NetworkSpec.ControlPlaneOutboundLB = tc.controlPlaneOutboundLB
			}

			if tc.nodeOutboundLB != nil {
				azureCluster.Spec.NetworkSpec.NodeOutboundLB = tc.nodeOutboundLB
			}

			azureCluster.Default()

			clusterScope := &ClusterScope{
				AzureCluster: azureCluster,
			}
			clusterScope.AzureCluster.SetBackendPoolNameDefault()
			got := clusterScope.OutboundLBName(tc.role)
			g.Expect(tc.expected).Should(Equal(got))
		})
	}
}

func TestBackendPoolName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		featureGate featuregate.Feature

		customAPIServerBackendPoolName    string
		customNodeBackendPoolName         string
		customControlPlaneBackendPoolName string

		expectedAPIServerBackendPoolName    string
		expectedNodeBackendPoolName         string
		expectedControlPlaneBackendPoolName string
	}{
		{
			name:                                "With default backend pool names",
			clusterName:                         "my-cluster",
			expectedAPIServerBackendPoolName:    "APIServerLBName-backendPool",
			expectedNodeBackendPoolName:         "NodeOutboundLBName-outboundBackendPool",
			expectedControlPlaneBackendPoolName: "my-cluster-outbound-lb-outboundBackendPool",
		},
		{
			name:                                "With default backend pool names feature gate enabled",
			clusterName:                         "my-cluster",
			featureGate:                         feature.APIServerILB,
			expectedAPIServerBackendPoolName:    "APIServerLBName-backendPool",
			expectedNodeBackendPoolName:         "NodeOutboundLBName-outboundBackendPool",
			expectedControlPlaneBackendPoolName: "my-cluster-outbound-lb-outboundBackendPool",
		},
		{
			name:        "With custom node backend pool name",
			clusterName: "my-cluster",

			// setting custom name for node pool name only, others should stay the same
			customNodeBackendPoolName: "custom-node-poolname",

			expectedAPIServerBackendPoolName:    "APIServerLBName-backendPool",
			expectedNodeBackendPoolName:         "custom-node-poolname",
			expectedControlPlaneBackendPoolName: "my-cluster-outbound-lb-outboundBackendPool",
		},
		{
			name:        "With custom backends pool name",
			clusterName: "my-cluster",

			// setting custom names for all backends pools
			customAPIServerBackendPoolName:    "custom-api-server-poolname",
			customNodeBackendPoolName:         "custom-node-poolname",
			customControlPlaneBackendPoolName: "custom-control-plane-poolname",

			expectedAPIServerBackendPoolName:    "custom-api-server-poolname",
			expectedNodeBackendPoolName:         "custom-node-poolname",
			expectedControlPlaneBackendPoolName: "custom-control-plane-poolname",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			if tc.featureGate == feature.APIServerILB {
				featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, tc.featureGate, true)
			}

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.clusterName,
					Namespace: "default",
				},
			}

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
					ControlPlaneEnabled: true,
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Role: infrav1.SubnetNode,
									Name: "node",
								},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "APIServerLBName",
						},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "ControlPlaneOutboundLBName",
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "NodeOutboundLBName",
						},
					},
				},
			}

			azureCluster.Default()

			if tc.customAPIServerBackendPoolName != "" {
				azureCluster.Spec.NetworkSpec.APIServerLB.BackendPool.Name = tc.customAPIServerBackendPoolName
			}

			if tc.customNodeBackendPoolName != "" {
				azureCluster.Spec.NetworkSpec.NodeOutboundLB.BackendPool.Name = tc.customNodeBackendPoolName
			}

			if tc.customControlPlaneBackendPoolName != "" {
				azureCluster.Spec.NetworkSpec.ControlPlaneOutboundLB.BackendPool.Name = tc.customControlPlaneBackendPoolName
			}

			clusterScope := &ClusterScope{
				Cluster:      cluster,
				AzureCluster: azureCluster,
			}
			clusterScope.AzureCluster.SetBackendPoolNameDefault()
			got := clusterScope.LBSpecs()
			if tc.featureGate == feature.APIServerILB {
				g.Expect(got).To(HaveLen(4))
			} else {
				g.Expect(got).To(HaveLen(3))
			}

			// API server backend pool name
			apiServerLBSpec := got[0].(*loadbalancers.LBSpec)
			g.Expect(apiServerLBSpec.BackendPoolName).To(Equal(tc.expectedAPIServerBackendPoolName))
			g.Expect(apiServerLBSpec.Role).To(Equal(infrav1.APIServerRole))

			if tc.featureGate == feature.APIServerILB {
				// API server backend pool name
				apiServerILBSpec := got[1].(*loadbalancers.LBSpec)
				g.Expect(apiServerILBSpec.BackendPoolName).To(Equal(tc.expectedAPIServerBackendPoolName + "-internal"))
				g.Expect(apiServerILBSpec.Role).To(Equal(infrav1.APIServerRoleInternal))

				// Node backend pool name
				NodeLBSpec := got[2].(*loadbalancers.LBSpec)
				g.Expect(NodeLBSpec.BackendPoolName).To(Equal(tc.expectedNodeBackendPoolName))
				g.Expect(NodeLBSpec.Role).To(Equal(infrav1.NodeOutboundRole))

				// Control Plane backend pool name
				controlPlaneLBSpec := got[3].(*loadbalancers.LBSpec)
				g.Expect(controlPlaneLBSpec.BackendPoolName).To(Equal(tc.expectedControlPlaneBackendPoolName))
				g.Expect(controlPlaneLBSpec.Role).To(Equal(infrav1.ControlPlaneOutboundRole))
			} else {
				// Node backend pool name
				NodeLBSpec := got[1].(*loadbalancers.LBSpec)
				g.Expect(NodeLBSpec.BackendPoolName).To(Equal(tc.expectedNodeBackendPoolName))
				g.Expect(NodeLBSpec.Role).To(Equal(infrav1.NodeOutboundRole))

				// Control Plane backend pool name
				controlPlaneLBSpec := got[2].(*loadbalancers.LBSpec)
				g.Expect(controlPlaneLBSpec.BackendPoolName).To(Equal(tc.expectedControlPlaneBackendPoolName))
				g.Expect(controlPlaneLBSpec.Role).To(Equal(infrav1.ControlPlaneOutboundRole))
			}
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
					ControlPlaneEnabled: true,
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			}

			if tc.loadBalancerName != "" {
				azureCluster.Spec.NetworkSpec.NodeOutboundLB = &infrav1.LoadBalancerSpec{
					Name: tc.loadBalancerName,
				}
			}

			azureCluster.Default()

			clusterScope := &ClusterScope{
				AzureCluster: azureCluster,
			}
			clusterScope.AzureCluster.SetBackendPoolNameDefault()
			got := clusterScope.OutboundPoolName(infrav1.Node)
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

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.clusterName,
					Namespace: "default",
				},
			}
			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						Location: tc.location,
					},
					ResourceGroup: tc.resourceGroup,
				},
			}

			clusterScope := &ClusterScope{
				Cluster:      cluster,
				AzureCluster: azureCluster,
			}
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

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AdditionalTags: tc.azureClusterAdditionalTags,
					},
				},
			}

			clusterScope := &ClusterScope{
				AzureCluster: azureCluster,
			}
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
				APIServerPort: ptr.To[int32](7000),
			},
			expectAPIServerPort: 7000,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

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
				},
			}

			clusterScope := &ClusterScope{
				Cluster:      cluster,
				AzureCluster: azureCluster,
			}
			got := clusterScope.APIServerPort()
			g.Expect(tc.expectAPIServerPort).Should(Equal(got))
		})
	}
}

func TestFailureDomains(t *testing.T) {
	tests := []struct {
		name                 string
		expectFailureDomains []*string
		clusterName          string
		azureClusterStatus   infrav1.AzureClusterStatus
	}{
		{
			name:                 "Empty azure cluster status",
			expectFailureDomains: []*string{},
			clusterName:          "my-cluster",
		},

		{
			name:                 "Single failure domain present in azure cluster status",
			expectFailureDomains: []*string{ptr.To("failure-domain-id")},
			clusterName:          "my-cluster",
			azureClusterStatus: infrav1.AzureClusterStatus{
				FailureDomains: map[string]clusterv1.FailureDomainSpec{
					"failure-domain-id": {},
				},
			},
		},
		{
			name:                 "Multiple failure domains present in azure cluster status",
			expectFailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
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

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Status: tc.azureClusterStatus,
			}

			clusterScope := &ClusterScope{
				AzureCluster: azureCluster,
			}
			got := clusterScope.FailureDomains()
			g.Expect(tc.expectFailureDomains).Should(ConsistOf(got))
		})
	}
}

func TestClusterScope_LBSpecs(t *testing.T) {
	tests := []struct {
		name         string
		featureGate  featuregate.Feature
		azureCluster *infrav1.AzureCluster
		want         []azure.ResourceSpecGetter
	}{
		{
			name: "API Server LB, Control Plane Oubound LB, and Node Outbound LB",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AdditionalTags: infrav1.Tags{
							"foo": "bar",
						},
						SubscriptionID: "123",
						Location:       "westus2",
					},
					ControlPlaneEnabled: true,
					ResourceGroup:       "my-rg",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							Name:          "my-vnet",
							ResourceGroup: "my-rg",
						},
						Subnets: []infrav1.SubnetSpec{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Name: "cp-subnet",
									Role: infrav1.SubnetControlPlane,
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Name: "node-subnet",
									Role: infrav1.SubnetNode,
								},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "api-server-lb",
							BackendPool: infrav1.BackendPool{
								Name: "api-server-lb-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](30),
								SKU:                  infrav1.SKUStandard,
							},
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "api-server-lb-frontend-ip",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "api-server-lb-frontend-ip",
									},
								},
							},
						},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "cp-outbound-lb",
							BackendPool: infrav1.BackendPool{
								Name: "cp-outbound-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](15),
								SKU:                  infrav1.SKUStandard,
							},
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cp-outbound-lb-frontend-ip",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "cp-outbound-lb-frontend-ip",
									},
								},
							},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "node-outbound-lb",
							BackendPool: infrav1.BackendPool{
								Name: "node-outbound-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](50),
								SKU:                  infrav1.SKUStandard,
							},
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "node-outbound-lb-frontend-ip",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "node-outbound-lb-frontend-ip",
									},
								},
							},
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{
				&loadbalancers.LBSpec{
					Name:              "api-server-lb",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					ClusterName:       "my-cluster",
					Location:          "westus2",
					VNetName:          "my-vnet",
					VNetResourceGroup: "my-rg",
					SubnetName:        "cp-subnet",
					FrontendIPConfigs: []infrav1.FrontendIP{
						{
							Name: "api-server-lb-frontend-ip",
							PublicIP: &infrav1.PublicIPSpec{
								Name: "api-server-lb-frontend-ip",
							},
						},
					},
					APIServerPort:        6443,
					Type:                 infrav1.Public,
					SKU:                  infrav1.SKUStandard,
					Role:                 infrav1.APIServerRole,
					BackendPoolName:      "api-server-lb-backend-pool",
					IdleTimeoutInMinutes: ptr.To[int32](30),
					AdditionalTags: infrav1.Tags{
						"foo": "bar",
					},
				},
				&loadbalancers.LBSpec{
					Name:              "node-outbound-lb",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					ClusterName:       "my-cluster",
					Location:          "westus2",
					VNetName:          "my-vnet",
					VNetResourceGroup: "my-rg",
					FrontendIPConfigs: []infrav1.FrontendIP{
						{
							Name: "node-outbound-lb-frontend-ip",
							PublicIP: &infrav1.PublicIPSpec{
								Name: "node-outbound-lb-frontend-ip",
							},
						},
					},
					Type:                 infrav1.Public,
					SKU:                  infrav1.SKUStandard,
					Role:                 infrav1.NodeOutboundRole,
					BackendPoolName:      "node-outbound-backend-pool",
					IdleTimeoutInMinutes: ptr.To[int32](50),
					AdditionalTags: infrav1.Tags{
						"foo": "bar",
					},
				},
				&loadbalancers.LBSpec{
					Name:              "cp-outbound-lb",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					ClusterName:       "my-cluster",
					Location:          "westus2",
					VNetName:          "my-vnet",
					VNetResourceGroup: "my-rg",
					FrontendIPConfigs: []infrav1.FrontendIP{
						{
							Name: "cp-outbound-lb-frontend-ip",
							PublicIP: &infrav1.PublicIPSpec{
								Name: "cp-outbound-lb-frontend-ip",
							},
						},
					},
					Type:                 infrav1.Public,
					SKU:                  infrav1.SKUStandard,
					BackendPoolName:      "cp-outbound-backend-pool",
					IdleTimeoutInMinutes: ptr.To[int32](15),
					Role:                 infrav1.ControlPlaneOutboundRole,
					AdditionalTags: infrav1.Tags{
						"foo": "bar",
					},
				},
			},
		},
		{
			name:        "API Server LB, Control Plane Oubound LB, and Node Outbound LB with feature gate",
			featureGate: feature.APIServerILB,
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						AdditionalTags: infrav1.Tags{
							"foo": "bar",
						},
						SubscriptionID: "123",
						Location:       "westus2",
					},
					ControlPlaneEnabled: true,
					ResourceGroup:       "my-rg",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							Name:          "my-vnet",
							ResourceGroup: "my-rg",
						},
						Subnets: []infrav1.SubnetSpec{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Name: "cp-subnet",
									Role: infrav1.SubnetControlPlane,
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Name: "node-subnet",
									Role: infrav1.SubnetNode,
								},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "api-server-lb",
							BackendPool: infrav1.BackendPool{
								Name: "api-server-lb-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](30),
								SKU:                  infrav1.SKUStandard,
							},
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "api-server-lb-frontend-ip",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "api-server-lb-frontend-ip",
									},
								},
							},
						},
						ControlPlaneOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "cp-outbound-lb",
							BackendPool: infrav1.BackendPool{
								Name: "cp-outbound-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](15),
								SKU:                  infrav1.SKUStandard,
							},
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "cp-outbound-lb-frontend-ip",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "cp-outbound-lb-frontend-ip",
									},
								},
							},
						},
						NodeOutboundLB: &infrav1.LoadBalancerSpec{
							Name: "node-outbound-lb",
							BackendPool: infrav1.BackendPool{
								Name: "node-outbound-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type:                 infrav1.Public,
								IdleTimeoutInMinutes: ptr.To[int32](50),
								SKU:                  infrav1.SKUStandard,
							},
							FrontendIPs: []infrav1.FrontendIP{
								{
									Name: "node-outbound-lb-frontend-ip",
									PublicIP: &infrav1.PublicIPSpec{
										Name: "node-outbound-lb-frontend-ip",
									},
								},
							},
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{
				&loadbalancers.LBSpec{
					Name:              "api-server-lb",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					ClusterName:       "my-cluster",
					Location:          "westus2",
					VNetName:          "my-vnet",
					VNetResourceGroup: "my-rg",
					SubnetName:        "cp-subnet",
					FrontendIPConfigs: []infrav1.FrontendIP{
						{
							Name: "api-server-lb-frontend-ip",
							PublicIP: &infrav1.PublicIPSpec{
								Name: "api-server-lb-frontend-ip",
							},
						},
					},
					APIServerPort:        6443,
					Type:                 infrav1.Public,
					SKU:                  infrav1.SKUStandard,
					Role:                 infrav1.APIServerRole,
					BackendPoolName:      "api-server-lb-backend-pool",
					IdleTimeoutInMinutes: ptr.To[int32](30),
					AdditionalTags: infrav1.Tags{
						"foo": "bar",
					},
				},
				&loadbalancers.LBSpec{
					Name:              "api-server-lb-internal",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					ClusterName:       "my-cluster",
					Location:          "westus2",
					VNetName:          "my-vnet",
					VNetResourceGroup: "my-rg",
					SubnetName:        "cp-subnet",
					FrontendIPConfigs: []infrav1.FrontendIP{
						{
							Name: "api-server-lb-internal-ip",
							FrontendIPClass: infrav1.FrontendIPClass{
								PrivateIPAddress: infrav1.DefaultInternalLBIPAddress,
							},
						},
					},
					APIServerPort:        6443,
					Type:                 infrav1.Internal,
					SKU:                  infrav1.SKUStandard,
					Role:                 infrav1.APIServerRoleInternal,
					BackendPoolName:      "api-server-lb-backend-pool-internal",
					IdleTimeoutInMinutes: ptr.To[int32](30),
					AdditionalTags: infrav1.Tags{
						"foo": "bar",
					},
				},
				&loadbalancers.LBSpec{
					Name:              "node-outbound-lb",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					ClusterName:       "my-cluster",
					Location:          "westus2",
					VNetName:          "my-vnet",
					VNetResourceGroup: "my-rg",
					FrontendIPConfigs: []infrav1.FrontendIP{
						{
							Name: "node-outbound-lb-frontend-ip",
							PublicIP: &infrav1.PublicIPSpec{
								Name: "node-outbound-lb-frontend-ip",
							},
						},
					},
					Type:                 infrav1.Public,
					SKU:                  infrav1.SKUStandard,
					Role:                 infrav1.NodeOutboundRole,
					BackendPoolName:      "node-outbound-backend-pool",
					IdleTimeoutInMinutes: ptr.To[int32](50),
					AdditionalTags: infrav1.Tags{
						"foo": "bar",
					},
				},
				&loadbalancers.LBSpec{
					Name:              "cp-outbound-lb",
					ResourceGroup:     "my-rg",
					SubscriptionID:    "123",
					ClusterName:       "my-cluster",
					Location:          "westus2",
					VNetName:          "my-vnet",
					VNetResourceGroup: "my-rg",
					FrontendIPConfigs: []infrav1.FrontendIP{
						{
							Name: "cp-outbound-lb-frontend-ip",
							PublicIP: &infrav1.PublicIPSpec{
								Name: "cp-outbound-lb-frontend-ip",
							},
						},
					},
					Type:                 infrav1.Public,
					SKU:                  infrav1.SKUStandard,
					BackendPoolName:      "cp-outbound-backend-pool",
					IdleTimeoutInMinutes: ptr.To[int32](15),
					Role:                 infrav1.ControlPlaneOutboundRole,
					AdditionalTags: infrav1.Tags{
						"foo": "bar",
					},
				},
			},
		},
		{
			name: "Private API Server LB",
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						Location:       "westus2",
					},
					ControlPlaneEnabled: true,
					ResourceGroup:       "my-rg",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							Name:          "my-vnet",
							ResourceGroup: "my-rg",
						},
						Subnets: []infrav1.SubnetSpec{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Name: "cp-subnet",
									Role: infrav1.SubnetControlPlane,
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Name: "node-subnet",
									Role: infrav1.SubnetNode,
								},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "api-server-lb",
							BackendPool: infrav1.BackendPool{
								Name: "api-server-lb-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type:                 infrav1.Internal,
								IdleTimeoutInMinutes: ptr.To[int32](30),
								SKU:                  infrav1.SKUStandard,
							},
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{
				&loadbalancers.LBSpec{
					Name:                 "api-server-lb",
					ResourceGroup:        "my-rg",
					SubscriptionID:       "123",
					ClusterName:          "my-cluster",
					Location:             "westus2",
					VNetName:             "my-vnet",
					VNetResourceGroup:    "my-rg",
					SubnetName:           "cp-subnet",
					APIServerPort:        6443,
					Type:                 infrav1.Internal,
					SKU:                  infrav1.SKUStandard,
					Role:                 infrav1.APIServerRole,
					BackendPoolName:      "api-server-lb-backend-pool",
					IdleTimeoutInMinutes: ptr.To[int32](30),
					AdditionalTags:       infrav1.Tags{},
				},
			},
		},
		{
			name:        "Private API Server LB",
			featureGate: feature.APIServerILB,
			azureCluster: &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
						Location:       "westus2",
					},
					ControlPlaneEnabled: true,
					ResourceGroup:       "my-rg",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: infrav1.VnetSpec{
							Name:          "my-vnet",
							ResourceGroup: "my-rg",
						},
						Subnets: []infrav1.SubnetSpec{
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Name: "cp-subnet",
									Role: infrav1.SubnetControlPlane,
								},
							},
							{
								SubnetClassSpec: infrav1.SubnetClassSpec{
									Name: "node-subnet",
									Role: infrav1.SubnetNode,
								},
							},
						},
						APIServerLB: &infrav1.LoadBalancerSpec{
							Name: "api-server-lb",
							BackendPool: infrav1.BackendPool{
								Name: "api-server-lb-backend-pool",
							},
							LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
								Type:                 infrav1.Internal,
								IdleTimeoutInMinutes: ptr.To[int32](30),
								SKU:                  infrav1.SKUStandard,
							},
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{
				&loadbalancers.LBSpec{
					Name:                 "api-server-lb",
					ResourceGroup:        "my-rg",
					SubscriptionID:       "123",
					ClusterName:          "my-cluster",
					Location:             "westus2",
					VNetName:             "my-vnet",
					VNetResourceGroup:    "my-rg",
					SubnetName:           "cp-subnet",
					APIServerPort:        6443,
					Type:                 infrav1.Internal,
					SKU:                  infrav1.SKUStandard,
					Role:                 infrav1.APIServerRole,
					BackendPoolName:      "api-server-lb-backend-pool",
					IdleTimeoutInMinutes: ptr.To[int32](30),
					AdditionalTags:       infrav1.Tags{},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.featureGate == feature.APIServerILB {
				featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, tc.featureGate, true)
			}
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.azureCluster.Name,
					Namespace: "default",
				},
			}

			clusterScope := &ClusterScope{
				Cluster:      cluster,
				AzureCluster: tc.azureCluster,
				AzureClients: AzureClients{
					EnvironmentSettings: auth.EnvironmentSettings{
						Values: map[string]string{
							auth.SubscriptionID: tc.azureCluster.Spec.SubscriptionID,
						},
					},
				},
			}
			if got := clusterScope.LBSpecs(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("LBSpecs() diff between expected result and actual result (%v): %s", got, cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestExtendedLocationName(t *testing.T) {
	tests := []struct {
		name             string
		clusterName      string
		extendedLocation infrav1.ExtendedLocationSpec
	}{
		{
			name:        "Empty extendedLocatioName",
			clusterName: "my-cluster",
			extendedLocation: infrav1.ExtendedLocationSpec{
				Name: "",
				Type: "",
			},
		},
		{
			name:        "Non empty extendedLocationName",
			clusterName: "my-cluster",
			extendedLocation: infrav1.ExtendedLocationSpec{
				Name: "ex-loc-name",
				Type: "ex-loc-type",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						ExtendedLocation: &infrav1.ExtendedLocationSpec{
							Name: tc.extendedLocation.Name,
							Type: tc.extendedLocation.Type,
						},
					},
				},
			}

			clusterScope := &ClusterScope{
				AzureCluster: azureCluster,
			}

			got := clusterScope.ExtendedLocationName()
			g.Expect(tc.extendedLocation.Name).Should(Equal(got))
		})
	}
}

func TestExtendedLocationType(t *testing.T) {
	tests := []struct {
		name             string
		clusterName      string
		extendedLocation infrav1.ExtendedLocationSpec
	}{
		{
			name:        "Empty extendedLocatioType",
			clusterName: "my-cluster",
			extendedLocation: infrav1.ExtendedLocationSpec{
				Name: "",
				Type: "",
			},
		},
		{
			name:        "Non empty extendedLocationType",
			clusterName: "my-cluster",
			extendedLocation: infrav1.ExtendedLocationSpec{
				Name: "ex-loc-name",
				Type: "ex-loc-type",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.clusterName,
				},
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						ExtendedLocation: &infrav1.ExtendedLocationSpec{
							Name: tc.extendedLocation.Name,
							Type: tc.extendedLocation.Type,
						},
					},
				},
			}

			clusterScope := &ClusterScope{
				AzureCluster: azureCluster,
			}

			got := clusterScope.ExtendedLocationType()
			g.Expect(tc.extendedLocation.Type).Should(Equal(got))
		})
	}
}

func TestVNetPeerings(t *testing.T) {
	fakeSubscriptionID := "123"

	tests := []struct {
		name                 string
		subscriptionID       string
		azureClusterVNetSpec infrav1.VnetSpec
		want                 []azure.ResourceSpecGetter
	}{
		{
			name:           "VNet peerings are not specified",
			subscriptionID: fakeSubscriptionID,
			azureClusterVNetSpec: infrav1.VnetSpec{
				ResourceGroup: "rg1",
				Name:          "vnet1",
			},
			want: []azure.ResourceSpecGetter{},
		},
		{
			name:           "One VNet peering is specified",
			subscriptionID: fakeSubscriptionID,
			azureClusterVNetSpec: infrav1.VnetSpec{
				ResourceGroup: "rg1",
				Name:          "vnet1",
				Peerings: infrav1.VnetPeerings{
					{
						VnetPeeringClassSpec: infrav1.VnetPeeringClassSpec{
							ResourceGroup:  "rg2",
							RemoteVnetName: "vnet2",
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{
				&vnetpeerings.VnetPeeringSpec{
					PeeringName:         "vnet1-To-vnet2",
					SourceResourceGroup: "rg1",
					SourceVnetName:      "vnet1",
					RemoteResourceGroup: "rg2",
					RemoteVnetName:      "vnet2",
					SubscriptionID:      fakeSubscriptionID,
				},
				&vnetpeerings.VnetPeeringSpec{
					PeeringName:         "vnet2-To-vnet1",
					SourceResourceGroup: "rg2",
					SourceVnetName:      "vnet2",
					RemoteResourceGroup: "rg1",
					RemoteVnetName:      "vnet1",
					SubscriptionID:      fakeSubscriptionID,
				},
			},
		},
		{
			name:           "One VNet peering with optional properties is specified",
			subscriptionID: fakeSubscriptionID,
			azureClusterVNetSpec: infrav1.VnetSpec{
				ResourceGroup: "rg1",
				Name:          "vnet1",
				Peerings: infrav1.VnetPeerings{
					{
						VnetPeeringClassSpec: infrav1.VnetPeeringClassSpec{
							ResourceGroup:  "rg2",
							RemoteVnetName: "vnet2",
							ForwardPeeringProperties: infrav1.VnetPeeringProperties{
								AllowForwardedTraffic: ptr.To(true),
								AllowGatewayTransit:   ptr.To(false),
								UseRemoteGateways:     ptr.To(true),
							},
							ReversePeeringProperties: infrav1.VnetPeeringProperties{
								AllowForwardedTraffic: ptr.To(true),
								AllowGatewayTransit:   ptr.To(true),
								UseRemoteGateways:     ptr.To(false),
							},
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{
				&vnetpeerings.VnetPeeringSpec{
					PeeringName:           "vnet1-To-vnet2",
					SourceResourceGroup:   "rg1",
					SourceVnetName:        "vnet1",
					RemoteResourceGroup:   "rg2",
					RemoteVnetName:        "vnet2",
					SubscriptionID:        fakeSubscriptionID,
					AllowForwardedTraffic: ptr.To(true),
					AllowGatewayTransit:   ptr.To(false),
					UseRemoteGateways:     ptr.To(true),
				},
				&vnetpeerings.VnetPeeringSpec{
					PeeringName:           "vnet2-To-vnet1",
					SourceResourceGroup:   "rg2",
					SourceVnetName:        "vnet2",
					RemoteResourceGroup:   "rg1",
					RemoteVnetName:        "vnet1",
					SubscriptionID:        fakeSubscriptionID,
					AllowForwardedTraffic: ptr.To(true),
					AllowGatewayTransit:   ptr.To(true),
					UseRemoteGateways:     ptr.To(false),
				},
			},
		},
		{
			name:           "Two VNet peerings are specified",
			subscriptionID: fakeSubscriptionID,
			azureClusterVNetSpec: infrav1.VnetSpec{
				ResourceGroup: "rg1",
				Name:          "vnet1",
				Peerings: infrav1.VnetPeerings{
					{
						VnetPeeringClassSpec: infrav1.VnetPeeringClassSpec{
							ResourceGroup:  "rg2",
							RemoteVnetName: "vnet2",
							ForwardPeeringProperties: infrav1.VnetPeeringProperties{
								AllowForwardedTraffic: ptr.To(true),
								AllowGatewayTransit:   ptr.To(false),
								UseRemoteGateways:     ptr.To(true),
							},
							ReversePeeringProperties: infrav1.VnetPeeringProperties{
								AllowForwardedTraffic: ptr.To(true),
								AllowGatewayTransit:   ptr.To(true),
								UseRemoteGateways:     ptr.To(false),
							},
						},
					},
					{
						VnetPeeringClassSpec: infrav1.VnetPeeringClassSpec{
							ResourceGroup:  "rg3",
							RemoteVnetName: "vnet3",
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{
				&vnetpeerings.VnetPeeringSpec{
					PeeringName:           "vnet1-To-vnet2",
					SourceResourceGroup:   "rg1",
					SourceVnetName:        "vnet1",
					RemoteResourceGroup:   "rg2",
					RemoteVnetName:        "vnet2",
					SubscriptionID:        fakeSubscriptionID,
					AllowForwardedTraffic: ptr.To(true),
					AllowGatewayTransit:   ptr.To(false),
					UseRemoteGateways:     ptr.To(true),
				},
				&vnetpeerings.VnetPeeringSpec{
					PeeringName:           "vnet2-To-vnet1",
					SourceResourceGroup:   "rg2",
					SourceVnetName:        "vnet2",
					RemoteResourceGroup:   "rg1",
					RemoteVnetName:        "vnet1",
					SubscriptionID:        fakeSubscriptionID,
					AllowForwardedTraffic: ptr.To(true),
					AllowGatewayTransit:   ptr.To(true),
					UseRemoteGateways:     ptr.To(false),
				},
				&vnetpeerings.VnetPeeringSpec{
					PeeringName:         "vnet1-To-vnet3",
					SourceResourceGroup: "rg1",
					SourceVnetName:      "vnet1",
					RemoteResourceGroup: "rg3",
					RemoteVnetName:      "vnet3",
					SubscriptionID:      fakeSubscriptionID,
				},
				&vnetpeerings.VnetPeeringSpec{
					PeeringName:         "vnet3-To-vnet1",
					SourceResourceGroup: "rg3",
					SourceVnetName:      "vnet3",
					RemoteResourceGroup: "rg1",
					RemoteVnetName:      "vnet1",
					SubscriptionID:      fakeSubscriptionID,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			clusterName := "my-cluster"
			clusterNamespace := "default"

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
			}
			azureCluster := &infrav1.AzureCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterNamespace,
				},
				Spec: infrav1.AzureClusterSpec{
					ResourceGroup: "rg1",
					NetworkSpec: infrav1.NetworkSpec{
						Vnet: tc.azureClusterVNetSpec,
					},
				},
			}

			clusterScope := &ClusterScope{
				Cluster:      cluster,
				AzureCluster: azureCluster,
				AzureClients: AzureClients{
					EnvironmentSettings: auth.EnvironmentSettings{
						Values: map[string]string{
							auth.SubscriptionID: tc.subscriptionID,
						},
					},
				},
			}
			got := clusterScope.VnetPeeringSpecs()
			g.Expect(tc.want).To(Equal(got))
		})
	}
}

func TestPrivateEndpointSpecs(t *testing.T) {
	tests := []struct {
		name         string
		clusterScope ClusterScope
		want         []azure.ASOResourceSpecGetter[*asonetworkv1api20220701.PrivateEndpoint]
	}{
		{
			name: "returns empty private endpoints list if no subnets are specified",
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
			want: make([]azure.ASOResourceSpecGetter[*asonetworkv1api20220701.PrivateEndpoint], 0),
		},
		{
			name: "returns empty private endpoints list if no private endpoints are specified",
			clusterScope: ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: []infrav1.SubnetSpec{
								{
									SubnetClassSpec: infrav1.SubnetClassSpec{
										PrivateEndpoints: infrav1.PrivateEndpoints{},
									},
								},
							},
						},
					},
				},
				cache: &ClusterCache{},
			},
			want: make([]azure.ASOResourceSpecGetter[*asonetworkv1api20220701.PrivateEndpoint], 0),
		},
		{
			name: "returns list of private endpoint specs if private endpoints are specified",
			clusterScope: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-cluster",
						Namespace: "dummy-ns",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "dummy-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Subnets: []infrav1.SubnetSpec{
								{
									ID: "dummy-subnet-id",
									SubnetClassSpec: infrav1.SubnetClassSpec{
										PrivateEndpoints: []infrav1.PrivateEndpointSpec{
											{
												Name:                       "my-private-endpoint",
												Location:                   "westus2",
												CustomNetworkInterfaceName: "my-custom-nic",
												PrivateIPAddresses: []string{
													"IP1",
													"IP2",
												},
												ApplicationSecurityGroups: []string{
													"ASG1",
													"ASG2",
												},
												PrivateLinkServiceConnections: []infrav1.PrivateLinkServiceConnection{
													{
														Name:                 "my-pls-connection",
														RequestMessage:       "my-request-message",
														PrivateLinkServiceID: "my-pls-id",
														GroupIDs: []string{
															"my-group-id-1",
														},
													},
												},
											},
											{
												Name:                       "my-private-endpoint-2",
												Location:                   "westus2",
												CustomNetworkInterfaceName: "my-custom-nic-2",
												PrivateIPAddresses: []string{
													"IP3",
													"IP4",
												},
												ApplicationSecurityGroups: []string{
													"ASG3",
													"ASG4",
												},
												PrivateLinkServiceConnections: []infrav1.PrivateLinkServiceConnection{
													{
														Name:                 "my-pls-connection",
														RequestMessage:       "my-request-message",
														PrivateLinkServiceID: "my-pls-id",
														GroupIDs: []string{
															"my-group-id-1",
														},
													},
												},
											},
										},
									},
								},
								{
									ID: "dummy-subnet-id-2",
									SubnetClassSpec: infrav1.SubnetClassSpec{
										PrivateEndpoints: []infrav1.PrivateEndpointSpec{
											{
												Name:                       "my-private-endpoint-3",
												Location:                   "westus2",
												CustomNetworkInterfaceName: "my-custom-nic-3",
												PrivateIPAddresses: []string{
													"IP5",
													"IP6",
												},
												ApplicationSecurityGroups: []string{
													"ASG5",
													"ASG6",
												},
												PrivateLinkServiceConnections: []infrav1.PrivateLinkServiceConnection{
													{
														Name:                 "my-pls-connection",
														RequestMessage:       "my-request-message",
														PrivateLinkServiceID: "my-pls-id",
														GroupIDs: []string{
															"my-group-id-1",
														},
													},
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
			want: []azure.ASOResourceSpecGetter[*asonetworkv1api20220701.PrivateEndpoint]{
				&privateendpoints.PrivateEndpointSpec{
					Name:                       "my-private-endpoint",
					ResourceGroup:              "dummy-rg",
					Location:                   "westus2",
					CustomNetworkInterfaceName: "my-custom-nic",
					PrivateIPAddresses: []string{
						"IP1",
						"IP2",
					},
					SubnetID: "dummy-subnet-id",
					ApplicationSecurityGroups: []string{
						"ASG1",
						"ASG2",
					},
					ClusterName: "my-cluster",
					PrivateLinkServiceConnections: []privateendpoints.PrivateLinkServiceConnection{
						{
							Name:                 "my-pls-connection",
							RequestMessage:       "my-request-message",
							PrivateLinkServiceID: "my-pls-id",
							GroupIDs: []string{
								"my-group-id-1",
							},
						},
					},
					AdditionalTags: make(infrav1.Tags, 0),
				},
				&privateendpoints.PrivateEndpointSpec{
					Name:                       "my-private-endpoint-2",
					ResourceGroup:              "dummy-rg",
					Location:                   "westus2",
					CustomNetworkInterfaceName: "my-custom-nic-2",
					PrivateIPAddresses: []string{
						"IP3",
						"IP4",
					},
					SubnetID: "dummy-subnet-id",
					ApplicationSecurityGroups: []string{
						"ASG3",
						"ASG4",
					},
					ClusterName: "my-cluster",
					PrivateLinkServiceConnections: []privateendpoints.PrivateLinkServiceConnection{
						{
							Name:                 "my-pls-connection",
							RequestMessage:       "my-request-message",
							PrivateLinkServiceID: "my-pls-id",
							GroupIDs: []string{
								"my-group-id-1",
							},
						},
					},
					AdditionalTags: make(infrav1.Tags, 0),
				},
				&privateendpoints.PrivateEndpointSpec{
					Name:                       "my-private-endpoint-3",
					ResourceGroup:              "dummy-rg",
					Location:                   "westus2",
					CustomNetworkInterfaceName: "my-custom-nic-3",
					PrivateIPAddresses: []string{
						"IP5",
						"IP6",
					},
					SubnetID: "dummy-subnet-id-2",
					ApplicationSecurityGroups: []string{
						"ASG5",
						"ASG6",
					},
					ClusterName: "my-cluster",
					PrivateLinkServiceConnections: []privateendpoints.PrivateLinkServiceConnection{
						{
							Name:                 "my-pls-connection",
							RequestMessage:       "my-request-message",
							PrivateLinkServiceID: "my-pls-id",
							GroupIDs: []string{
								"my-group-id-1",
							},
						},
					},
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.clusterScope.PrivateEndpointSpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PrivateEndpointSpecs() = %s, want %s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func TestSetFailureDomain(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		discoveredFDs clusterv1.FailureDomains
		specifiedFDs  clusterv1.FailureDomains
		expectedFDs   clusterv1.FailureDomains
	}{
		"no failure domains specified": {
			discoveredFDs: clusterv1.FailureDomains{
				"fd1": clusterv1.FailureDomainSpec{ControlPlane: true},
				"fd2": clusterv1.FailureDomainSpec{ControlPlane: false},
			},
			expectedFDs: clusterv1.FailureDomains{
				"fd1": clusterv1.FailureDomainSpec{ControlPlane: true},
				"fd2": clusterv1.FailureDomainSpec{ControlPlane: false},
			},
		},
		"no failure domains discovered": {
			specifiedFDs: clusterv1.FailureDomains{"fd1": clusterv1.FailureDomainSpec{ControlPlane: true}},
		},
		"failure domain specified without intersection": {
			discoveredFDs: clusterv1.FailureDomains{"fd1": clusterv1.FailureDomainSpec{ControlPlane: true}},
			specifiedFDs:  clusterv1.FailureDomains{"fd2": clusterv1.FailureDomainSpec{ControlPlane: false}},
			expectedFDs:   clusterv1.FailureDomains{"fd1": clusterv1.FailureDomainSpec{ControlPlane: true}},
		},
		"failure domain override to false succeeds": {
			discoveredFDs: clusterv1.FailureDomains{"fd1": clusterv1.FailureDomainSpec{ControlPlane: true}},
			specifiedFDs:  clusterv1.FailureDomains{"fd1": clusterv1.FailureDomainSpec{ControlPlane: false}},
			expectedFDs:   clusterv1.FailureDomains{"fd1": clusterv1.FailureDomainSpec{ControlPlane: false}},
		},
		"failure domain override to true fails": {
			discoveredFDs: clusterv1.FailureDomains{"fd1": clusterv1.FailureDomainSpec{ControlPlane: false}},
			specifiedFDs:  clusterv1.FailureDomains{"fd1": clusterv1.FailureDomainSpec{ControlPlane: true}},
			expectedFDs:   clusterv1.FailureDomains{"fd1": clusterv1.FailureDomainSpec{ControlPlane: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			c := ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
							FailureDomains: tc.specifiedFDs,
							IdentityRef: &corev1.ObjectReference{
								Kind: infrav1.AzureClusterIdentityKind,
							},
						},
					},
				},
			}

			for fdName, fd := range tc.discoveredFDs {
				c.SetFailureDomain(fdName, fd)
			}

			for fdName, fd := range tc.expectedFDs {
				g.Expect(fdName).Should(BeKeyOf(c.AzureCluster.Status.FailureDomains))
				g.Expect(c.AzureCluster.Status.FailureDomains[fdName].ControlPlane).To(Equal(fd.ControlPlane))

				delete(c.AzureCluster.Status.FailureDomains, fdName)
			}

			g.Expect(c.AzureCluster.Status.FailureDomains).To(BeEmpty())
		})
	}
}

func TestGroupSpecs(t *testing.T) {
	cases := []struct {
		name     string
		input    ClusterScope
		expected []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]
	}{
		{
			name: "virtualNetwork belongs to a different resource group",
			input: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "dummy-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ResourceGroup: "different-rg",
							},
						},
					},
				},
			},
			expected: []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
				&groups.GroupSpec{
					Name:           "dummy-rg",
					AzureName:      "dummy-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
				&groups.GroupSpec{
					Name:           "different-rg",
					AzureName:      "different-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
		{
			name: "virtualNetwork belongs to a same resource group",
			input: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "dummy-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ResourceGroup: "dummy-rg",
							},
						},
					},
				},
			},
			expected: []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
				&groups.GroupSpec{
					Name:           "dummy-rg",
					AzureName:      "dummy-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
		{
			name: "virtualNetwork resource group not specified",
			input: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "dummy-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								Name: "vnet1",
							},
						},
					},
				},
			},
			expected: []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
				&groups.GroupSpec{
					Name:           "dummy-rg",
					AzureName:      "dummy-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
		{
			name: "virtualNetwork belongs to different resource group with non-k8s name",
			input: ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						ResourceGroup: "dummy-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ResourceGroup: "my_custom_rg",
								Name:          "vnet1",
							},
						},
					},
				},
			},
			expected: []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
				&groups.GroupSpec{
					Name:           "dummy-rg",
					AzureName:      "dummy-rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
				&groups.GroupSpec{
					Name:           "my-custom-rg",
					AzureName:      "my_custom_rg",
					ClusterName:    "cluster1",
					Location:       "",
					AdditionalTags: make(infrav1.Tags, 0),
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &ClusterScope{
				AzureCluster: c.input.AzureCluster,
				Cluster:      c.input.Cluster,
			}
			if got := s.GroupSpecs(); !reflect.DeepEqual(got, c.expected) {
				t.Errorf("GroupSpecs() = %s, want %s", specArrayToString(got), specArrayToString(c.expected))
			}
		})
	}
}

func TestAPIServerLBName(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *ClusterScope
		expected string
	}{
		{
			name: "APIServerLB is not nil",
			cluster: &ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{
							APIServerLB: &infrav1.LoadBalancerSpec{
								Name: "test-lb",
							},
						},
					},
				},
			},
			expected: "test-lb",
		},
		{
			name: "APIServerLB is nil",
			cluster: &ClusterScope{
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						NetworkSpec: infrav1.NetworkSpec{},
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := tt.cluster.APIServerLBName()
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}
