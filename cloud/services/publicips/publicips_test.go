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

package publicips

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"

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

const expectedInvalidSpec = "invalid PublicIP Specification"

func TestInvalidPublicIPSpec(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := gomock.NewController(t)
	publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}

	client := fake.NewFakeClient(cluster)

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
	g.Expect(err).NotTo(HaveOccurred())

	s := &Service{
		Scope:  clusterScope,
		Client: publicIPsMock,
	}

	// Wrong Spec
	wrongSpec := &network.LoadBalancer{}

	err = s.Reconcile(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	_, err = s.Get(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Delete(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))
}

func TestGetPublicIP(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name          string
		publicIPsSpec Spec
		expectedError string
		expect        func(m *mock_publicips.MockClientMockRecorder)
	}{
		{
			name: "get existing publicip",
			publicIPsSpec: Spec{
				Name: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicips.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil)
			},
		},
		{
			name: "publicip not found",
			publicIPsSpec: Spec{
				Name: "my-publicip",
			},
			expectedError: "publicip my-publicip not found: #: Not found: StatusCode=404",
			expect: func(m *mock_publicips.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "publicip retrieval fails",
			publicIPsSpec: Spec{
				Name: "my-publicip",
			},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicips.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicIPsMock.EXPECT())

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
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: publicIPsMock,
			}

			_, err = s.Get(context.TODO(), &tc.publicIPsSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestReconcilePublicLoadBalancer(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name          string
		publicIPsSpec Spec
		expectedError string
		expect        func(m *mock_publicips.MockClientMockRecorder)
	}{
		{
			name: "can create a public IP",
			publicIPsSpec: Spec{
				Name: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publicip", gomock.AssignableToTypeOf(network.PublicIPAddress{}))
			},
		},
		{
			name: "fail to create a public IP",
			publicIPsSpec: Spec{
				Name: "my-publicip",
			},
			expectedError: "cannot create public ip: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publicip", gomock.AssignableToTypeOf(network.PublicIPAddress{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicIPsMock.EXPECT())

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
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: publicIPsMock,
			}

			err = s.Reconcile(context.TODO(), &tc.publicIPsSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeletePublicIP(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name          string
		publicIPsSpec Spec
		expectedError string
		expect        func(m *mock_publicips.MockClientMockRecorder)
	}{
		{
			name: "successfully delete an existing public ip",
			publicIPsSpec: Spec{
				Name: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicips.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-publicip")
			},
		},
		{
			name: "public ip already deleted",
			publicIPsSpec: Spec{
				Name: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicips.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-publicip").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "public ip deletion fails",
			publicIPsSpec: Spec{
				Name: "my-publicip",
			},
			expectedError: "failed to delete public ip my-publicip in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicips.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-publicip").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicIPsMock.EXPECT())

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
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: publicIPsMock,
			}

			err = s.Delete(context.TODO(), &tc.publicIPsSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
