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

package publicips_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"

	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips/mock_publicips"

	"github.com/golang/mock/gomock"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

var (
	serviceName = "publicips"
	ipSpec1     = publicips.PublicIPSpec{
		Name:           "my-publicip",
		DNSName:        "fakedns.mydomain.io",
		ResourceGroup:  "test-group",
		Location:       "location",
		ClusterName:    "cluster-name",
		AdditionalTags: infrav1.Tags{"foo": "bar"},
		Zones:          []string{"1", "2", "3"},
	}
	ipSpec2 = publicips.PublicIPSpec{
		Name:           "my-publicip-ipv6",
		IsIPv6:         true,
		DNSName:        "fakename.mydomain.io",
		ResourceGroup:  "test-group",
		Location:       "location",
		ClusterName:    "cluster-name",
		AdditionalTags: infrav1.Tags{"foo": "bar"},
		Zones:          []string{"1", "2", "3"},
	}
	fakePublicIPSpecs = []publicips.PublicIPSpec{
		ipSpec1,
		ipSpec2,
	}
	errCreate           = errors.New("error creating public IP")
	errCreate2          = errors.New("different error creating public IP")
	errGet              = errors.New("error getting public IP")
	errDelete           = errors.New("error deleting public IP")
	errDelete2          = errors.New("different error deleting public IP")
	errTimeout          = errors.New("timed out while waiting for operation to complete")
	fakeCreateFuture, _ = azureautorest.NewFutureFromResponse(&http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Request: &http.Request{
			URL:    &url.URL{},
			Method: http.MethodPut,
		},
	})
	fakeDeleteFuture, _ = azureautorest.NewFutureFromResponse(&http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Request: &http.Request{
			URL:    &url.URL{},
			Method: http.MethodDelete,
		},
	})
)

