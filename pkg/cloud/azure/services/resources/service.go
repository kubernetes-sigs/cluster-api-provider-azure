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

package resources

import (
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
)

// Service holds a collection of interfaces.
// The interfaces are broken down like this to group functions together.
// One alternative is to have a large list of functions from the ec2 client.
type Service struct {
	scope *actuators.Scope
}

// NewService returns a new service given the api clients.
func NewService(scope *actuators.Scope) *Service {
	return &Service{
		scope: scope,
	}
}

// TODO: Remove this once scope code is in.
/*
import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
)

// Service implements the AzureResourceManagementClient interface.
type Service struct {
	DeploymentsClient resources.DeploymentsClient
	GroupsClient      resources.GroupsClient
	ctx               context.Context
}

// NewService returns a new instance of Service.
func NewService(subscriptionID string) *Service {
	return &Service{
		DeploymentsClient: resources.NewDeploymentsClient(subscriptionID),
		GroupsClient:      resources.NewGroupsClient(subscriptionID),
		ctx:               context.Background(),
	}
}

// SetAuthorizer sets the authorizer component of the azure clients.
func (s *Service) SetAuthorizer(authorizer autorest.Authorizer) {
	s.DeploymentsClient.BaseClient.Client.Authorizer = authorizer
	s.GroupsClient.BaseClient.Client.Authorizer = authorizer
}

// ResourceName extracts the name of the resource from its ID.
func ResourceName(id string) (string, error) {
	parts := strings.Split(id, "/")
	name := parts[len(parts)-1]
	if len(name) == 0 {
		return "", fmt.Errorf("identifier did not contain resource name")
	}
	return name, nil
}
*/
