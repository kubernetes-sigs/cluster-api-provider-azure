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

package networkinterfaces

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/asyncpoller/mock_asyncpoller"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/networkinterfaces/mock_networkinterfaces"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeNICSpec1 = NICSpec{
		Name:                  "nic-1",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		VNetResourceGroup:     "my-rg",
		AcceleratedNetworking: nil,
		SKU:                   &fakeSku,
	}
	fakeNICSpec2 = NICSpec{
		Name:                  "nic-2",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		VNetResourceGroup:     "my-rg",
		AcceleratedNetworking: nil,
		SKU:                   &fakeSku,
	}
	fakeNICSpec3 = NICSpec{
		Name:                  "nic-3",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		VNetName:              "my-vnet",
		VNetResourceGroup:     "my-rg",
		AcceleratedNetworking: nil,
		SKU:                   &fakeSku,
		IPConfigs:             []IPConfig{{}, {}},
	}
	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
)

func TestReconcileNetworkInterface(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no network interface specs are found",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder) {
				s.NICSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "successfully create a network interface",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder) {
				s.NICSpecs().Return([]azure.ResourceSpecGetter{&fakeNICSpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNICSpec1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.NetworkInterfaceReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "successfully create a network interface with multiple IPConfigs",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder) {
				s.NICSpecs().Return([]azure.ResourceSpecGetter{&fakeNICSpec3})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNICSpec3, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.NetworkInterfaceReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "successfully create multiple network interfaces",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder) {
				s.NICSpecs().Return([]azure.ResourceSpecGetter{&fakeNICSpec1, &fakeNICSpec2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNICSpec1, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNICSpec2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.NetworkInterfaceReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "network interface create fails",
			expectedError: internalError.Error(),
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder) {
				s.NICSpecs().Return([]azure.ResourceSpecGetter{&fakeNICSpec1, &fakeNICSpec2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNICSpec1, serviceName).Return(nil, internalError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNICSpec2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.NetworkInterfaceReadyCondition, serviceName, internalError)
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
			scopeMock := mock_networkinterfaces.NewMockNICScope(mockCtrl)
			asyncMock := mock_asyncpoller.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				fmt.Print(cmp.Diff(err.Error(), tc.expectedError))

				g.Expect(err.Error()).To(Equal(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteNetworkInterface(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no network interface specs are found",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder) {
				s.NICSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "successfully delete an existing network interface",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder) {
				s.NICSpecs().Return([]azure.ResourceSpecGetter{&fakeNICSpec1})
				r.DeleteResource(gomockinternal.AContext(), &fakeNICSpec1, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.NetworkInterfaceReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "successfully delete multiple existing network interfaces",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder) {
				s.NICSpecs().Return([]azure.ResourceSpecGetter{&fakeNICSpec1, &fakeNICSpec2})
				r.DeleteResource(gomockinternal.AContext(), &fakeNICSpec1, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeNICSpec2, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.NetworkInterfaceReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "network interface deletion fails",
			expectedError: internalError.Error(),
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, r *mock_asyncpoller.MockReconcilerMockRecorder) {
				s.NICSpecs().Return([]azure.ResourceSpecGetter{&fakeNICSpec1, &fakeNICSpec2})
				r.DeleteResource(gomockinternal.AContext(), &fakeNICSpec1, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeNICSpec2, serviceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.NetworkInterfaceReadyCondition, serviceName, internalError)
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
			scopeMock := mock_networkinterfaces.NewMockNICScope(mockCtrl)
			asyncMock := mock_asyncpoller.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
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
