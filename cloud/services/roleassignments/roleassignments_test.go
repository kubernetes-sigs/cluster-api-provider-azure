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

package roleassignments

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/authorization/mgmt/authorization"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"k8s.io/klog/klogr"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/roleassignments/mock_roleassignments"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/scalesets/mock_scalesets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines/mock_virtualmachines"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileRoleAssignmentsVM(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, m *mock_roleassignments.MockclientMockRecorder, v *mock_virtualmachines.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "create a role assignment",
			expectedError: "",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, m *mock_roleassignments.MockclientMockRecorder, v *mock_virtualmachines.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.RoleAssignmentSpecs().Return([]azure.RoleAssignmentSpec{
					{
						MachineName:  "test-vm",
						ResourceType: azure.VirtualMachine,
					},
				})
				v.Get(gomockinternal.AContext(), "my-rg", "test-vm").Return(compute.VirtualMachine{
					Identity: &compute.VirtualMachineIdentity{
						PrincipalID: to.StringPtr("000"),
					},
				}, nil)
				m.Create(gomockinternal.AContext(), "/subscriptions/12345/", gomock.AssignableToTypeOf("uuid"), gomock.AssignableToTypeOf(authorization.RoleAssignmentCreateParameters{
					Properties: &authorization.RoleAssignmentProperties{
						RoleDefinitionID: to.StringPtr("/subscriptions/12345/providers/Microsoft.Authorization/roleDefinitions/b24988ac-6180-42a0-ab88-20f7382dd24c"),
						PrincipalID:      to.StringPtr("000"),
					},
				}))
			},
		},
		{
			name:          "error getting VM",
			expectedError: "cannot get VM to assign role to system assigned identity: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, m *mock_roleassignments.MockclientMockRecorder, v *mock_virtualmachines.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.RoleAssignmentSpecs().Return([]azure.RoleAssignmentSpec{
					{
						MachineName:  "test-vm",
						ResourceType: azure.VirtualMachine,
					},
				})
				v.Get(gomockinternal.AContext(), "my-rg", "test-vm").Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "return error when creating a role assignment",
			expectedError: "cannot assign role to VM system assigned identity: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, m *mock_roleassignments.MockclientMockRecorder, v *mock_virtualmachines.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.RoleAssignmentSpecs().Return([]azure.RoleAssignmentSpec{
					{
						MachineName:  "test-vm",
						ResourceType: azure.VirtualMachine,
					},
				})
				v.Get(gomockinternal.AContext(), "my-rg", "test-vm").Return(compute.VirtualMachine{
					Identity: &compute.VirtualMachineIdentity{
						PrincipalID: to.StringPtr("000"),
					},
				}, nil)
				m.Create(gomockinternal.AContext(), "/subscriptions/12345/", gomock.AssignableToTypeOf("uuid"), gomock.AssignableToTypeOf(authorization.RoleAssignmentCreateParameters{})).Return(authorization.RoleAssignment{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_roleassignments.NewMockRoleAssignmentScope(mockCtrl)
			clientMock := mock_roleassignments.NewMockclient(mockCtrl)
			vmMock := mock_virtualmachines.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), vmMock.EXPECT())

			s := &Service{
				Scope:                 scopeMock,
				client:                clientMock,
				virtualMachinesClient: vmMock,
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
func TestReconcileRoleAssignmentsVMSS(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, m *mock_roleassignments.MockclientMockRecorder, v *mock_scalesets.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "create a role assignment",
			expectedError: "",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, m *mock_roleassignments.MockclientMockRecorder, v *mock_scalesets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.RoleAssignmentSpecs().Return([]azure.RoleAssignmentSpec{
					{
						MachineName:  "test-vmss",
						ResourceType: azure.VirtualMachineScaleSet,
					},
				})
				v.Get(gomockinternal.AContext(), "my-rg", "test-vmss").Return(compute.VirtualMachineScaleSet{
					Identity: &compute.VirtualMachineScaleSetIdentity{
						PrincipalID: to.StringPtr("000"),
					},
				}, nil)
				m.Create(gomockinternal.AContext(), "/subscriptions/12345/", gomock.AssignableToTypeOf("uuid"), gomock.AssignableToTypeOf(authorization.RoleAssignmentCreateParameters{
					Properties: &authorization.RoleAssignmentProperties{
						RoleDefinitionID: to.StringPtr("/subscriptions/12345/providers/Microsoft.Authorization/roleDefinitions/b24988ac-6180-42a0-ab88-20f7382dd24c"),
						PrincipalID:      to.StringPtr("000"),
					},
				}))
			},
		},
		{
			name:          "error getting VMSS",
			expectedError: "cannot get VMSS to assign role to system assigned identity: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, m *mock_roleassignments.MockclientMockRecorder, v *mock_scalesets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.RoleAssignmentSpecs().Return([]azure.RoleAssignmentSpec{
					{
						MachineName:  "test-vmss",
						ResourceType: azure.VirtualMachineScaleSet,
					},
				})
				v.Get(gomockinternal.AContext(), "my-rg", "test-vmss").Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "return error when creating a role assignment",
			expectedError: "cannot assign role to VMSS system assigned identity: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, m *mock_roleassignments.MockclientMockRecorder, v *mock_scalesets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.RoleAssignmentSpecs().Return([]azure.RoleAssignmentSpec{
					{
						MachineName:  "test-vmss",
						ResourceType: azure.VirtualMachineScaleSet,
					},
				})
				v.Get(gomockinternal.AContext(), "my-rg", "test-vmss").Return(compute.VirtualMachineScaleSet{
					Identity: &compute.VirtualMachineScaleSetIdentity{
						PrincipalID: to.StringPtr("000"),
					},
				}, nil)
				m.Create(gomockinternal.AContext(), "/subscriptions/12345/", gomock.AssignableToTypeOf("uuid"), gomock.AssignableToTypeOf(authorization.RoleAssignmentCreateParameters{})).Return(authorization.RoleAssignment{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_roleassignments.NewMockRoleAssignmentScope(mockCtrl)
			clientMock := mock_roleassignments.NewMockclient(mockCtrl)
			vmssMock := mock_scalesets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), vmssMock.EXPECT())

			s := &Service{
				Scope:                        scopeMock,
				client:                       clientMock,
				virtualMachineScaleSetClient: vmssMock,
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
