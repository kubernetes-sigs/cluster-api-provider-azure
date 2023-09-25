/*
Copyright 2021 The Kubernetes Authors.

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

package natgateways

import (
	"context"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// NatGatewaySpec defines the specification for a NAT gateway.
type NatGatewaySpec struct {
	Name           string
	Namespace      string
	ResourceGroup  string
	SubscriptionID string
	Location       string
	NatGatewayIP   infrav1.PublicIPSpec
	ClusterName    string
	AdditionalTags infrav1.Tags
}

// ResourceRef implements aso.ResourceSpecGetter.
func (s *NatGatewaySpec) ResourceRef() *asonetworkv1.NatGateway {
	return &asonetworkv1.NatGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
		},
	}
}

// Parameters returns the parameters for the NAT gateway.
func (s *NatGatewaySpec) Parameters(ctx context.Context, existing *asonetworkv1.NatGateway) (*asonetworkv1.NatGateway, error) {
	_, log, done := tele.StartSpanWithLogger(ctx, "natgateways.Service.Parameters")
	defer done()

	natGatewayToCreate := &asonetworkv1.NatGateway{}
	if existing != nil {
		if hasPublicIP(existing, s.NatGatewayIP.Name) {
			// Skip update for NAT gateway as it exists with expected values
			log.V(4).Info("nat gateway already exists", "PublicIP Name", s.NatGatewayIP.Name)
			return existing, nil
		}
		natGatewayToCreate = existing
	}

	natGatewayToCreate.Spec = asonetworkv1.NatGateway_Spec{
		Location: ptr.To(s.Location),
		Sku: &asonetworkv1.NatGatewaySku{
			Name: ptr.To(asonetworkv1.NatGatewaySku_Name_Standard),
		},
		PublicIpAddresses: []asonetworkv1.ApplicationGatewaySubResource{
			{
				Reference: &genruntime.ResourceReference{
					ARMID: azure.PublicIPID(s.SubscriptionID, s.ResourceGroup, s.NatGatewayIP.Name),
				},
			},
		},
		Tags: infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        ptr.To(s.Name),
			Additional:  s.AdditionalTags,
		}),
	}

	return natGatewayToCreate, nil
}

func hasPublicIP(natGateway *asonetworkv1.NatGateway, publicIPName string) bool {
	for _, publicIP := range natGateway.Status.PublicIpAddresses {
		if publicIP.Id != nil {
			resource, err := azureutil.ParseResourceID(*publicIP.Id)
			if err != nil {
				continue
			}
			if resource.Name == publicIPName {
				return true
			}
		}
	}
	return false
}

// WasManaged implements azure.ASOResourceSpecGetter.
func (s *NatGatewaySpec) WasManaged(resource *asonetworkv1.NatGateway) bool {
	// TODO: Earlier implementation of IsManaged(with ASO's framework is morphed into WasManaged) was checking for s.Scope.IsVnetManaged()
	// we would want to update this to check for the same once we have the support for VnetManaged in ASO
	// we return true for now to manage the nat gateway via CAPZ
	return true
}

var _ aso.TagsGetterSetter[*asonetworkv1.NatGateway] = (*NatGatewaySpec)(nil)

// GetAdditionalTags implements aso.TagsGetterSetter.
func (s *NatGatewaySpec) GetAdditionalTags() infrav1.Tags {
	return s.AdditionalTags
}

// GetDesiredTags implements aso.TagsGetterSetter.
func (*NatGatewaySpec) GetDesiredTags(resource *asonetworkv1.NatGateway) infrav1.Tags {
	return resource.Spec.Tags
}

// GetActualTags implements aso.TagsGetterSetter.
func (*NatGatewaySpec) GetActualTags(resource *asonetworkv1.NatGateway) infrav1.Tags {
	return resource.Status.Tags
}

// SetTags implements aso.TagsGetterSetter.
func (*NatGatewaySpec) SetTags(resource *asonetworkv1.NatGateway, tags infrav1.Tags) {
	resource.Spec.Tags = tags
}
