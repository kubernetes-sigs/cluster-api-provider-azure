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

package publicips

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"

	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

	"sigs.k8s.io/cluster-api-provider-azure/azure"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips/mock_publicips"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

var (
	ipSpec1 = azure.PublicIPSpec{
		Name:          "my-publicip",
		DNSName:       "fakedns.mydomain.io",
		ResourceGroup: "test-group",
	}
	ipSpec2 = azure.PublicIPSpec{
		Name:          "my-publicip-ipv6",
		IsIPv6:        true,
		DNSName:       "fakename.mydomain.io",
		ResourceGroup: "test-group",
	}
	fakePublicIPSpecs = []azure.PublicIPSpec{
		ipSpec1,
		ipSpec2,
	}
	errCreate           = errors.New("error creating public IP")
	errCreate2          = errors.New("different error creating public IP")
	errTimeout          = errors.New("timed out while waiting for operation to complete")
	fakeCreateFuture, _ = azureautorest.NewFutureFromResponse(&http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Request: &http.Request{
			URL:    &url.URL{},
			Method: http.MethodPut,
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
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
					{
						Name: "my-publicip-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-publicip"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("bar"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip-2").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-publicip-2"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("buzz"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip-2")
			},
		},
		{
			name:          "public ip already deleted",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
					{
						Name: "my-publicip-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip-2").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-public-ip-2"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("buzz"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip-2")
			},
		},
		{
			name:          "public ip deletion fails",
			expectedError: "failed to delete public IP my-publicip in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-publicip"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("bar"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "skip unmanaged public ip deletion",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
					{
						Name: "my-publicip-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-public-ip"),
					Tags: map[string]*string{
						"foo": to.StringPtr("bar"),
					},
				}, nil)
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip-2").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-publicip-2"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("buzz"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip-2")
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
