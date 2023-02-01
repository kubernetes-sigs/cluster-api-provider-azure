/*
Copyright 2019 The Kubernetes Authors.

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

package subnets

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets/mock_subnets"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeSubnetSpec1 = SubnetSpec{
		Name:              "my-subnet-1",
		ResourceGroup:     "my-rg",
		SubscriptionID:    "123",
		CIDRs:             []string{"10.0.0.0/16"},
		IsVNetManaged:     true,
		VNetName:          "my-vnet",
		VNetResourceGroup: "my-rg",
		RouteTableName:    "my-subnet_route_table",
		SecurityGroupName: "my-sg-1",
		Role:              infrav1.SubnetNode,
	}

	fakeSubnet1 = network.Subnet{
		ID:   pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet-1"),
		Name: pointer.String("my-subnet-1"),
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefix: pointer.String("10.0.0.0/16"),
			RouteTable: &network.RouteTable{
				ID:   pointer.String("rt-id"),
				Name: pointer.String("my-subnet_route_table"),
			},
			NetworkSecurityGroup: &network.SecurityGroup{
				ID:   pointer.String("sg-id-1"),
				Name: pointer.String("my-sg-1"),
			},
		},
	}

	fakeSubnetSpec2 = SubnetSpec{
		Name:              "my-subnet-2",
		ResourceGroup:     "my-rg",
		SubscriptionID:    "123",
		CIDRs:             []string{"10.2.0.0/16"},
		IsVNetManaged:     true,
		VNetName:          "my-vnet",
		VNetResourceGroup: "my-rg",
		RouteTableName:    "my-subnet_route_table",
		SecurityGroupName: "my-sg-2",
		Role:              infrav1.SubnetNode,
	}

	fakeSubnet2 = network.Subnet{
		ID:   pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet-2"),
		Name: pointer.String("my-subnet-2"),
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefix: pointer.String("10.2.0.0/16"),
			RouteTable: &network.RouteTable{
				ID:   pointer.String("rt-id"),
				Name: pointer.String("my-subnet_route_table"),
			},
			NetworkSecurityGroup: &network.SecurityGroup{
				ID:   pointer.String("sg-id-2"),
				Name: pointer.String("my-sg-2"),
			},
		},
	}

	fakeSubnetSpecNotManaged = SubnetSpec{
		Name:              "my-subnet-1",
		ResourceGroup:     "my-rg",
		SubscriptionID:    "123",
		CIDRs:             []string{"10.0.0.0/16"},
		IsVNetManaged:     false,
		VNetName:          "my-vnet",
		VNetResourceGroup: "my-vnet-rg",
		RouteTableName:    "my-subnet_route_table",
		SecurityGroupName: "my-sg-1",
		Role:              infrav1.SubnetNode,
	}
	fakeSubnetNotManaged = network.Subnet{
		ID:   pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet-1"),
		Name: pointer.String("my-subnet-1"),
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefix: pointer.String("10.0.0.0/16"),
			RouteTable: &network.RouteTable{
				ID:   pointer.String("rt-id"),
				Name: pointer.String("my-subnet_route_table"),
			},
			NetworkSecurityGroup: &network.SecurityGroup{
				ID:   pointer.String("sg-id-1"),
				Name: pointer.String("my-sg-1"),
			},
		},
	}

	fakeCtrlPlaneSubnetSpec = SubnetSpec{
		Name:              "my-subnet-ctrl-plane",
		ResourceGroup:     "my-rg",
		SubscriptionID:    "123",
		CIDRs:             []string{"10.1.0.0/16"},
		IsVNetManaged:     true,
		VNetName:          "my-vnet",
		VNetResourceGroup: "my-rg",
		RouteTableName:    "my-subnet_route_table",
		SecurityGroupName: "my-sg",
		Role:              infrav1.SubnetControlPlane,
	}

	fakeIpv6SubnetSpec = SubnetSpec{
		Name:              "my-ipv6-subnet",
		ResourceGroup:     "my-rg",
		SubscriptionID:    "123",
		CIDRs:             []string{"10.0.0.0/16", "2001:1234:5678:9abd::/64"},
		IsVNetManaged:     true,
		VNetName:          "my-vnet",
		VNetResourceGroup: "my-rg",
		RouteTableName:    "my-subnet_route_table",
		SecurityGroupName: "my-sg",
		Role:              infrav1.SubnetNode,
	}

	fakeIpv6Subnet = network.Subnet{
		ID:   pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-ipv6-subnet"),
		Name: pointer.String("my-ipv6-subnet"),
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefixes: &[]string{
				"10.0.0.0/16",
				"2001:1234:5678:9abd::/64",
			},
			RouteTable:           &network.RouteTable{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
			NetworkSecurityGroup: &network.SecurityGroup{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
		},
	}

	fakeIpv6SubnetSpecCP = SubnetSpec{
		Name:              "my-ipv6-subnet-cp",
		ResourceGroup:     "my-rg",
		SubscriptionID:    "123",
		CIDRs:             []string{"10.2.0.0/16", "2001:1234:5678:9abc::/64"},
		IsVNetManaged:     true,
		VNetName:          "my-vnet",
		VNetResourceGroup: "my-rg",
		RouteTableName:    "my-subnet_route_table",
		SecurityGroupName: "my-sg",
		Role:              infrav1.SubnetNode,
	}

	fakeIpv6SubnetCP = network.Subnet{
		ID:   pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-ipv6-subnet-cp"),
		Name: pointer.String("my-ipv6-subnet-cp"),
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefixes: &[]string{
				"10.2.0.0/16",
				"2001:1234:5678:9abc::/64",
			},
			RouteTable:           &network.RouteTable{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
			NetworkSecurityGroup: &network.SecurityGroup{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
		},
	}

	notASubnet    = "not a subnet"
	notASubnetErr = errors.Errorf("%T is not a network.Subnet", notASubnet)
	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
)

func TestReconcileSubnets(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no subnet specs are found",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "create subnet",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeSubnetSpec1})

				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSubnetSpec1, serviceName).Return(fakeSubnet1, nil)
				s.UpdateSubnetID(fakeSubnetSpec1.Name, pointer.StringDeref(fakeSubnet1.ID, ""))
				s.UpdateSubnetCIDRs(fakeSubnetSpec1.Name, []string{pointer.StringDeref(fakeSubnet1.AddressPrefix, "")})

				s.IsVnetManaged().AnyTimes().Return(true)
				s.UpdatePutStatus(infrav1.SubnetsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create multiple subnets",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeSubnetSpec1, &fakeSubnetSpec2})

				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSubnetSpec1, serviceName).Return(fakeSubnet1, nil)
				s.UpdateSubnetID(fakeSubnetSpec1.Name, pointer.StringDeref(fakeSubnet1.ID, ""))
				s.UpdateSubnetCIDRs(fakeSubnetSpec1.Name, []string{pointer.StringDeref(fakeSubnet1.AddressPrefix, "")})

				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSubnetSpec2, serviceName).Return(fakeSubnet2, nil)
				s.UpdateSubnetID(fakeSubnetSpec2.Name, pointer.StringDeref(fakeSubnet2.ID, ""))
				s.UpdateSubnetCIDRs(fakeSubnetSpec2.Name, []string{pointer.StringDeref(fakeSubnet2.AddressPrefix, "")})

				s.IsVnetManaged().AnyTimes().Return(true)
				s.UpdatePutStatus(infrav1.SubnetsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "don't update ready condition when subnet not managed",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeSubnetSpecNotManaged})

				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSubnetSpecNotManaged, serviceName).Return(fakeSubnetNotManaged, nil)
				s.UpdateSubnetID(fakeSubnetSpecNotManaged.Name, pointer.StringDeref(fakeSubnetNotManaged.ID, ""))
				s.UpdateSubnetCIDRs(fakeSubnetSpecNotManaged.Name, []string{pointer.StringDeref(fakeSubnetNotManaged.AddressPrefix, "")})

				s.IsVnetManaged().AnyTimes().Return(false)
			},
		},
		{
			name:          "create ipv6 subnet",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeIpv6SubnetSpec})

				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeIpv6SubnetSpec, serviceName).Return(fakeIpv6Subnet, nil)
				s.UpdateSubnetID(fakeIpv6SubnetSpec.Name, pointer.StringDeref(fakeIpv6Subnet.ID, ""))
				s.UpdateSubnetCIDRs(fakeIpv6SubnetSpec.Name, azure.StringSlice(fakeIpv6Subnet.AddressPrefixes))

				s.IsVnetManaged().AnyTimes().Return(true)
				s.UpdatePutStatus(infrav1.SubnetsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create multiple ipv6 subnets",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeIpv6SubnetSpec, &fakeIpv6SubnetSpecCP})

				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeIpv6SubnetSpec, serviceName).Return(fakeIpv6Subnet, nil)
				s.UpdateSubnetID(fakeIpv6SubnetSpec.Name, pointer.StringDeref(fakeIpv6Subnet.ID, ""))
				s.UpdateSubnetCIDRs(fakeIpv6SubnetSpec.Name, azure.StringSlice(fakeIpv6Subnet.AddressPrefixes))

				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeIpv6SubnetSpecCP, serviceName).Return(fakeIpv6SubnetCP, nil)
				s.UpdateSubnetID(fakeIpv6SubnetSpecCP.Name, pointer.StringDeref(fakeIpv6SubnetCP.ID, ""))
				s.UpdateSubnetCIDRs(fakeIpv6SubnetSpecCP.Name, azure.StringSlice(fakeIpv6SubnetCP.AddressPrefixes))

				s.IsVnetManaged().AnyTimes().Return(true)
				s.UpdatePutStatus(infrav1.SubnetsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "fail to create subnet",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeSubnetSpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSubnetSpec1, serviceName).Return(nil, internalError)

				s.IsVnetManaged().AnyTimes().Return(true)
				s.UpdatePutStatus(infrav1.SubnetsReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "create returns a non subnet",
			expectedError: notASubnetErr.Error(),
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeSubnetSpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSubnetSpec1, serviceName).Return(notASubnet, nil)
			},
		},
		{
			name:          "fail to create subnets",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeSubnetSpec1, &fakeSubnetSpec2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSubnetSpec1, serviceName).Return(nil, internalError)

				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeSubnetSpec2, serviceName).Return(fakeSubnet2, nil)
				s.UpdateSubnetID(fakeSubnetSpec2.Name, pointer.StringDeref(fakeSubnet2.ID, ""))
				s.UpdateSubnetCIDRs(fakeSubnetSpec2.Name, []string{pointer.StringDeref(fakeSubnet2.AddressPrefix, "")})

				s.IsVnetManaged().AnyTimes().Return(true)
				s.UpdatePutStatus(infrav1.SubnetsReadyCondition, serviceName, internalError)
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
			scopeMock := mock_subnets.NewMockSubnetScope(mockCtrl)
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

func TestDeleteSubnets(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no subnet specs are found",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().AnyTimes().Return(true)
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "subnets deleted successfully",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().AnyTimes().Return(true)
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeSubnetSpec1, &fakeSubnetSpec2})
				r.DeleteResource(gomockinternal.AContext(), &fakeSubnetSpec1, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeSubnetSpec2, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.SubnetsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "node subnet and controlplane subnet deleted successfully",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().AnyTimes().Return(true)
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeSubnetSpec1, &fakeCtrlPlaneSubnetSpec})
				r.DeleteResource(gomockinternal.AContext(), &fakeSubnetSpec1, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeCtrlPlaneSubnetSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.SubnetsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "skip delete if vnet is not managed",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().AnyTimes().Return(false)
			},
		},
		{
			name:          "fail delete subnet",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.IsVnetManaged().AnyTimes().Return(true)
				s.SubnetSpecs().Return([]azure.ResourceSpecGetter{&fakeSubnetSpec1})
				r.DeleteResource(gomockinternal.AContext(), &fakeSubnetSpec1, serviceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.SubnetsReadyCondition, serviceName, internalError)
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
			scopeMock := mock_subnets.NewMockSubnetScope(mockCtrl)
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
