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

package securitygroups

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/klogr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/securitygroups/mock_securitygroups"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileSecurityGroups(t *testing.T) {
	testcases := []struct {
		name   string
		expect func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder)
	}{
		{
			name: "security groups do not exist",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder) {
				s.NSGSpecs().Return([]azure.NSGSpec{
					{
						Name: "nsg-one",
						IngressRules: infrav1.IngressRules{
							{
								Name:             "first-rule",
								Description:      "a test rule",
								Protocol:         "*",
								Priority:         400,
								SourcePorts:      to.StringPtr("*"),
								DestinationPorts: to.StringPtr("*"),
								Source:           to.StringPtr("*"),
								Destination:      to.StringPtr("*"),
							},
							{
								Name:             "second-rule",
								Description:      "another test rule",
								Protocol:         "*",
								Priority:         450,
								SourcePorts:      to.StringPtr("*"),
								DestinationPorts: to.StringPtr("*"),
								Source:           to.StringPtr("*"),
								Destination:      to.StringPtr("*"),
							},
						},
					},
					{
						Name:         "nsg-two",
						IngressRules: infrav1.IngressRules{},
					},
				})
				s.IsVnetManaged().Return(true)
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "nsg-one").Return(network.SecurityGroup{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "nsg-one", gomockinternal.DiffEq(network.SecurityGroup{
					SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
						SecurityRules: &[]network.SecurityRule{
							{
								SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
									Description:              to.StringPtr("a test rule"),
									SourcePortRange:          to.StringPtr("*"),
									DestinationPortRange:     to.StringPtr("*"),
									SourceAddressPrefix:      to.StringPtr("*"),
									DestinationAddressPrefix: to.StringPtr("*"),
									Protocol:                 "*",
									Direction:                "Inbound",
									Access:                   "Allow",
									Priority:                 to.Int32Ptr(400),
								},
								Name: to.StringPtr("first-rule"),
							},
							{
								SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
									Description:              to.StringPtr("another test rule"),
									SourcePortRange:          to.StringPtr("*"),
									DestinationPortRange:     to.StringPtr("*"),
									SourceAddressPrefix:      to.StringPtr("*"),
									DestinationAddressPrefix: to.StringPtr("*"),
									Protocol:                 "*",
									Direction:                "Inbound",
									Access:                   "Allow",
									Priority:                 to.Int32Ptr(450),
								},
								Name: to.StringPtr("second-rule"),
							},
						},
					},
					Etag:     nil,
					Location: to.StringPtr("test-location"),
				}))
				m.Get(gomockinternal.AContext(), "my-rg", "nsg-two").Return(network.SecurityGroup{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "nsg-two", gomockinternal.DiffEq(network.SecurityGroup{
					SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
						SecurityRules: &[]network.SecurityRule{},
					},
					Etag:     nil,
					Location: to.StringPtr("test-location"),
				}))
			},
		}, {
			name: "security group exists",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder) {
				s.NSGSpecs().Return([]azure.NSGSpec{
					{
						Name: "nsg-one",
						IngressRules: infrav1.IngressRules{
							{
								Name:             "first-rule",
								Description:      "a test rule",
								Protocol:         "*",
								Priority:         400,
								SourcePorts:      to.StringPtr("*"),
								DestinationPorts: to.StringPtr("*"),
								Source:           to.StringPtr("*"),
								Destination:      to.StringPtr("*"),
							},
						},
					},
					{
						Name:         "nsg-two",
						IngressRules: infrav1.IngressRules{},
					},
				})
				s.IsVnetManaged().AnyTimes().Return(true)
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("test-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "nsg-one").Return(network.SecurityGroup{
					Response: autorest.Response{},
					SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
						SecurityRules: &[]network.SecurityRule{
							{
								SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
									Description:              to.StringPtr("a different rule"),
									Protocol:                 "*",
									SourcePortRange:          to.StringPtr("*"),
									DestinationPortRange:     to.StringPtr("*"),
									SourceAddressPrefix:      to.StringPtr("*"),
									DestinationAddressPrefix: to.StringPtr("*"),
									Priority:                 to.Int32Ptr(300),
									Access:                   network.SecurityRuleAccessDeny,
									Direction:                network.SecurityRuleDirectionOutbound,
								},
								Name: to.StringPtr("foo-rule"),
							},
						},
					},
					Etag: to.StringPtr("test-etag"),
					ID:   to.StringPtr("fake/nsg/id"),
					Name: to.StringPtr("nsg-one"),
				}, nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "nsg-one", gomockinternal.DiffEq(network.SecurityGroup{
					SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
						SecurityRules: &[]network.SecurityRule{
							{
								SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
									Description:              to.StringPtr("a different rule"),
									SourcePortRange:          to.StringPtr("*"),
									DestinationPortRange:     to.StringPtr("*"),
									SourceAddressPrefix:      to.StringPtr("*"),
									DestinationAddressPrefix: to.StringPtr("*"),
									Protocol:                 "*",
									Direction:                "Outbound",
									Access:                   "Deny",
									Priority:                 to.Int32Ptr(300),
								},
								Name: to.StringPtr("foo-rule"),
							},
							{
								SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
									Description:              to.StringPtr("a test rule"),
									SourcePortRange:          to.StringPtr("*"),
									DestinationPortRange:     to.StringPtr("*"),
									SourceAddressPrefix:      to.StringPtr("*"),
									DestinationAddressPrefix: to.StringPtr("*"),
									Protocol:                 "*",
									Direction:                "Inbound",
									Access:                   "Allow",
									Priority:                 to.Int32Ptr(400),
								},
								Name: to.StringPtr("first-rule"),
							},
						},
					},
					Etag:     to.StringPtr("test-etag"),
					Location: to.StringPtr("test-location"),
				}))
				m.Get(gomockinternal.AContext(), "my-rg", "nsg-two").Return(network.SecurityGroup{
					Response: autorest.Response{},
					SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
						SecurityRules: &[]network.SecurityRule{},
					},
					Etag: to.StringPtr("test-etag"),
					ID:   to.StringPtr("fake/nsg/two/id"),
					Name: to.StringPtr("nsg-two"),
				}, nil)
			},
		}, {
			name: "skipping network security group reconcile in custom VNet mode",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder) {
				s.IsVnetManaged().Return(false)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
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

			scopeMock := mock_securitygroups.NewMockNSGScope(mockCtrl)
			clientMock := mock_securitygroups.NewMockclient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
			}

			g.Expect(s.Reconcile(context.TODO())).To(Succeed())
		})
	}
}

