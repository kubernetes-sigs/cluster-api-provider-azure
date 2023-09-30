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

package v1beta1

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capifeature "sigs.k8s.io/cluster-api/feature"
)

func TestDefaultingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing amcp defaulting webhook with no baseline")
	amcp := &AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
		},
		Spec: AzureManagedControlPlaneSpec{
			ResourceGroupName: "fooRg",
			Location:          "fooLocation",
			Version:           "1.17.5",
			SSHPublicKey:      ptr.To(""),
		},
	}
	mcpw := &azureManagedControlPlaneWebhook{}
	err := mcpw.Default(context.Background(), amcp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*amcp.Spec.NetworkPlugin).To(Equal("azure"))
	g.Expect(amcp.Spec.Version).To(Equal("v1.17.5"))
	g.Expect(*amcp.Spec.SSHPublicKey).NotTo(BeEmpty())
	g.Expect(amcp.Spec.NodeResourceGroupName).To(Equal("MC_fooRg_fooName_fooLocation"))
	g.Expect(amcp.Spec.VirtualNetwork.Name).To(Equal("fooName"))
	g.Expect(amcp.Spec.VirtualNetwork.CIDRBlock).To(Equal(defaultAKSVnetCIDR))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooName"))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.CIDRBlock).To(Equal(defaultAKSNodeSubnetCIDR))
	g.Expect(amcp.Spec.SKU.Tier).To(Equal(FreeManagedControlPlaneTier))
	g.Expect(amcp.Spec.Identity.Type).To(Equal(ManagedControlPlaneIdentityTypeSystemAssigned))
	g.Expect(*amcp.Spec.OIDCIssuerProfile.Enabled).To(BeFalse())
	g.Expect(amcp.Spec.DNSPrefix).ToNot(BeNil())
	g.Expect(*amcp.Spec.DNSPrefix).To(Equal(amcp.Name))

	t.Logf("Testing amcp defaulting webhook with baseline")
	netPlug := "kubenet"
	netPol := "azure"
	amcp.Spec.NetworkPlugin = &netPlug
	amcp.Spec.NetworkPolicy = &netPol
	amcp.Spec.Version = "9.99.99"
	amcp.Spec.SSHPublicKey = nil
	amcp.Spec.NodeResourceGroupName = "fooNodeRg"
	amcp.Spec.VirtualNetwork.Name = "fooVnetName"
	amcp.Spec.VirtualNetwork.Subnet.Name = "fooSubnetName"
	amcp.Spec.SKU.Tier = PaidManagedControlPlaneTier
	amcp.Spec.OIDCIssuerProfile = &OIDCIssuerProfile{
		Enabled: ptr.To(true),
	}
	amcp.Spec.DNSPrefix = ptr.To("test-prefix")

	err = mcpw.Default(context.Background(), amcp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*amcp.Spec.NetworkPlugin).To(Equal(netPlug))
	g.Expect(*amcp.Spec.NetworkPolicy).To(Equal(netPol))
	g.Expect(amcp.Spec.Version).To(Equal("v9.99.99"))
	g.Expect(amcp.Spec.SSHPublicKey).To(BeNil())
	g.Expect(amcp.Spec.NodeResourceGroupName).To(Equal("fooNodeRg"))
	g.Expect(amcp.Spec.VirtualNetwork.Name).To(Equal("fooVnetName"))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooSubnetName"))
	g.Expect(amcp.Spec.SKU.Tier).To(Equal(StandardManagedControlPlaneTier))
	g.Expect(*amcp.Spec.OIDCIssuerProfile.Enabled).To(BeTrue())
	g.Expect(amcp.Spec.DNSPrefix).ToNot(BeNil())
	g.Expect(*amcp.Spec.DNSPrefix).To(Equal("test-prefix"))
	t.Logf("Testing amcp defaulting webhook with overlay")
	amcp = &AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
		},
		Spec: AzureManagedControlPlaneSpec{
			ResourceGroupName: "fooRg",
			Location:          "fooLocation",
			Version:           "1.17.5",
			SSHPublicKey:      ptr.To(""),
			NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
		},
	}
	err = mcpw.Default(context.Background(), amcp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(amcp.Spec.VirtualNetwork.CIDRBlock).To(Equal(defaultAKSVnetCIDRForOverlay))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.CIDRBlock).To(Equal(defaultAKSNodeSubnetCIDRForOverlay))
}

