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
	"errors"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

	"testing"

	"k8s.io/klog/klogr"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/availabilitysets/mock_availabilitysets"
)

func TestReconcileAvailabilitySets(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder)
		setupSKUs     func(svc *Service)
	}{
		{
			name:          "create or update availability set",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).MinTimes(2).Return(klogr.New())
				s.ResourceGroup().Return("my-rg")
				s.AvailabilitySetSpecs().Return([]azure.AvailabilitySetSpec{{Name: "as-name"}}).Times(2)
				s.ClusterName().Return("cl-name")
				s.AdditionalTags().Return(map[string]string{})
				s.Location().Return("test-location")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "as-name",
					compute.AvailabilitySet{
						Sku: &compute.Sku{Name: to.StringPtr("Aligned")},
						AvailabilitySetProperties: &compute.AvailabilitySetProperties{
							PlatformFaultDomainCount: pointer.Int32Ptr(3),
						},
						Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_cl-name": to.StringPtr("owned"),
							"sigs.k8s.io_cluster-api-provider-azure_role": to.StringPtr("common"), "Name": to.StringPtr("as-name")},
						Location: to.StringPtr("test-location"),
					}).Return(compute.AvailabilitySet{}, nil)
			},
			setupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Aligned"),
						Kind: to.StringPtr(string(resourceskus.AvailabilitySets)),
						Capabilities: &[]compute.ResourceSkuCapabilities{
							{
								Name:  to.StringPtr(resourceskus.MaximumPlatformFaultDomainCount),
								Value: to.StringPtr("3"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			name:          "noop if there are no availability set specs",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.AvailabilitySetSpecs().Return(nil)
			},
			setupSKUs: func(svc *Service) {
			},
		},
		{
			name:          "return error",
			expectedError: "failed to create availability set as-name: something went wrong",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().Return("my-rg")
				s.AvailabilitySetSpecs().Return([]azure.AvailabilitySetSpec{{Name: "as-name"}}).Times(2)
				s.ClusterName().Return("cl-name")
				s.AdditionalTags().Return(map[string]string{})
				s.Location().Return("test-location")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "as-name",
					gomock.AssignableToTypeOf(compute.AvailabilitySet{})).Return(compute.AvailabilitySet{}, errors.New("something went wrong"))
			},
			setupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Aligned"),
						Kind: to.StringPtr(string(resourceskus.AvailabilitySets)),
						Capabilities: &[]compute.ResourceSkuCapabilities{
							{
								Name:  to.StringPtr(resourceskus.MaximumPlatformFaultDomainCount),
								Value: to.StringPtr("3"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

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
			clientMock := mock_availabilitysets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
			}
			tc.setupSKUs(s)

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
		expect        func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder)
	}{
		{
			name:          "deletes availability set",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().Return("my-rg")
				s.AvailabilitySetSpecs().Return([]azure.AvailabilitySetSpec{{Name: "as-name"}})
				m.Delete(gomockinternal.AContext(), "my-rg", "as-name").Return(nil)
			},
		},
		{
			name:          "noop if there are no availability sets",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.AvailabilitySetSpecs().Return(nil)
			},
		},
		{
			name:          "returns error",
			expectedError: "failed to delete availability set as-name in resource group my-rg: something went wrong",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().Return("my-rg").MinTimes(2)
				s.AvailabilitySetSpecs().Return([]azure.AvailabilitySetSpec{{Name: "as-name"}})
				m.Delete(gomockinternal.AContext(), "my-rg", "as-name").Return(errors.New("something went wrong"))
			},
		},
		{
			name:          "noop if availability set is not found",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().Return("my-rg")
				s.AvailabilitySetSpecs().Return([]azure.AvailabilitySetSpec{{Name: "as-name"}})
				m.Delete(gomockinternal.AContext(), "my-rg", "as-name").Return(autorest.DetailedError{StatusCode: 404})
			},
		},
		{
			name:          "noop if availability set is not found, and continue to the next one",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().Return("my-rg").MinTimes(2)
				s.AvailabilitySetSpecs().Return([]azure.AvailabilitySetSpec{{Name: "as-name-not-found"}, {Name: "as-name"}})
				m.Delete(gomockinternal.AContext(), "my-rg", "as-name-not-found").Return(autorest.DetailedError{StatusCode: 404})
				m.Delete(gomockinternal.AContext(), "my-rg", "as-name").Return(nil)
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
			clientMock := mock_availabilitysets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
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
