/*
Copyright 2025 The Kubernetes Authors.

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

package userassignedidentities

import (
	"context"

	"github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/conditionalaso"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"
)

const (
	serviceName = "userassignedidentities"
	// UserIdentitiesReadyCondition defines the condition type for user assigned identities readiness.
	UserIdentitiesReadyCondition clusterv1.ConditionType = "UserIdentitiesReady"
)

// UserAssignedIdentityScope defines the scope interface for a user assigned identity service.
type UserAssignedIdentityScope interface {
	conditionalaso.Scope
	UserAssignedIdentitySpecs() []azure.ASOResourceSpecGetter[*v1api20230131.UserAssignedIdentity]
}

// New creates a new service.
func New(scope UserAssignedIdentityScope) *conditionalaso.Service[*v1api20230131.UserAssignedIdentity, UserAssignedIdentityScope] {
	svc := conditionalaso.NewService[*v1api20230131.UserAssignedIdentity, UserAssignedIdentityScope](serviceName, scope)
	svc.ListFunc = list
	svc.Specs = scope.UserAssignedIdentitySpecs()
	svc.ConditionType = UserIdentitiesReadyCondition
	return svc
}

func list(ctx context.Context, client client.Client, opts ...client.ListOption) ([]*v1api20230131.UserAssignedIdentity, error) {
	list := &v1api20230131.UserAssignedIdentityList{}
	err := client.List(ctx, list, opts...)
	return slice.ToPtrs(list.Items), err
}
