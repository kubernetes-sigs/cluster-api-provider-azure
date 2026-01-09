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

package roleassignmentsaso

import (
	"context"

	asoauthorizationv1api20220401 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/conditionalaso"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"
)

const serviceName = "roleassignmentsaso"

// KubernetesRoleAssignmentScope defines the scope interface for a Kubernetes role assignment service.
type KubernetesRoleAssignmentScope interface {
	conditionalaso.Scope
	KubernetesRoleAssignmentSpecs() []azure.ASOResourceSpecGetter[*asoauthorizationv1api20220401.RoleAssignment]
}

// New creates a new service.
func New(scope KubernetesRoleAssignmentScope) *conditionalaso.Service[*asoauthorizationv1api20220401.RoleAssignment, KubernetesRoleAssignmentScope] {
	svc := conditionalaso.NewService[*asoauthorizationv1api20220401.RoleAssignment, KubernetesRoleAssignmentScope](serviceName, scope)
	svc.ListFunc = list
	svc.Specs = scope.KubernetesRoleAssignmentSpecs()
	// Convert v1beta1.ConditionType to v1beta2.ConditionType
	svc.ConditionType = clusterv1.ConditionType(string(infrav1.RoleAssignmentReadyCondition))
	return svc
}

func list(ctx context.Context, client client.Client, opts ...client.ListOption) ([]*asoauthorizationv1api20220401.RoleAssignment, error) {
	list := &asoauthorizationv1api20220401.RoleAssignmentList{}
	err := client.List(ctx, list, opts...)
	return slice.ToPtrs(list.Items), err
}
