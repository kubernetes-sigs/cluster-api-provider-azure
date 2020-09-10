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

package subnets

import (
	"github.com/go-logr/logr"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// SubnetScope defines the scope interface for a subnet service.
type SubnetScope interface {
	azure.ClusterDescriber
	logr.Logger
	SubnetSpecs() []azure.SubnetSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope SubnetScope
	Client
}

// NewService creates a new service.
func NewService(scope SubnetScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope),
	}
}
