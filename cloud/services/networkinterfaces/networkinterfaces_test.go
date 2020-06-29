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

package networkinterfaces

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/inboundnatrules/mock_inboundnatrules"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/internalloadbalancers/mock_internalloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces/mock_networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicloadbalancers/mock_publicloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus/mock_resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets/mock_subnets"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	expectedInvalidSpec = "invalid network interface specification"
	subscriptionID      = "123"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestInvalidNetworkInterface(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := gomock.NewController(t)
	netInterfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}

	client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Client:  client,
		Cluster: cluster,
		AzureCluster: &infrav1.AzureCluster{
			Spec: infrav1.AzureClusterSpec{
				Location: "test-location",
				ResourceGroup:  "my-rg",
				SubscriptionID: subscriptionID,
				NetworkSpec: infrav1.NetworkSpec{
					Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
				},
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	s := &Service{
		Scope:  clusterScope,
		Client: netInterfaceMock,
	}

	// Wrong Spec
	wrongSpec := &network.PublicIPAddress{}

	err = s.Reconcile(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Delete(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))
}

func TestReconcileNetworkInterface(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name             string
		netInterfaceSpec Spec
		expectedError    string
		expect           func(m *mock_networkinterfaces.MockClientMockRecorder,
			mSubnet *mock_subnets.MockClientMockRecorder,
			mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
			mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
			mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
			mPublicIP *mock_publicips.MockClientMockRecorder,
			mResourceSku *mock_resourceskus.MockClient)
	}{
		{
			name: "get subnets fails",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicLoadBalancerName: "my-cluster",
				MachineRole:            infrav1.Node,
			},
			expectedError: "failed to get subnets: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "node network interface create fails",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicLoadBalancerName: "my-cluster",
				MachineRole:            infrav1.Node,
			},
			expectedError: "failed to create network interface my-net-interface in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				mResourceSku.EXPECT().HasAcceleratedNetworking(gomock.Any(), gomock.Any())
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").
						Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-cluster").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{})).
						Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name: "node network interface with Static private IP successfully created",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				StaticIPAddress:        "1.2.3.4",
				PublicLoadBalancerName: "my-cluster",
				MachineRole:            infrav1.Node,
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				mResourceSku.EXPECT().HasAcceleratedNetworking(gomock.Any(), gomock.Any())
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-cluster").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{})))
			},
		},
		{
			name: "node network interface with Dynamic private IP successfully created",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicLoadBalancerName: "my-cluster",
				MachineRole:            infrav1.Node,
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				mResourceSku.EXPECT().HasAcceleratedNetworking(gomock.Any(), gomock.Any())
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-cluster").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{})))
			},
		},
		{
			name: "control plane network interface successfully created",
			netInterfaceSpec: Spec{
				Name:                     "my-net-interface",
				VnetName:                 "my-vnet",
				SubnetName:               "my-subnet",
				PublicLoadBalancerName:   "my-publiclb",
				InternalLoadBalancerName: "my-internal-lb",
				MachineRole:              infrav1.ControlPlane,
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				mResourceSku.EXPECT().HasAcceleratedNetworking(gomock.Any(), gomock.Any())
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").
						Return(network.Subnet{ID: to.StringPtr("my-subnet-id")}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{
						Name: to.StringPtr("my-publiclb"),
						ID:   pointer.StringPtr("my-publiclb-id"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									ID: to.StringPtr("frontend-ip-config-id"),
								},
							},
							BackendAddressPools: &[]network.BackendAddressPool{
								{
									ID: pointer.StringPtr("my-backend-pool-id"),
								},
							},
							InboundNatRules: &[]network.InboundNatRule{},
						}}, nil),
					mInboundNATRules.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", "azure-test1", network.InboundNatRule{
						Name: pointer.StringPtr("azure-test1"),
						InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
							FrontendPort:         to.Int32Ptr(22),
							BackendPort:          to.Int32Ptr(22),
							EnableFloatingIP:     to.BoolPtr(false),
							IdleTimeoutInMinutes: to.Int32Ptr(4),
							FrontendIPConfiguration: &network.SubResource{
								ID: to.StringPtr("frontend-ip-config-id"),
							},
							Protocol: network.TransportProtocolTCP,
						},
					}),
					mInternalLoadBalancer.Get(context.TODO(), "my-rg", "my-internal-lb").
						Return(network.LoadBalancer{
							ID: pointer.StringPtr("my-internal-lb-id"),
							LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
								BackendAddressPools: &[]network.BackendAddressPool{
									{
										ID: pointer.StringPtr("my-internal-backend-pool-id"),
									},
								},
							}}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", matchers.DiffEq(network.Interface{
						Location: to.StringPtr("test-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(false),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										Subnet:                          &network.Subnet{ID: to.StringPtr("my-subnet-id")},
										PrivateIPAllocationMethod:       network.Dynamic,
										LoadBalancerInboundNatRules:     &[]network.InboundNatRule{{ID: to.StringPtr("my-publiclb-id/inboundNatRules/azure-test1")}},
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("my-backend-pool-id")}, {ID: to.StringPtr("my-internal-backend-pool-id")}},
									},
								},
							},
						},
					})))
			},
		},
		{
			name: "control plane network interface fail to get public LB",
			netInterfaceSpec: Spec{
				Name:                     "my-net-interface",
				VnetName:                 "my-vnet",
				SubnetName:               "my-subnet",
				PublicLoadBalancerName:   "my-publiclb",
				InternalLoadBalancerName: "my-internal-lb",
				MachineRole:              infrav1.ControlPlane,
			},
			expectedError: "failed to get public LB: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-publiclb").
						Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name: "control plane network interface fail to create NAT rule",
			netInterfaceSpec: Spec{
				Name:                     "my-net-interface",
				VnetName:                 "my-vnet",
				SubnetName:               "my-subnet",
				PublicLoadBalancerName:   "my-publiclb",
				InternalLoadBalancerName: "my-internal-lb",
				MachineRole:              infrav1.ControlPlane,
			},
			expectedError: "failed to create NAT rule: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{
						Name: to.StringPtr("my-publiclb"),
						ID:   pointer.StringPtr("my-publiclb-id"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									ID: to.StringPtr("frontend-ip-config-id"),
								},
							},
							BackendAddressPools: &[]network.BackendAddressPool{
								{
									ID: pointer.StringPtr("my-backend-pool-id"),
								},
							},
							InboundNatRules: &[]network.InboundNatRule{
								{
									Name: pointer.StringPtr("other-machine-nat-rule"),
									ID:   pointer.StringPtr("some-natrules-id"),
									InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
										FrontendPort: to.Int32Ptr(22),
									},
								},
								{
									Name: pointer.StringPtr("other-machine-nat-rule-2"),
									ID:   pointer.StringPtr("some-natrules-id-2"),
									InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
										FrontendPort: to.Int32Ptr(2201),
									},
								},
							},
						}}, nil),
					mInboundNATRules.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", "azure-test1", network.InboundNatRule{
						Name: pointer.StringPtr("azure-test1"),
						InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
							FrontendPort:         to.Int32Ptr(2202),
							BackendPort:          to.Int32Ptr(22),
							EnableFloatingIP:     to.BoolPtr(false),
							IdleTimeoutInMinutes: to.Int32Ptr(4),
							FrontendIPConfiguration: &network.SubResource{
								ID: to.StringPtr("frontend-ip-config-id"),
							},
							Protocol: network.TransportProtocolTCP,
						},
					}).
						Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name: "control plane network interface fail to get internal LB",
			netInterfaceSpec: Spec{
				Name:                     "my-net-interface",
				VnetName:                 "my-vnet",
				SubnetName:               "my-subnet",
				InternalLoadBalancerName: "my-internal-lb",
				MachineRole:              infrav1.ControlPlane,
			},
			expectedError: "failed to get internalLB: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mInternalLoadBalancer.Get(context.TODO(), "my-rg", "my-internal-lb").
						Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name: "network interface with Public IP successfully created",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicIPName:           "my-public-ip",
				PublicLoadBalancerName: "my-cluster",
				MachineRole:            infrav1.Node,
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				mResourceSku.EXPECT().HasAcceleratedNetworking(gomock.Any(), gomock.Any())
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-cluster").Return(getFakeNodeOutboundLoadBalancer(), nil),
					mPublicIP.Get(context.TODO(), "my-rg", "my-public-ip").Return(network.PublicIPAddress{}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{})))
			},
		},
		{
			name: "network interface with Public IP fail to get Public IP",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicIPName:           "my-public-ip",
				PublicLoadBalancerName: "my-cluster",
				MachineRole:            infrav1.Node,
			},
			expectedError: "failed to get publicIP: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-cluster").Return(getFakeNodeOutboundLoadBalancer(), nil),
					mPublicIP.Get(context.TODO(), "my-rg", "my-public-ip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name: "network interface with accelerated networking successfully created",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicLoadBalancerName: "my-cluster",
				MachineRole:            infrav1.Node,
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				mResourceSku.EXPECT().HasAcceleratedNetworking(context.TODO(), gomock.Any()).Return(true, nil)
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-cluster").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", matchers.DiffEq(network.Interface{
						Location: to.StringPtr("test-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(true),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										Subnet:                          &network.Subnet{},
										PrivateIPAllocationMethod:       network.Dynamic,
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("cluster-name-outboundBackendPool")}},
									},
								},
							},
						},
					})),
				)
			},
		},
		{
			name: "network interface without accelerated networking successfully created",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicLoadBalancerName: "my-cluster",
				MachineRole:            infrav1.Node,
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				mResourceSku.EXPECT().HasAcceleratedNetworking(context.TODO(), gomock.Any()).Return(false, nil)
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-cluster").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", matchers.DiffEq(network.Interface{
						Location: to.StringPtr("test-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(false),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										Subnet:                          &network.Subnet{},
										PrivateIPAllocationMethod:       network.Dynamic,
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("cluster-name-outboundBackendPool")}},
									},
								},
							},
						},
					})),
				)
			},
		},
		{
			name: "network interface fails to get accelerated networking capability",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicLoadBalancerName: "my-cluster",
				MachineRole:            infrav1.Node,
			},
			expectedError: "failed to get accelerated networking capability: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder,
				mInternalLoadBalancer *mock_internalloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
				mResourceSku *mock_resourceskus.MockClient) {
				mResourceSku.EXPECT().HasAcceleratedNetworking(context.TODO(), gomock.Any()).Return(
					false, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicLoadBalancer.Get(context.TODO(), "my-rg", "my-cluster").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{})),
				)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			netInterfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			subnetMock := mock_subnets.NewMockClient(mockCtrl)
			publicLoadBalancerMock := mock_publicloadbalancers.NewMockClient(mockCtrl)
			inboundNatRulesMock := mock_inboundnatrules.NewMockClient(mockCtrl)
			internalLoadBalancerMock := mock_internalloadbalancers.NewMockClient(mockCtrl)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)
			resourceSkusMock := mock_resourceskus.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}
			client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								Name:          "my-vnet",
								ResourceGroup: "my-rg",
							},
							Subnets: []*infrav1.SubnetSpec{{
								Name: "my-subnet",
								Role: infrav1.SubnetNode,
							}},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			azureMachine := &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "azure-test1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "test1",
						},
					},
				},
			}
			machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
				Client:  client,
				Machine: &clusterv1.Machine{},
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureMachine: azureMachine,
				ClusterScope: clusterScope,
			})
			g.Expect(err).NotTo(HaveOccurred())

			tc.expect(netInterfaceMock.EXPECT(), subnetMock.EXPECT(),
				publicLoadBalancerMock.EXPECT(), inboundNatRulesMock.EXPECT(),
				internalLoadBalancerMock.EXPECT(), publicIPsMock.EXPECT(),
				resourceSkusMock)

			s := &Service{
				Scope:                       clusterScope,
				MachineScope:                machineScope,
				Client:                      netInterfaceMock,
				SubnetsClient:               subnetMock,
				PublicLoadBalancersClient:   publicLoadBalancerMock,
				InboundNATRulesClient:       inboundNatRulesMock,
				InternalLoadBalancersClient: internalLoadBalancerMock,
				PublicIPsClient:             publicIPsMock,
				ResourceSkusClient:          resourceSkusMock,
			}

			err = s.Reconcile(context.TODO(), &tc.netInterfaceSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteNetworkInterface(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name             string
		netInterfaceSpec Spec
		expectedError    string
		expect           func(m *mock_networkinterfaces.MockClientMockRecorder, mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder)
	}{
		{
			name: "successfully delete an existing network interface",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				PublicLoadBalancerName: "my-public-lb",
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder, mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-net-interface")
				mInboundNATRules.Delete(context.TODO(), "my-rg", "my-public-lb", "azure-test1")
			},
		},
		{
			name: "network interface already deleted",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				PublicLoadBalancerName: "my-public-lb",
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder, mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-net-interface").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mInboundNATRules.Delete(context.TODO(), "my-rg", "my-public-lb", "azure-test1")
			},
		},
		{
			name: "network interface deletion fails",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				PublicLoadBalancerName: "my-public-lb",
			},
			expectedError: "failed to delete network interface my-net-interface in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder, mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-net-interface").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "NAT rule already deleted",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				PublicLoadBalancerName: "my-public-lb",
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder, mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-net-interface").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mInboundNATRules.Delete(context.TODO(), "my-rg", "my-public-lb", "azure-test1")
			},
		},
		{
			name: "NAT rule deletion fails",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				PublicLoadBalancerName: "my-public-lb",
			},
			expectedError: "failed to delete inbound NAT rule azure-test1 in load balancer my-public-lb: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder, mInboundNATRules *mock_inboundnatrules.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-net-interface")
				mInboundNATRules.Delete(context.TODO(), "my-rg", "my-public-lb", "azure-test1").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			netInterfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			inboundNatRulesMock := mock_inboundnatrules.NewMockClient(mockCtrl)
			publicIPMock := mock_publicips.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}
			client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
							Subnets: []*infrav1.SubnetSpec{{
								Name: "my-subnet",
								Role: infrav1.SubnetNode,
							}},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			azureMachine := &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "azure-test1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "test1",
						},
					},
				},
			}
			machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
				Client:  client,
				Machine: &clusterv1.Machine{},
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureMachine: azureMachine,
				ClusterScope: clusterScope,
			})
			g.Expect(err).NotTo(HaveOccurred())

			tc.expect(netInterfaceMock.EXPECT(), inboundNatRulesMock.EXPECT(), publicIPMock.EXPECT())

			s := &Service{
				Scope:                 clusterScope,
				MachineScope:          machineScope,
				Client:                netInterfaceMock,
				InboundNATRulesClient: inboundNatRulesMock,
				PublicIPsClient:       publicIPMock,
			}

			err = s.Delete(context.TODO(), &tc.netInterfaceSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getFakeNodeOutboundLoadBalancer() network.LoadBalancer {
	return network.LoadBalancer{
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					ID: to.StringPtr("frontend-ip-config-id"),
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					ID: pointer.StringPtr("cluster-name-outboundBackendPool"),
				},
			},
		}}
}
