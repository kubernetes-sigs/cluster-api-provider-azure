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
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips/mock_publicips"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

var (
	fakePublicIPSpec1 = PublicIPSpec{
		Name:           "my-publicip",
		ResourceGroup:  "my-rg",
		DNSName:        "fakedns.mydomain.io",
		IsIPv6:         false,
		ClusterName:    "my-cluster",
		Location:       "centralIndia",
		FailureDomains: []string{"failure-domain-id-1", "failure-domain-id-2", "failure-domain-id-3"},
		AdditionalTags: infrav1.Tags{
			"Name": "my-publicip-ipv6",
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
		},
	}
	fakePublicIPSpec2 = PublicIPSpec{
		Name:           "my-publicip-2",
		ResourceGroup:  "my-rg",
		DNSName:        "fakedns2-52959.uksouth.cloudapp.azure.com",
		IsIPv6:         false,
		ClusterName:    "my-cluster",
		Location:       "centralIndia",
		FailureDomains: []string{"failure-domain-id-1", "failure-domain-id-2", "failure-domain-id-3"},
		AdditionalTags: infrav1.Tags{
			"Name": "my-publicip-ipv6",
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
		},
	}
	fakePublicIPSpec3 = PublicIPSpec{
		Name:           "my-publicip-3",
		ResourceGroup:  "my-rg",
		DNSName:        "",
		IsIPv6:         false,
		ClusterName:    "my-cluster",
		Location:       "centralIndia",
		FailureDomains: []string{"failure-domain-id-1", "failure-domain-id-2", "failure-domain-id-3"},
		AdditionalTags: infrav1.Tags{
			"Name": "my-publicip-ipv6",
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
		},
	}
	fakePublicIPSpecIpv6 = PublicIPSpec{
		Name:           "my-publicip-ipv6",
		ResourceGroup:  "my-rg",
		DNSName:        "fakename.mydomain.io",
		IsIPv6:         true,
		ClusterName:    "my-cluster",
		Location:       "centralIndia",
		FailureDomains: []string{"failure-domain-id-1", "failure-domain-id-2", "failure-domain-id-3"},
		AdditionalTags: infrav1.Tags{
			"Name": "my-publicip-ipv6",
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
			"foo": "bar",
		},
	}

	fakeUnmanagedPublicIP = network.PublicIPAddress{
		Name: to.StringPtr("my-publicip-1"),
		Tags: map[string]*string{
			"foo": to.StringPtr("bar"),
		},
	}
	fakeManagedPublicIP = network.PublicIPAddress{
		Name: to.StringPtr("my-publicip-2"),
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
			"foo": to.StringPtr("bar"),
		},
	}

	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")
)

func TestReconcilePublicIP(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "successfully create public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})
				r.CreateResource(gomockinternal.AContext(), &fakePublicIPSpec1, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePublicIPSpec2, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePublicIPSpec3, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePublicIPSpecIpv6, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "fail to create a public IP",
			expectedError: internalError.Error(),
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})
				r.CreateResource(gomockinternal.AContext(), &fakePublicIPSpec1, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePublicIPSpec2, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), &fakePublicIPSpec3, serviceName).Return(nil, internalError)
				r.CreateResource(gomockinternal.AContext(), &fakePublicIPSpecIpv6, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, internalError)
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
			getterMock := mock_async.NewMockGetter(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), getterMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Getter:     getterMock,
				Reconciler: reconcilerMock,
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
		expect        func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "successfully delete managed public IPs and ignore unmanaged public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})

				m.Get(gomockinternal.AContext(), &fakePublicIPSpec1).Return(fakeManagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec1, serviceName).Return(nil)

				m.Get(gomockinternal.AContext(), &fakePublicIPSpec2).Return(fakeManagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec2, serviceName).Return(nil)

				m.Get(gomockinternal.AContext(), &fakePublicIPSpec3).Return(fakeUnmanagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")

				m.Get(gomockinternal.AContext(), &fakePublicIPSpecIpv6).Return(fakeManagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpecIpv6, serviceName).Return(nil)

				s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "noop if no managed public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})

				m.Get(gomockinternal.AContext(), &fakePublicIPSpec1).Return(fakeUnmanagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")

				m.Get(gomockinternal.AContext(), &fakePublicIPSpec2).Return(fakeUnmanagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")

				m.Get(gomockinternal.AContext(), &fakePublicIPSpec3).Return(fakeUnmanagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")

				m.Get(gomockinternal.AContext(), &fakePublicIPSpecIpv6).Return(fakeUnmanagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")
			},
		},
		{
			name:          "fail to delete managed public IP",
			expectedError: internalError.Error(),
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})

				m.Get(gomockinternal.AContext(), &fakePublicIPSpec1).Return(fakeManagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec1, serviceName).Return(nil)

				m.Get(gomockinternal.AContext(), &fakePublicIPSpec2).Return(fakeManagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec2, serviceName).Return(nil)

				m.Get(gomockinternal.AContext(), &fakePublicIPSpec3).Return(fakeManagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec3, serviceName).Return(internalError)

				m.Get(gomockinternal.AContext(), &fakePublicIPSpecIpv6).Return(fakeManagedPublicIP, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpecIpv6, serviceName).Return(nil)

				s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, internalError)
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
			getterMock := mock_async.NewMockGetter(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), getterMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Getter:     getterMock,
				Reconciler: reconcilerMock,
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
