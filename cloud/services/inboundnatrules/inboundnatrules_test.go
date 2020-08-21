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

package inboundnatrules

import (
	"context"
	"k8s.io/klog/klogr"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/inboundnatrules/mock_inboundnatrules"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/loadbalancers/mock_loadbalancers"
)

func TestReconcileInboundNATRule(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
			m *mock_inboundnatrules.MockClientMockRecorder,
			mLoadBalancer *mock_loadbalancers.MockClientMockRecorder)
	}{
		{
			name:          "NAT rule successfully created",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.InboundNatSpecs().Return([]azure.InboundNatSpec{
					{
						Name:             "my-machine",
						LoadBalancerName: "my-lb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{
						Name: to.StringPtr("my-lb"),
						ID:   pointer.StringPtr("my-lb-id"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									ID: to.StringPtr("frontend-ip-config-id"),
								},
							},
							InboundNatRules: &[]network.InboundNatRule{},
						}}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-lb", "my-machine", network.InboundNatRule{
						Name: pointer.StringPtr("my-machine"),
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
					}))
			},
		},
		{
			name:          "fail to get LB",
			expectedError: "failed to get Load Balancer my-public-lb: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.InboundNatSpecs().Return([]azure.InboundNatSpec{
					{
						Name:             "my-machine",
						LoadBalancerName: "my-public-lb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").
						Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name:          "fail to create NAT rule",
			expectedError: "failed to create inbound NAT rule my-machine: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.InboundNatSpecs().Return([]azure.InboundNatSpec{
					{
						Name:             "my-machine",
						LoadBalancerName: "my-public-lb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").Return(network.LoadBalancer{
						Name: to.StringPtr("my-public-lb"),
						ID:   pointer.StringPtr("my-public-lb-id"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									ID: to.StringPtr("frontend-ip-config-id"),
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
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-public-lb", "my-machine", network.InboundNatRule{
						Name: pointer.StringPtr("my-machine"),
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
			name:          "NAT rule already exists",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.InboundNatSpecs().Return([]azure.InboundNatSpec{
					{
						Name:             "my-machine-nat-rule",
						LoadBalancerName: "my-public-lb",
					},
					{
						Name:             "my-other-nat-rule",
						LoadBalancerName: "my-other-public-lb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").Return(network.LoadBalancer{
						Name: to.StringPtr("my-public-lb"),
						ID:   pointer.StringPtr("my-public-lb-id"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									ID: to.StringPtr("frontend-ip-config-id"),
								},
							},
							InboundNatRules: &[]network.InboundNatRule{
								{
									Name: pointer.StringPtr("my-machine-nat-rule"),
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
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-other-public-lb").Return(network.LoadBalancer{
						Name: to.StringPtr("my-other-public-lb"),
						ID:   pointer.StringPtr("my-public-lb-id"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									ID: to.StringPtr("frontend-ip-config-id"),
								},
							},
							InboundNatRules: &[]network.InboundNatRule{},
						}}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-other-public-lb", "my-other-nat-rule", gomock.AssignableToTypeOf(network.InboundNatRule{})))
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
			scopeMock := mock_inboundnatrules.NewMockInboundNatScope(mockCtrl)
			clientMock := mock_inboundnatrules.NewMockClient(mockCtrl)
			loadBalancerMock := mock_loadbalancers.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), loadBalancerMock.EXPECT())

			s := &Service{
				Scope:               scopeMock,
				Client:              clientMock,
				LoadBalancersClient: loadBalancerMock,
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

func TestDeleteNetworkInterface(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
			m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete an existing NAT rule",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.InboundNatSpecs().Return([]azure.InboundNatSpec{
					{
						Name:             "azure-md-0",
						LoadBalancerName: "my-public-lb",
					},
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-public-lb", "azure-md-0")
			},
		},
		{
			name:          "NAT rule already deleted",
			expectedError: "",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.InboundNatSpecs().Return([]azure.InboundNatSpec{
					{
						Name:             "azure-md-1",
						LoadBalancerName: "my-public-lb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(context.TODO(), "my-rg", "my-public-lb", "azure-md-1").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "NAT rule deletion fails",
			expectedError: "failed to delete inbound NAT rule azure-md-2: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.InboundNatSpecs().Return([]azure.InboundNatSpec{
					{
						Name:             "azure-md-2",
						LoadBalancerName: "my-public-lb",
					},
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-public-lb", "azure-md-2").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_inboundnatrules.NewMockInboundNatScope(mockCtrl)
			clientMock := mock_inboundnatrules.NewMockClient(mockCtrl)
			loadBalancerMock := mock_loadbalancers.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), loadBalancerMock.EXPECT())

			s := &Service{
				Scope:               scopeMock,
				Client:              clientMock,
				LoadBalancersClient: loadBalancerMock,
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

func TestNatRuleExists(t *testing.T) {
	testcases := []struct {
		name           string
		ruleName       string
		existingRules  []network.InboundNatRule
		expectedResult bool
		expectedPorts  map[int32]struct{}
		expect         func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
			m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder)
	}{
		{
			name:     "Rule exists",
			ruleName: "my-rule",
			existingRules: []network.InboundNatRule{
				{
					InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
						FrontendPort: to.Int32Ptr(2201),
					},
					Name: to.StringPtr("some-rule"),
				},
				{
					InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
						FrontendPort: to.Int32Ptr(22),
					},
					Name: to.StringPtr("my-rule"),
				},
			},
			expectedResult: true,
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).Return(klogr.New())
			},
		},
		{
			name:     "Rule doesn't exist",
			ruleName: "my-rule",
			existingRules: []network.InboundNatRule{
				{
					InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
						FrontendPort: to.Int32Ptr(22),
					},
					Name: to.StringPtr("other-rule"),
				},
				{
					InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
						FrontendPort: to.Int32Ptr(2205),
					},
					Name: to.StringPtr("other-rule-2"),
				},
			},
			expectedResult: false,
			expectedPorts: map[int32]struct{}{
				22:   {},
				2205: {},
			},
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
			},
		},
		{
			name:           "No rules exist",
			ruleName:       "my-rule",
			existingRules:  []network.InboundNatRule{},
			expectedResult: false,
			expectedPorts:  map[int32]struct{}{},
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
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
			scopeMock := mock_inboundnatrules.NewMockInboundNatScope(mockCtrl)
			clientMock := mock_inboundnatrules.NewMockClient(mockCtrl)
			loadBalancerMock := mock_loadbalancers.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), loadBalancerMock.EXPECT())

			s := &Service{
				Scope:               scopeMock,
				Client:              clientMock,
				LoadBalancersClient: loadBalancerMock,
			}

			ports := make(map[int32]struct{})
			exists := s.natRuleExists(ports)(tc.existingRules, tc.ruleName)
			g.Expect(exists).To(Equal(tc.expectedResult))
			if !exists {
				g.Expect(ports).To(Equal(tc.expectedPorts))
			}
		})
	}
}

