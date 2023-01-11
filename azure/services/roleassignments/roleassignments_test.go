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
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/roleassignments/mock_roleassignments"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets/mock_scalesets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachines"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeVMSpec = virtualmachines.VMSpec{
		Name:          "test-vm",
		ResourceGroup: "my-rg",
	}
	fakePrincipalID     = "fake-p-id"
	fakeRoleAssignment1 = RoleAssignmentSpec{
		MachineName:   "test-vm",
		ResourceGroup: "my-rg",
		ResourceType:  azure.VirtualMachine,
		PrincipalID:   pointer.String("fake-principal-id"),
	}
	fakeRoleAssignment2 = RoleAssignmentSpec{
		MachineName:   "test-vmss",
		ResourceGroup: "my-rg",
		ResourceType:  azure.VirtualMachineScaleSet,
	}

	emptyRoleAssignmentSpec = RoleAssignmentSpec{}
	fakeRoleAssignmentSpecs = []azure.ResourceSpecGetter{&fakeRoleAssignment1, &fakeRoleAssignment2, &emptyRoleAssignmentSpec}
)

func TestReconcileRoleAssignmentsVM(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
		expectedError string
	}{
		{
			name:          "create a role assignment",
			expectedError: "",

			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				m *mock_async.MockGetterMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.Name().Return(fakeRoleAssignment1.MachineName)
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentResourceType().Return("VirtualMachine")
				s.RoleAssignmentSpecs(&fakePrincipalID).Return(fakeRoleAssignmentSpecs[:1])
				m.Get(gomockinternal.AContext(), &fakeVMSpec).Return(compute.VirtualMachine{
					Identity: &compute.VirtualMachineIdentity{
						PrincipalID: &fakePrincipalID,
					},
				}, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRoleAssignment1, serviceName).Return(&fakeRoleAssignment1, nil)
			},
		},
		{
			name:          "error getting VM",
			expectedError: "failed to assign role to system assigned identity: failed to get principal ID for VM: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				m *mock_async.MockGetterMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.Name().Return(fakeRoleAssignment1.MachineName)
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentResourceType().Return("VirtualMachine")
				m.Get(gomockinternal.AContext(), &fakeVMSpec).Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error"))
			},
		},
		{
			name:          "return error when creating a role assignment",
			expectedError: "cannot assign role to VirtualMachine system assigned identity: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				m *mock_async.MockGetterMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.Name().Return(fakeRoleAssignment1.MachineName)
				s.RoleAssignmentResourceType().Return("VirtualMachine")
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentSpecs(&fakePrincipalID).Return(fakeRoleAssignmentSpecs[0:1])
				m.Get(gomockinternal.AContext(), &fakeVMSpec).Return(compute.VirtualMachine{
					Identity: &compute.VirtualMachineIdentity{
						PrincipalID: &fakePrincipalID,
					},
				}, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRoleAssignment1, serviceName).Return(&RoleAssignmentSpec{},
					autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error"))
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
			vmGetterMock := mock_async.NewMockGetter(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), vmGetterMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:                 scopeMock,
				virtualMachinesGetter: vmGetterMock,
				Reconciler:            asyncMock,
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
		name   string
		expect func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder,
			mvmss *mock_scalesets.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "create a role assignment",
			expectedError: "",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				r *mock_async.MockReconcilerMockRecorder,
				mvmss *mock_scalesets.MockClientMockRecorder) {
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentSpecs(&fakePrincipalID).Return(fakeRoleAssignmentSpecs[1:2])
				s.RoleAssignmentResourceType().Return(azure.VirtualMachineScaleSet)
				s.ResourceGroup().Return("my-rg")
				s.Name().Return("test-vmss")
				mvmss.Get(gomockinternal.AContext(), "my-rg", "test-vmss").Return(compute.VirtualMachineScaleSet{
					Identity: &compute.VirtualMachineScaleSetIdentity{
						PrincipalID: &fakePrincipalID,
					},
				}, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRoleAssignment2, serviceName).Return(&fakeRoleAssignment2, nil)
			},
		},
		{
			name:          "error getting VMSS",
			expectedError: "failed to assign role to system assigned identity: failed to get principal ID for VMSS: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				r *mock_async.MockReconcilerMockRecorder,
				mvmss *mock_scalesets.MockClientMockRecorder) {
				s.RoleAssignmentResourceType().Return(azure.VirtualMachineScaleSet)
				s.ResourceGroup().Return("my-rg")
				s.Name().Return("test-vmss")
				s.HasSystemAssignedIdentity().Return(true)
				mvmss.Get(gomockinternal.AContext(), "my-rg", "test-vmss").Return(compute.VirtualMachineScaleSet{},
					autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error"))
			},
		},
		{
			name:          "return error when creating a role assignment",
			expectedError: fmt.Sprintf("cannot assign role to %s system assigned identity: #: Internal Server Error: StatusCode=500", azure.VirtualMachineScaleSet),
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				r *mock_async.MockReconcilerMockRecorder,
				mvmss *mock_scalesets.MockClientMockRecorder) {
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentSpecs(&fakePrincipalID).Return(fakeRoleAssignmentSpecs[1:2])
				s.RoleAssignmentResourceType().Return(azure.VirtualMachineScaleSet)
				s.ResourceGroup().Return("my-rg")
				s.Name().Return("test-vmss")
				mvmss.Get(gomockinternal.AContext(), "my-rg", "test-vmss").Return(compute.VirtualMachineScaleSet{
					Identity: &compute.VirtualMachineScaleSetIdentity{
						PrincipalID: &fakePrincipalID,
					},
				}, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRoleAssignment2, serviceName).Return(&RoleAssignmentSpec{},
					autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error"))
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
			asyncMock := mock_async.NewMockReconciler(mockCtrl)
			vmMock := mock_scalesets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT(), vmMock.EXPECT())

			s := &Service{
				Scope:                        scopeMock,
				Reconciler:                   asyncMock,
				virtualMachineScaleSetClient: vmMock,
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
