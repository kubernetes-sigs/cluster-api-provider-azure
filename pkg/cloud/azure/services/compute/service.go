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
package compute

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-04-01/compute"
	"github.com/Azure/go-autorest/autorest"
)

type Service struct {
	DisksClient           compute.DisksClient
	VirtualMachinesClient compute.VirtualMachinesClient
	ctx                   context.Context
}

func NewService(subscriptionId string) *Service {
	return &Service{
		DisksClient:           compute.NewDisksClient(subscriptionId),
		VirtualMachinesClient: compute.NewVirtualMachinesClient(subscriptionId),
		ctx: context.Background(),
	}
}

func (s *Service) SetAuthorizer(authorizer autorest.Authorizer) {
	s.DisksClient.BaseClient.Client.Authorizer = authorizer
	s.VirtualMachinesClient.BaseClient.Client.Authorizer = authorizer
}
