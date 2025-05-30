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
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/roleassignments/mock_roleassignments"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets/mock_scalesets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachines"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
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
		PrincipalID:   ptr.To("fake-principal-id"),
	}
	fakeRoleAssignment2 = RoleAssignmentSpec{
		MachineName:   "test-vmss",
		ResourceGroup: "my-rg",
		ResourceType:  azure.VirtualMachineScaleSet,
	}

	emptyRoleAssignmentSpec = RoleAssignmentSpec{}
	fakeRoleAssignmentSpecs = []azure.ResourceSpecGetter{&fakeRoleAssignment1, &fakeRoleAssignment2, &emptyRoleAssignmentSpec}
	fakeVMSSSpec            = scalesets.ScaleSetSpec{
		Name:          "test-vmss",
		ResourceGroup: "my-rg",
	}
)

func internalError() *azcore.ResponseError {
	return &azcore.ResponseError{
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Internal Server Error: StatusCode=500")),
			StatusCode: http.StatusInternalServerError,
		},
	}
}

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
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.Name().Return(fakeRoleAssignment1.MachineName)
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentResourceType().Return("VirtualMachine")
				s.RoleAssignmentSpecs(&fakePrincipalID).Return(fakeRoleAssignmentSpecs[:1])
				m.Get(gomockinternal.AContext(), &fakeVMSpec).Return(armcompute.VirtualMachine{
					Identity: &armcompute.VirtualMachineIdentity{
						PrincipalID: &fakePrincipalID,
					},
				}, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRoleAssignment1, serviceName).Return(&fakeRoleAssignment1, nil)
			},
		},
		{
			name:          "error getting VM",
			expectedError: "failed to assign role to system assigned identity: failed to get principal ID for VM:.*#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				m *mock_async.MockGetterMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.Name().Return(fakeRoleAssignment1.MachineName)
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentResourceType().Return("VirtualMachine")
				m.Get(gomockinternal.AContext(), &fakeVMSpec).Return(armcompute.VirtualMachine{}, internalError())
			},
		},
		{
			name:          "return error when creating a role assignment",
			expectedError: "cannot assign role to VirtualMachine system assigned identity:.*#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				m *mock_async.MockGetterMockRecorder,
				r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.SubscriptionID().AnyTimes().Return("12345")
				s.ResourceGroup().Return("my-rg")
				s.Name().Return(fakeRoleAssignment1.MachineName)
				s.RoleAssignmentResourceType().Return("VirtualMachine")
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentSpecs(&fakePrincipalID).Return(fakeRoleAssignmentSpecs[0:1])
				m.Get(gomockinternal.AContext(), &fakeVMSpec).Return(armcompute.VirtualMachine{
					Identity: &armcompute.VirtualMachineIdentity{
						PrincipalID: &fakePrincipalID,
					},
				}, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRoleAssignment1, serviceName).Return(&RoleAssignmentSpec{},
					internalError())
			},
		},
	}

	for _, tc := range testcases {
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

			err := s.Reconcile(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(strings.ReplaceAll(err.Error(), "\n", "")).To(MatchRegexp(tc.expectedError))
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
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentSpecs(&fakePrincipalID).Return(fakeRoleAssignmentSpecs[1:2])
				s.RoleAssignmentResourceType().Return(azure.VirtualMachineScaleSet)
				s.ResourceGroup().Return("my-rg")
				s.Name().Return("test-vmss")
				mvmss.Get(gomockinternal.AContext(), &fakeVMSSSpec).Return(armcompute.VirtualMachineScaleSet{
					Identity: &armcompute.VirtualMachineScaleSetIdentity{
						PrincipalID: &fakePrincipalID,
					},
				}, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRoleAssignment2, serviceName).Return(&fakeRoleAssignment2, nil)
			},
		},
		{
			name:          "error getting VMSS",
			expectedError: "failed to assign role to system assigned identity: failed to get principal ID for VMSS:.*#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				r *mock_async.MockReconcilerMockRecorder,
				mvmss *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.RoleAssignmentResourceType().Return(azure.VirtualMachineScaleSet)
				s.ResourceGroup().Return("my-rg")
				s.Name().Return("test-vmss")
				s.HasSystemAssignedIdentity().Return(true)
				mvmss.Get(gomockinternal.AContext(), &fakeVMSSSpec).Return(armcompute.VirtualMachineScaleSet{},
					internalError())
			},
		},
		{
			name:          "return error when creating a role assignment",
			expectedError: fmt.Sprintf("cannot assign role to %s system assigned identity:.*#: Internal Server Error: StatusCode=500", azure.VirtualMachineScaleSet),
			expect: func(s *mock_roleassignments.MockRoleAssignmentScopeMockRecorder,
				r *mock_async.MockReconcilerMockRecorder,
				mvmss *mock_scalesets.MockClientMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.HasSystemAssignedIdentity().Return(true)
				s.RoleAssignmentSpecs(&fakePrincipalID).Return(fakeRoleAssignmentSpecs[1:2])
				s.RoleAssignmentResourceType().Return(azure.VirtualMachineScaleSet)
				s.ResourceGroup().Return("my-rg")
				s.Name().Return("test-vmss")
				mvmss.Get(gomockinternal.AContext(), &fakeVMSSSpec).Return(armcompute.VirtualMachineScaleSet{
					Identity: &armcompute.VirtualMachineScaleSetIdentity{
						PrincipalID: &fakePrincipalID,
					},
				}, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRoleAssignment2, serviceName).Return(&RoleAssignmentSpec{},
					internalError())
			},
		},
	}

	for _, tc := range testcases {
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
				virtualMachineScaleSetGetter: vmMock,
			}

			err := s.Reconcile(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(strings.ReplaceAll(err.Error(), "\n", "")).To(MatchRegexp(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
