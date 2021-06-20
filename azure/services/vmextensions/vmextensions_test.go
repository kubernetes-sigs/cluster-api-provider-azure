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

package vmextensions

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vmextensions/mock_vmextensions"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileVMExtension(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, m *mock_vmextensions.MockclientMockRecorder)
	}{
		{
			name:          "extension is in succeeded state",
			expectedError: "",
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, m *mock_vmextensions.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VMExtensionSpecs().Return([]azure.VMExtensionSpec{
					{
						Name:      "my-extension-1",
						VMName:    "my-vm",
						Publisher: "some-publisher",
						Version:   "1.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm", "my-extension-1").Return(compute.VirtualMachineExtension{
					VirtualMachineExtensionProperties: &compute.VirtualMachineExtensionProperties{
						Publisher:         to.StringPtr("some-publisher"),
						Type:              to.StringPtr("my-extension-1"),
						ProvisioningState: to.StringPtr(string(compute.ProvisioningStateSucceeded)),
					},
					ID:   to.StringPtr("fake/id"),
					Name: to.StringPtr("my-extension-1"),
				}, nil)
				s.SetBootstrapConditions(string(compute.ProvisioningStateSucceeded), "my-extension-1")
			},
		},
		{
			name:          "extension is in failed state",
			expectedError: "",
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, m *mock_vmextensions.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VMExtensionSpecs().Return([]azure.VMExtensionSpec{
					{
						Name:      "my-extension-1",
						VMName:    "my-vm",
						Publisher: "some-publisher",
						Version:   "1.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm", "my-extension-1").Return(compute.VirtualMachineExtension{
					VirtualMachineExtensionProperties: &compute.VirtualMachineExtensionProperties{
						Publisher:         to.StringPtr("some-publisher"),
						Type:              to.StringPtr("my-extension-1"),
						ProvisioningState: to.StringPtr(string(compute.ProvisioningStateFailed)),
					},
					ID:   to.StringPtr("fake/id"),
					Name: to.StringPtr("my-extension-1"),
				}, nil)
				s.SetBootstrapConditions(string(compute.ProvisioningStateFailed), "my-extension-1")
			},
		},
		{
			name:          "extension is still creating",
			expectedError: "",
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, m *mock_vmextensions.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VMExtensionSpecs().Return([]azure.VMExtensionSpec{
					{
						Name:      "my-extension-1",
						VMName:    "my-vm",
						Publisher: "some-publisher",
						Version:   "1.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm", "my-extension-1").Return(compute.VirtualMachineExtension{
					VirtualMachineExtensionProperties: &compute.VirtualMachineExtensionProperties{
						Publisher:         to.StringPtr("some-publisher"),
						Type:              to.StringPtr("my-extension-1"),
						ProvisioningState: to.StringPtr(string(compute.ProvisioningStateCreating)),
					},
					ID:   to.StringPtr("fake/id"),
					Name: to.StringPtr("my-extension-1"),
				}, nil)
				s.SetBootstrapConditions(string(compute.ProvisioningStateCreating), "my-extension-1")
			},
		},
		{
			name:          "reconcile multiple extensions",
			expectedError: "",
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, m *mock_vmextensions.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VMExtensionSpecs().Return([]azure.VMExtensionSpec{
					{
						Name:      "my-extension-1",
						VMName:    "my-vm",
						Publisher: "some-publisher",
						Version:   "1.0",
					},
					{
						Name:      "other-extension",
						VMName:    "my-vm",
						Publisher: "other-publisher",
						Version:   "2.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm", "my-extension-1").
					Return(compute.VirtualMachineExtension{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdateAsync(gomockinternal.AContext(), "my-rg", "my-vm", "my-extension-1", gomock.AssignableToTypeOf(compute.VirtualMachineExtension{}))
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm", "other-extension").
					Return(compute.VirtualMachineExtension{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdateAsync(gomockinternal.AContext(), "my-rg", "my-vm", "other-extension", gomock.AssignableToTypeOf(compute.VirtualMachineExtension{}))
			},
		},
		{
			name:          "error getting the extension",
			expectedError: "failed to get vm extension my-extension-1 on vm my-vm: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, m *mock_vmextensions.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VMExtensionSpecs().Return([]azure.VMExtensionSpec{
					{
						Name:      "my-extension-1",
						VMName:    "my-vm",
						Publisher: "some-publisher",
						Version:   "1.0",
					},
					{
						Name:      "other-extension",
						VMName:    "my-vm",
						Publisher: "other-publisher",
						Version:   "2.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm", "my-extension-1").
					Return(compute.VirtualMachineExtension{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "error creating the extension",
			expectedError: "failed to create VM extension my-extension-1 on VM my-vm in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, m *mock_vmextensions.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VMExtensionSpecs().Return([]azure.VMExtensionSpec{
					{
						Name:      "my-extension-1",
						VMName:    "my-vm",
						Publisher: "some-publisher",
						Version:   "1.0",
					},
					{
						Name:      "other-extension",
						VMName:    "my-vm",
						Publisher: "other-publisher",
						Version:   "2.0",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm", "my-extension-1").
					Return(compute.VirtualMachineExtension{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdateAsync(gomockinternal.AContext(), "my-rg", "my-vm", "my-extension-1", gomock.AssignableToTypeOf(compute.VirtualMachineExtension{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_vmextensions.NewMockVMExtensionScope(mockCtrl)
			clientMock := mock_vmextensions.NewMockclient(mockCtrl)

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
