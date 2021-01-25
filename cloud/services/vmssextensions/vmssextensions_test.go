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

package vmssextensions

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/vmssextensions/mock_vmssextensions"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/klogr"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileVMSSExtension(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_vmssextensions.MockVMSSExtensionScopeMockRecorder, m *mock_vmssextensions.MockclientMockRecorder)
	}{
		{
			name:          "reconcile multiple extensions",
			expectedError: "",
			expect: func(s *mock_vmssextensions.MockVMSSExtensionScopeMockRecorder, m *mock_vmssextensions.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VMSSExtensionSpecs().Return([]azure.VMSSExtensionSpec{
					{
						Name:         "my-extension-1",
						ScaleSetName: "my-vmss",
						Publisher:    "some-publisher",
						Version:      "1.0",
					},
					{
						Name:         "other-extension",
						ScaleSetName: "my-vmss",
						Publisher:    "other-publisher",
						Version:      "2.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", "my-extension-1", gomock.AssignableToTypeOf(compute.VirtualMachineScaleSetExtension{}))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", "other-extension", gomock.AssignableToTypeOf(compute.VirtualMachineScaleSetExtension{}))
			},
		},
		{
			name:          "error creating the extension",
			expectedError: "failed to create VM extension my-extension-1 on scale set my-vmss in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_vmssextensions.MockVMSSExtensionScopeMockRecorder, m *mock_vmssextensions.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VMSSExtensionSpecs().Return([]azure.VMSSExtensionSpec{
					{
						Name:         "my-extension-1",
						ScaleSetName: "my-vmss",
						Publisher:    "some-publisher",
						Version:      "1.0",
					},
					{
						Name:         "other-extension",
						ScaleSetName: "my-vmss",
						Publisher:    "other-publisher",
						Version:      "2.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", "my-extension-1", gomock.AssignableToTypeOf(compute.VirtualMachineScaleSetExtension{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))

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
			scopeMock := mock_vmssextensions.NewMockVMSSExtensionScope(mockCtrl)
			clientMock := mock_vmssextensions.NewMockclient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
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
