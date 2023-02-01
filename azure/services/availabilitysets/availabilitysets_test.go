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

package availabilitysets

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/availabilitysets/mock_availabilitysets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeFaultDomainCount = 3
	fakeSku              = resourceskus.SKU{
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  pointer.String(resourceskus.MaximumPlatformFaultDomainCount),
				Value: pointer.String(strconv.Itoa(fakeFaultDomainCount)),
			},
		},
	}
	fakeSetSpec = AvailabilitySetSpec{
		Name:           "test-as",
		ResourceGroup:  "test-rg",
		ClusterName:    "test-cluster",
		Location:       "test-location",
		SKU:            &fakeSku,
		AdditionalTags: map[string]string{},
	}
	fakeSetSpecMissing = AvailabilitySetSpec{
		Name:           "test-as",
		ResourceGroup:  "test-rg",
		ClusterName:    "test-cluster",
		Location:       "test-location",
		SKU:            nil,
		AdditionalTags: map[string]string{},
	}
	internalError  = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
	parameterError = errors.Errorf("some error with parameters")
	notFoundError  = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not Found")
	fakeSetWithVMs = compute.AvailabilitySet{
		AvailabilitySetProperties: &compute.AvailabilitySetProperties{
			VirtualMachines: &[]compute.SubResource{
				{ID: pointer.String("vm-id")},
			},
		},
	}
)

func TestReconcileAvailabilitySets(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "create or update availability set",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSetSpec, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.AvailabilitySetReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "noop if no availability set spec returns nil",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(nil)
			},
		},
		{
			name:          "missing required value in availability set spec",
			expectedError: "some error with parameters",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpecMissing)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSetSpecMissing, serviceName).Return(nil, parameterError)
				s.UpdatePutStatus(infrav1.AvailabilitySetReadyCondition, serviceName, parameterError)
			},
		},
		{
			name:          "error in creating availability set",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSetSpec, serviceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.AvailabilitySetReadyCondition, serviceName, internalError)
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
			scopeMock := mock_availabilitysets.NewMockAvailabilitySetScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
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

func TestDeleteAvailabilitySets(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "deletes availability set",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpec)
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), &fakeSetSpec).Return(compute.AvailabilitySet{}, nil),
					r.DeleteResource(gomockinternal.AContext(), &fakeSetSpec, serviceName).Return(nil),
					s.UpdateDeleteStatus(infrav1.AvailabilitySetReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "noop if AvailabilitySetSpec returns nil",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(nil)
			},
		},
		{
			name:          "delete proceeds with missing required value in availability set spec",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpecMissing)
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), &fakeSetSpecMissing).Return(compute.AvailabilitySet{}, nil),
					r.DeleteResource(gomockinternal.AContext(), &fakeSetSpecMissing, serviceName).Return(nil),
					s.UpdateDeleteStatus(infrav1.AvailabilitySetReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "noop if availability set has vms",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpec)
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), &fakeSetSpec).Return(fakeSetWithVMs, nil),
					s.UpdateDeleteStatus(infrav1.AvailabilitySetReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "availability set not found",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpec)
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), &fakeSetSpec).Return(nil, notFoundError),
					s.UpdateDeleteStatus(infrav1.AvailabilitySetReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "error in getting availability set",
			expectedError: "failed to get availability set test-as in resource group test-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpec)
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), &fakeSetSpec).Return(nil, internalError),
					s.UpdateDeleteStatus(infrav1.AvailabilitySetReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to get availability set test-as in resource group test-rg: #: Internal Server Error: StatusCode=500")),
				)
			},
		},
		{
			name:          "availability set get result is not an availability set",
			expectedError: "string is not a compute.AvailabilitySet",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpec)
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), &fakeSetSpec).Return("not an availability set", nil),
					s.UpdateDeleteStatus(infrav1.AvailabilitySetReadyCondition, serviceName, gomockinternal.ErrStrEq("string is not a compute.AvailabilitySet")),
				)
			},
		},
		{
			name:          "error in deleting availability set",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AvailabilitySetSpec().Return(&fakeSetSpec)
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), &fakeSetSpec).Return(compute.AvailabilitySet{}, nil),
					r.DeleteResource(gomockinternal.AContext(), &fakeSetSpec, serviceName).Return(internalError),
					s.UpdateDeleteStatus(infrav1.AvailabilitySetReadyCondition, serviceName, internalError),
				)
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
			scopeMock := mock_availabilitysets.NewMockAvailabilitySetScope(mockCtrl)
			getterMock := mock_async.NewMockGetter(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), getterMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Getter:     getterMock,
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
