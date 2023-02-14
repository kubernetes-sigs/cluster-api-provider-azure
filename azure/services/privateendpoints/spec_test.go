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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-05-01/network"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
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
			existing: network.PrivateEndpoint{
				Name: pointer.String("test-private-endpoint1"),
				PrivateEndpointProperties: &network.PrivateEndpointProperties{
					Subnet: &network.Subnet{
						ID: pointer.String("test-subnet"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							PrivateEndpointNetworkPolicies:    network.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled,
							PrivateLinkServiceNetworkPolicies: network.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled,
						},
					},
					ApplicationSecurityGroups: &[]network.ApplicationSecurityGroup{{
						ID: pointer.String("asg1"),
					}},
					PrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{{
						Name: pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].Name),
						PrivateLinkServiceConnectionProperties: &network.PrivateLinkServiceConnectionProperties{
							PrivateLinkServiceID: pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
							GroupIds:             nil,
							RequestMessage:       pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].RequestMessage),
						},
					}},
					ManualPrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{},
					ProvisioningState:                   "Succeeded",
				},
				Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": pointer.String("owned"), "Name": pointer.String("test-private-endpoint1")},
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
			existing: network.PrivateEndpoint{
				Name: pointer.String("test-private-endpoint-manual"),
				PrivateEndpointProperties: &network.PrivateEndpointProperties{
					Subnet: &network.Subnet{
						ID: pointer.String("test-subnet"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							PrivateEndpointNetworkPolicies:    network.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled,
							PrivateLinkServiceNetworkPolicies: network.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled,
						},
					},
					ApplicationSecurityGroups: &[]network.ApplicationSecurityGroup{{
						ID: pointer.String("asg1"),
					}},
					ManualPrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{{
						Name: pointer.String(privateEndpoint1Manual.PrivateLinkServiceConnections[0].Name),
						PrivateLinkServiceConnectionProperties: &network.PrivateLinkServiceConnectionProperties{
							PrivateLinkServiceID: pointer.String(privateEndpoint1Manual.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
							GroupIds:             &[]string{"aa", "bb"},
							RequestMessage:       pointer.String(privateEndpoint1Manual.PrivateLinkServiceConnections[0].RequestMessage),
						},
					}},
					PrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{},
					ProvisioningState:             "Succeeded",
				},
				Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": pointer.String("owned"), "Name": pointer.String("test-private-endpoint-manual")},
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
			existing: network.PrivateEndpoint{
				Name:     pointer.String("test-private-endpoint2"),
				Location: pointer.String("test-location"),
				PrivateEndpointProperties: &network.PrivateEndpointProperties{
					Subnet: &network.Subnet{
						ID: pointer.String("test-subnet"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							PrivateEndpointNetworkPolicies:    network.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled,
							PrivateLinkServiceNetworkPolicies: network.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled,
						},
					},
					ApplicationSecurityGroups: &[]network.ApplicationSecurityGroup{{
						ID: pointer.String("asg1"),
					}},
					PrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{{
						Name: pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].Name),
						PrivateLinkServiceConnectionProperties: &network.PrivateLinkServiceConnectionProperties{
							PrivateLinkServiceID: pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
							GroupIds:             &[]string{"aa", "bb"},
							RequestMessage:       pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].RequestMessage),
						},
					}},
					ManualPrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{},
					ProvisioningState:                   "Succeeded",
					IPConfigurations: &[]network.PrivateEndpointIPConfiguration{
						{
							PrivateEndpointIPConfigurationProperties: &network.PrivateEndpointIPConfigurationProperties{
								PrivateIPAddress: pointer.String("10.0.0.1"),
							},
						},
					},
					CustomNetworkInterfaceName: pointer.String("test-if-name"),
				},
				Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": pointer.String("owned"), "Name": pointer.String("test-private-endpoint2")},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.PrivateEndpoint{}))
				g.Expect(result).To(Equal(network.PrivateEndpoint{
					Name:     pointer.String("test-private-endpoint2"),
					Location: pointer.String("test-location"),
					PrivateEndpointProperties: &network.PrivateEndpointProperties{
						Subnet: &network.Subnet{
							ID: pointer.String("test-subnet"),
							SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
								PrivateEndpointNetworkPolicies:    network.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled,
								PrivateLinkServiceNetworkPolicies: network.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled,
							},
						},
						ApplicationSecurityGroups: &[]network.ApplicationSecurityGroup{{
							ID: pointer.String("asg1"),
						}},
						PrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{{
							Name: pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].Name),
							PrivateLinkServiceConnectionProperties: &network.PrivateLinkServiceConnectionProperties{
								PrivateLinkServiceID: pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
								GroupIds:             &[]string{"aa", "bb"},
								RequestMessage:       pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].RequestMessage),
							},
						}},
						ManualPrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{},
						IPConfigurations: &[]network.PrivateEndpointIPConfiguration{
							{
								PrivateEndpointIPConfigurationProperties: &network.PrivateEndpointIPConfigurationProperties{
									PrivateIPAddress: pointer.String("10.0.0.1"),
								},
							},
							{
								PrivateEndpointIPConfigurationProperties: &network.PrivateEndpointIPConfigurationProperties{
									PrivateIPAddress: pointer.String("10.0.0.2"),
								},
							},
						},
						CustomNetworkInterfaceName: pointer.String("test-if-name"),
					},
					Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": pointer.String("owned"), "Name": pointer.String("test-private-endpoint2")},
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
				g.Expect(result).To(BeAssignableToTypeOf(network.PrivateEndpoint{}))
				g.Expect(result).To(Equal(network.PrivateEndpoint{
					Name:     pointer.String("test-private-endpoint2"),
					Location: pointer.String("test-location"),
					PrivateEndpointProperties: &network.PrivateEndpointProperties{
						Subnet: &network.Subnet{
							ID: pointer.String("test-subnet"),
							SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
								PrivateEndpointNetworkPolicies:    network.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled,
								PrivateLinkServiceNetworkPolicies: network.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled,
							},
						},
						ApplicationSecurityGroups: &[]network.ApplicationSecurityGroup{{
							ID: pointer.String("asg1"),
						}},
						PrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{{
							Name: pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].Name),
							PrivateLinkServiceConnectionProperties: &network.PrivateLinkServiceConnectionProperties{
								PrivateLinkServiceID: pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].PrivateLinkServiceID),
								GroupIds:             &[]string{"aa", "bb"},
								RequestMessage:       pointer.String(privateEndpoint1.PrivateLinkServiceConnections[0].RequestMessage),
							},
						}},
						ManualPrivateLinkServiceConnections: &[]network.PrivateLinkServiceConnection{},
						IPConfigurations: &[]network.PrivateEndpointIPConfiguration{
							{
								PrivateEndpointIPConfigurationProperties: &network.PrivateEndpointIPConfigurationProperties{
									PrivateIPAddress: pointer.String("10.0.0.1"),
								},
							},
							{
								PrivateEndpointIPConfigurationProperties: &network.PrivateEndpointIPConfigurationProperties{
									PrivateIPAddress: pointer.String("10.0.0.2"),
								},
							},
						},
						CustomNetworkInterfaceName: pointer.String("test-if-name"),
					},
					Tags: map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": pointer.String("owned"), "Name": pointer.String("test-private-endpoint2")},
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
