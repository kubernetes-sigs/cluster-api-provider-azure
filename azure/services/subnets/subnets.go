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
	"context"
	"fmt"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"
)

const serviceName = "subnets"

// SubnetScope defines the scope interface for a subnet service.
type SubnetScope interface {
	aso.Scope
	UpdateSubnetID(string, string)
	UpdateSubnetCIDRs(string, []string)
	SubnetSpecs() []azure.ASOResourceSpecGetter[*asonetworkv1.VirtualNetworksSubnet]
	Subnet(name string) infrav1.SubnetSpec
}

// New creates a new service.
func New(scope SubnetScope) *aso.Service[*asonetworkv1.VirtualNetworksSubnet, SubnetScope] {
	svc := aso.NewService[*asonetworkv1.VirtualNetworksSubnet, SubnetScope](serviceName, scope)
	svc.ListFunc = list
	svc.Specs = scope.SubnetSpecs()
	svc.ConditionType = infrav1.SubnetsReadyCondition
	svc.PostCreateOrUpdateResourceHook = postCreateOrUpdateResourceHook
	return svc
}

func postCreateOrUpdateResourceHook(_ context.Context, scope SubnetScope, subnet *asonetworkv1.VirtualNetworksSubnet, err error) error {
	if err != nil {
		return err
	}
	if subnet.Status.ProvisioningState == nil || *subnet.Status.ProvisioningState != asonetworkv1.ProvisioningState_STATUS_Succeeded {
		return nil
	}

	name := subnet.AzureName()
	// scope.UpdateSubnetID(name, ptr.Deref(subnet.Status.Id, ""))
	// scope.UpdateSubnetCIDRs(name, converters.GetSubnetAddresses(*subnet))
	actualCIDRs := converters.GetSubnetAddresses(*subnet)
	desiredCIDRs := scope.Subnet(name).CIDRBlocks
	fmt.Printf("DEBUG: Subnet Name: %s, Desired CIDRs: %v, Actual CIDRs: %v\n", name, desiredCIDRs, actualCIDRs)

	// Only update if the desired CIDRs match the actual CIDRs?
	if slicesEqual(desiredCIDRs, actualCIDRs) {
		fmt.Printf("DEBUG: Updating Subnet ID and CIDRs for Subnet: %s\n", name)
		scope.UpdateSubnetID(name, ptr.Deref(subnet.Status.Id, ""))
		scope.UpdateSubnetCIDRs(name, actualCIDRs)
	} else {
		fmt.Printf("DEBUG: CIDRs do not match for Subnet: %s. Skipping update.\n", name)
	}

	return nil
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]struct{}, len(a))
	for _, v := range a {
		aMap[v] = struct{}{}
	}
	for _, v := range b {
		if _, exists := aMap[v]; !exists {
			return false
		}
	}
	return true
}

func list(ctx context.Context, client client.Client, opts ...client.ListOption) ([]*asonetworkv1.VirtualNetworksSubnet, error) {
	list := &asonetworkv1.VirtualNetworksSubnetList{}
	err := client.List(ctx, list, opts...)
	return slice.ToPtrs(list.Items), err
}
