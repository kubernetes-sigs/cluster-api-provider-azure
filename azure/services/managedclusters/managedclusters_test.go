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

package managedclusters

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/managedclusters/mock_managedclusters"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcile(t *testing.T) {
	provisioningstatetestcases := []struct {
		name                     string
		provisioningStatesToTest []string
		expectedError            string
		expect                   func(m *mock_managedclusters.MockClientMockRecorder, provisioningstate string, s *mock_managedclusters.MockManagedClusterScopeMockRecorder)
	}{
		{
			name:                     "managedcluster in terminal provisioning state",
			provisioningStatesToTest: []string{"Canceled", "Succeeded", "Failed"},
			expectedError:            "",
			expect: func(m *mock_managedclusters.MockClientMockRecorder, provisioningstate string, s *mock_managedclusters.MockManagedClusterScopeMockRecorder) {
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-managedcluster", gomock.Any()).Return(containerservice.ManagedCluster{ManagedClusterProperties: &containerservice.ManagedClusterProperties{
					Fqdn:              pointer.String("my-managedcluster-fqdn"),
					ProvisioningState: &provisioningstate,
				}}, nil)
				m.Get(gomockinternal.AContext(), "my-rg", "my-managedcluster").Return(containerservice.ManagedCluster{ManagedClusterProperties: &containerservice.ManagedClusterProperties{
					Fqdn:              pointer.String("my-managedcluster-fqdn"),
					ProvisioningState: &provisioningstate,
				}}, nil)
				m.GetCredentials(gomockinternal.AContext(), "my-rg", "my-managedcluster").Times(1)
				s.ClusterName().AnyTimes().Return("my-managedcluster")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ManagedClusterSpec().AnyTimes().Return(azure.ManagedClusterSpec{
					Name:              "my-managedcluster",
					ResourceGroupName: "my-rg",
				}, nil)
				s.SetControlPlaneEndpoint(gomock.Any()).Times(1)
				s.SetKubeConfigData(gomock.Any()).Times(1)
			},
		},
		{
			name:                     "managedcluster in nonterminal provisioning state",
			provisioningStatesToTest: []string{"Deleting", "InProgress", "randomStringHere"},
			expectedError:            "Unable to update existing managed cluster in non terminal state. Managed cluster must be in one of the following provisioning states: canceled, failed, or succeeded. Actual state",
			expect: func(m *mock_managedclusters.MockClientMockRecorder, provisioningstate string, s *mock_managedclusters.MockManagedClusterScopeMockRecorder) {
				m.Get(gomockinternal.AContext(), "my-rg", "my-managedcluster").Return(containerservice.ManagedCluster{ManagedClusterProperties: &containerservice.ManagedClusterProperties{
					ProvisioningState: &provisioningstate,
				}}, nil)
				s.ClusterName().AnyTimes().Return("my-managedcluster")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ManagedClusterSpec().AnyTimes().Return(azure.ManagedClusterSpec{
					Name:              "my-managedcluster",
					ResourceGroupName: "my-rg",
				}, nil)
			},
		},
	}

	for _, tc := range provisioningstatetestcases {
		for _, provisioningstate := range tc.provisioningStatesToTest {
			t.Logf("Testing managedcluster provision state: " + provisioningstate)
			t.Run(tc.name, func(t *testing.T) {
				g := NewWithT(t)

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				scopeMock := mock_managedclusters.NewMockManagedClusterScope(mockCtrl)
				clientMock := mock_managedclusters.NewMockClient(mockCtrl)

				tc.expect(clientMock.EXPECT(), provisioningstate, scopeMock.EXPECT())

				s := &Service{
					Scope:  scopeMock,
					Client: clientMock,
				}

				err := s.Reconcile(context.TODO())
				if tc.expectedError != "" {
					g.Expect(err).To(HaveOccurred())
					g.Expect(err.Error()).To(HavePrefix(tc.expectedError))
					g.Expect(err.Error()).To(ContainSubstring(provisioningstate))
				} else {
					g.Expect(err).NotTo(HaveOccurred())
				}
			})
		}
	}

	testcases := []struct {
		name          string
		expectedError string
		expect        func(m *mock_managedclusters.MockClientMockRecorder, s *mock_managedclusters.MockManagedClusterScopeMockRecorder)
	}{
		{
			name:          "no managedcluster exists",
			expectedError: "",
			expect: func(m *mock_managedclusters.MockClientMockRecorder, s *mock_managedclusters.MockManagedClusterScopeMockRecorder) {
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-managedcluster", gomock.Any()).Return(containerservice.ManagedCluster{ManagedClusterProperties: &containerservice.ManagedClusterProperties{}}, nil)
				m.Get(gomockinternal.AContext(), "my-rg", "my-managedcluster").Return(containerservice.ManagedCluster{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
				m.GetCredentials(gomockinternal.AContext(), "my-rg", "my-managedcluster").Times(1)
				s.ClusterName().AnyTimes().Return("my-managedcluster")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ManagedClusterSpec().AnyTimes().Return(azure.ManagedClusterSpec{
					Name:              "my-managedcluster",
					ResourceGroupName: "my-rg",
				}, nil)
				s.GetAgentPoolSpecs(gomockinternal.AContext()).AnyTimes().Return([]azure.AgentPoolSpec{
					{
						Name:         "my-agentpool",
						SKU:          "Standard_D4s_v3",
						Replicas:     1,
						OSDiskSizeGB: 0,
					},
				}, nil)
				s.SetKubeConfigData(gomock.Any()).Times(1)
			},
		},
	}

	for _, tc := range testcases {
		t.Logf("Testing " + tc.name)
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_managedclusters.NewMockManagedClusterScope(mockCtrl)
			clientMock := mock_managedclusters.NewMockClient(mockCtrl)

			tc.expect(clientMock.EXPECT(), scopeMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
