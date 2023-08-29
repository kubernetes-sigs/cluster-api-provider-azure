/*
Copyright 2023 The Kubernetes Authors.

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

package privatelinks

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/privatelinks/mock_privatelinks"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

const (
	fakePrivateLinkValidSpec1Name = fakePrivateLinkName + "-valid-1"
	fakePrivateLinkValidSpec2Name = fakePrivateLinkName + "-valid-2"
)

var (
	// fakePrivateLinkValidSpec1 is valid private link spec with:
	// - 1 allowed subscription,
	// - 1 auto-approved subscription,
	// - disabled proxy protocol and
	// - additional tag "hello:capz".
	fakePrivateLinkValidSpec1 = PrivateLinkSpec{
		Name:              fakePrivateLinkValidSpec1Name,
		ResourceGroup:     fakeClusterName,
		SubscriptionID:    fakeSubscriptionID1,
		Location:          fakeRegion,
		VNetResourceGroup: fakeVNetResourceGroup,
		VNet:              fakeVNetName,
		NATIPConfiguration: []NATIPConfiguration{
			{
				AllocationMethod: string(network.Dynamic),
				Subnet:           fakeSubnetName,
			},
		},
		LoadBalancerName: fakeLbName,
		LBFrontendIPConfigNames: []string{
			fakeLbIPConfigName1,
		},
		AllowedSubscriptions: []string{
			fakeSubscriptionID1,
			fakeSubscriptionID2,
		},
		AutoApprovedSubscriptions: []string{
			fakeSubscriptionID1,
			fakeSubscriptionID2,
		},
		EnableProxyProtocol: ptr.To(false),
		ClusterName:         fakeClusterName,
		AdditionalTags: map[string]string{
			"hello": "capz",
		},
	}

	// fakePrivateLinkValidSpec2 is valid private link spec with:
	// - 2 allowed subscriptions,
	// - 2 auto-approved subscription,
	// - enabled proxy protocol and
	// - additional tag "hello:capz".
	fakePrivateLinkValidSpec2 = PrivateLinkSpec{
		Name:              fakePrivateLinkValidSpec2Name,
		ResourceGroup:     fakeClusterName,
		SubscriptionID:    fakeSubscriptionID1,
		Location:          fakeRegion,
		VNetResourceGroup: fakeVNetResourceGroup,
		VNet:              fakeVNetName,
		NATIPConfiguration: []NATIPConfiguration{
			{
				AllocationMethod: string(network.Dynamic),
				Subnet:           fakeSubnetName,
			},
		},
		LoadBalancerName: fakeLbName,
		LBFrontendIPConfigNames: []string{
			fakeLbIPConfigName1,
		},
		AllowedSubscriptions: []string{
			fakeSubscriptionID1,
		},
		AutoApprovedSubscriptions: []string{
			fakeSubscriptionID1,
		},
		EnableProxyProtocol: ptr.To(true),
		ClusterName:         fakeClusterName,
		AdditionalTags: map[string]string{
			"hello": "capz",
		},
	}

	emptyPrivateLinkSpec = PrivateLinkSpec{}

	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
	notDoneError  = azure.NewOperationNotDoneError(&infrav1.Future{})
)

func TestReconcilePrivateLink(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
		expectedError string
	}{
		{
			name:          "successfully create one private link",
			expectedError: "",
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateLinkValidSpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec1, ServiceName).Return(&fakePrivateLinkValidSpec1, nil)
				s.UpdatePutStatus(infrav1.PrivateLinksReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "successfully create multiple private links",
			expectedError: "",
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateLinkValidSpec1, &fakePrivateLinkValidSpec2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec1, ServiceName).Return(&fakePrivateLinkValidSpec1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec2, ServiceName).Return(&fakePrivateLinkValidSpec2, nil)
				s.UpdatePutStatus(infrav1.PrivateLinksReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "error when creating a private link using an empty spec",
			expectedError: internalError.Error(),
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&emptyPrivateLinkSpec})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &emptyPrivateLinkSpec, ServiceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.PrivateLinksReadyCondition, ServiceName, internalError)
			},
		},
		{
			name:          "not done error in creating is ignored",
			expectedError: internalError.Error(),
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateLinkValidSpec1, &fakePrivateLinkValidSpec2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec1, ServiceName).Return(&fakePrivateLinkValidSpec1, internalError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec2, ServiceName).Return(&fakePrivateLinkValidSpec2, notDoneError)
				s.UpdatePutStatus(infrav1.PrivateLinksReadyCondition, ServiceName, internalError)
			},
		},
		{
			name:          "not done error in creating remains",
			expectedError: notDoneError.Error(),
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateLinkValidSpec1, &fakePrivateLinkValidSpec2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec1, ServiceName).Return(&fakePrivateLinkValidSpec1, notDoneError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec2, ServiceName).Return(&fakePrivateLinkValidSpec2, nil)
				s.UpdatePutStatus(infrav1.PrivateLinksReadyCondition, ServiceName, notDoneError)
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
			scopeMock := mock_privatelinks.NewMockPrivateLinkScope(mockCtrl)
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

func TestDeletePrivateLinks(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "delete a private link",
			expectedError: "",
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateLinkValidSpec1})
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec1, ServiceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.PrivateLinksReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "delete multiple private links",
			expectedError: "",
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateLinkValidSpec1, &fakePrivateLinkValidSpec2})
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec2, ServiceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.PrivateLinksReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "noop if no private link specs are found",
			expectedError: "",
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "error when deleting a private link",
			expectedError: internalError.Error(),
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateLinkValidSpec1, &fakePrivateLinkValidSpec2})
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec2, ServiceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.PrivateLinksReadyCondition, ServiceName, internalError)
			},
		},
		{
			name:          "not done error when deleting a private link is ignored",
			expectedError: internalError.Error(),
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateLinkValidSpec1, &fakePrivateLinkValidSpec2})
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec1, ServiceName).Return(internalError)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec2, ServiceName).Return(notDoneError)
				s.UpdateDeleteStatus(infrav1.PrivateLinksReadyCondition, ServiceName, internalError)
			},
		},
		{
			name:          "not done error when deleting a private link remains",
			expectedError: notDoneError.Error(),
			expect: func(s *mock_privatelinks.MockPrivateLinkScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.PrivateLinkSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateLinkValidSpec1, &fakePrivateLinkValidSpec2})
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateLinkValidSpec2, ServiceName).Return(notDoneError)
				s.UpdateDeleteStatus(infrav1.PrivateLinksReadyCondition, ServiceName, notDoneError)
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
			scopeMock := mock_privatelinks.NewMockPrivateLinkScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

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
