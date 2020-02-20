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

package publicloadbalancers

import (
	"context"
	"net/http"
	"testing"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicloadbalancers/mock_publicloadbalancers"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestGetPublicLB(t *testing.T) {
	testcases := []struct {
		name          string
		publicLBSpec  Spec
		expectedError string
		expect        func(m *mock_publicloadbalancers.MockClientMockRecorder)
	}{
		{
			name: "get existing public load balancer",
			publicLBSpec: Spec{
				Name: "my-publiclb",
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{}, nil)
			},
		},
		{
			name: "public load balancer not found",
			publicLBSpec: Spec{
				Name: "my-publiclb",
			},
			expectedError: "load balancer my-publiclb not found: #: Not found: StatusCode=404",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "public load balancer retrieval fails",
			publicLBSpec: Spec{
				Name: "my-publiclb",
			},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicLBMock := mock_publicloadbalancers.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicLBMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create test context: %v", err)
			}

			s := &Service{
				Scope:  clusterScope,
				Client: publicLBMock,
			}

			_, err = s.Get(context.TODO(), &tc.publicLBSpec)
			if err != nil {
				if tc.expectedError == "" || err.Error() != tc.expectedError {
					t.Fatalf("got an unexpected error: %v", err)
				}
			} else {
				if tc.expectedError != "" {
					t.Fatalf("expected an error: %v", tc.expectedError)

				}
			}
		})
	}
}

func TestReconcilePublicLoadBalancer(t *testing.T) {
	testcases := []struct {
		name          string
		publicLBSpec  Spec
		expectedError string
		expect        func(m *mock_publicloadbalancers.MockClientMockRecorder,
			publicIP *mock_publicips.MockClientMockRecorder)
	}{
		{
			name: "public IP does not exist",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "public ip my-publicip not found in RG my-rg: #: Not found: StatusCode=404",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{}))
				publicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "public IP retrieval fails",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "failed to look for existing public IP: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{}))
				publicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "successfully create a public LB",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{})).Return(nil)
				publicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil)
			},
		},
		{
			name: "fail to create a public LB",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "cannot create public load balancer: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				publicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicLBMock := mock_publicloadbalancers.NewMockClient(mockCtrl)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicLBMock.EXPECT(), publicIPsMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create test context: %v", err)
			}

			s := &Service{
				Scope:           clusterScope,
				Client:          publicLBMock,
				PublicIPsClient: publicIPsMock,
			}

			if err := s.Reconcile(context.TODO(), &tc.publicLBSpec); err != nil {
				if tc.expectedError == "" || err.Error() != tc.expectedError {
					t.Fatalf("got an unexpected error: %v", err)
				}
			} else {
				if tc.expectedError != "" {
					t.Fatalf("expected an error: %v", tc.expectedError)

				}
			}
		})
	}
}

func TestDeletePublicLB(t *testing.T) {
	testcases := []struct {
		name          string
		publicLBSpec  Spec
		expectedError string
		expect        func(m *mock_publicloadbalancers.MockClientMockRecorder)
	}{
		{
			name: "successfully delete an existing public load balancer",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-publiclb")
			},
		},
		{
			name: "public load balancer already deleted",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-publiclb").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "internal load balancer deletion fails",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "failed to delete public load balancer my-publiclb in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-publiclb").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicLBMock := mock_publicloadbalancers.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicLBMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create test context: %v", err)
			}

			s := &Service{
				Scope:  clusterScope,
				Client: publicLBMock,
			}

			if err := s.Delete(context.TODO(), &tc.publicLBSpec); err != nil {
				if tc.expectedError == "" || err.Error() != tc.expectedError {
					t.Fatalf("got an unexpected error: %v", err)
				}
			} else {
				if tc.expectedError != "" {
					t.Fatalf("expected an error: %v", tc.expectedError)

				}
			}
		})
	}
}
