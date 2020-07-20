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

package subnets

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/routetables/mock_routetables"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/securitygroups/mock_securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets/mock_subnets"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	subscriptionID = "123"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestReconcileSubnets(t *testing.T) {
	testcases := []struct {
		name          string
		subnetSpec    Spec
		vnetSpec      *infrav1.VnetSpec
		subnets       []*infrav1.SubnetSpec
		expectedError string
		expect        func(m *mock_subnets.MockClientMockRecorder, m1 *mock_routetables.MockClientMockRecorder, m2 *mock_securitygroups.MockClientMockRecorder)
	}{
		{
			name: "subnet does not exist",
			subnetSpec: Spec{
				Name:                "my-subnet",
				CIDR:                "10.0.0.0/16",
				VnetName:            "my-vnet",
				RouteTableName:      "my-subnet_route_table",
				SecurityGroupName:   "my-sg",
				Role:                infrav1.SubnetNode,
				InternalLBIPAddress: "10.0.0.10",
			},
			vnetSpec:      &infrav1.VnetSpec{Name: "my-vnet"},
			subnets:       []*infrav1.SubnetSpec{},
			expectedError: "",
			expect: func(m *mock_subnets.MockClientMockRecorder, m1 *mock_routetables.MockClientMockRecorder, m2 *mock_securitygroups.MockClientMockRecorder) {
				m.Get(context.TODO(), "", "my-vnet", "my-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))

				m1.Get(context.TODO(), "my-rg", "my-subnet_route_table").
					Return(network.RouteTable{}, nil)

				m2.Get(context.TODO(), "my-rg", "my-sg").
					Return(network.SecurityGroup{}, nil)

				m.CreateOrUpdate(context.TODO(), "", "my-vnet", "my-subnet", gomock.AssignableToTypeOf(network.Subnet{}))
			},
		},
		{
			name: "vnet was provided but subnet is missing",
			subnetSpec: Spec{
				Name:                "my-subnet",
				CIDR:                "10.0.0.0/16",
				VnetName:            "custom-vnet",
				RouteTableName:      "my-subnet_route_table",
				SecurityGroupName:   "my-sg",
				Role:                infrav1.SubnetNode,
				InternalLBIPAddress: "10.0.0.10",
			},
			vnetSpec:      &infrav1.VnetSpec{ResourceGroup: "custom-vnet-rg", Name: "custom-vnet", ID: "id1"},
			subnets:       []*infrav1.SubnetSpec{},
			expectedError: "vnet was provided but subnet my-subnet is missing",
			expect: func(m *mock_subnets.MockClientMockRecorder, m1 *mock_routetables.MockClientMockRecorder, m2 *mock_securitygroups.MockClientMockRecorder) {
				m.Get(context.TODO(), "custom-vnet-rg", "custom-vnet", "my-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "vnet was provided and subnet exists",
			subnetSpec: Spec{
				Name:                "my-subnet",
				CIDR:                "10.0.0.0/16",
				VnetName:            "my-vnet",
				RouteTableName:      "my-subnet_route_table",
				SecurityGroupName:   "my-sg",
				Role:                infrav1.SubnetNode,
				InternalLBIPAddress: "10.0.0.10",
			},
			vnetSpec: &infrav1.VnetSpec{Name: "my-vnet"},
			subnets: []*infrav1.SubnetSpec{{
				Name: "my-subnet",
				Role: infrav1.SubnetNode,
			}},
			expectedError: "",
			expect: func(m *mock_subnets.MockClientMockRecorder, m1 *mock_routetables.MockClientMockRecorder, m2 *mock_securitygroups.MockClientMockRecorder) {
				m.Get(context.TODO(), "", "my-vnet", "my-subnet").
					Return(network.Subnet{
						ID:   to.StringPtr("subnet-id"),
						Name: to.StringPtr("my-subnet"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: to.StringPtr("10.0.0.0/16"),
							RouteTable: &network.RouteTable{
								ID:   to.StringPtr("rt-id"),
								Name: to.StringPtr("my-subnet_route_table"),
							},
							NetworkSecurityGroup: &network.SecurityGroup{
								ID:   to.StringPtr("sg-id"),
								Name: to.StringPtr("my-sg"),
							},
						},
					}, nil)
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			subnetMock := mock_subnets.NewMockClient(mockCtrl)
			rtMock := mock_routetables.NewMockClient(mockCtrl)
			sgMock := mock_securitygroups.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)

			tc.expect(subnetMock.EXPECT(), rtMock.EXPECT(), sgMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet:    *tc.vnetSpec,
							Subnets: tc.subnets,
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:                clusterScope,
				Client:               subnetMock,
				SecurityGroupsClient: sgMock,
				RouteTablesClient:    rtMock,
			}

			err = s.Reconcile(context.TODO(), &tc.subnetSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteSubnets(t *testing.T) {
	testcases := []struct {
		name       string
		subnetSpec Spec
		vnetSpec   *infrav1.VnetSpec
		expect     func(m *mock_subnets.MockClientMockRecorder)
	}{
		{
			name: "subnet exists",
			subnetSpec: Spec{
				Name:                "my-subnet",
				CIDR:                "10.0.0.0/16",
				VnetName:            "my-vnet",
				RouteTableName:      "my-subnet_route_table",
				SecurityGroupName:   "my-sg",
				Role:                infrav1.SubnetNode,
				InternalLBIPAddress: "10.0.0.10",
			},
			vnetSpec: &infrav1.VnetSpec{Name: "my-vnet"},
			expect: func(m *mock_subnets.MockClientMockRecorder) {
				m.Delete(context.TODO(), "", "my-vnet", "my-subnet")
			},
		},
		{
			name: "subnet already deleted",
			subnetSpec: Spec{
				Name:                "my-subnet",
				CIDR:                "10.0.0.0/16",
				VnetName:            "my-vnet",
				RouteTableName:      "my-subnet_route_table",
				SecurityGroupName:   "my-sg",
				Role:                infrav1.SubnetNode,
				InternalLBIPAddress: "10.0.0.10",
			},
			vnetSpec: &infrav1.VnetSpec{Name: "my-vnet"},
			expect: func(m *mock_subnets.MockClientMockRecorder) {
				m.Delete(context.TODO(), "", "my-vnet", "my-subnet").
					Return(autorest.NewErrorWithResponse("", "my-vnet", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "skip delete if vnet is managed",
			subnetSpec: Spec{
				Name:                "my-subnet",
				CIDR:                "10.0.0.0/16",
				VnetName:            "custom-vnet",
				RouteTableName:      "my-subnet_route_table",
				SecurityGroupName:   "my-sg",
				Role:                infrav1.SubnetNode,
				InternalLBIPAddress: "10.0.0.10",
			},
			vnetSpec: &infrav1.VnetSpec{ResourceGroup: "custom-vnet-rg", Name: "custom-vnet", ID: "id1"},
			expect:   func(m *mock_subnets.MockClientMockRecorder) {},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			subnetMock := mock_subnets.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)

			tc.expect(subnetMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: *tc.vnetSpec,
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: subnetMock,
			}

			g.Expect(s.Delete(context.TODO(), &tc.subnetSpec)).To(Succeed())
		})
	}
}