func TestValidatingWebhook(t *testing.T) {
	// NOTE: AzureManageControlPlane is behind AKS feature gate flag; the webhook
	// must prevent creating new objects in case the feature flag is disabled.
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()

	tests := []struct {
		name      string
		amcp      AzureManagedControlPlane
		expectErr bool
	}{
		{
			name: "Testing valid DNSServiceIP",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.17.8",
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid DNSServiceIP",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10.3"),
					Version:      "v1.17.8",
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid DNSServiceIP",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.11"),
					Version:      "v1.17.8",
				},
			},
			expectErr: true,
		},
		{
			name: "Testing empty DNSServiceIP",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.17.8",
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid Version",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "honk",
				},
			},
			expectErr: true,
		},
		{
			name: "not following the Kubernetes Version pattern",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "1.19.0",
				},
			},
			expectErr: true,
		},
		{
			name: "Version not set",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "",
				},
			},
			expectErr: true,
		},
		{
			name: "Valid Version",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.17.8",
				},
			},
			expectErr: false,
		},
		{
			name: "Valid Managed AADProfile",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Valid LoadBalancerProfile",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						ManagedOutboundIPs:     ptr.To(10),
						AllocatedOutboundPorts: ptr.To(1000),
						IdleTimeoutInMinutes:   ptr.To(60),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid LoadBalancerProfile.ManagedOutboundIPs",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						ManagedOutboundIPs: ptr.To(200),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid LoadBalancerProfile.AllocatedOutboundPorts",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						AllocatedOutboundPorts: ptr.To(80000),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid LoadBalancerProfile.IdleTimeoutInMinutes",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						IdleTimeoutInMinutes: ptr.To(600),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "LoadBalancerProfile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						ManagedOutboundIPs: ptr.To(1),
						OutboundIPs: []string{
							"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/foo-bar/providers/Microsoft.Network/publicIPAddresses/my-public-ip",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid CIDR for AuthorizedIPRanges",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					APIServerAccessProfile: &APIServerAccessProfile{
						AuthorizedIPRanges: []string{"1.2.3.400/32"},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing valid AutoScalerProfile",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						BalanceSimilarNodeGroups:      (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsFalse))),
						Expander:                      (*Expander)(ptr.To(string(ExpanderRandom))),
						MaxEmptyBulkDelete:            ptr.To("10"),
						MaxGracefulTerminationSec:     ptr.To("600"),
						MaxNodeProvisionTime:          ptr.To("10m"),
						MaxTotalUnreadyPercentage:     ptr.To("45"),
						NewPodScaleUpDelay:            ptr.To("10m"),
						OkTotalUnreadyCount:           ptr.To("3"),
						ScanInterval:                  ptr.To("60s"),
						ScaleDownDelayAfterAdd:        ptr.To("10m"),
						ScaleDownDelayAfterDelete:     ptr.To("10s"),
						ScaleDownDelayAfterFailure:    ptr.To("10m"),
						ScaleDownUnneededTime:         ptr.To("10m"),
						ScaleDownUnreadyTime:          ptr.To("10m"),
						ScaleDownUtilizationThreshold: ptr.To("0.5"),
						SkipNodesWithLocalStorage:     (*SkipNodesWithLocalStorage)(ptr.To(string(SkipNodesWithLocalStorageTrue))),
						SkipNodesWithSystemPods:       (*SkipNodesWithSystemPods)(ptr.To(string(SkipNodesWithSystemPodsTrue))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderRandom",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						Expander: (*Expander)(ptr.To(string(ExpanderRandom))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderLeastWaste",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						Expander: (*Expander)(ptr.To(string(ExpanderLeastWaste))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderMostPods",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						Expander: (*Expander)(ptr.To(string(ExpanderMostPods))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderPriority",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						Expander: (*Expander)(ptr.To(string(ExpanderPriority))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsTrue",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						BalanceSimilarNodeGroups: (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsTrue))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsFalse",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						BalanceSimilarNodeGroups: (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsFalse))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxEmptyBulkDelete",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						MaxEmptyBulkDelete: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxGracefulTerminationSec",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						MaxGracefulTerminationSec: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxNodeProvisionTime",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						MaxNodeProvisionTime: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxTotalUnreadyPercentage",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						MaxTotalUnreadyPercentage: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.NewPodScaleUpDelay",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						NewPodScaleUpDelay: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.OkTotalUnreadyCount",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						OkTotalUnreadyCount: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScanInterval",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScanInterval: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterAdd",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownDelayAfterAdd: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterDelete",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownDelayAfterDelete: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterFailure",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownDelayAfterFailure: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnneededTime",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownUnneededTime: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnreadyTime",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownUnreadyTime: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUtilizationThreshold",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownUtilizationThreshold: ptr.To("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageTrue",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						SkipNodesWithLocalStorage: (*SkipNodesWithLocalStorage)(ptr.To(string(SkipNodesWithLocalStorageTrue))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageFalse",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						SkipNodesWithLocalStorage: (*SkipNodesWithLocalStorage)(ptr.To(string(SkipNodesWithLocalStorageFalse))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithSystemPodsTrue",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						SkipNodesWithSystemPods: (*SkipNodesWithSystemPods)(ptr.To(string(SkipNodesWithSystemPodsTrue))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithSystemPodsFalse",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						SkipNodesWithSystemPods: (*SkipNodesWithSystemPods)(ptr.To(string(SkipNodesWithSystemPodsFalse))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid Identity: SystemAssigned",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					Identity: &Identity{
						Type: ManagedControlPlaneIdentityTypeSystemAssigned,
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid Identity: UserAssigned",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					Identity: &Identity{
						Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
						UserAssignedIdentityResourceID: "/resource/id",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid Identity: SystemAssigned with UserAssigned values",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					Identity: &Identity{
						Type:                           ManagedControlPlaneIdentityTypeSystemAssigned,
						UserAssignedIdentityResourceID: "/resource/id",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid Identity: UserAssigned with missing properties",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					Identity: &Identity{
						Type: ManagedControlPlaneIdentityTypeUserAssigned,
					},
				},
			},
			expectErr: true,
		},
		{
			name: "overlay cannot be used with kubenet",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version:           "v1.24.1",
					NetworkPlugin:     ptr.To("kubenet"),
					NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
				},
			},
			expectErr: true,
		},
		{
			name: "overlay can be used with azure",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					Version:           "v1.24.1",
					NetworkPlugin:     ptr.To("azure"),
					NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		// client is used to fetch the AzureManagedControlPlane, we do not want to return an error on client.Get
		client := mockClient{ReturnError: false}
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			mcpw := &azureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateCreate(context.Background(), &tt.amcp)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureManagedControlPlane_ValidateCreate(t *testing.T) {
	// NOTE: AzureManageControlPlane is behind AKS feature gate flag; the webhook
	// must prevent creating new objects in case the feature flag is disabled.
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()

	tests := []struct {
		name     string
		amcp     *AzureManagedControlPlane
		wantErr  bool
		errorLen int
	}{
		{
			name:    "all valid",
			amcp:    getKnownValidAzureManagedControlPlane(),
			wantErr: false,
		},
		{
			name:     "invalid DNSServiceIP",
			amcp:     createAzureManagedControlPlane("192.168.0.10.3", "v1.18.0", generateSSHPublicKey(true)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name:     "invalid DNSServiceIP",
			amcp:     createAzureManagedControlPlane("192.168.0.11", "v1.18.0", generateSSHPublicKey(true)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name:     "invalid sshKey",
			amcp:     createAzureManagedControlPlane("192.168.0.10", "v1.18.0", generateSSHPublicKey(false)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name:     "invalid sshKey with a simple text and invalid DNSServiceIP",
			amcp:     createAzureManagedControlPlane("192.168.0.10.3", "v1.18.0", "invalid_sshkey_honk"),
			wantErr:  true,
			errorLen: 2,
		},
		{
			name:     "invalid version",
			amcp:     createAzureManagedControlPlane("192.168.0.10", "honk.version", generateSSHPublicKey(true)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "Testing inValid DNSPrefix for starting with invalid characters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("-thisi$"),
					Version:   "v1.17.8",
				},
			},
			wantErr: true,
		},
		{
			name: "Testing inValid DNSPrefix with more then 54 characters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("thisisaverylong$^clusternameconsistingofmorethan54characterswhichshouldbeinvalid"),
					Version:   "v1.17.8",
				},
			},
			wantErr: true,
		},
		{
			name: "Testing inValid DNSPrefix with underscore",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("no_underscore"),
					Version:   "v1.17.8",
				},
			},
			wantErr: true,
		},
		{
			name: "Testing inValid DNSPrefix with special characters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("no-dollar$@%"),
					Version:   "v1.17.8",
				},
			},
			wantErr: true,
		},
		{
			name: "Testing Valid DNSPrefix with hyphen characters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("hyphen-allowed"),
					Version:   "v1.17.8",
				},
			},
			wantErr: false,
		},
		{
			name: "Testing Valid DNSPrefix with hyphen characters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("palette-test07"),
					Version:   "v1.17.8",
				},
			},
			wantErr: false,
		},
		{
			name: "Testing valid DNSPrefix ",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("thisisavlerylongclu7l0sternam3leconsistingofmorethan54"),
					Version:   "v1.17.8",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid name with microsoft",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "microsoft-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.23.5",
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "invalid name with windows",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "a-windows-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.23.5",
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "set Spec.ControlPlaneEndpoint.Host during create (clusterctl move scenario)",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "my-host",
					},
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "can set Spec.ControlPlaneEndpoint.Port during create (clusterctl move scenario)",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Port: 444,
					},
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "DisableLocalAccounts cannot be set for non AAD clusters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version:              "v1.21.2",
					DisableLocalAccounts: ptr.To[bool](true),
				},
			},
			wantErr: true,
		},
		{
			name: "DisableLocalAccounts can be set for AAD clusters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					AADProfile: &AADProfile{
						Managed:             true,
						AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
					},
					DisableLocalAccounts: ptr.To[bool](true),
				},
			},
			wantErr: false,
		},
	}
	client := mockClient{ReturnError: false}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mcpw := &azureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateCreate(context.Background(), tc.amcp)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tc.errorLen > 0 {
					g.Expect(err).To(HaveLen(tc.errorLen))
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureManagedControlPlane_ValidateCreateFailure(t *testing.T) {
	tests := []struct {
		name      string
		amcp      *AzureManagedControlPlane
		deferFunc func()
	}{
		{
			name:      "feature gate explicitly disabled",
			amcp:      getKnownValidAzureManagedControlPlane(),
			deferFunc: utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, false),
		},
		{
			name:      "feature gate implicitly disabled",
			amcp:      getKnownValidAzureManagedControlPlane(),
			deferFunc: func() {},
		},
	}
	client := mockClient{ReturnError: false}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.deferFunc()
			g := NewWithT(t)
			mcpw := &azureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateCreate(context.Background(), tc.amcp)
			g.Expect(err).To(HaveOccurred())
		})
	}
}

