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

package loadbalancers

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/loadbalancers/mock_loadbalancers"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakePublicAPILBSpec = LBSpec{
		Name:                 "my-publiclb",
		ResourceGroup:        "my-rg",
		SubscriptionID:       "123",
		ClusterName:          "my-cluster",
		Location:             "my-location",
		Role:                 infrav1.APIServerRole,
		Type:                 infrav1.Public,
		SKU:                  infrav1.SKUStandard,
		SubnetName:           "my-cp-subnet",
		BackendPoolName:      "my-publiclb-backendPool",
		IdleTimeoutInMinutes: pointer.Int32(4),
		FrontendIPConfigs: []infrav1.FrontendIP{
			{
				Name: "my-publiclb-frontEnd",
				PublicIP: &infrav1.PublicIPSpec{
					Name:    "my-publicip",
					DNSName: "my-cluster.12345.mydomain.com",
				},
			},
		},
		APIServerPort: 6443,
	}

	fakeInternalAPILBSpec = LBSpec{
		Name:                 "my-private-lb",
		ResourceGroup:        "my-rg",
		SubscriptionID:       "123",
		ClusterName:          "my-cluster",
		Location:             "my-location",
		Role:                 infrav1.APIServerRole,
		Type:                 infrav1.Internal,
		SKU:                  infrav1.SKUStandard,
		SubnetName:           "my-cp-subnet",
		BackendPoolName:      "my-private-lb-backendPool",
		IdleTimeoutInMinutes: pointer.Int32(4),
		FrontendIPConfigs: []infrav1.FrontendIP{
			{
				Name: "my-private-lb-frontEnd",
				FrontendIPClass: infrav1.FrontendIPClass{
					PrivateIPAddress: "10.0.0.10",
				},
			},
		},
		APIServerPort: 6443,
	}

	fakeNodeOutboundLBSpec = LBSpec{
		Name:                 "my-cluster",
		ResourceGroup:        "my-rg",
		SubscriptionID:       "123",
		ClusterName:          "my-cluster",
		Location:             "my-location",
		Role:                 infrav1.NodeOutboundRole,
		Type:                 infrav1.Public,
		SKU:                  infrav1.SKUStandard,
		BackendPoolName:      "my-cluster-outboundBackendPool",
		IdleTimeoutInMinutes: pointer.Int32(30),
		FrontendIPConfigs: []infrav1.FrontendIP{
			{
				Name: "my-cluster-frontEnd",
				PublicIP: &infrav1.PublicIPSpec{
					Name: "outbound-publicip",
				},
			},
		},
	}

	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
)

func TestReconcileLoadBalancer(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no LBSpecs are found",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "fail to create a public LB",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{&fakePublicAPILBSpec})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicAPILBSpec, serviceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.LoadBalancersReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "create public apiserver LB",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{&fakePublicAPILBSpec})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicAPILBSpec, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.LoadBalancersReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create internal apiserver LB",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{&fakeInternalAPILBSpec})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeInternalAPILBSpec, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.LoadBalancersReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create node outbound LB",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{&fakeNodeOutboundLBSpec})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNodeOutboundLBSpec, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.LoadBalancersReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create multiple LBs",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{&fakePublicAPILBSpec, &fakeInternalAPILBSpec, &fakeNodeOutboundLBSpec})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePublicAPILBSpec, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeInternalAPILBSpec, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeNodeOutboundLBSpec, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.LoadBalancersReadyCondition, serviceName, nil)
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

			scopeMock := mock_loadbalancers.NewMockLBScope(mockCtrl)
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

func TestDeleteLoadBalancer(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no LBSpecs are found",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "delete a load balancer",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{&fakePublicAPILBSpec})
				r.DeleteResource(gomockinternal.AContext(), &fakePublicAPILBSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.LoadBalancersReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete multiple load balancers",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{&fakePublicAPILBSpec, &fakeInternalAPILBSpec, &fakeNodeOutboundLBSpec})
				r.DeleteResource(gomockinternal.AContext(), &fakePublicAPILBSpec, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeInternalAPILBSpec, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeNodeOutboundLBSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.LoadBalancersReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "load balancer deletion fails",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.LBSpecs().Return([]azure.ResourceSpecGetter{&fakePublicAPILBSpec})
				r.DeleteResource(gomockinternal.AContext(), &fakePublicAPILBSpec, serviceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.LoadBalancersReadyCondition, serviceName, internalError)
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

			scopeMock := mock_loadbalancers.NewMockLBScope(mockCtrl)
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
