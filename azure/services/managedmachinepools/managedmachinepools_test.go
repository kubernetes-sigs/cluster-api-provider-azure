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

package managedmachinepools

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/managedmachinepools/mock_managedmachinepools"
	infraexpv1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiexp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
)

func TestReconcile(t *testing.T) {
	provisioningstatetestcases := []struct {
		name                     string
		nodePoolSpec             azure.AKSNodePoolSpec
		provisioningStatesToTest []string
		expectedError            string
		expect                   func(m *mock_managedmachinepools.MockClientMockRecorder, provisioningstate string)
	}{
		{
			name: "nodepool in terminal provisioning state",
			nodePoolSpec: azure.AKSNodePoolSpec{
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				Name:          "my-nodepool",
			},
			provisioningStatesToTest: []string{"Canceled", "Succeeded", "Failed"},
			expectedError:            "",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder, provisioningstate string) {
				pv := provisioningstate
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-cluster", "my-nodepool", gomock.Any(), gomock.Any()).Return(nil)
				m.Get(gomockinternal.AContext(), "my-rg", "my-cluster", "my-nodepool").Return(containerservice.AgentPool{ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
					ProvisioningState: &pv,
				}}, nil)
			},
		},
		{
			name: "nodepool in nonterminal provisioning state",
			nodePoolSpec: azure.AKSNodePoolSpec{
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				Name:          "my-nodepool",
			},
			provisioningStatesToTest: []string{"Deleting", "InProgress", "randomStringHere"},
			expectedError:            "Unable to update existing node pool in non terminal state. Node pool must be in one of the following provisioning states: canceled, failed, or succeeded. Actual state:",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder, provisioningstate string) {
				m.Get(gomockinternal.AContext(), "my-rg", "my-cluster", "my-nodepool").Return(containerservice.AgentPool{ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
					ProvisioningState: &provisioningstate,
				}}, nil)
			},
		},
	}

	for _, tc := range provisioningstatetestcases {
		for _, provisioningstate := range tc.provisioningStatesToTest {
			tc := tc
			provisioningstate := provisioningstate
			t.Logf("Testing nodepool provision state: " + provisioningstate)
			t.Run(tc.name, func(t *testing.T) {
				g := NewWithT(t)
				t.Parallel()

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()

				agentpoolsMock := mock_managedmachinepools.NewMockClient(mockCtrl)
				machinePoolScope := &scope.ManagedMachinePoolScope{
					ControlPlane: &infraexpv1.AzureManagedControlPlane{
						ObjectMeta: metav1.ObjectMeta{
							Name: tc.nodePoolSpec.Cluster,
						},
						Spec: infraexpv1.AzureManagedControlPlaneSpec{
							ResourceGroupName: tc.nodePoolSpec.ResourceGroup,
						},
					},
					MachinePool: &capiexp.MachinePool{},
					InfraMachinePool: &infraexpv1.AzureManagedMachinePool{
						ObjectMeta: metav1.ObjectMeta{
							Name: tc.nodePoolSpec.Name,
						},
						Spec: infraexpv1.AzureManagedMachinePoolSpec{
							Name: &tc.nodePoolSpec.Name,
						},
					},
				}

				tc.expect(agentpoolsMock.EXPECT(), provisioningstate)

				s := &Service{
					Client: agentpoolsMock,
					scope:  machinePoolScope,
				}

				err := s.Reconcile(context.TODO())
				if tc.expectedError != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
					g.Expect(err.Error()).To(ContainSubstring(provisioningstate))
				} else {
					g.Expect(err).NotTo(HaveOccurred())
				}
			})
		}
	}

	testcases := []struct {
		name          string
		nodePoolsSpec azure.AKSNodePoolSpec
		expectedError string
		expect        func(m *mock_managedmachinepools.MockClientMockRecorder)
	}{
		{
			name: "no nodepool exists",
			nodePoolsSpec: azure.AKSNodePoolSpec{
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				Name:          "my-nodepool",
			},
			expectedError: "",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder) {
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-cluster", "my-nodepool", gomock.Any(), gomock.Any()).Return(nil)
				m.Get(gomockinternal.AContext(), "my-rg", "my-cluster", "my-nodepool").Return(containerservice.AgentPool{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
			},
		},
		{
			name: "fail to get existing node pool",
			nodePoolsSpec: azure.AKSNodePoolSpec{
				Name:          "my-node-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "SKU123",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
				MaxPods:       to.Int32Ptr(12),
				OsDiskType:    to.StringPtr(string(containerservice.OSDiskTypeManaged)),
			},
			expectedError: "failed to get existing node pool: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder) {
				m.Get(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool").Return(containerservice.AgentPool{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "can create a Node Pool",
			nodePoolsSpec: azure.AKSNodePoolSpec{
				Name:          "my-node-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "SKU123",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
				MaxPods:       to.Int32Ptr(12),
				OsDiskType:    to.StringPtr(string(containerservice.OSDiskTypeManaged)),
			},
			expectedError: "",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder) {
				m.Get(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool").Return(containerservice.AgentPool{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool", gomock.AssignableToTypeOf(containerservice.AgentPool{}), gomock.Any()).Return(nil)
			},
		},
		{
			name: "fail to create a Node Pool",
			nodePoolsSpec: azure.AKSNodePoolSpec{
				Name:          "my-node-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "SKU123",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
				MaxPods:       to.Int32Ptr(12),
				OsDiskType:    to.StringPtr(string(containerservice.OSDiskTypeManaged)),
			},
			expectedError: "failed to create or update node pool: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder) {
				m.Get(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool").Return(containerservice.AgentPool{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool", gomock.AssignableToTypeOf(containerservice.AgentPool{}), gomock.Any()).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "fail to update a Node Pool",
			nodePoolsSpec: azure.AKSNodePoolSpec{
				Name:          "my-node-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "SKU123",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
				MaxPods:       to.Int32Ptr(12),
				OsDiskType:    to.StringPtr(string(containerservice.OSDiskTypeManaged)),
			},
			expectedError: "failed to create or update node pool: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder) {
				m.Get(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool").Return(containerservice.AgentPool{
					ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
						Count:               to.Int32Ptr(3),
						OsDiskSizeGB:        to.Int32Ptr(20),
						VMSize:              to.StringPtr(string(containerservice.VMSizeTypesStandardA1)),
						OrchestratorVersion: to.StringPtr("9.99.9999"),
						ProvisioningState:   to.StringPtr("Failed"),
					},
				}, nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool", gomock.AssignableToTypeOf(containerservice.AgentPool{}), gomock.Any()).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "no update needed on Node Pool",
			nodePoolsSpec: azure.AKSNodePoolSpec{
				Name:          "my-node-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
				SKU:           "Standard_D2s_v3",
				Version:       to.StringPtr("9.99.9999"),
				Replicas:      2,
				OSDiskSizeGB:  100,
				MaxPods:       to.Int32Ptr(12),
				OsDiskType:    to.StringPtr(string(containerservice.OSDiskTypeEphemeral)),
			},
			expectedError: "",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder) {
				m.Get(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool").Return(containerservice.AgentPool{
					ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
						Count:               to.Int32Ptr(2),
						OsDiskSizeGB:        to.Int32Ptr(100),
						VMSize:              to.StringPtr(string(containerservice.VMSizeTypesStandardD2sV3)),
						OsType:              containerservice.OSTypeLinux,
						OrchestratorVersion: to.StringPtr("9.99.9999"),
						ProvisioningState:   to.StringPtr("Succeeded"),
						VnetSubnetID:        to.StringPtr(""),
						MaxPods:             to.Int32Ptr(12),
						OsDiskType:          containerservice.OSDiskTypeEphemeral,
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

			replicas := tc.nodePoolsSpec.Replicas
			osDiskSizeGB := tc.nodePoolsSpec.OSDiskSizeGB

			agentpoolsMock := mock_managedmachinepools.NewMockClient(mockCtrl)
			machinePoolScope := &scope.ManagedMachinePoolScope{
				ControlPlane: &infraexpv1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.nodePoolsSpec.Cluster,
					},
					Spec: infraexpv1.AzureManagedControlPlaneSpec{
						ResourceGroupName: tc.nodePoolsSpec.ResourceGroup,
					},
				},
				MachinePool: &capiexp.MachinePool{
					Spec: capiexp.MachinePoolSpec{
						Replicas: &replicas,
						Template: capi.MachineTemplateSpec{
							Spec: capi.MachineSpec{
								Version: tc.nodePoolsSpec.Version,
							},
						},
					},
				},
				InfraMachinePool: &infraexpv1.AzureManagedMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.nodePoolsSpec.Name,
					},
					Spec: infraexpv1.AzureManagedMachinePoolSpec{
						Name:         &tc.nodePoolsSpec.Name,
						SKU:          tc.nodePoolsSpec.SKU,
						OSDiskSizeGB: &osDiskSizeGB,
						MaxPods:      to.Int32Ptr(12),
						OsDiskType:   to.StringPtr(string(containerservice.OSDiskTypeManaged)),
					},
				},
			}

			tc.expect(agentpoolsMock.EXPECT())

			s := &Service{
				Client: agentpoolsMock,
				scope:  machinePoolScope,
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

func TestDeleteAgentPools(t *testing.T) {
	testcases := []struct {
		name          string
		nodePoolsSpec azure.AKSNodePoolSpec
		expectedError string
		expect        func(m *mock_managedmachinepools.MockClientMockRecorder)
	}{
		{
			name: "successfully delete an existing node pool",
			nodePoolsSpec: azure.AKSNodePoolSpec{
				Name:          "my-node-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
			},
			expectedError: "",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder) {
				m.Delete(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool")
			},
		},
		{
			name: "node pool already deleted",
			nodePoolsSpec: azure.AKSNodePoolSpec{
				Name:          "my-node-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
			},
			expectedError: "",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder) {
				m.Delete(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "node pool deletion fails",
			nodePoolsSpec: azure.AKSNodePoolSpec{
				Name:          "my-node-pool",
				ResourceGroup: "my-rg",
				Cluster:       "my-cluster",
			},
			expectedError: "failed to delete node pool my-node-pool in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_managedmachinepools.MockClientMockRecorder) {
				m.Delete(gomockinternal.AContext(), "my-rg", "my-cluster", "my-node-pool").
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

			agentPoolsMock := mock_managedmachinepools.NewMockClient(mockCtrl)
			machinePoolScope := &scope.ManagedMachinePoolScope{
				ControlPlane: &infraexpv1.AzureManagedControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.nodePoolsSpec.Cluster,
					},
					Spec: infraexpv1.AzureManagedControlPlaneSpec{
						ResourceGroupName: tc.nodePoolsSpec.ResourceGroup,
					},
				},
				MachinePool: &capiexp.MachinePool{},
				InfraMachinePool: &infraexpv1.AzureManagedMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.nodePoolsSpec.Name,
					},
					Spec: infraexpv1.AzureManagedMachinePoolSpec{
						Name: &tc.nodePoolsSpec.Name,
					},
				},
			}

			tc.expect(agentPoolsMock.EXPECT())

			s := &Service{
				Client: agentPoolsMock,
				scope:  machinePoolScope,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
