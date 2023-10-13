/*
Copyright 2023 The Kubernetes Authors.

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
	"testing"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	fakeNatGatewaySpec = &NatGatewaySpec{
		Name:           "my-natgateway",
		Namespace:      "dummy-ns",
		ResourceGroup:  "my-rg",
		SubscriptionID: "123",
		Location:       "eastus",
		NatGatewayIP: infrav1.PublicIPSpec{
			Name:    "my-natgateway-ip",
			DNSName: "Standard",
		},
		ClusterName:    "my-cluster",
		IsVnetManaged:  true,
		AdditionalTags: infrav1.Tags{},
	}
	locationPtr        = ptr.To("eastus")
	standardSKUPtr     = ptr.To(asonetworkv1.NatGatewaySku_Name_Standard)
	existingNatGateway = &asonetworkv1.NatGateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "network.azure.com/v1api20220701",
			Kind:       "NatGateway",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-natgateway",
			Namespace: "dummy-ns",
		},
		Spec: asonetworkv1.NatGateway_Spec{
			AzureName:            "my-natgateway",
			IdleTimeoutInMinutes: ptr.To(6),
			Location:             locationPtr,
			Owner: &genruntime.KnownResourceReference{
				Name: "my-rg",
			},
			PublicIpAddresses: []asonetworkv1.ApplicationGatewaySubResource{
				{
					Reference: &genruntime.ResourceReference{
						ARMID: "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/my-natgateway-ip",
					},
				},
			},
			Sku: &asonetworkv1.NatGatewaySku{
				Name: ptr.To(asonetworkv1.NatGatewaySku_Name_Standard),
			},
			Tags: map[string]string{
				"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": "owned",
				"Name": "my-natgateway",
			},
		},
		Status: asonetworkv1.NatGateway_STATUS{
			Id:                   ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/natGateways/test-1111-node-natgw-1"),
			IdleTimeoutInMinutes: ptr.To(4),
			Location:             locationPtr,
			Name:                 ptr.To("my-natgateway"),
			ProvisioningState:    ptr.To(asonetworkv1.ApplicationGatewayProvisioningState_STATUS_Succeeded),
			PublicIpAddresses: []asonetworkv1.ApplicationGatewaySubResource_STATUS{
				{
					Id: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/my-natgateway-ip"),
				},
			},
			Sku: &asonetworkv1.NatGatewaySku_STATUS{
				Name: ptr.To(asonetworkv1.NatGatewaySku_Name_STATUS_Standard),
			},
		},
	}
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name         string
		spec         *NatGatewaySpec
		existingSpec *asonetworkv1.NatGateway
		expect       func(g *WithT, existing *asonetworkv1.NatGateway, parameters *asonetworkv1.NatGateway)
	}{
		{
			name:         "create a new NAT Gateway spec when existing aso resource is nil",
			spec:         fakeNatGatewaySpec,
			existingSpec: nil,
			expect: func(g *WithT, existing *asonetworkv1.NatGateway, parameters *asonetworkv1.NatGateway) {
				g.Expect(parameters).NotTo(BeNil())
				g.Expect(parameters.Spec.AzureName).NotTo(BeNil())
				g.Expect(parameters.Spec.AzureName).To(Equal("my-natgateway"))
				g.Expect(parameters.Spec.Owner).NotTo(BeNil())
				g.Expect(parameters.Spec.Owner.Name).To(Equal("my-rg"))
				g.Expect(parameters.Spec.Location).NotTo(BeNil())
				g.Expect(parameters.Spec.Location).To(Equal(locationPtr))
				g.Expect(parameters.Spec.Sku.Name).NotTo(BeNil())
				g.Expect(parameters.Spec.Sku.Name).To(Equal(standardSKUPtr))
				g.Expect(parameters.Spec.PublicIpAddresses).To(HaveLen(1))
				g.Expect(parameters.Spec.PublicIpAddresses[0].Reference.ARMID).To(Equal("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/my-natgateway-ip"))
				g.Expect(parameters.Spec.Tags).NotTo(BeEmpty())
				g.Expect(parameters.Spec.Tags).To(HaveKeyWithValue("Name", "my-natgateway"))
				g.Expect(parameters.Spec.Tags).To(HaveKeyWithValue("sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster", "owned"))
			},
		},
		{
			name:         "reconcile a NAT Gateway spec when there is an existing aso resource. User added extra spec fields",
			spec:         fakeNatGatewaySpec,
			existingSpec: existingNatGateway,
			expect: func(g *WithT, existing *asonetworkv1.NatGateway, parameters *asonetworkv1.NatGateway) {
				diff := cmp.Diff(existing, parameters)
				g.Expect(diff).To(BeEmpty())
			},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, _ := tc.spec.Parameters(context.TODO(), tc.existingSpec.DeepCopy())
			tc.expect(g, tc.existingSpec, result)
		})
	}
}
