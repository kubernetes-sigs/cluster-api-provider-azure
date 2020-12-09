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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/managedclusters/mock_managedclusters"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcile(t *testing.T) {
	provisioningstatetestcases := []struct {
		name                     string
		managedclusterspec       Spec
		provisioningStatesToTest []string
		expectedError            string
		expect                   func(m *mock_managedclusters.MockClientMockRecorder, provisioningstate string)
	}{
		{
			name: "managedcluster in terminal provisioning state",
			managedclusterspec: Spec{
				Name:              "my-managedcluster",
				ResourceGroupName: "my-rg",
			},
			provisioningStatesToTest: []string{"Canceled", "Succeeded", "Failed"},
			expectedError:            "",
			expect: func(m *mock_managedclusters.MockClientMockRecorder, provisioningstate string) {
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-managedcluster", gomock.Any()).Return(nil)
				m.Get(gomockinternal.AContext(), "my-rg", "my-managedcluster").Return(containerservice.ManagedCluster{ManagedClusterProperties: &containerservice.ManagedClusterProperties{
					ProvisioningState: &provisioningstate,
				}}, nil)
			},
		},
		{
			name: "managedcluster in nonterminal provisioning state",
			managedclusterspec: Spec{
				Name:              "my-managedcluster",
				ResourceGroupName: "my-rg",
			},
			provisioningStatesToTest: []string{"Deleting", "InProgress", "randomStringHere"},
			expectedError:            "",
			expect: func(m *mock_managedclusters.MockClientMockRecorder, provisioningstate string) {
				m.Get(gomockinternal.AContext(), "my-rg", "my-managedcluster").Return(containerservice.ManagedCluster{ManagedClusterProperties: &containerservice.ManagedClusterProperties{
					ProvisioningState: &provisioningstate,
				}}, nil)
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

				managedclusterMock := mock_managedclusters.NewMockClient(mockCtrl)

				tc.expect(managedclusterMock.EXPECT(), provisioningstate)

				s := &Service{
					Client: managedclusterMock,
				}

				err := s.Reconcile(context.TODO(), &tc.managedclusterspec)
				if tc.expectedError != "" {
					g.Expect(err).To(HaveOccurred())
					g.Expect(err).To(MatchError(tc.expectedError))
				} else {
					g.Expect(err).NotTo(HaveOccurred())
				}
			})
		}
	}

	testcases := []struct {
		name               string
		managedclusterspec Spec
		expectedError      string
		expect             func(m *mock_managedclusters.MockClientMockRecorder)
	}{
		{
			name: "no managedcluster exists",
			managedclusterspec: Spec{
				Name:              "my-managedcluster",
				ResourceGroupName: "my-rg",
			},
			expectedError: "",
			expect: func(m *mock_managedclusters.MockClientMockRecorder) {
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-managedcluster", gomock.Any()).Return(nil)
				m.Get(gomockinternal.AContext(), "my-rg", "my-managedcluster").Return(containerservice.ManagedCluster{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
			},
		},
	}

	for _, tc := range testcases {
		t.Logf("Testing " + tc.name)
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			managedclusterMock := mock_managedclusters.NewMockClient(mockCtrl)

			tc.expect(managedclusterMock.EXPECT())

			s := &Service{
				Client: managedclusterMock,
			}

			err := s.Reconcile(context.TODO(), &tc.managedclusterspec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
