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

package publicips

import (
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"

	"github.com/go-logr/logr"
)

// PublicIPScope defines the scope interface for a public IP service.
type PublicIPScope interface {
	logr.Logger
	azure.Authorizer
	azure.ClusterDescriber
	PublicIPSpecs() []azure.PublicIPSpec
}

// Service provides operations on Azure resources.
type Service struct {
	Scope PublicIPScope
	Client
}

// NewService creates a new service.
func NewService(scope PublicIPScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope.SubscriptionID(), scope),
	}
}
