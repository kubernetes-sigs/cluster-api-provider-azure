/*
Copyright The Kubernetes Authors.

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

package privatelinks

import (
	"fmt"
	"testing"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

const (
	fakeRegion            = "westeurope"
	fakeSubscriptionID    = "abcd"
	fakeClusterName       = "my-cluster"
	fakeVNetResourceGroup = fakeClusterName
	fakeVNetName          = fakeClusterName + "-vnet"
	fakeSubnetName        = fakeClusterName + "-node-subnet"
	fakeLbName            = fakeClusterName + "-internal-lb"
	fakeLbIPConfigName    = fakeLbName + "-frontend1"
	fakePrivateLinkName   = "apiserver-privatelink"
)

var (
	fakePrivateLinkSpec = PrivateLinkSpec{
		Name:              fakePrivateLinkName,
		ResourceGroup:     fakeClusterName,
		SubscriptionID:    fakeSubscriptionID,
		Location:          fakeRegion,
		VNetResourceGroup: fakeVNetResourceGroup,
		VNet:              fakeVNetName,
		NATIPConfiguration: []NATIPConfiguration{
			{
				AllocationMethod: infrav1.NATIPAllocationMethodDynamic,
				Subnet:           fakeSubnetName,
			},
		},
		LoadBalancerName: fakeLbName,
		LBFrontendIPConfigNames: []string{
			fakeLbIPConfigName,
		},
		AllowedSubscriptions: []string{
			fakeSubscriptionID,
		},
		AutoApprovedSubscriptions: []string{
			fakeSubscriptionID,
		},
		EnableProxyProtocol: ptr.To(true),
		ClusterName:         fakeClusterName,
		AdditionalTags: map[string]string{
			"hello": "capz",
		},
	}

	fakePrivateLinkSpecWithoutSubscriptions = PrivateLinkSpec{
		Name:              fakePrivateLinkName,
		ResourceGroup:     fakeClusterName,
		SubscriptionID:    fakeSubscriptionID,
		Location:          fakeRegion,
		VNetResourceGroup: fakeVNetResourceGroup,
		VNet:              fakeVNetName,
		NATIPConfiguration: []NATIPConfiguration{
			{
				AllocationMethod: infrav1.NATIPAllocationMethodDynamic,
				Subnet:           fakeSubnetName,
			},
		},
		LoadBalancerName: fakeLbName,
		LBFrontendIPConfigNames: []string{
			fakeLbIPConfigName,
		},
		EnableProxyProtocol: ptr.To(true),
		ClusterName:         fakeClusterName,
		AdditionalTags: map[string]string{
			"hello": "capz",
		},
	}

	// fakePrivateLink is Azure PrivateLinkService that corresponds to fakePrivateLinkSpec.
	fakePrivateLink = asonetworkv1.PrivateLinkService{
		Spec: asonetworkv1.PrivateLinkService_Spec{
			AzureName: fakePrivateLinkName,
			Location:  ptr.To(fakeRegion),
			IpConfigurations: []asonetworkv1.PrivateLinkServiceIpConfiguration{
				{
					Name: ptr.To(fmt.Sprintf("%s-natipconfig-1", fakeSubnetName)),
					Subnet: &asonetworkv1.Subnet_PrivateLinkService_SubResourceEmbedded{
						Reference: &genruntime.ResourceReference{
							ARMID: fmt.Sprintf(
								"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
								fakeSubscriptionID,
								fakeVNetResourceGroup,
								fakeVNetName,
								fakeSubnetName),
						},
					},
					PrivateIPAllocationMethod: ptr.To(asonetworkv1.IPAllocationMethod_Dynamic),
					Primary:                   ptr.To(true),
				},
			},
			LoadBalancerFrontendIpConfigurations: []asonetworkv1.FrontendIPConfiguration_PrivateLinkService_SubResourceEmbedded{
				{
					Reference: &genruntime.ResourceReference{
						ARMID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s",
							fakeSubscriptionID,
							fakeClusterName,
							fakeLbName,
							fakeLbIPConfigName),
					},
				},
			},
			Visibility: &asonetworkv1.ResourceSet{
				Subscriptions: []string{
					fakeSubscriptionID,
				},
			},
			AutoApproval: &asonetworkv1.ResourceSet{
				Subscriptions: []string{
					fakeSubscriptionID,
				},
			},
			EnableProxyProtocol: ptr.To(true),
			Tags: map[string]string{
				"sigs.k8s.io_cluster-api-provider-azure_cluster_" + fakeClusterName: "owned",
				"Name":  fakePrivateLinkName,
				"hello": "capz",
			},
			Owner: &genruntime.KnownResourceReference{
				Name: fakePrivateLinkSpec.ResourceGroup,
			},
		},
	}

	// fakePrivateLinkWithoutSubscriptions corresponds to fakePrivateLinkSpecWithoutSubscriptions
	fakePrivateLinkWithoutSubscriptions = asonetworkv1.PrivateLinkService{
		Spec: asonetworkv1.PrivateLinkService_Spec{
			AzureName: fakePrivateLinkName,
			Location:  ptr.To(fakeRegion),
			IpConfigurations: []asonetworkv1.PrivateLinkServiceIpConfiguration{
				{
					Name: ptr.To(fmt.Sprintf("%s-natipconfig-1", fakeSubnetName)),
					Subnet: &asonetworkv1.Subnet_PrivateLinkService_SubResourceEmbedded{
						Reference: &genruntime.ResourceReference{
							ARMID: fmt.Sprintf(
								"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
								fakeSubscriptionID,
								fakeVNetResourceGroup,
								fakeVNetName,
								fakeSubnetName),
						},
					},
					PrivateIPAllocationMethod: ptr.To(asonetworkv1.IPAllocationMethod_Dynamic),
					Primary:                   ptr.To(true),
				},
			},
			LoadBalancerFrontendIpConfigurations: []asonetworkv1.FrontendIPConfiguration_PrivateLinkService_SubResourceEmbedded{
				{
					Reference: &genruntime.ResourceReference{
						ARMID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s",
							fakeSubscriptionID,
							fakeClusterName,
							fakeLbName,
							fakeLbIPConfigName),
					},
				},
			},
			EnableProxyProtocol: ptr.To(true),
			Tags: map[string]string{
				"sigs.k8s.io_cluster-api-provider-azure_cluster_" + fakeClusterName: "owned",
				"Name":  fakePrivateLinkName,
				"hello": "capz",
			},
			Owner: &genruntime.KnownResourceReference{
				Name: fakePrivateLinkSpec.ResourceGroup,
			},
		},
	}
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name     string
		spec     PrivateLinkSpec
		existing *asonetworkv1.PrivateLinkService
		expected *asonetworkv1.PrivateLinkService
	}{
		{
			name:     "no existing PrivateLink",
			spec:     fakePrivateLinkSpec,
			existing: nil,
			expected: &fakePrivateLink,
		},
		{
			name: "with existing PrivateLink",
			spec: fakePrivateLinkSpec,
			existing: &asonetworkv1.PrivateLinkService{
				Status: asonetworkv1.PrivateLinkService_STATUS{
					Id: ptr.To("status is preserved"),
				},
			},
			expected: &asonetworkv1.PrivateLinkService{
				Spec: fakePrivateLink.Spec,
				Status: asonetworkv1.PrivateLinkService_STATUS{
					Id: ptr.To("status is preserved"),
				},
			},
		},
		{
			name:     "removing subscriptions from existing PrivateLink",
			spec:     fakePrivateLinkSpecWithoutSubscriptions,
			existing: &fakePrivateLink,
			expected: &fakePrivateLinkWithoutSubscriptions,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(t.Context(), tc.existing.DeepCopy())
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cmp.Diff(tc.expected, result)).To(BeEmpty())
		})
	}
}
