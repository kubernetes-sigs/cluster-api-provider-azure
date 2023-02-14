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
	"sort"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-05-01/network"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// PrivateLinkServiceConnection defines the specification for a private link service connection associated with a private endpoint.
type PrivateLinkServiceConnection struct {
	Name                 string
	PrivateLinkServiceID string
	GroupIDs             []string
	RequestMessage       string
}

// PrivateEndpointSpec defines the specification for a private endpoint.
type PrivateEndpointSpec struct {
	Name                          string
	ResourceGroup                 string
	Location                      string
	CustomNetworkInterfaceName    string
	PrivateIPAddresses            []string
	SubnetID                      string
	ApplicationSecurityGroups     []string
	ManualApproval                bool
	PrivateLinkServiceConnections []PrivateLinkServiceConnection
	AdditionalTags                infrav1.Tags
	ClusterName                   string
}

// ResourceName returns the name of the private endpoint.
func (s *PrivateEndpointSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *PrivateEndpointSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for private endpoints.
func (s *PrivateEndpointSpec) OwnerResourceName() string {
	return ""
}

// Parameters returns the parameters for the PrivateEndpointSpec.
func (s *PrivateEndpointSpec) Parameters(ctx context.Context, existing interface{}) (interface{}, error) {
	_, log, done := tele.StartSpanWithLogger(ctx, "privateendpoints.Service.Parameters")
	defer done()

	privateEndpointProperties := network.PrivateEndpointProperties{
		Subnet: &network.Subnet{
			ID: &s.SubnetID,
			SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
				PrivateEndpointNetworkPolicies:    network.VirtualNetworkPrivateEndpointNetworkPoliciesDisabled,
				PrivateLinkServiceNetworkPolicies: network.VirtualNetworkPrivateLinkServiceNetworkPoliciesEnabled,
			},
		},
	}

	if s.CustomNetworkInterfaceName != "" {
		privateEndpointProperties.CustomNetworkInterfaceName = pointer.String(s.CustomNetworkInterfaceName)
	}

	if len(s.PrivateIPAddresses) > 0 {
		privateIPAddresses := make([]network.PrivateEndpointIPConfiguration, 0, len(s.PrivateIPAddresses))
		for _, address := range s.PrivateIPAddresses {
			ipConfig := &network.PrivateEndpointIPConfigurationProperties{PrivateIPAddress: pointer.String(address)}

			privateIPAddresses = append(privateIPAddresses, network.PrivateEndpointIPConfiguration{
				PrivateEndpointIPConfigurationProperties: ipConfig,
			})
		}
		privateEndpointProperties.IPConfigurations = &privateIPAddresses
	}

	privateLinkServiceConnections := make([]network.PrivateLinkServiceConnection, 0, len(s.PrivateLinkServiceConnections))
	for _, privateLinkServiceConnection := range s.PrivateLinkServiceConnections {
		linkServiceConnection := network.PrivateLinkServiceConnection{
			Name: pointer.String(privateLinkServiceConnection.Name),
			PrivateLinkServiceConnectionProperties: &network.PrivateLinkServiceConnectionProperties{
				PrivateLinkServiceID: pointer.String(privateLinkServiceConnection.PrivateLinkServiceID),
			},
		}

		if len(privateLinkServiceConnection.GroupIDs) > 0 {
			linkServiceConnection.PrivateLinkServiceConnectionProperties.GroupIds = &privateLinkServiceConnection.GroupIDs
		}

		if privateLinkServiceConnection.RequestMessage != "" {
			linkServiceConnection.PrivateLinkServiceConnectionProperties.RequestMessage = pointer.String(privateLinkServiceConnection.RequestMessage)
		}
		privateLinkServiceConnections = append(privateLinkServiceConnections, linkServiceConnection)
	}

	if s.ManualApproval {
		privateEndpointProperties.ManualPrivateLinkServiceConnections = &privateLinkServiceConnections
		privateEndpointProperties.PrivateLinkServiceConnections = &[]network.PrivateLinkServiceConnection{}
	} else {
		privateEndpointProperties.PrivateLinkServiceConnections = &privateLinkServiceConnections
		privateEndpointProperties.ManualPrivateLinkServiceConnections = &[]network.PrivateLinkServiceConnection{}
	}

	applicationSecurityGroups := make([]network.ApplicationSecurityGroup, 0, len(s.ApplicationSecurityGroups))

	for _, applicationSecurityGroup := range s.ApplicationSecurityGroups {
		applicationSecurityGroups = append(applicationSecurityGroups, network.ApplicationSecurityGroup{
			ID: pointer.String(applicationSecurityGroup),
		})
	}

	privateEndpointProperties.ApplicationSecurityGroups = &applicationSecurityGroups

	newPrivateEndpoint := network.PrivateEndpoint{
		Name:                      pointer.String(s.Name),
		PrivateEndpointProperties: &privateEndpointProperties,
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        pointer.String(s.Name),
			Additional:  s.AdditionalTags,
		})),
	}

	if s.Location != "" {
		newPrivateEndpoint.Location = pointer.String(s.Location)
	}

	if existing != nil {
		existingPE, ok := existing.(network.PrivateEndpoint)
		if !ok {
			return nil, errors.Errorf("%T is not a network.PrivateEndpoint", existing)
		}

		ps := existingPE.ProvisioningState
		if string(ps) != string(infrav1.Canceled) && string(ps) != string(infrav1.Failed) && string(ps) != string(infrav1.Succeeded) {
			return nil, azure.WithTransientError(errors.Errorf("Unable to update existing private endpoint in non-terminal state. Service Endpoint must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: %s", ps), 20*time.Second)
		}

		normalizedExistingPE := normalizePrivateEndpoint(existingPE)
		normalizedExistingPE = sortSlicesPrivateEndpoint(normalizedExistingPE)

		newPrivateEndpoint = sortSlicesPrivateEndpoint(newPrivateEndpoint)

		diff := cmp.Diff(&normalizedExistingPE, &newPrivateEndpoint)
		if diff == "" {
			// PrivateEndpoint is up-to-date, nothing to do
			log.V(4).Info("no changes found between user-updated spec and existing spec")
			return nil, nil
		}
		log.V(4).Info("found a diff between the desired spec and the existing privateendpoint", "difference", diff)
	}

	return newPrivateEndpoint, nil
}

