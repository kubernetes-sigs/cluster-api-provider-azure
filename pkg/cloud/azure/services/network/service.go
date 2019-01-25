/*
Copyright 2018 The Kubernetes Authors.

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

package network

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest"
)

// Service provides the instances of the azure network API clients.
type Service struct {
	InterfacesClient        network.InterfacesClient
	PublicIPAddressesClient network.PublicIPAddressesClient
	SecurityGroupsClient    network.SecurityGroupsClient
	VirtualNetworksClient   network.VirtualNetworksClient
	ctx                     context.Context
}

// NewService returns a new instance of Service.
func NewService(subscriptionID string) *Service {
	return &Service{
		InterfacesClient:        network.NewInterfacesClient(subscriptionID),
		PublicIPAddressesClient: network.NewPublicIPAddressesClient(subscriptionID),
		SecurityGroupsClient:    network.NewSecurityGroupsClient(subscriptionID),
		VirtualNetworksClient:   network.NewVirtualNetworksClient(subscriptionID),
		ctx:                     context.Background(),
	}
}

// SetAuthorizer populates the authorizer component of the network client objects.
func (s *Service) SetAuthorizer(authorizer autorest.Authorizer) {
	s.InterfacesClient.BaseClient.Client.Authorizer = authorizer
	s.PublicIPAddressesClient.BaseClient.Client.Authorizer = authorizer
	s.SecurityGroupsClient.BaseClient.Client.Authorizer = authorizer
	s.VirtualNetworksClient.BaseClient.Client.Authorizer = authorizer
}