func TestReconcilePublicIP(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder)
	}{
		{
			name:          "creation of multiple public IPs succeeds",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				gomock.InOrder(
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec1).Return(nil, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec2).Return(nil, nil),
					s.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "first public IP creation fails",
			expectedError: "failed to create resource test-group/my-publicip (service: publicips): error creating public IP",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				gomock.InOrder(
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec1).Return(nil, errCreate),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec2).Return(nil, nil),
					s.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to create resource test-group/my-publicip (service: publicips): error creating public IP")),
				)
			},
		},
		{
			name:          "second public IP creation is in progress and not done",
			expectedError: "transient reconcile error occurred: operation type PUT on Azure resource test-group/my-publicip-ipv6 is not done. Object will be requeued after 15s",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				gomock.InOrder(
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec1).Return(nil, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec2).Return(&fakeCreateFuture, errTimeout),
					s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{})),
					s.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, gomockinternal.ErrStrEq("transient reconcile error occurred: operation type PUT on Azure resource test-group/my-publicip-ipv6 is not done. Object will be requeued after 15s")),
				)
			},
		},
		{
			name:          "return the most pressing error when first public IP creation fails and second public IP creation is in progress and not done",
			expectedError: "failed to create resource test-group/my-publicip (service: publicips): error creating public IP",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				gomock.InOrder(
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec1).Return(nil, errCreate),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec2).Return(&fakeCreateFuture, errTimeout),
					s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{})),
					s.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to create resource test-group/my-publicip (service: publicips): error creating public IP")),
				)
			},
		},
		{
			name:          "return the last most pressing error when both public IP creation fails",
			expectedError: "failed to create resource test-group/my-publicip-ipv6 (service: publicips): different error creating public IP",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				gomock.InOrder(
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec1).Return(nil, errCreate),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.CreateOrUpdateAsync(gomockinternal.AContext(), ipSpec2).Return(nil, errCreate2),
					s.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to create resource test-group/my-publicip-ipv6 (service: publicips): different error creating public IP")),
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
			scopeMock := mock_publicips.NewMockPublicIPScope(mockCtrl)
			clientMock := mock_publicips.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &publicips.Service{
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

func TestDeletePublicIP(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete two existing public IP",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				s.ResourceGroup().AnyTimes().Return("test-group")
				s.ClusterName().AnyTimes().Return("cluster-name")

				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("bar"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec1).Return(nil, nil),
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip-ipv6").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip-ipv6"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("buzz"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec2).Return(nil, nil),
					s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "skip unmanaged public ip deletion",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				s.ResourceGroup().AnyTimes().Return("test-group")
				s.ClusterName().AnyTimes().Return("cluster-name")

				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-public-ip"),
						Tags: map[string]*string{
							"foo": to.StringPtr("bar"),
						},
					}, nil),
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip-ipv6").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip-ipv6"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("buzz"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec2).Return(nil, nil),
					s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "failure while getting public IP management state",
			expectedError: "could not get management state of test-group/my-publicip public ip: error getting public IP",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				s.ResourceGroup().AnyTimes().Return("test-group")
				s.ClusterName().AnyTimes().Return("cluster-name")

				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip").Return(network.PublicIPAddress{}, errGet),
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip-ipv6").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip-ipv6"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("buzz"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec2).Return(nil, nil),
					s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, gomockinternal.ErrStrEq("could not get management state of test-group/my-publicip public ip: error getting public IP")),
				)
			},
		},
		{
			name:          "skip deletion of public ip that is already deleted",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				s.ResourceGroup().AnyTimes().Return("test-group")
				s.ClusterName().AnyTimes().Return("cluster-name")

				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")),
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip-ipv6").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-public-ipv6"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("buzz"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec2).Return(nil, nil),
					s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "public ip deletion fails",
			expectedError: "failed to delete resource test-group/my-publicip (service: publicips): error deleting public IP",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				s.ResourceGroup().AnyTimes().Return("test-group")
				s.ClusterName().AnyTimes().Return("cluster-name")

				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("bar"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec1).Return(nil, errDelete),
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip-ipv6").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip-ipv6"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("buzz"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec2).Return(nil, nil),
					s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to delete resource test-group/my-publicip (service: publicips): error deleting public IP")),
				)
			},
		},
		{
			name:          "second public IP deletion is in progress and not done",
			expectedError: "transient reconcile error occurred: operation type DELETE on Azure resource test-group/my-publicip-ipv6 is not done. Object will be requeued after 15s",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				s.ResourceGroup().AnyTimes().Return("test-group")
				s.ClusterName().AnyTimes().Return("cluster-name")

				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("bar"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec1).Return(nil, nil),
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip-ipv6").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip-ipv6"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("buzz"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec2).Return(&fakeDeleteFuture, errTimeout),
					s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{})),
					s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, gomockinternal.ErrStrEq("transient reconcile error occurred: operation type DELETE on Azure resource test-group/my-publicip-ipv6 is not done. Object will be requeued after 15s")),
				)
			},
		},
		{
			name:          "return the most pressing error when first public IP deletion fails and second public IP deletion is in progress and not done",
			expectedError: "failed to delete resource test-group/my-publicip (service: publicips): error deleting public IP",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				s.ResourceGroup().AnyTimes().Return("test-group")
				s.ClusterName().AnyTimes().Return("cluster-name")

				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("bar"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec1).Return(nil, errDelete),
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip-ipv6").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip-ipv6"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("buzz"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec2).Return(&fakeDeleteFuture, errTimeout),
					s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{})),
					s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to delete resource test-group/my-publicip (service: publicips): error deleting public IP")),
				)
			},
		},
		{
			name:          "return the last most pressing error when both public IP deletion fails",
			expectedError: "failed to delete resource test-group/my-publicip-ipv6 (service: publicips): different error deleting public IP",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return(fakePublicIPSpecs)
				s.ResourceGroup().AnyTimes().Return("test-group")
				s.ClusterName().AnyTimes().Return("cluster-name")

				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("bar"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec1).Return(nil, errDelete),
					m.Get(gomockinternal.AContext(), "test-group", "my-publicip-ipv6").Return(network.PublicIPAddress{
						Name: to.StringPtr("my-publicip-ipv6"),
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"foo": to.StringPtr("buzz"),
						},
					}, nil),
					s.GetLongRunningOperationState("my-publicip-ipv6", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), ipSpec2).Return(nil, errDelete2),
					s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to delete resource test-group/my-publicip-ipv6 (service: publicips): different error deleting public IP")),
				)
			},
		},
		// TODO(karuppiah7890): Test for - Return the most pressing error when there's an error when deleting public IP (first) and an error getting public IP management state (second) (pressing error). This test will fail, need to implement this.
		// For fixing the test, store error-getting-management-state only when result is not public-ip-deletion-failure that is - when result is nil or when result is public IP deletion in progress.
		// TODO(karuppiah7890): Test for - Return the most pressing error when there's an error getting public IP management state (pressing error) and a public IP deletion is in progress. This should already pass
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_publicips.NewMockPublicIPScope(mockCtrl)
			clientMock := mock_publicips.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &publicips.Service{
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
