/*
Copyright 2021 The Kubernetes Authors.

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

package scalesetvms

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesetvms/mock_scalesetvms"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	uniformScaleSetVMSpec = &ScaleSetVMSpec{
		Name:          "my-vmss",
		InstanceID:    "0",
		ResourceGroup: "my-rg",
		ScaleSetName:  "my-vmss",
		ProviderID:    "azure:///subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/0",
		ResourceID:    "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/0",
		IsFlex:        false,
	}
	uniformScaleSetVM = armcompute.VirtualMachineScaleSetVM{
		ID: &uniformScaleSetVMSpec.ResourceID,
	}

	flexScaleSetVMSpec = &ScaleSetVMSpec{
		Name:          "my-vmss",
		InstanceID:    "0",
		ResourceGroup: "my-rg",
		ScaleSetName:  "my-vmss",
		ProviderID:    "azure:///subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/my-vmss",
		ResourceID:    "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/my-vmss",
		IsFlex:        true,
	}
	flexGetter = &VMSSFlexGetter{
		Name:          flexScaleSetVMSpec.Name,
		ResourceGroup: flexScaleSetVMSpec.ResourceGroup,
	}
	flexScaleSetVM = armcompute.VirtualMachine{
		Name: &uniformScaleSetVMSpec.Name,
	}
)

func errInternal() *azcore.ResponseError {
	return &azcore.ResponseError{
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Internal Server Error: StatusCode=500")),
			StatusCode: http.StatusInternalServerError,
		},
	}
}

func TestReconcileVMSS(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder)
		expectedError string
	}{
		{
			name:          "get a uniform vmss vm",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(uniformScaleSetVMSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), uniformScaleSetVMSpec, serviceName).Return(uniformScaleSetVM, nil)
				s.SetVMSSVM(converters.SDKToVMSSVM(uniformScaleSetVM))
			},
		},
		{
			name:          "get a vmss flex vm",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(flexScaleSetVMSpec)
				v.CreateOrUpdateResource(gomockinternal.AContext(), flexGetter, serviceName).Return(flexScaleSetVM, nil)
				s.SetVMSSVM(converters.SDKVMToVMSSVM(flexScaleSetVM, infrav1.FlexibleOrchestrationMode))
			},
		},
		{
			name:          "uniform vmss vm doesn't exist yet",
			expectedError: "instance does not exist yet. Object will be requeued after 30s",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(uniformScaleSetVMSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), uniformScaleSetVMSpec, serviceName).Return(nil, nil)
			},
		},
		{
			name:          "vmss flex vm doesn't exist yet",
			expectedError: "instance does not exist yet. Object will be requeued after 30s",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(flexScaleSetVMSpec)
				v.CreateOrUpdateResource(gomockinternal.AContext(), flexGetter, serviceName).Return(nil, nil)
			},
		},
		{
			name:          "error getting uniform vmss vm",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(uniformScaleSetVMSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), uniformScaleSetVMSpec, serviceName).Return(nil, errInternal())
			},
		},
		{
			name:          "error getting vmss flex vm",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(flexScaleSetVMSpec)
				v.CreateOrUpdateResource(gomockinternal.AContext(), flexGetter, serviceName).Return(nil, errInternal())
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

			scopeMock := mock_scalesetvms.NewMockScaleSetVMScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)
			vmAsyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(g, scopeMock.EXPECT(), asyncMock.EXPECT(), vmAsyncMock.EXPECT())

			s := &Service{
				Scope:        scopeMock,
				Reconciler:   asyncMock,
				VMReconciler: vmAsyncMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteVMSS(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder)
		expectedError string
	}{
		{
			name:          "delete a uniform vmss vm",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(uniformScaleSetVMSpec)
				r.DeleteResource(gomockinternal.AContext(), uniformScaleSetVMSpec, serviceName).Return(nil)
				s.SetVMSSVMState(infrav1.Deleted)
			},
		},
		{
			name:          "delete a vmss flex vm",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(flexScaleSetVMSpec)
				v.DeleteResource(gomockinternal.AContext(), flexGetter, serviceName).Return(nil)
				s.SetVMSSVMState(infrav1.Deleted)
			},
		},
		{
			name:          "error when deleting a uniform vmss vm",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(uniformScaleSetVMSpec)
				r.DeleteResource(gomockinternal.AContext(), uniformScaleSetVMSpec, serviceName).Return(errInternal())
				s.SetVMSSVMState(infrav1.Deleting)
			},
		},
		{
			name:          "error when deleting a vmss flex vm",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(g *WithT, s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder, v *mock_async.MockReconcilerMockRecorder) {
				s.ScaleSetVMSpec().Return(flexScaleSetVMSpec)
				v.DeleteResource(gomockinternal.AContext(), flexGetter, serviceName).Return(errInternal())
				s.SetVMSSVMState(infrav1.Deleting)
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

			scopeMock := mock_scalesetvms.NewMockScaleSetVMScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)
			vmAsyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(g, scopeMock.EXPECT(), asyncMock.EXPECT(), vmAsyncMock.EXPECT())

			s := &Service{
				Scope:        scopeMock,
				Reconciler:   asyncMock,
				VMReconciler: vmAsyncMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
