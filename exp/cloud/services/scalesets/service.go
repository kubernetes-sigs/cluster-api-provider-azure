/*
Copyright 2020 The Kubernetes Authors.

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

package scalesets

import (
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	infrav1expscope "sigs.k8s.io/cluster-api-provider-azure/exp/cloud/scope"
)

// Service provides operations on azure resources
type Service struct {
	Client
	Scope            *scope.ClusterScope
	MachinePoolScope *infrav1expscope.MachinePoolScope
}

// NewService creates a new service.
func NewService(scope *scope.ClusterScope, machinePoolScope *infrav1expscope.MachinePoolScope) *Service {
	return &Service{
		Scope:            scope,
		MachinePoolScope: machinePoolScope,
		Client:           NewClient(scope.SubscriptionID, scope.Authorizer),
	}
}
