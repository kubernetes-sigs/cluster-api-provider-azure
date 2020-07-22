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

package securitygroups

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/securitygroups/mock_securitygroups"

	"github.com/Azure/go-autorest/autorest"
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

func TestReconcileSecurityGroups(t *testing.T) {
	testcases := []struct {
		name           string
		sgName         string
		isControlPlane bool
		vnetSpec       *infrav1.VnetSpec
		expect         func(m *mock_securitygroups.MockClientMockRecorder, m1 *mock_securitygroups.MockClientMockRecorder)
	}{
		{
			name:           "security group does not exists",
			sgName:         "my-sg",
			isControlPlane: true,
			vnetSpec:       &infrav1.VnetSpec{},
			expect: func(m *mock_securitygroups.MockClientMockRecorder, m1 *mock_securitygroups.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-sg")
				m1.CreateOrUpdate(context.TODO(), "my-rg", "my-sg", gomock.AssignableToTypeOf(network.SecurityGroup{}))
			},
		}, {
			name:           "security group does not exist and it's not for a control plane",
			sgName:         "my-sg",
			isControlPlane: false,
			vnetSpec:       &infrav1.VnetSpec{},
			expect: func(m *mock_securitygroups.MockClientMockRecorder, m1 *mock_securitygroups.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-sg")
				m1.CreateOrUpdate(context.TODO(), "my-rg", "my-sg", gomock.AssignableToTypeOf(network.SecurityGroup{}))
			},
		}, {
			name:           "skipping network security group reconcile in custom vnet mode",
			sgName:         "my-sg",
			isControlPlane: false,
			vnetSpec:       &infrav1.VnetSpec{ResourceGroup: "custom-vnet-rg", Name: "custom-vnet", ID: "id1"},
			expect: func(m *mock_securitygroups.MockClientMockRecorder, m1 *mock_securitygroups.MockClientMockRecorder) {

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

			sgMock := mock_securitygroups.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)

			tc.expect(sgMock.EXPECT(), sgMock.EXPECT())

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
				Client: sgMock,
			}

			sgSpec := &Spec{
				Name:           tc.sgName,
				IsControlPlane: tc.isControlPlane,
			}
			g.Expect(s.Reconcile(context.TODO(), sgSpec)).To(Succeed())
		})
	}
}

func TestDeleteSecurityGroups(t *testing.T) {
	testcases := []struct {
		name   string
		sgName string
		expect func(m *mock_securitygroups.MockClientMockRecorder)
	}{
		{
			name:   "security group exists",
			sgName: "my-sg",
			expect: func(m *mock_securitygroups.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-sg")
			},
		},
		{
			name:   "security group already deleted",
			sgName: "my-sg",
			expect: func(m *mock_securitygroups.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-sg").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
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

			sgMock := mock_securitygroups.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)

			tc.expect(sgMock.EXPECT())

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
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: sgMock,
			}

			sgSpec := &Spec{
				Name:           tc.sgName,
				IsControlPlane: false,
			}

			g.Expect(s.Delete(context.TODO(), sgSpec)).To(Succeed())
		})
	}
}
