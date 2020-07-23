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

package groups

import (
	"github.com/go-logr/logr"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Service provides operations on azure resources
type Service struct {
	Scope GroupScope
	Client
}

// GroupScope defines the scope interface for a group service.
type GroupScope interface {
	logr.Logger
	azure.ClusterDescriber
	azure.Authorizer
}

// NewService creates a new service.
func NewService(scope GroupScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope.SubscriptionID(), scope),
	}
}