func TestGetAvailablePort(t *testing.T) {
	testcases := []struct {
		name               string
		portsInput         map[int32]struct{}
		expectedError      string
		expectedPortResult int32
		expect             func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
			m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder)
	}{
		{
			name:               "Empty ports",
			portsInput:         map[int32]struct{}{},
			expectedError:      "",
			expectedPortResult: 22,
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).Return(klogr.New())
			},
		},
		{
			name: "22 taken",
			portsInput: map[int32]struct{}{
				22: {},
			},
			expectedError:      "",
			expectedPortResult: 2201,
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).Return(klogr.New())
			},
		},
		{
			name: "Existing ports",
			portsInput: map[int32]struct{}{
				22:   {},
				2201: {},
				2202: {},
				2204: {},
			},
			expectedError:      "",
			expectedPortResult: 2203,
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).Return(klogr.New())
			},
		},
		{
			name:               "No ports available",
			portsInput:         getFullPortsMap(),
			expectedError:      "No available SSH Frontend ports",
			expectedPortResult: 0,
			expect: func(s *mock_inboundnatrules.MockInboundNatScopeMockRecorder,
				m *mock_inboundnatrules.MockClientMockRecorder, mLoadBalancer *mock_loadbalancers.MockClientMockRecorder) {
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
			scopeMock := mock_inboundnatrules.NewMockInboundNatScope(mockCtrl)
			clientMock := mock_inboundnatrules.NewMockClient(mockCtrl)
			loadBalancerMock := mock_loadbalancers.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), loadBalancerMock.EXPECT())

			s := &Service{
				Scope:               scopeMock,
				Client:              clientMock,
				LoadBalancersClient: loadBalancerMock,
			}

			res, err := s.getAvailablePort(tc.portsInput)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).To(Equal(tc.expectedPortResult))
			}
		})
	}
}

func getFullPortsMap() map[int32]struct{} {
	res := map[int32]struct{}{
		22: {},
	}
	for i := 2201; i < 2220; i++ {
		res[int32(i)] = struct{}{}
	}
	return res
}
