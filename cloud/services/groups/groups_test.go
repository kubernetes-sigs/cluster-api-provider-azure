/*
Copyright 2020 The Kubernetes Authors.

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

package groups

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/groups/mock_groups"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

const (
	subscriptionID = "123"
)

func TestReconcileGroups(t *testing.T) {
	testcases := []struct {
		name               string
		clusterScopeParams scope.ClusterScopeParams
		expectedError      string
		expect             func(m *mock_groups.MockClientMockRecorder)
	}{
		{
			name: "resource group already exist",
			clusterScopeParams: scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			},
			expectedError: "",
			expect: func(m *mock_groups.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg").Return(resources.Group{}, nil)
			},
		},
		{
			name: "create a resource group",
			clusterScopeParams: scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			},
			expectedError: "",
			expect: func(m *mock_groups.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "", gomock.AssignableToTypeOf(resources.Group{})).Return(resources.Group{}, nil)
				m.Get(context.TODO(), "").Return(resources.Group{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "return error when creating a resource group",
			clusterScopeParams: scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_groups.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "", gomock.AssignableToTypeOf(resources.Group{})).Return(resources.Group{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				m.Get(context.TODO(), "").Return(resources.Group{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			groupsMock := mock_groups.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)

			tc.expect(groupsMock.EXPECT())

			tc.clusterScopeParams.Client = client
			tc.clusterScopeParams.Cluster = cluster
			clusterScope, err := scope.NewClusterScope(tc.clusterScopeParams)
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: groupsMock,
			}

			err = s.Reconcile(context.TODO(), nil)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteGroups(t *testing.T) {
	testcases := []struct {
		name               string
		clusterScopeParams scope.ClusterScopeParams
		expectedError      string
		expect             func(m *mock_groups.MockClientMockRecorder)
	}{
		{
			name: "error getting the resource group management state",
			clusterScopeParams: scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						SubscriptionID: subscriptionID,
						ResourceGroup:  "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			},
			expectedError: "could not get resource group management state: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_groups.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg").Return(resources.Group{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "skip deletion in unmanaged mode",
			clusterScopeParams: scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						SubscriptionID: subscriptionID,
						ResourceGroup:  "my-rg",
					},
				},
			},
			expectedError: "",
			expect: func(m *mock_groups.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg").Return(resources.Group{}, nil)
			},
		},
		{
			name: "resource group already deleted",
			clusterScopeParams: scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						SubscriptionID: subscriptionID,
						ResourceGroup:  "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
						AdditionalTags: infrav1.Tags{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
						},
					},
				},
			},
			expectedError: "",
			expect: func(m *mock_groups.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
				m.Get(context.TODO(), "my-rg").Return(resources.Group{
					Tags: converters.TagsToMap(infrav1.Tags{
						"Name": "my-rg",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
						"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
					}),
				}, nil)
			},
		},
		{
			name: "resource group deletion fails",
			clusterScopeParams: scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						SubscriptionID: subscriptionID,
						ResourceGroup:  "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
						AdditionalTags: infrav1.Tags{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
						},
					},
				},
			},
			expectedError: "failed to delete resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_groups.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				m.Get(context.TODO(), "my-rg").Return(resources.Group{
					Tags: converters.TagsToMap(infrav1.Tags{
						"Name": "my-rg",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
						"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
					}),
				}, nil)
			},
		},
		{
			name: "resource group deletion successfully",
			clusterScopeParams: scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						SubscriptionID: subscriptionID,
						ResourceGroup:  "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
						AdditionalTags: infrav1.Tags{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
						},
					},
				},
			},
			expectedError: "",
			expect: func(m *mock_groups.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg").Return(nil)
				m.Get(context.TODO(), "my-rg").Return(resources.Group{
					Tags: converters.TagsToMap(infrav1.Tags{
						"Name": "my-rg",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
						"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
					}),
				}, nil)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			groupsMock := mock_groups.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)

			tc.expect(groupsMock.EXPECT())

			tc.clusterScopeParams.Client = client
			tc.clusterScopeParams.Cluster = cluster
			clusterScope, err := scope.NewClusterScope(tc.clusterScopeParams)
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: groupsMock,
			}

			err = s.Delete(context.TODO(), nil)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
