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

package natgateways

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/natgateways/mock_natgateways"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

var (
	natGatewaySpec1 = NatGatewaySpec{
		Name:           "my-node-natgateway-1",
		ResourceGroup:  "my-rg",
		SubscriptionID: "my-sub",
		Location:       "westus",
		ClusterName:    "my-cluster",
		NatGatewayIP:   infrav1.PublicIPSpec{Name: "pip-node-subnet"},
	}
	natGateway1 = network.NatGateway{
		ID: ptr.To("/subscriptions/my-sub/resourceGroups/my-rg/providers/Microsoft.Network/natGateways/my-node-natgateway-1"),
	}
	customVNetTags = infrav1.Tags{
		"Name": "my-vnet",
		"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "shared",
		"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
	}
	ownedVNetTags = infrav1.Tags{
		"Name": "my-vnet",
		"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
		"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
	}
	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
)

func TestReconcileNatGateways(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no NAT gateways specs are found",
			tags:          customVNetTags,
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().Return(true)
				s.NatGatewaySpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "NAT gateways in custom vnet mode",
			tags:          customVNetTags,
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().Return(false)
			},
		},
		{
			name:          "NAT gateway create successfully",
			tags:          ownedVNetTags,
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().Return(true)
				s.NatGatewaySpecs().Return([]azure.ResourceSpecGetter{&natGatewaySpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &natGatewaySpec1, serviceName).Return(natGateway1, nil)
				s.SetNatGatewayIDInSubnets(natGatewaySpec1.Name, *natGateway1.ID)
				s.UpdatePutStatus(infrav1.NATGatewaysReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "fail to create a NAT gateway",
			tags:          ownedVNetTags,
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().Return(true)
				s.NatGatewaySpecs().Return([]azure.ResourceSpecGetter{&natGatewaySpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &natGatewaySpec1, serviceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.NATGatewaysReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "result is not a NAT gateway",
			tags:          ownedVNetTags,
			expectedError: "created resource string is not a network.NatGateway",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().Return(true)
				s.NatGatewaySpecs().Return([]azure.ResourceSpecGetter{&natGatewaySpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &natGatewaySpec1, serviceName).Return("not a nat gateway", nil)
				s.UpdatePutStatus(infrav1.NATGatewaysReadyCondition, serviceName, gomockinternal.ErrStrEq("created resource string is not a network.NatGateway"))
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
			scopeMock := mock_natgateways.NewMockNatGatewayScope(mockCtrl)
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

func TestDeleteNatGateway(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no NAT gateways specs are found",
			tags:          ownedVNetTags,
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().Return(true)
				s.NatGatewaySpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "NAT gateways in custom vnet mode",
			tags:          customVNetTags,
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().Return(false)
			},
		},
		{
			name:          "NAT gateway deleted successfully",
			tags:          ownedVNetTags,
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().Return(true)
				s.NatGatewaySpecs().Return([]azure.ResourceSpecGetter{&natGatewaySpec1})
				r.DeleteResource(gomockinternal.AContext(), &natGatewaySpec1, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.NATGatewaysReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "NAT gateway deletion fails",
			tags:          ownedVNetTags,
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().Return(true)
				s.NatGatewaySpecs().Return([]azure.ResourceSpecGetter{&natGatewaySpec1})
				r.DeleteResource(gomockinternal.AContext(), &natGatewaySpec1, serviceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.NATGatewaysReadyCondition, serviceName, internalError)
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
			scopeMock := mock_natgateways.NewMockNatGatewayScope(mockCtrl)
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