func TestDeleteSecurityGroups(t *testing.T) {
	testcases := []struct {
		name   string
		expect func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder)
	}{
		{
			name: "security groups exist",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder) {
				s.NSGSpecs().Return([]azure.NSGSpec{
					{
						Name: "nsg-one",
						IngressRules: infrav1.IngressRules{
							{
								Name:             "first-rule",
								Description:      "a test rule",
								Protocol:         "all",
								Priority:         400,
								SourcePorts:      to.StringPtr("*"),
								DestinationPorts: to.StringPtr("*"),
								Source:           to.StringPtr("*"),
								Destination:      to.StringPtr("*"),
							},
						},
					},
					{
						Name:         "nsg-two",
						IngressRules: infrav1.IngressRules{},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(gomockinternal.AContext(), "my-rg", "nsg-one")
				m.Delete(gomockinternal.AContext(), "my-rg", "nsg-two")
			},
		},
		{
			name: "security group already deleted",
			expect: func(s *mock_securitygroups.MockNSGScopeMockRecorder, m *mock_securitygroups.MockclientMockRecorder) {
				s.NSGSpecs().Return([]azure.NSGSpec{
					{
						Name:         "nsg-one",
						IngressRules: infrav1.IngressRules{},
					},
					{
						Name:         "nsg-two",
						IngressRules: infrav1.IngressRules{},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(gomockinternal.AContext(), "my-rg", "nsg-one").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Delete(gomockinternal.AContext(), "my-rg", "nsg-two")
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

			scopeMock := mock_securitygroups.NewMockNSGScope(mockCtrl)
			clientMock := mock_securitygroups.NewMockclient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
			}

			g.Expect(s.Delete(context.TODO())).To(Succeed())
		})
	}
}
