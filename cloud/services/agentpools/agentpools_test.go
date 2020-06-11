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

package agentpools

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/agentpools/mock_agentpools"
)

func TestReconcile(t *testing.T) {
	g := NewWithT(t)

	provisioningstatetestcases := []struct {
		name                     string
		agentpoolspec            Spec
		provisioningStatesToTest []string
		expectedError            string
		expect                   func(m *mock_agentpools.MockClientMockRecorder, provisioningstate string)
	}{
		{
			name: "agentpool in terminal provisioning state",
			agentpoolspec: Spec{
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				Name:          "my-agentpool",
			},
			provisioningStatesToTest: []string{"Canceled", "Succeeded", "Failed"},
			expectedError:            "",
			expect: func(m *mock_agentpools.MockClientMockRecorder, provisioningstate string) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-cluster", "my-agentpool", gomock.Any()).Return(nil)
				m.Get(context.TODO(), "my-rg", "my-cluster", "my-agentpool").Return(containerservice.AgentPool{ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
					ProvisioningState: &provisioningstate,
				}}, nil)
			},
		},
		{
			name: "agentpool in nonterminal provisioning state",
			agentpoolspec: Spec{
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				Name:          "my-agentpool",
			},
			provisioningStatesToTest: []string{"Deleting", "InProgress", "randomStringHere"},
			expectedError:            "",
			expect: func(m *mock_agentpools.MockClientMockRecorder, provisioningstate string) {
				m.Get(context.TODO(), "my-rg", "my-cluster", "my-agentpool").Return(containerservice.AgentPool{ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
					ProvisioningState: &provisioningstate,
				}}, nil)
			},
		},
	}

	for _, tc := range provisioningstatetestcases {
		for _, provisioningstate := range tc.provisioningStatesToTest {
			t.Logf("Testing agentpool provision state: " + provisioningstate)
			t.Run(tc.name, func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				agentpoolsMock := mock_agentpools.NewMockClient(mockCtrl)

				tc.expect(agentpoolsMock.EXPECT(), provisioningstate)

				s := &Service{
					Client: agentpoolsMock,
				}

				err := s.Reconcile(context.TODO(), &tc.agentpoolspec)
				if tc.expectedError != "" {
					g.Expect(err).To(HaveOccurred())
					g.Expect(err).To(MatchError(tc.expectedError))
				} else {
					g.Expect(err).NotTo(HaveOccurred())
					mockCtrl.Finish()
				}
			})
		}
	}

	testcases := []struct {
		name          string
		agentpoolspec Spec
		expectedError string
		expect        func(m *mock_agentpools.MockClientMockRecorder)
	}{
		{
			name: "no agentpool exists",
			agentpoolspec: Spec{
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				Name:          "my-agentpool",
			},
			expectedError: "",
			expect: func(m *mock_agentpools.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-cluster", "my-agentpool", gomock.Any()).Return(nil)
				m.Get(context.TODO(), "my-rg", "my-cluster", "my-agentpool").Return(containerservice.AgentPool{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
			},
		},
	}

	for _, tc := range testcases {
		t.Logf("Testing " + tc.name)
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			agentpoolsMock := mock_agentpools.NewMockClient(mockCtrl)

			tc.expect(agentpoolsMock.EXPECT())

			s := &Service{
				Client: agentpoolsMock,
			}

			err := s.Reconcile(context.TODO(), &tc.agentpoolspec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				mockCtrl.Finish()
			}
		})
	}
}
