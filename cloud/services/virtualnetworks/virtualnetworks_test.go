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

package virtualnetworks

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks/mock_virtualnetworks"

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

func TestReconcileVnet(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name   string
		input  *infrav1.VnetSpec
		output *infrav1.VnetSpec
		expect func(m *mock_virtualnetworks.MockClientMockRecorder)
	}{
		{
			name:  "managed vnet exists",
			input: &infrav1.VnetSpec{ResourceGroup: "my-rg", Name: "vnet-exists"},
			output: &infrav1.VnetSpec{ResourceGroup: "my-rg", ID: "azure/fake/id", Name: "vnet-exists", CidrBlock: "10.0.0.0/8", Tags: infrav1.Tags{
				"Name": "vnet-exists",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			}},
			expect: func(m *mock_virtualnetworks.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "vnet-exists").
					Return(network.VirtualNetwork{
						ID:   to.StringPtr("azure/fake/id"),
						Name: to.StringPtr("vnet-exists"),
						VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
							AddressSpace: &network.AddressSpace{
								AddressPrefixes: to.StringSlicePtr([]string{"10.0.0.0/8"}),
							},
						},
						Tags: map[string]*string{
							"Name": to.StringPtr("vnet-exists"),
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
							"sigs.k8s.io_cluster-api-provider-azure_role":                 to.StringPtr("common"),
						},
					}, nil)
			},
		},
		{
			name:   "managed vnet does not exist",
			input:  &infrav1.VnetSpec{ResourceGroup: "my-rg", Name: "vnet-new", CidrBlock: "10.0.0.0/8"},
			output: &infrav1.VnetSpec{ResourceGroup: "my-rg", Name: "vnet-new", CidrBlock: "10.0.0.0/8"},
			expect: func(m *mock_virtualnetworks.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "vnet-new").
					Return(network.VirtualNetwork{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))

				m.CreateOrUpdate(context.TODO(), "my-rg", "vnet-new", gomock.AssignableToTypeOf(network.VirtualNetwork{}))
			},
		},
		{
			name:   "unmanaged vnet exists",
			input:  &infrav1.VnetSpec{ResourceGroup: "custom-vnet-rg", Name: "custom-vnet", CidrBlock: "10.0.0.0/16"},
			output: &infrav1.VnetSpec{ResourceGroup: "custom-vnet-rg", ID: "azure/custom-vnet/id", Name: "custom-vnet", CidrBlock: "10.0.0.0/16", Tags: infrav1.Tags{"Name": "my-custom-vnet"}},
			expect: func(m *mock_virtualnetworks.MockClientMockRecorder) {
				m.Get(context.TODO(), "custom-vnet-rg", "custom-vnet").
					Return(network.VirtualNetwork{
						ID:   to.StringPtr("azure/custom-vnet/id"),
						Name: to.StringPtr("custom-vnet"),
						VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
							AddressSpace: &network.AddressSpace{
								AddressPrefixes: to.StringSlicePtr([]string{"10.0.0.0/16"}),
							},
						},
						Tags: map[string]*string{
							"Name": to.StringPtr("my-custom-vnet"),
						},
					}, nil)
			},
		},
		{
			name:   "custom vnet not found",
			input:  &infrav1.VnetSpec{ResourceGroup: "custom-vnet-rg", Name: "custom-vnet", CidrBlock: "10.0.0.0/16"},
			output: &infrav1.VnetSpec{ResourceGroup: "custom-vnet-rg", Name: "custom-vnet", CidrBlock: "10.0.0.0/16"},
			expect: func(m *mock_virtualnetworks.MockClientMockRecorder) {
				m.Get(context.TODO(), "custom-vnet-rg", "custom-vnet").
					Return(network.VirtualNetwork{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))

				m.CreateOrUpdate(context.TODO(), "custom-vnet-rg", "custom-vnet", gomock.AssignableToTypeOf(network.VirtualNetwork{}))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			vnetMock := mock_virtualnetworks.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec:       clusterv1.ClusterSpec{},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(vnetMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: *tc.input,
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: vnetMock,
			}

			vnetSpec := &Spec{
				Name:          clusterScope.Vnet().Name,
				ResourceGroup: clusterScope.Vnet().ResourceGroup,
				CIDR:          clusterScope.Vnet().CidrBlock,
			}

			err = s.Reconcile(context.TODO(), vnetSpec)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(clusterScope.Vnet()).To(Equal(tc.output))

			if !reflect.DeepEqual(clusterScope.Vnet(), tc.output) {
				expected, _ := json.MarshalIndent(tc.output, "", "\t")
				actual, _ := json.MarshalIndent(clusterScope.Vnet(), "", "\t")
				t.Errorf("Expected %s, got %s", string(expected), string(actual))
			}
		})
	}
}

func TestDeleteVnet(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name   string
		input  *infrav1.VnetSpec
		expect func(m *mock_virtualnetworks.MockClientMockRecorder)
	}{
		{
			name: "managed vnet exists",
			input: &infrav1.VnetSpec{ResourceGroup: "my-rg", Name: "vnet-exists", ID: "azure/vnet/id", Tags: infrav1.Tags{
				"Name": "vnet-exists",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			}},
			expect: func(m *mock_virtualnetworks.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "vnet-exists")
			},
		},
		{
			name: "managed vnet already deleted",
			input: &infrav1.VnetSpec{ResourceGroup: "my-rg", Name: "vnet-exists", ID: "azure/vnet/id", Tags: infrav1.Tags{
				"Name": "vnet-exists",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			}},
			expect: func(m *mock_virtualnetworks.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "vnet-exists").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:   "unmanaged vnet",
			input:  &infrav1.VnetSpec{ResourceGroup: "my-rg", Name: "my-vnet", ID: "azure/custom-vnet/id"},
			expect: func(m *mock_virtualnetworks.MockClientMockRecorder) {},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			vnetMock := mock_virtualnetworks.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(vnetMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: *tc.input,
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: vnetMock,
			}

			vnetSpec := &Spec{
				Name:          clusterScope.Vnet().Name,
				ResourceGroup: clusterScope.Vnet().ResourceGroup,
				CIDR:          clusterScope.Vnet().CidrBlock,
			}

			g.Expect(s.Delete(context.TODO(), vnetSpec)).To(Succeed())
		})
	}
}
