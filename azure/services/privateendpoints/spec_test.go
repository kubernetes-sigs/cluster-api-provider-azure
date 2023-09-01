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

package privateendpoints

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	privateEndpoint1 = infrav1.PrivateEndpointSpec{
		Name:                          "test-private-endpoint1",
		ApplicationSecurityGroups:     []string{"asg1"},
		PrivateLinkServiceConnections: []infrav1.PrivateLinkServiceConnection{{PrivateLinkServiceID: "testPl", RequestMessage: "Please approve my connection."}},
		ManualApproval:                false,
	}

	privateEndpoint1Manual = infrav1.PrivateEndpointSpec{
		Name:                          "test-private-endpoint-manual",
		ApplicationSecurityGroups:     []string{"asg1"},
		PrivateLinkServiceConnections: []infrav1.PrivateLinkServiceConnection{{PrivateLinkServiceID: "testPl", RequestMessage: "Please approve my connection.", GroupIDs: []string{"aa", "bb"}}},
		ManualApproval:                true,
		CustomNetworkInterfaceName:    "test-if-name",
		PrivateIPAddresses:            []string{"10.0.0.1", "10.0.0.2"},
		Location:                      "test-location",
	}

	privateEndpoint2 = infrav1.PrivateEndpointSpec{
		Name:                          "test-private-endpoint2",
		ApplicationSecurityGroups:     []string{"asg1"},
		PrivateLinkServiceConnections: []infrav1.PrivateLinkServiceConnection{{PrivateLinkServiceID: "testPl", RequestMessage: "Please approve my connection.", GroupIDs: []string{"aa", "bb"}}},
		ManualApproval:                false,
		CustomNetworkInterfaceName:    "test-if-name",
		PrivateIPAddresses:            []string{"10.0.0.1", "10.0.0.2"},
		Location:                      "test-location",
	}
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *PrivateEndpointSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name: "PrivateEndpoint already exists with the same config",
			spec: &PrivateEndpointSpec{
				Name:                      privateEndpoint1.Name,
				ResourceGroup:             "test-group",
				ClusterName:               "my-cluster",
				ApplicationSecurityGroups: privateEndpoint1.ApplicationSecurityGroups,
				PrivateLinkServiceConnections: []PrivateLinkServiceConnection{{
					Name:                 privateEndpoint1.PrivateLinkServiceConnections[0].Name,
					GroupIDs:             privateEndpoint1.PrivateLinkServiceConnections[0].GroupIDs,
					PrivateLinkServiceID: privateEndpoint1.PrivateLinkServiceConnections[0].PrivateLinkServiceID,
					RequestMessage:       privateEndpoint1.PrivateLinkServiceConnections[0].RequestMessage,
				}},
				SubnetID: "test-subnet",
			},
			// See https://learn.microsoft.com/en-us/rest/api/virtualnetwork/private-endpoints/get?tabs=Go for more options
			existing: armnetwork.PrivateEndpoint{
				Name: ptr.To("test-private-endpoint1"),
				Properties: &armnetwork.PrivateEndpointProperties{
					Subnet: &armnetwork.Subnet{
						ID: ptr.To("test-subnet"),
						Properties: &armnetwork.SubnetPropertiesFormat{
							PrivateEndpointNetworkPolicies:    ptr.To(armnetwork.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled),
							PrivateLinkServiceNetworkPolicies: ptr.To(armnetwork.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled),
						},
					},
					ApplicationSecurityGroups: []*armnetwork.ApplicationSecurityGroup{{
						ID: ptr.To("asg1"),
					}},
					PrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{{
						Name: ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].Name),
						Properties: &armnetwork.PrivateLinkServiceConnectionProperties{
							PrivateLinkServiceID: ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
							GroupIDs:             nil,
							RequestMessage:       ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].RequestMessage),
						},
					}},
					ManualPrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{},
					ProvisioningState:                   ptr.To(armnetwork.ProvisioningStateSucceeded),
				},
				Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"), "Name": ptr.To("test-private-endpoint1")},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name: "PrivateEndpoint with manual approval already exists with the same config",
			spec: &PrivateEndpointSpec{
				Name:                      privateEndpoint1Manual.Name,
				ResourceGroup:             "test-group",
				ClusterName:               "my-cluster",
				ApplicationSecurityGroups: privateEndpoint1Manual.ApplicationSecurityGroups,
				PrivateLinkServiceConnections: []PrivateLinkServiceConnection{{
					Name:                 privateEndpoint1Manual.PrivateLinkServiceConnections[0].Name,
					GroupIDs:             privateEndpoint1Manual.PrivateLinkServiceConnections[0].GroupIDs,
					PrivateLinkServiceID: privateEndpoint1Manual.PrivateLinkServiceConnections[0].PrivateLinkServiceID,
					RequestMessage:       privateEndpoint1Manual.PrivateLinkServiceConnections[0].RequestMessage,
				}},
				SubnetID:       "test-subnet",
				ManualApproval: privateEndpoint1Manual.ManualApproval,
			},
			// See https://learn.microsoft.com/en-us/rest/api/virtualnetwork/private-endpoints/get?tabs=Go for more options
			existing: armnetwork.PrivateEndpoint{
				Name: ptr.To("test-private-endpoint-manual"),
				Properties: &armnetwork.PrivateEndpointProperties{
					Subnet: &armnetwork.Subnet{
						ID: ptr.To("test-subnet"),
						Properties: &armnetwork.SubnetPropertiesFormat{
							PrivateEndpointNetworkPolicies:    ptr.To(armnetwork.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled),
							PrivateLinkServiceNetworkPolicies: ptr.To(armnetwork.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled),
						},
					},
					ApplicationSecurityGroups: []*armnetwork.ApplicationSecurityGroup{{
						ID: ptr.To("asg1"),
					}},
					ManualPrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{{
						Name: ptr.To(privateEndpoint1Manual.PrivateLinkServiceConnections[0].Name),
						Properties: &armnetwork.PrivateLinkServiceConnectionProperties{
							PrivateLinkServiceID: ptr.To(privateEndpoint1Manual.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
							GroupIDs:             []*string{ptr.To("aa"), ptr.To("bb")},
							RequestMessage:       ptr.To(privateEndpoint1Manual.PrivateLinkServiceConnections[0].RequestMessage),
						},
					}},
					PrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{},
					ProvisioningState:             ptr.To(armnetwork.ProvisioningStateSucceeded),
				},
				Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"), "Name": ptr.To("test-private-endpoint-manual")},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name: "PrivateEndpoint already exists, but missing an IP address",
			spec: &PrivateEndpointSpec{
				Name:                      privateEndpoint2.Name,
				Location:                  privateEndpoint2.Location,
				ResourceGroup:             "test-group",
				ClusterName:               "my-cluster",
				ApplicationSecurityGroups: privateEndpoint2.ApplicationSecurityGroups,
				PrivateLinkServiceConnections: []PrivateLinkServiceConnection{{
					Name:                 privateEndpoint2.PrivateLinkServiceConnections[0].Name,
					GroupIDs:             privateEndpoint2.PrivateLinkServiceConnections[0].GroupIDs,
					PrivateLinkServiceID: privateEndpoint2.PrivateLinkServiceConnections[0].PrivateLinkServiceID,
					RequestMessage:       privateEndpoint2.PrivateLinkServiceConnections[0].RequestMessage,
				}},
				SubnetID:                   "test-subnet",
				PrivateIPAddresses:         privateEndpoint2.PrivateIPAddresses,
				CustomNetworkInterfaceName: "test-if-name",
			},
			existing: armnetwork.PrivateEndpoint{
				Name:     ptr.To("test-private-endpoint2"),
				Location: ptr.To("test-location"),
				Properties: &armnetwork.PrivateEndpointProperties{
					Subnet: &armnetwork.Subnet{
						ID: ptr.To("test-subnet"),
						Properties: &armnetwork.SubnetPropertiesFormat{
							PrivateEndpointNetworkPolicies:    ptr.To(armnetwork.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled),
							PrivateLinkServiceNetworkPolicies: ptr.To(armnetwork.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled),
						},
					},
					ApplicationSecurityGroups: []*armnetwork.ApplicationSecurityGroup{{
						ID: ptr.To("asg1"),
					}},
					PrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{{
						Name: ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].Name),
						Properties: &armnetwork.PrivateLinkServiceConnectionProperties{
							PrivateLinkServiceID: ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
							GroupIDs:             []*string{ptr.To("aa"), ptr.To("bb")},
							RequestMessage:       ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].RequestMessage),
						},
					}},
					ManualPrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{},
					ProvisioningState:                   ptr.To(armnetwork.ProvisioningStateSucceeded),
					IPConfigurations: []*armnetwork.PrivateEndpointIPConfiguration{
						{
							Properties: &armnetwork.PrivateEndpointIPConfigurationProperties{
								PrivateIPAddress: ptr.To("10.0.0.1"),
							},
						},
					},
					CustomNetworkInterfaceName: ptr.To("test-if-name"),
				},
				Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"), "Name": ptr.To("test-private-endpoint2")},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.PrivateEndpoint{}))
				g.Expect(result).To(Equal(armnetwork.PrivateEndpoint{
					Name:     ptr.To("test-private-endpoint2"),
					Location: ptr.To("test-location"),
					Properties: &armnetwork.PrivateEndpointProperties{
						Subnet: &armnetwork.Subnet{
							ID: ptr.To("test-subnet"),
							Properties: &armnetwork.SubnetPropertiesFormat{
								PrivateEndpointNetworkPolicies:    ptr.To(armnetwork.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled),
								PrivateLinkServiceNetworkPolicies: ptr.To(armnetwork.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled),
							},
						},
						ApplicationSecurityGroups: []*armnetwork.ApplicationSecurityGroup{{
							ID: ptr.To("asg1"),
						}},
						PrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{{
							Name: ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].Name),
							Properties: &armnetwork.PrivateLinkServiceConnectionProperties{
								PrivateLinkServiceID: ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
								GroupIDs:             []*string{ptr.To("aa"), ptr.To("bb")},
								RequestMessage:       ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].RequestMessage),
							},
						}},
						ManualPrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{},
						IPConfigurations: []*armnetwork.PrivateEndpointIPConfiguration{
							{
								Properties: &armnetwork.PrivateEndpointIPConfigurationProperties{
									PrivateIPAddress: ptr.To("10.0.0.1"),
								},
							},
							{
								Properties: &armnetwork.PrivateEndpointIPConfigurationProperties{
									PrivateIPAddress: ptr.To("10.0.0.2"),
								},
							},
						},
						CustomNetworkInterfaceName: ptr.To("test-if-name"),
					},
					Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"), "Name": ptr.To("test-private-endpoint2")},
				}))
			},
		},
		{
			name: "PrivateEndpoint doesn't exist",
			spec: &PrivateEndpointSpec{
				Name:                      privateEndpoint2.Name,
				Location:                  privateEndpoint2.Location,
				ResourceGroup:             "test-group",
				ClusterName:               "my-cluster",
				ApplicationSecurityGroups: privateEndpoint2.ApplicationSecurityGroups,
				PrivateLinkServiceConnections: []PrivateLinkServiceConnection{{
					Name:                 privateEndpoint2.PrivateLinkServiceConnections[0].Name,
					GroupIDs:             privateEndpoint2.PrivateLinkServiceConnections[0].GroupIDs,
					PrivateLinkServiceID: privateEndpoint2.PrivateLinkServiceConnections[0].PrivateLinkServiceID,
					RequestMessage:       privateEndpoint2.PrivateLinkServiceConnections[0].RequestMessage,
				}},
				SubnetID:                   "test-subnet",
				PrivateIPAddresses:         privateEndpoint2.PrivateIPAddresses,
				CustomNetworkInterfaceName: "test-if-name",
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.PrivateEndpoint{}))
				g.Expect(result).To(Equal(armnetwork.PrivateEndpoint{
					Name:     ptr.To("test-private-endpoint2"),
					Location: ptr.To("test-location"),
					Properties: &armnetwork.PrivateEndpointProperties{
						Subnet: &armnetwork.Subnet{
							ID: ptr.To("test-subnet"),
							Properties: &armnetwork.SubnetPropertiesFormat{
								PrivateEndpointNetworkPolicies:    ptr.To(armnetwork.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled),
								PrivateLinkServiceNetworkPolicies: ptr.To(armnetwork.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled),
							},
						},
						ApplicationSecurityGroups: []*armnetwork.ApplicationSecurityGroup{{
							ID: ptr.To("asg1"),
						}},
						PrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{{
							Name: ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].Name),
							Properties: &armnetwork.PrivateLinkServiceConnectionProperties{
								PrivateLinkServiceID: ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
								GroupIDs:             []*string{ptr.To("aa"), ptr.To("bb")},
								RequestMessage:       ptr.To(privateEndpoint1.PrivateLinkServiceConnections[0].RequestMessage),
							},
						}},
						ManualPrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{},
						IPConfigurations: []*armnetwork.PrivateEndpointIPConfiguration{
							{
								Properties: &armnetwork.PrivateEndpointIPConfigurationProperties{
									PrivateIPAddress: ptr.To("10.0.0.1"),
								},
							},
							{
								Properties: &armnetwork.PrivateEndpointIPConfigurationProperties{
									PrivateIPAddress: ptr.To("10.0.0.2"),
								},
							},
						},
						CustomNetworkInterfaceName: ptr.To("test-if-name"),
					},
					Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"), "Name": ptr.To("test-private-endpoint2")},
				}))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, result)
		})
	}
}