func TestAzureManagedControlPlane_ValidateUpdate(t *testing.T) {
	commonSSHKey := generateSSHPublicKey(true)
	tests := []struct {
		name    string
		oldAMCP *AzureManagedControlPlane
		amcp    *AzureManagedControlPlane
		wantErr bool
	}{
		{
			name:    "can't add a SSHPublicKey to an existing AzureManagedControlPlane",
			oldAMCP: createAzureManagedControlPlane("192.168.0.10", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane("192.168.0.10", "v1.18.0", generateSSHPublicKey(true)),
			wantErr: true,
		},
		{
			name:    "same SSHPublicKey is valid",
			oldAMCP: createAzureManagedControlPlane("192.168.0.10", "v1.18.0", commonSSHKey),
			amcp:    createAzureManagedControlPlane("192.168.0.10", "v1.18.0", commonSSHKey),
			wantErr: false,
		},
		{
			name:    "AzureManagedControlPlane with invalid serviceIP",
			oldAMCP: createAzureManagedControlPlane("", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane("192.168.0.10.3", "v1.18.0", generateSSHPublicKey(true)),
			wantErr: true,
		},
		{
			name:    "AzureManagedControlPlane with invalid serviceIP",
			oldAMCP: createAzureManagedControlPlane("", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane("192.168.0.11", "v1.18.0", generateSSHPublicKey(true)),
			wantErr: true,
		},
		{
			name:    "AzureManagedControlPlane with invalid version",
			oldAMCP: createAzureManagedControlPlane("", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane("192.168.0.10", "1.999.9", generateSSHPublicKey(true)),
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane AddonProfiles is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AddonProfiles: []AddonProfile{
						{
							Name:    "first-addon-profile",
							Enabled: true,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane AddonProfiles can be disabled",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AddonProfiles: []AddonProfile{
						{
							Name:    "first-addon-profile",
							Enabled: true,
						},
					},
					Version: "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AddonProfiles: []AddonProfile{
						{
							Name:    "first-addon-profile",
							Enabled: false,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane AddonProfiles cannot update to empty array",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AddonProfiles: []AddonProfile{
						{
							Name:    "first-addon-profile",
							Enabled: true,
						},
					},
					Version: "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane AddonProfiles cannot be completely removed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AddonProfiles: []AddonProfile{
						{
							Name:    "first-addon-profile",
							Enabled: true,
						},
						{
							Name:    "second-addon-profile",
							Enabled: true,
						},
					},
					Version: "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AddonProfiles: []AddonProfile{
						{
							Name:    "first-addon-profile",
							Enabled: true,
						},
					},
					Version: "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane SubscriptionID is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:   ptr.To("192.168.0.10"),
					SubscriptionID: "212ec1q8",
					Version:        "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:   ptr.To("192.168.0.10"),
					SubscriptionID: "212ec1q9",
					Version:        "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane ResourceGroupName is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:      ptr.To("192.168.0.10"),
					ResourceGroupName: "hello-1",
					Version:           "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:      ptr.To("192.168.0.10"),
					ResourceGroupName: "hello-2",
					Version:           "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NodeResourceGroupName is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:          ptr.To("192.168.0.10"),
					NodeResourceGroupName: "hello-1",
					Version:               "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:          ptr.To("192.168.0.10"),
					NodeResourceGroupName: "hello-2",
					Version:               "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane Location is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Location:     "westeurope",
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Location:     "eastus",
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane SSHPublicKey is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSServiceIP is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.1.1"),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSServiceIP is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPlugin is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  ptr.To("192.168.0.10"),
					NetworkPlugin: ptr.To("azure"),
					Version:       "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  ptr.To("192.168.0.10"),
					NetworkPlugin: ptr.To("kubenet"),
					Version:       "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPlugin is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  ptr.To("192.168.0.10"),
					NetworkPlugin: ptr.To("azure"),
					Version:       "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPolicy is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  ptr.To("192.168.0.10"),
					NetworkPolicy: ptr.To("azure"),
					Version:       "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  ptr.To("192.168.0.10"),
					NetworkPolicy: ptr.To("calico"),
					Version:       "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPolicy is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  ptr.To("192.168.0.10"),
					NetworkPolicy: ptr.To("azure"),
					Version:       "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane LoadBalancerSKU is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:    ptr.To("192.168.0.10"),
					LoadBalancerSKU: ptr.To(LoadBalancerSKUStandard),
					Version:         "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:    ptr.To("192.168.0.10"),
					LoadBalancerSKU: ptr.To(LoadBalancerSKUBasic),
					Version:         "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane LoadBalancerSKU is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:    ptr.To("192.168.0.10"),
					LoadBalancerSKU: ptr.To(LoadBalancerSKUStandard),
					Version:         "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane ManagedAad can be set after cluster creation",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane ManagedAad cannot be disabled",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version:    "v1.18.0",
					AADProfile: &AADProfile{},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane managed field cannot set to false",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed: false,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane adminGroupObjectIDs cannot set to empty",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed:             true,
						AdminGroupObjectIDs: []string{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane ManagedAad cannot be disabled",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane EnablePrivateCluster is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
					APIServerAccessProfile: &APIServerAccessProfile{
						EnablePrivateCluster: ptr.To(true),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane AuthorizedIPRanges is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
					APIServerAccessProfile: &APIServerAccessProfile{
						AuthorizedIPRanges: []string{"192.168.0.1/32"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane.VirtualNetwork Name is mutable",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
					VirtualNetwork: ManagedControlPlaneVirtualNetwork{
						Name:          "test-network",
						CIDRBlock:     "10.0.0.0/8",
						ResourceGroup: "test-rg",
						Subnet: ManagedControlPlaneSubnet{
							Name:      "test-subnet",
							CIDRBlock: "10.0.2.0/24",
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane.VirtualNetwork Name is mutable",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
					VirtualNetwork: ManagedControlPlaneVirtualNetwork{
						Name:          "test-network",
						CIDRBlock:     "10.0.0.0/8",
						ResourceGroup: "test-rg",
						Subnet: ManagedControlPlaneSubnet{
							Name:      "test-subnet",
							CIDRBlock: "10.0.2.0/24",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane.VirtualNetwork Name is mutable",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
					VirtualNetwork: ManagedControlPlaneVirtualNetwork{
						Name:          "test-network",
						CIDRBlock:     "10.0.0.0/8",
						ResourceGroup: "test-rg",
						Subnet: ManagedControlPlaneSubnet{
							Name:      "test-subnet",
							CIDRBlock: "10.0.2.0/24",
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: ptr.To("192.168.0.10"),
					Version:      "v1.18.0",
					VirtualNetwork: ManagedControlPlaneVirtualNetwork{
						Name:          "test-network",
						CIDRBlock:     "10.0.0.0/8",
						ResourceGroup: "test-rg",
						Subnet: ManagedControlPlaneSubnet{
							Name:      "test-subnet",
							CIDRBlock: "10.0.2.0/24",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "OutboundType update",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					OutboundType: (*ManagedControlPlaneOutboundType)(ptr.To(string(ManagedControlPlaneOutboundTypeUserDefinedRouting))),
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					OutboundType: (*ManagedControlPlaneOutboundType)(ptr.To(string(ManagedControlPlaneOutboundTypeLoadBalancer))),
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane HTTPProxyConfig is immutable",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					HTTPProxyConfig: &HTTPProxyConfig{
						HTTPProxy:  ptr.To("http://1.2.3.4:8080"),
						HTTPSProxy: ptr.To("https://5.6.7.8:8443"),
						NoProxy:    []string{"endpoint1", "endpoint2"},
						TrustedCA:  ptr.To("ca"),
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					HTTPProxyConfig: &HTTPProxyConfig{
						HTTPProxy:  ptr.To("http://10.20.3.4:8080"),
						HTTPSProxy: ptr.To("https://5.6.7.8:8443"),
						NoProxy:    []string{"endpoint1", "endpoint2"},
						TrustedCA:  ptr.To("ca"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "NetworkPluginMode cannot change to \"overlay\" when NetworkPolicy is set",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					NetworkPolicy:     ptr.To("anything"),
					NetworkPluginMode: nil,
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					NetworkPolicy:     ptr.To("anything"),
					NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
				},
			},
			wantErr: true,
		},
		{
			name: "NetworkPluginMode can change to \"overlay\" when NetworkPolicy is not set",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					NetworkPolicy:     nil,
					NetworkPluginMode: nil,
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
					Version:           "v0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "NetworkPolicy is allowed when NetworkPluginMode is not changed",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					NetworkPolicy:     ptr.To("anything"),
					NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					NetworkPolicy:     ptr.To("anything"),
					NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
					Version:           "v0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane OIDCIssuerProfile.Enabled false -> false OK",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					OIDCIssuerProfile: &OIDCIssuerProfile{
						Enabled: ptr.To(false),
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version: "v0.0.0",
					OIDCIssuerProfile: &OIDCIssuerProfile{
						Enabled: ptr.To(false),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane OIDCIssuerProfile.Enabled false -> true OK",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					OIDCIssuerProfile: &OIDCIssuerProfile{
						Enabled: ptr.To(false),
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version: "v0.0.0",
					OIDCIssuerProfile: &OIDCIssuerProfile{
						Enabled: ptr.To(true),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane OIDCIssuerProfile.Enabled true -> false err",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					OIDCIssuerProfile: &OIDCIssuerProfile{
						Enabled: ptr.To(true),
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version: "v0.0.0",
					OIDCIssuerProfile: &OIDCIssuerProfile{
						Enabled: ptr.To(false),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane OIDCIssuerProfile.Enabled true -> true OK",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					OIDCIssuerProfile: &OIDCIssuerProfile{
						Enabled: ptr.To(true),
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version: "v0.0.0",
					OIDCIssuerProfile: &OIDCIssuerProfile{
						Enabled: ptr.To(true),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable error",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks-1"),
					Version:   "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					Version:   "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable no error",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					Version:   "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					Version:   "v1.18.0",
				},
			},
			wantErr: false,
		},
		{
			name: "DisableLocalAccounts can be set only for AAD enabled clusters",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed:             true,
						AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version:              "v1.18.0",
					DisableLocalAccounts: ptr.To[bool](true),
					AADProfile: &AADProfile{
						Managed:             true,
						AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "DisableLocalAccounts cannot be disabled",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed:             true,
						AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
					},
					DisableLocalAccounts: ptr.To[bool](true),
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed:             true,
						AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "DisableLocalAccounts cannot be disabled",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
					AADProfile: &AADProfile{
						Managed:             true,
						AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
					},
					DisableLocalAccounts: ptr.To[bool](true),
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version:              "v1.18.0",
					DisableLocalAccounts: ptr.To[bool](false),
					AADProfile: &AADProfile{
						Managed:             true,
						AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable error nil -> capz-aks",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					Version:   "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					Version:   "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "DisableLocalAccounts cannot be set for non AAD clusters",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					Version:              "v1.18.0",
					DisableLocalAccounts: ptr.To[bool](true),
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable error nil -> empty",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					Version:   "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To(""),
					Version:   "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable no error nil -> nil",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					Version:   "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					Version:   "v1.18.0",
				},
			},
			wantErr: false,
		},
	}
	client := mockClient{ReturnError: false}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mcpw := &azureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateUpdate(context.Background(), tc.oldAMCP, tc.amcp)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createAzureManagedControlPlane(serviceIP, version, sshKey string) *AzureManagedControlPlane {
	return &AzureManagedControlPlane{
		ObjectMeta: getAMCPMetaData(),
		Spec: AzureManagedControlPlaneSpec{
			SSHPublicKey: &sshKey,
			DNSServiceIP: ptr.To(serviceIP),
			Version:      version,
		},
	}
}

func getKnownValidAzureManagedControlPlane() *AzureManagedControlPlane {
	return &AzureManagedControlPlane{
		ObjectMeta: getAMCPMetaData(),
		Spec: AzureManagedControlPlaneSpec{
			DNSServiceIP: ptr.To("192.168.0.10"),
			Version:      "v1.18.0",
			SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
			AADProfile: &AADProfile{
				Managed: true,
				AdminGroupObjectIDs: []string{
					"616077a8-5db7-4c98-b856-b34619afg75h",
				},
			},
		},
	}
}

func getAMCPMetaData() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: "test-AMCP",
		Labels: map[string]string{
			"cluster.x-k8s.io/cluster-name": "test-cluster",
		},
		Namespace: "default",
	}
}
