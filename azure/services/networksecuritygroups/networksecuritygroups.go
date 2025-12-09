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

package networksecuritygroups

import (
	"context"

	"github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/conditionalaso"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"
)

const (
	serviceName = "networksecuritygroups"
	// NetworkSecurityGroupsCondition defines the condition type for network security groups readiness.
	NetworkSecurityGroupsCondition clusterv1.ConditionType = "NetworkSecurityGroupsReady"
)

// NetworkSecurityGroupScope defines the scope interface for a Kubernetes role assignment service.
type NetworkSecurityGroupScope interface {
	conditionalaso.Scope
	NetworkSecurityGroupSpecs() []azure.ASOResourceSpecGetter[*v1api20201101.NetworkSecurityGroup]
}

// New creates a new service.
func New(scope NetworkSecurityGroupScope) *conditionalaso.Service[*v1api20201101.NetworkSecurityGroup, NetworkSecurityGroupScope] {
	svc := conditionalaso.NewService[*v1api20201101.NetworkSecurityGroup, NetworkSecurityGroupScope](serviceName, scope)
	svc.ListFunc = list
	svc.Specs = scope.NetworkSecurityGroupSpecs()
	svc.ConditionType = NetworkSecurityGroupsCondition
	return svc
}

func list(ctx context.Context, client client.Client, opts ...client.ListOption) ([]*v1api20201101.NetworkSecurityGroup, error) {
	list := &v1api20201101.NetworkSecurityGroupList{}
	err := client.List(ctx, list, opts...)
	return slice.ToPtrs(list.Items), err
}
