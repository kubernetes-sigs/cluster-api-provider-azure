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

package virtualmachineextensions

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachineextensions/mock_virtualmachineextensions"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const expectedInvalidSpec = "invalid vm extension specification"

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestInvalidVMExtensions(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := gomock.NewController(t)
	vmextensionsMock := mock_virtualmachineextensions.NewMockClient(mockCtrl)

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
		Client: vmextensionsMock,
	}

	// Wrong Spec
	wrongSpec := &network.PublicIPAddress{}

	_, err = s.Get(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Reconcile(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Delete(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))
}

func TestGetVMExtensions(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name            string
		vmExtensionSpec Spec
		expectedError   string
		expect          func(m *mock_virtualmachineextensions.MockClientMockRecorder)
	}{
		{
			name: "get existing vm extension",
			vmExtensionSpec: Spec{
				Name:       "my-vmext",
				VMName:     "my-vm",
				ScriptData: "",
			},
			expectedError: "",
			expect: func(m *mock_virtualmachineextensions.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-vm", "my-vmext").Return(compute.VirtualMachineExtension{}, nil)
			},
		},
		{
			name: "vm extension not found",
			vmExtensionSpec: Spec{
				Name:       "my-vmext",
				VMName:     "my-vm",
				ScriptData: "",
			},
			expectedError: "vm extension my-vmext not found: #: Not found: StatusCode=404",
			expect: func(m *mock_virtualmachineextensions.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-vm", "my-vmext").Return(compute.VirtualMachineExtension{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "vm extension retrieval fails",
			vmExtensionSpec: Spec{
				Name:       "my-vmext",
				VMName:     "my-vm",
				ScriptData: "",
			},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_virtualmachineextensions.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-vm", "my-vmext").Return(compute.VirtualMachineExtension{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			vmExtensionMock := mock_virtualmachineextensions.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(vmExtensionMock.EXPECT())

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
				Client: vmExtensionMock,
			}

			_, err = s.Get(context.TODO(), &tc.vmExtensionSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestReconcileVMExtensions(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name            string
		vmExtensionSpec Spec
		expectedError   string
		expect          func(m *mock_virtualmachineextensions.MockClientMockRecorder)
	}{
		{
			name: "vm extension create successfully",
			vmExtensionSpec: Spec{
				Name:       "my-vmext",
				VMName:     "my-vm",
				ScriptData: "",
			},
			expectedError: "",
			expect: func(m *mock_virtualmachineextensions.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-vm", "my-vmext", gomock.AssignableToTypeOf(compute.VirtualMachineExtension{}))
			},
		},
		{
			name: "fail to create a vm extension",
			vmExtensionSpec: Spec{
				Name:       "my-vmext",
				VMName:     "my-vm",
				ScriptData: "",
			},
			expectedError: "cannot create vm extension: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_virtualmachineextensions.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-vm", "my-vmext", gomock.AssignableToTypeOf(compute.VirtualMachineExtension{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			vmExtensionMock := mock_virtualmachineextensions.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(vmExtensionMock.EXPECT())

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
							Vnet: infrav1.VnetSpec{
								ID:            "my-vnet-id",
								Name:          "my-vnet",
								ResourceGroup: "my-rg",
							},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: vmExtensionMock,
			}

			err = s.Reconcile(context.TODO(), &tc.vmExtensionSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteVMExtensions(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name            string
		vmExtensionSpec Spec
		expectedError   string
		expect          func(m *mock_virtualmachineextensions.MockClientMockRecorder)
	}{
		{
			name: "vm extension deleted successfully",
			vmExtensionSpec: Spec{
				Name:       "my-vmext",
				VMName:     "my-vm",
				ScriptData: "",
			},
			expectedError: "",
			expect: func(m *mock_virtualmachineextensions.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-vm", "my-vmext")
			},
		},
		{
			name: "vm extension already deleted",
			vmExtensionSpec: Spec{
				Name:       "my-vmext",
				VMName:     "my-vm",
				ScriptData: "",
			},
			expectedError: "",
			expect: func(m *mock_virtualmachineextensions.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-vm", "my-vmext").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
			},
		},
		{
			name: "vm extension deletion fails",
			vmExtensionSpec: Spec{
				Name:       "my-vmext",
				VMName:     "my-vm",
				ScriptData: "",
			},
			expectedError: "failed to delete vm extension my-vmext in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_virtualmachineextensions.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-vm", "my-vmext").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			vmExtensionMock := mock_virtualmachineextensions.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(vmExtensionMock.EXPECT())

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
							Vnet: infrav1.VnetSpec{
								ID:            "my-vnet-id",
								Name:          "my-vnet",
								ResourceGroup: "my-rg",
							},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: vmExtensionMock,
			}

			err = s.Delete(context.TODO(), &tc.vmExtensionSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
