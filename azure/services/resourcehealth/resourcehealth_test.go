/*
Copyright 2022 The Kubernetes Authors.

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

package resourcehealth

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcehealth/armresourcehealth"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourcehealth/mock_resourcehealth"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileResourceHealth(t *testing.T) {
	testcases := []struct {
		name            string
		featureDisabled bool
		filterEnabled   bool
		expect          func(s *mock_resourcehealth.MockResourceHealthScopeMockRecorder, m *mock_resourcehealth.MockclientMockRecorder, f *mock_resourcehealth.MockAvailabilityStatusFiltererMockRecorder)
		expectedError   string
	}{
		{
			name: "available resource",
			expect: func(s *mock_resourcehealth.MockResourceHealthScopeMockRecorder, m *mock_resourcehealth.MockclientMockRecorder, _ *mock_resourcehealth.MockAvailabilityStatusFiltererMockRecorder) {
				s.AvailabilityStatusResource().Times(1)
				s.AvailabilityStatusResourceURI().Times(1)
				m.GetByResource(gomockinternal.AContext(), gomock.Any()).Times(1).Return(armresourcehealth.AvailabilityStatus{
					Properties: &armresourcehealth.AvailabilityStatusProperties{
						AvailabilityState: ptr.To(armresourcehealth.AvailabilityStateValuesAvailable),
					},
				}, nil)
			},
			expectedError: "",
		},
		{
			name: "unavailable resource",
			expect: func(s *mock_resourcehealth.MockResourceHealthScopeMockRecorder, m *mock_resourcehealth.MockclientMockRecorder, _ *mock_resourcehealth.MockAvailabilityStatusFiltererMockRecorder) {
				s.AvailabilityStatusResource().Times(1)
				s.AvailabilityStatusResourceURI().Times(1)
				m.GetByResource(gomockinternal.AContext(), gomock.Any()).Times(1).Return(armresourcehealth.AvailabilityStatus{
					Properties: &armresourcehealth.AvailabilityStatusProperties{
						AvailabilityState: ptr.To(armresourcehealth.AvailabilityStateValuesUnavailable),
						Summary:           ptr.To("summary"),
					},
				}, nil)
			},
			expectedError: "resource is not available: summary",
		},
		{
			name: "API error",
			expect: func(s *mock_resourcehealth.MockResourceHealthScopeMockRecorder, m *mock_resourcehealth.MockclientMockRecorder, _ *mock_resourcehealth.MockAvailabilityStatusFiltererMockRecorder) {
				s.AvailabilityStatusResourceURI().Times(1).Return("myURI")
				m.GetByResource(gomockinternal.AContext(), gomock.Any()).Times(1).Return(armresourcehealth.AvailabilityStatus{}, errors.New("some API error"))
			},
			expectedError: "failed to get availability status for resource myURI: some API error",
		},
		{
			name:          "filter",
			filterEnabled: true,
			expect: func(s *mock_resourcehealth.MockResourceHealthScopeMockRecorder, m *mock_resourcehealth.MockclientMockRecorder, f *mock_resourcehealth.MockAvailabilityStatusFiltererMockRecorder) {
				s.AvailabilityStatusResource().Times(1)
				s.AvailabilityStatusResourceURI().Times(1)
				m.GetByResource(gomockinternal.AContext(), gomock.Any()).Times(1).Return(armresourcehealth.AvailabilityStatus{
					Properties: &armresourcehealth.AvailabilityStatusProperties{
						AvailabilityState: ptr.To(armresourcehealth.AvailabilityStateValuesUnavailable),
						Summary:           ptr.To("summary"),
					},
				}, nil)
				// ignore the above status
				f.AvailabilityStatusFilter(gomock.Any()).Return(conditions.TrueCondition(infrav1.AzureResourceAvailableCondition))
			},
			expectedError: "",
		},
		{
			name:            "feature disabled",
			featureDisabled: true,
			expect: func(s *mock_resourcehealth.MockResourceHealthScopeMockRecorder, _ *mock_resourcehealth.MockclientMockRecorder, _ *mock_resourcehealth.MockAvailabilityStatusFiltererMockRecorder) {
				s.AvailabilityStatusResource().Times(1)
			},
			expectedError: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_resourcehealth.NewMockResourceHealthScope(mockCtrl)
			clientMock := mock_resourcehealth.NewMockclient(mockCtrl)
			filtererMock := mock_resourcehealth.NewMockAvailabilityStatusFilterer(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), filtererMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
			}
			if tc.filterEnabled {
				s.Scope = struct {
					ResourceHealthScope
					AvailabilityStatusFilterer
				}{scopeMock, filtererMock}
			}

			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.AKSResourceHealth, !tc.featureDisabled)

			err := s.Reconcile(t.Context())

			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
