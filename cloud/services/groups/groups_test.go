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

package groups

import (
	"context"
	"net/http"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/groups/mock_groups"

	"github.com/golang/mock/gomock"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
)

func TestReconcileGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder)
	}{
		{
			name:          "resource group already exist",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().Return("my-rg")
				m.Get(context.TODO(), "my-rg").Return(resources.Group{}, nil)
			},
		},
		{
			name:          "create a resource group",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				m.Get(context.TODO(), "my-rg").Return(resources.Group{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(context.TODO(), "my-rg", gomock.AssignableToTypeOf(resources.Group{})).Return(resources.Group{}, nil)
			},
		},
		{
			name:          "return error when creating a resource group",
			expectedError: "failed to create resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				m.Get(context.TODO(), "my-rg").Return(resources.Group{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(context.TODO(), "my-rg", gomock.AssignableToTypeOf(resources.Group{})).Return(resources.Group{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_groups.NewMockGroupScope(mockCtrl)
			clientMock := mock_groups.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
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

func TestDeleteGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder)
	}{
		{
			name:          "error getting the resource group management state",
			expectedError: "could not get resource group management state: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(context.TODO(), "my-rg").Return(resources.Group{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "skip deletion in unmanaged mode",
			expectedError: azure.ErrNotOwned.Error(),
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				m.Get(context.TODO(), "my-rg").Return(resources.Group{}, nil)
			},
		},
		{
			name:          "resource group already deleted",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				gomock.InOrder(
					m.Get(context.TODO(), "my-rg").Return(resources.Group{
						Tags: converters.TagsToMap(infrav1.Tags{
							"Name": "my-rg",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_fake-cluster": "owned",
							"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
						}),
					}, nil),
					m.Delete(context.TODO(), "my-rg").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found")),
				)
			},
		},
		{
			name:          "resource group deletion fails",
			expectedError: "failed to delete resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				gomock.InOrder(
					m.Get(context.TODO(), "my-rg").Return(resources.Group{
						Tags: converters.TagsToMap(infrav1.Tags{
							"Name": "my-rg",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_fake-cluster": "owned",
							"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
						}),
					}, nil),
					m.Delete(context.TODO(), "my-rg").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")),
				)
			},
		},
		{
			name:          "resource group deletion successfully",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				gomock.InOrder(
					m.Get(context.TODO(), "my-rg").Return(resources.Group{
						Tags: converters.TagsToMap(infrav1.Tags{
							"Name": "my-rg",
							"sigs.k8s.io_cluster-api-provider-azure_cluster_fake-cluster": "owned",
							"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
						}),
					}, nil),
					m.Delete(context.TODO(), "my-rg").Return(nil),
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
			scopeMock := mock_groups.NewMockGroupScope(mockCtrl)
			clientMock := mock_groups.NewMockClient(mockCtrl)

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
