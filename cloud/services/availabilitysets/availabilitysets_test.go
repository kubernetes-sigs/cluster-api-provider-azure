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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

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
				s.AvailabilitySet().Return("as-name", true)
				s.ResourceGroup().Return("my-rg")
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
			name:          "noop if the machine does not need to be assigned an availability set (machines without a deployment)",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.AvailabilitySet().Return("as-name", false)
			},
			setupSKUs: func(svc *Service) {
			},
		},
		{
			name:          "return error",
			expectedError: "failed to create availability set as-name: something went wrong",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AvailabilitySet().Return("as-name", true)
				s.ResourceGroup().Return("my-rg")
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
				s.AvailabilitySet().Return("as-name", true)
				s.ResourceGroup().Return("my-rg").Times(2)
				m.Get(gomockinternal.AContext(), "my-rg", "as-name").
					Return(compute.AvailabilitySet{AvailabilitySetProperties: &compute.AvailabilitySetProperties{}}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "as-name").Return(nil)
			},
		},
		{
			name:          "noop if AvailabilitySet returns false",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.AvailabilitySet().Return("as-name", false)
			},
		},
		{
			name:          "noop if availability set has vms",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.AvailabilitySet().Return("as-name", true)
				s.ResourceGroup().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "as-name").Return(compute.AvailabilitySet{
					AvailabilitySetProperties: &compute.AvailabilitySetProperties{VirtualMachines: &[]compute.SubResource{
						{ID: to.StringPtr("vm-id")}}}}, nil)
			},
		},
		{
			name:          "noop if availability set is already deleted - get returns 404",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AvailabilitySet().Return("as-name", true)
				s.ResourceGroup().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "as-name").Return(compute.AvailabilitySet{},
					autorest.DetailedError{StatusCode: 404})
			},
		},
		{
			name:          "noop if availability set is already deleted - delete returns 404",
			expectedError: "",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AvailabilitySet().Return("as-name", true)
				s.ResourceGroup().Return("my-rg").Times(2)
				m.Get(gomockinternal.AContext(), "my-rg", "as-name").Return(compute.AvailabilitySet{}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "as-name").Return(autorest.DetailedError{StatusCode: 404})
			},
		},
		{
			name:          "returns error when availability set get fails",
			expectedError: "failed to get availability set as-name in resource group my-rg: something went wrong",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.AvailabilitySet().Return("as-name", true)
				s.ResourceGroup().Return("my-rg").Times(2)
				m.Get(gomockinternal.AContext(), "my-rg", "as-name").Return(compute.AvailabilitySet{},
					errors.New("something went wrong"))
			},
		},
		{
			name:          "returns error when delete fails",
			expectedError: "failed to delete availability set as-name in resource group my-rg: something went wrong",
			expect: func(s *mock_availabilitysets.MockAvailabilitySetScopeMockRecorder, m *mock_availabilitysets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AvailabilitySet().Return("as-name", true)
				s.ResourceGroup().Return("my-rg").Times(3)
				m.Get(gomockinternal.AContext(), "my-rg", "as-name").Return(compute.AvailabilitySet{}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "as-name").Return(errors.New("something went wrong"))
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
