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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vmssextensions/mock_vmssextensions"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileVMSSExtension(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_vmssextensions.MockVMSSExtensionScopeMockRecorder, m *mock_vmssextensions.MockclientMockRecorder)
	}{
		{
			name:          "extension already exists",
			expectedError: "",
			expect: func(s *mock_vmssextensions.MockVMSSExtensionScopeMockRecorder, m *mock_vmssextensions.MockclientMockRecorder) {
				s.VMSSExtensionSpecs().Return([]azure.ExtensionSpec{
					{
						Name:      "my-extension-1",
						VMName:    "my-vmss",
						Publisher: "some-publisher",
						Version:   "1.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss", "my-extension-1").Return(compute.VirtualMachineScaleSetExtension{
					Name: to.StringPtr("my-extension-1"),
					VirtualMachineScaleSetExtensionProperties: &compute.VirtualMachineScaleSetExtensionProperties{
						Publisher:         to.StringPtr("some-publisher"),
						Type:              to.StringPtr("my-extension-1"),
						ProvisioningState: to.StringPtr(string(compute.ProvisioningStateSucceeded)),
					},
					ID: to.StringPtr("some/fake/id"),
				}, nil)
				s.SetBootstrapConditions(gomockinternal.AContext(), string(compute.ProvisioningStateSucceeded), "my-extension-1")
			},
		},
		{
			name:          "extension does not exist",
			expectedError: "",
			expect: func(s *mock_vmssextensions.MockVMSSExtensionScopeMockRecorder, m *mock_vmssextensions.MockclientMockRecorder) {
				s.VMSSExtensionSpecs().Return([]azure.ExtensionSpec{
					{
						Name:      "my-extension-1",
						VMName:    "my-vmss",
						Publisher: "some-publisher",
						Version:   "1.0",
					},
					{
						Name:      "other-extension",
						VMName:    "my-vmss",
						Publisher: "other-publisher",
						Version:   "2.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss", "my-extension-1").
					Return(compute.VirtualMachineScaleSetExtension{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss", "other-extension").
					Return(compute.VirtualMachineScaleSetExtension{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "error getting the extension",
			expectedError: "failed to get vm extension my-extension-1 on scale set my-vmss: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_vmssextensions.MockVMSSExtensionScopeMockRecorder, m *mock_vmssextensions.MockclientMockRecorder) {
				s.VMSSExtensionSpecs().Return([]azure.ExtensionSpec{
					{
						Name:      "my-extension-1",
						VMName:    "my-vmss",
						Publisher: "some-publisher",
						Version:   "1.0",
					},
					{
						Name:      "other-extension",
						VMName:    "my-vmss",
						Publisher: "other-publisher",
						Version:   "2.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss", "my-extension-1").
					Return(compute.VirtualMachineScaleSetExtension{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