func normalizePrivateEndpoint(existingPE network.PrivateEndpoint) network.PrivateEndpoint {
	normalizedExistingPE := network.PrivateEndpoint{
		Name:     existingPE.Name,
		Location: existingPE.Location,
		PrivateEndpointProperties: &network.PrivateEndpointProperties{
			Subnet: &network.Subnet{
				ID: existingPE.Subnet.ID,
				SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
					PrivateEndpointNetworkPolicies:    existingPE.Subnet.PrivateEndpointNetworkPolicies,
					PrivateLinkServiceNetworkPolicies: existingPE.Subnet.PrivateLinkServiceNetworkPolicies,
				},
			},
			ApplicationSecurityGroups:  existingPE.ApplicationSecurityGroups,
			IPConfigurations:           existingPE.IPConfigurations,
			CustomNetworkInterfaceName: existingPE.CustomNetworkInterfaceName,
		},
		Tags: existingPE.Tags,
	}

	existingPrivateLinkServiceConnections := make([]network.PrivateLinkServiceConnection, 0, len(*existingPE.PrivateLinkServiceConnections))
	for _, privateLinkServiceConnection := range *existingPE.PrivateLinkServiceConnections {
		existingPrivateLinkServiceConnections = append(existingPrivateLinkServiceConnections, network.PrivateLinkServiceConnection{
			Name: privateLinkServiceConnection.Name,
			PrivateLinkServiceConnectionProperties: &network.PrivateLinkServiceConnectionProperties{
				PrivateLinkServiceID: privateLinkServiceConnection.PrivateLinkServiceID,
				RequestMessage:       privateLinkServiceConnection.RequestMessage,
				GroupIds:             privateLinkServiceConnection.GroupIds,
			},
		})
	}
	normalizedExistingPE.PrivateEndpointProperties.PrivateLinkServiceConnections = &existingPrivateLinkServiceConnections

	existingManualPrivateLinkServiceConnections := make([]network.PrivateLinkServiceConnection, 0, len(*existingPE.ManualPrivateLinkServiceConnections))
	for _, manualPrivateLinkServiceConnection := range *existingPE.ManualPrivateLinkServiceConnections {
		existingManualPrivateLinkServiceConnections = append(existingManualPrivateLinkServiceConnections, network.PrivateLinkServiceConnection{
			Name: manualPrivateLinkServiceConnection.Name,
			PrivateLinkServiceConnectionProperties: &network.PrivateLinkServiceConnectionProperties{
				PrivateLinkServiceID: manualPrivateLinkServiceConnection.PrivateLinkServiceID,
				RequestMessage:       manualPrivateLinkServiceConnection.RequestMessage,
				GroupIds:             manualPrivateLinkServiceConnection.GroupIds,
			},
		})
	}
	normalizedExistingPE.PrivateEndpointProperties.ManualPrivateLinkServiceConnections = &existingManualPrivateLinkServiceConnections

	return normalizedExistingPE
}

// Sort all slices in order to get the same order of elements for both new and existing private endpoints.
func sortSlicesPrivateEndpoint(privateEndpoint network.PrivateEndpoint) network.PrivateEndpoint {
	// Sort ManualPrivateLinkServiceConnections
	if privateEndpoint.ManualPrivateLinkServiceConnections != nil {
		sort.SliceStable(*privateEndpoint.ManualPrivateLinkServiceConnections, func(i, j int) bool {
			return *(*privateEndpoint.ManualPrivateLinkServiceConnections)[i].Name < *(*privateEndpoint.ManualPrivateLinkServiceConnections)[j].Name
		})
	}

	// Sort PrivateLinkServiceConnections
	if privateEndpoint.PrivateLinkServiceConnections != nil {
		sort.SliceStable(*privateEndpoint.PrivateLinkServiceConnections, func(i, j int) bool {
			return *(*privateEndpoint.PrivateLinkServiceConnections)[i].Name < *(*privateEndpoint.PrivateLinkServiceConnections)[j].Name
		})
	}

	// Sort IPConfigurations
	if privateEndpoint.IPConfigurations != nil {
		sort.SliceStable(*privateEndpoint.IPConfigurations, func(i, j int) bool {
			return *(*privateEndpoint.IPConfigurations)[i].PrivateIPAddress < *(*privateEndpoint.IPConfigurations)[j].PrivateIPAddress
		})
	}

	// Sort ApplicationSecurityGroups
	if privateEndpoint.ApplicationSecurityGroups != nil {
		sort.SliceStable(*privateEndpoint.ApplicationSecurityGroups, func(i, j int) bool {
			return *(*privateEndpoint.ApplicationSecurityGroups)[i].Name < *(*privateEndpoint.ApplicationSecurityGroups)[j].Name
		})
	}

	return privateEndpoint
}
