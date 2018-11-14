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
package resourcemanagement

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
)

type Service struct {
	DeploymentsClient resources.DeploymentsClient
	GroupsClient      resources.GroupsClient
	ctx               context.Context
}

func NewService(subscriptionId string) *Service {
	return &Service{
		DeploymentsClient: resources.NewDeploymentsClient(subscriptionId),
		GroupsClient:      resources.NewGroupsClient(subscriptionId),
		ctx:               context.Background(),
	}
}

func (s *Service) SetAuthorizer(authorizer autorest.Authorizer) {
	s.DeploymentsClient.BaseClient.Client.Authorizer = authorizer
	s.GroupsClient.BaseClient.Client.Authorizer = authorizer
}

func ResourceName(id string) (string, error) {
	parts := strings.Split(id, "/")
	name := parts[len(parts)-1]
	if len(name) == 0 {
		return "", fmt.Errorf("identifier did not contain resource name")
	}
	return name, nil
}
