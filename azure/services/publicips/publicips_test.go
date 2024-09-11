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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips/mock_publicips"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
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
		FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
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
		FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
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
		FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
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
		FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
		AdditionalTags: infrav1.Tags{
			"Name": "my-publicip-ipv6",
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
			"foo": "bar",
		},
	}

	managedTags = armresources.TagsResource{
		Properties: &armresources.Tags{
			Tags: map[string]*string{
				"foo": ptr.To("bar"),
				"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			},
		},
	}

	unmanagedTags = armresources.TagsResource{
		Properties: &armresources.Tags{
			Tags: map[string]*string{
				"foo":       ptr.To("bar"),
				"something": ptr.To("else"),
			},
		},
	}

	internalError = &azcore.ResponseError{
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Internal Server Error: StatusCode=500")),
			StatusCode: http.StatusInternalServerError,
		},
	}
)

func TestReconcilePublicIP(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, _ *mock_async.MockTagsGetterMockRecorder, _ *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "successfully create public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, _ *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicIPSpec1, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicIPSpec2, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicIPSpec3, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicIPSpecIpv6, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "fail to create a public IP",
			expectedError: internalError.Error(),
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, _ *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicIPSpec1, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicIPSpec2, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicIPSpec3, serviceName).Return(nil, internalError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicIPSpecIpv6, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, internalError)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_publicips.NewMockPublicIPScope(mockCtrl)
			tagsGetterMock := mock_async.NewMockTagsGetter(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), tagsGetterMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				TagsGetter: tagsGetterMock,
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
		expect        func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, _ *mock_async.MockTagsGetterMockRecorder, _ *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "successfully delete managed public IPs and ignore unmanaged public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpec1.ResourceGroupName(), fakePublicIPSpec1.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec1, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpec2.ResourceGroupName(), fakePublicIPSpec2.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec2, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpec3.ResourceGroupName(), fakePublicIPSpec3.ResourceName())).Return(unmanagedTags, nil)
				s.ClusterName().Return("my-cluster")

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpecIpv6.ResourceGroupName(), fakePublicIPSpecIpv6.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpecIpv6, serviceName).Return(nil)

				s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "noop if no managed public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, _ *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpec1.ResourceGroupName(), fakePublicIPSpec1.ResourceName())).Return(unmanagedTags, nil)
				s.ClusterName().Return("my-cluster")

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpec2.ResourceGroupName(), fakePublicIPSpec2.ResourceName())).Return(unmanagedTags, nil)
				s.ClusterName().Return("my-cluster")

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpec3.ResourceGroupName(), fakePublicIPSpec3.ResourceName())).Return(unmanagedTags, nil)
				s.ClusterName().Return("my-cluster")

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpecIpv6.ResourceGroupName(), fakePublicIPSpecIpv6.ResourceName())).Return(unmanagedTags, nil)
				s.ClusterName().Return("my-cluster")
			},
		},
		{
			name:          "fail to delete managed public IP",
			expectedError: internalError.Error(),
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PublicIPSpecs().Return([]azure.ResourceSpecGetter{&fakePublicIPSpec1, &fakePublicIPSpec2, &fakePublicIPSpec3, &fakePublicIPSpecIpv6})

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpec1.ResourceGroupName(), fakePublicIPSpec1.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec1, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpec2.ResourceGroupName(), fakePublicIPSpec2.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec2, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpec3.ResourceGroupName(), fakePublicIPSpec3.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpec3, serviceName).Return(internalError)

				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.PublicIPID("123", fakePublicIPSpecIpv6.ResourceGroupName(), fakePublicIPSpecIpv6.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return("my-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakePublicIPSpecIpv6, serviceName).Return(nil)

				s.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, internalError)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_publicips.NewMockPublicIPScope(mockCtrl)
			tagsGetterMock := mock_async.NewMockTagsGetter(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), tagsGetterMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				TagsGetter: tagsGetterMock,
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
