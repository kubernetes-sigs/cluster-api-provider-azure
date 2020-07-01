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
	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/agentpools/mock_agentpools"
)

const (
	expectedInvalidSpec = "invalid agent pool specification"
)

func TestInvalidAgentPoolsSpec(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	agentpoolsMock := mock_agentpools.NewMockClient(mockCtrl)

	s := &Service{
		Client: agentpoolsMock,
	}

	// Wrong Spec
	wrongSpec := &network.LoadBalancer{}

	err := s.Reconcile(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Delete(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))
}

func TestReconcile(t *testing.T) {
	provisioningstatetestcases := []struct {
		name                     string
		agentpoolSpec            Spec
		provisioningStatesToTest []string
		expectedError            string
		expect                   func(m *mock_agentpools.MockClientMockRecorder, provisioningstate string)
	}{
		{
			name: "agentpool in terminal provisioning state",
			agentpoolSpec: Spec{
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
			agentpoolSpec: Spec{
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
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				g := NewWithT(t)
				t.Parallel()

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()

				agentpoolsMock := mock_agentpools.NewMockClient(mockCtrl)

				tc.expect(agentpoolsMock.EXPECT(), provisioningstate)

				s := &Service{
					Client: agentpoolsMock,
				}

				err := s.Reconcile(context.TODO(), &tc.agentpoolSpec)
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
		name           string
		agentPoolsSpec Spec
		expectedError  string
		expect         func(m *mock_agentpools.MockClientMockRecorder)
	}{
		{
			name: "no agentpool exists",
			agentPoolsSpec: Spec{
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
		{
			name: "fail to get existing agent pool",
			agentPoolsSpec: Spec{
				Name:          "my-agent-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "SKU123",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
			},
			expectedError: "failed to get existing agent pool: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_agentpools.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-cluster", "my-agent-pool").Return(containerservice.AgentPool{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "can create an Agent Pool",
			agentPoolsSpec: Spec{
				Name:          "my-agent-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "SKU123",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
			},
			expectedError: "",
			expect: func(m *mock_agentpools.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-cluster", "my-agent-pool").Return(containerservice.AgentPool{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-cluster", "my-agent-pool", gomock.AssignableToTypeOf(containerservice.AgentPool{})).Return(nil)
			},
		},
		{
			name: "fail to create an Agent Pool",
			agentPoolsSpec: Spec{
				Name:          "my-agent-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "SKU123",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
			},
			expectedError: "failed to create or update agent pool: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_agentpools.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-cluster", "my-agent-pool").Return(containerservice.AgentPool{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-cluster", "my-agent-pool", gomock.AssignableToTypeOf(containerservice.AgentPool{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "fail to update an Agent Pool",
			agentPoolsSpec: Spec{
				Name:          "my-agent-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "SKU123",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
			},
			expectedError: "failed to create or update agent pool: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_agentpools.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-cluster", "my-agent-pool").Return(containerservice.AgentPool{
					ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
						Count:               to.Int32Ptr(3),
						OsDiskSizeGB:        to.Int32Ptr(20),
						VMSize:              containerservice.VMSizeTypesStandardA1,
						OrchestratorVersion: to.StringPtr("9.99.9999"),
						ProvisioningState:   to.StringPtr("Failed"),
					},
				}, nil)
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-cluster", "my-agent-pool", gomock.AssignableToTypeOf(containerservice.AgentPool{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "no update needed on Agent Pool",
			agentPoolsSpec: Spec{
				Name:          "my-agent-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "Standard_D2s_v3",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
			},
			expectedError: "",
			expect: func(m *mock_agentpools.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-cluster", "my-agent-pool").Return(containerservice.AgentPool{
					ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
						Count:               to.Int32Ptr(2),
						OsDiskSizeGB:        to.Int32Ptr(100),
						VMSize:              containerservice.VMSizeTypesStandardD2sV3,
						OrchestratorVersion: to.StringPtr("9.99.9999"),
						ProvisioningState:   to.StringPtr("Succeeded"),
					},
				}, nil)
			},
		},
	}

	for _, tc := range testcases {
		t.Logf("Testing " + tc.name)
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			agentpoolsMock := mock_agentpools.NewMockClient(mockCtrl)

			tc.expect(agentpoolsMock.EXPECT())

			s := &Service{
				Client: agentpoolsMock,
			}

			err := s.Reconcile(context.TODO(), &tc.agentPoolsSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteAgentPools(t *testing.T) {
	testcases := []struct {
		name           string
		agentPoolsSpec Spec
		expectedError  string
		expect         func(m *mock_agentpools.MockClientMockRecorder)
	}{
		{
			name: "successfully delete an existing agent pool",
			agentPoolsSpec: Spec{
				Name:          "my-agent-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
			},
			expectedError: "",
			expect: func(m *mock_agentpools.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-cluster", "my-agent-pool")
			},
		},
		{
			name: "agent pool already deleted",
			agentPoolsSpec: Spec{
				Name:          "my-agent-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
			},
			expectedError: "",
			expect: func(m *mock_agentpools.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-cluster", "my-agent-pool").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "agent pool deletion fails",
			agentPoolsSpec: Spec{
				Name:          "my-agent-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
			},
			expectedError: "failed to delete agent pool my-agent-pool in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_agentpools.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-cluster", "my-agent-pool").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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

			agentPoolsMock := mock_agentpools.NewMockClient(mockCtrl)

			tc.expect(agentPoolsMock.EXPECT())

			s := &Service{
				Client: agentPoolsMock,
			}

			err := s.Delete(context.TODO(), &tc.agentPoolsSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
