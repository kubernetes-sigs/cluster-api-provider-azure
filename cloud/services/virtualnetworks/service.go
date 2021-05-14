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

package virtualnetworks

import (
	"github.com/go-logr/logr"
	azure "github.com/niachary/cluster-api-provider-azure/cloud"
)

// VNetScope defines the scope interface for a virtual network service.
type VNetScope interface {
	logr.Logger
	azure.ClusterDescriber
	VNetSpecs() []azure.VNetSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope VNetScope
	Client
}

// NewService creates a new service.
func NewService(scope VNetScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope),
	}
}
