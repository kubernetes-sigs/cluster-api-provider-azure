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
	"k8s.io/apimachinery/pkg/util/validation/field"
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
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "fooCluster",
			},
		},
		Spec: AzureManagedControlPlaneSpec{
			AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
				Location: "fooLocation",
				Version:  "1.17.5",
				Extensions: []AKSExtension{
					{
						Name: "test-extension",
						Plan: &ExtensionPlan{
							Product:   "test-product",
							Publisher: "test-publisher",
						},
					},
				},
			},
			ResourceGroupName: "fooRg",
			SSHPublicKey:      ptr.To(""),
		},
	}
	mcpw := &azureManagedControlPlaneWebhook{}
	err := mcpw.Default(context.Background(), amcp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(amcp.Spec.NetworkPlugin).To(Equal(ptr.To(AzureNetworkPluginName)))
	g.Expect(amcp.Spec.LoadBalancerSKU).To(Equal(ptr.To("Standard")))
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
	g.Expect(amcp.Spec.DNSPrefix).NotTo(BeNil())
	g.Expect(*amcp.Spec.DNSPrefix).To(Equal(amcp.Name))
	g.Expect(amcp.Spec.Extensions[0].Plan.Name).To(Equal("fooName-test-product"))
	g.Expect(amcp.Spec.EnablePreviewFeatures).NotTo(BeNil())
	g.Expect(*amcp.Spec.EnablePreviewFeatures).To(BeFalse())

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
	amcp.Spec.FleetsMember = &FleetsMember{}
	amcp.Spec.AutoUpgradeProfile = &ManagedClusterAutoUpgradeProfile{
		UpgradeChannel: ptr.To(UpgradeChannelPatch),
	}
	amcp.Spec.SecurityProfile = &ManagedClusterSecurityProfile{
		AzureKeyVaultKms: &AzureKeyVaultKms{
			Enabled: true,
		},
		ImageCleaner: &ManagedClusterSecurityProfileImageCleaner{
			Enabled:       true,
			IntervalHours: ptr.To(48),
		},
	}
	amcp.Spec.EnablePreviewFeatures = ptr.To(true)

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
	g.Expect(amcp.Spec.DNSPrefix).NotTo(BeNil())
	g.Expect(*amcp.Spec.DNSPrefix).To(Equal("test-prefix"))
	g.Expect(amcp.Spec.FleetsMember.Name).To(Equal("fooCluster"))
	g.Expect(amcp.Spec.AutoUpgradeProfile).NotTo(BeNil())
	g.Expect(amcp.Spec.AutoUpgradeProfile.UpgradeChannel).NotTo(BeNil())
	g.Expect(*amcp.Spec.AutoUpgradeProfile.UpgradeChannel).To(Equal(UpgradeChannelPatch))
	g.Expect(amcp.Spec.SecurityProfile).NotTo(BeNil())
	g.Expect(amcp.Spec.SecurityProfile.AzureKeyVaultKms).NotTo(BeNil())
	g.Expect(amcp.Spec.SecurityProfile.ImageCleaner).NotTo(BeNil())
	g.Expect(amcp.Spec.SecurityProfile.ImageCleaner.IntervalHours).NotTo(BeNil())
	g.Expect(*amcp.Spec.SecurityProfile.ImageCleaner.IntervalHours).To(Equal(48))
	g.Expect(amcp.Spec.EnablePreviewFeatures).NotTo(BeNil())
	g.Expect(*amcp.Spec.EnablePreviewFeatures).To(BeTrue())

	t.Logf("Testing amcp defaulting webhook with overlay")
	amcp = &AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
		},
		Spec: AzureManagedControlPlaneSpec{
			AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
				Location:          "fooLocation",
				Version:           "1.17.5",
				NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
				AutoUpgradeProfile: &ManagedClusterAutoUpgradeProfile{
					UpgradeChannel: ptr.To(UpgradeChannelRapid),
				},
				SecurityProfile: &ManagedClusterSecurityProfile{
					Defender: &ManagedClusterSecurityProfileDefender{
						LogAnalyticsWorkspaceResourceID: "not empty",
						SecurityMonitoring: ManagedClusterSecurityProfileDefenderSecurityMonitoring{
							Enabled: true,
						},
					},
					WorkloadIdentity: &ManagedClusterSecurityProfileWorkloadIdentity{
						Enabled: true,
					},
				},
			},
			ResourceGroupName: "fooRg",
			SSHPublicKey:      ptr.To(""),
		},
	}
	err = mcpw.Default(context.Background(), amcp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(amcp.Spec.VirtualNetwork.CIDRBlock).To(Equal(defaultAKSVnetCIDRForOverlay))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.CIDRBlock).To(Equal(defaultAKSNodeSubnetCIDRForOverlay))
	g.Expect(amcp.Spec.AutoUpgradeProfile).NotTo(BeNil())
	g.Expect(amcp.Spec.AutoUpgradeProfile.UpgradeChannel).NotTo(BeNil())
	g.Expect(*amcp.Spec.AutoUpgradeProfile.UpgradeChannel).To(Equal(UpgradeChannelRapid))
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		expectErr bool
	}{
		{
			name:      "Invalid Version",
			version:   "honk",
			expectErr: true,
		},
		{
			name:      "not following the Kubernetes Version pattern: missing leading v",
			version:   "1.19.0",
			expectErr: true,
		},
		{
			name:      "Version not set",
			version:   "",
			expectErr: true,
		},
		{
			name:      "Valid Version",
			version:   "v1.17.8",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			allErrs := validateVersion(tt.version, field.NewPath("spec").Child("Version"))
			if tt.expectErr {
				g.Expect(allErrs).NotTo(BeNil())
			} else {
				g.Expect(allErrs).To(BeNil())
			}
		})
	}
}

func TestValidateLoadBalancerProfile(t *testing.T) {
	tests := []struct {
		name        string
		profile     *LoadBalancerProfile
		expectedErr field.Error
	}{
		{
			name: "Valid LoadBalancerProfile",
			profile: &LoadBalancerProfile{
				ManagedOutboundIPs:     ptr.To(10),
				AllocatedOutboundPorts: ptr.To(1000),
				IdleTimeoutInMinutes:   ptr.To(60),
			},
		},
		{
			name: "Invalid LoadBalancerProfile.ManagedOutboundIPs",
			profile: &LoadBalancerProfile{
				ManagedOutboundIPs: ptr.To(200),
			},
			expectedErr: field.Error{
				Type:     field.ErrorTypeInvalid,
				Field:    "spec.LoadBalancerProfile.ManagedOutboundIPs",
				BadValue: ptr.To(200),
				Detail:   "value should be in between 1 and 100",
			},
		},
		{
			name: "Invalid LoadBalancerProfile.IdleTimeoutInMinutes",
			profile: &LoadBalancerProfile{
				IdleTimeoutInMinutes: ptr.To(600),
			},
			expectedErr: field.Error{
				Type:     field.ErrorTypeInvalid,
				Field:    "spec.LoadBalancerProfile.IdleTimeoutInMinutes",
				BadValue: ptr.To(600),
				Detail:   "value should be in between 4 and 120",
			},
		},
		{
			name: "LoadBalancerProfile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs",
			profile: &LoadBalancerProfile{
				ManagedOutboundIPs: ptr.To(1),
				OutboundIPs: []string{
					"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/foo-bar/providers/Microsoft.Network/publicIPAddresses/my-public-ip",
				},
			},
			expectedErr: field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    "spec.LoadBalancerProfile",
				BadValue: ptr.To(2),
				Detail:   "load balancer profile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			allErrs := validateLoadBalancerProfile(tt.profile, field.NewPath("spec").Child("LoadBalancerProfile"))
			if tt.expectedErr != (field.Error{}) {
				g.Expect(allErrs).To(ContainElement(MatchError(tt.expectedErr.Error())))
			} else {
				g.Expect(allErrs).To(BeNil())
			}
		})
	}
}

func TestValidateAutoScalerProfile(t *testing.T) {
	tests := []struct {
		name      string
		profile   *AutoScalerProfile
		expectErr bool
	}{
		{
			name: "Valid AutoScalerProfile",
			profile: &AutoScalerProfile{
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
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderRandom",
			profile: &AutoScalerProfile{
				Expander: (*Expander)(ptr.To(string(ExpanderRandom))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderLeastWaste",
			profile: &AutoScalerProfile{
				Expander: (*Expander)(ptr.To(string(ExpanderLeastWaste))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderMostPods",
			profile: &AutoScalerProfile{
				Expander: (*Expander)(ptr.To(string(ExpanderMostPods))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderPriority",
			profile: &AutoScalerProfile{
				Expander: (*Expander)(ptr.To(string(ExpanderPriority))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsTrue",
			profile: &AutoScalerProfile{
				BalanceSimilarNodeGroups: (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsTrue))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsFalse",
			profile: &AutoScalerProfile{
				BalanceSimilarNodeGroups: (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsFalse))),
			},
			expectErr: false,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxEmptyBulkDelete",
			profile: &AutoScalerProfile{
				MaxEmptyBulkDelete: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxGracefulTerminationSec",
			profile: &AutoScalerProfile{
				MaxGracefulTerminationSec: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxNodeProvisionTime",
			profile: &AutoScalerProfile{
				MaxNodeProvisionTime: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxTotalUnreadyPercentage",
			profile: &AutoScalerProfile{
				MaxTotalUnreadyPercentage: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.NewPodScaleUpDelay",
			profile: &AutoScalerProfile{
				NewPodScaleUpDelay: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.OkTotalUnreadyCount",
			profile: &AutoScalerProfile{
				OkTotalUnreadyCount: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScanInterval",
			profile: &AutoScalerProfile{
				ScanInterval: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterAdd",
			profile: &AutoScalerProfile{
				ScaleDownDelayAfterAdd: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterDelete",
			profile: &AutoScalerProfile{
				ScaleDownDelayAfterDelete: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterFailure",
			profile: &AutoScalerProfile{
				ScaleDownDelayAfterFailure: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnneededTime",
			profile: &AutoScalerProfile{
				ScaleDownUnneededTime: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnreadyTime",
			profile: &AutoScalerProfile{
				ScaleDownUnreadyTime: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUtilizationThreshold",
			profile: &AutoScalerProfile{
				ScaleDownUtilizationThreshold: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageTrue",
			profile: &AutoScalerProfile{
				SkipNodesWithLocalStorage: (*SkipNodesWithLocalStorage)(ptr.To(string(SkipNodesWithLocalStorageTrue))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageFalse",
			profile: &AutoScalerProfile{
				SkipNodesWithSystemPods: (*SkipNodesWithSystemPods)(ptr.To(string(SkipNodesWithSystemPodsFalse))),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			allErrs := validateAutoScalerProfile(tt.profile, field.NewPath("spec").Child("AutoScalerProfile"))
			if tt.expectErr {
				g.Expect(allErrs).NotTo(BeNil())
			} else {
				g.Expect(allErrs).To(BeNil())
			}
		})
	}
}

func TestValidatingWebhook(t *testing.T) {
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.17.8",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid DNSServiceIP",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10.3"),
						Version:      "v1.17.8",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid DNSServiceIP",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.11"),
						Version:      "v1.17.8",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing empty DNSServiceIP",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid Version",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "honk",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "not following the Kubernetes Version pattern",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "1.19.0",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Version not set",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Valid Version",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.17.8",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Valid Managed AADProfile",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						AADProfile: &AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &LoadBalancerProfile{
							ManagedOutboundIPs:     ptr.To(10),
							AllocatedOutboundPorts: ptr.To(1000),
							IdleTimeoutInMinutes:   ptr.To(60),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &LoadBalancerProfile{
							ManagedOutboundIPs: ptr.To(200),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &LoadBalancerProfile{
							AllocatedOutboundPorts: ptr.To(80000),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &LoadBalancerProfile{
							IdleTimeoutInMinutes: ptr.To(600),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &LoadBalancerProfile{
							ManagedOutboundIPs: ptr.To(1),
							OutboundIPs: []string{
								"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/foo-bar/providers/Microsoft.Network/publicIPAddresses/my-public-ip",
							},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						APIServerAccessProfile: &APIServerAccessProfile{
							AuthorizedIPRanges: []string{"1.2.3.400/32"},
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
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
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderRandom",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							Expander: (*Expander)(ptr.To(string(ExpanderRandom))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							Expander: (*Expander)(ptr.To(string(ExpanderLeastWaste))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							Expander: (*Expander)(ptr.To(string(ExpanderMostPods))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							Expander: (*Expander)(ptr.To(string(ExpanderPriority))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							BalanceSimilarNodeGroups: (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsTrue))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							BalanceSimilarNodeGroups: (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsFalse))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							MaxEmptyBulkDelete: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							MaxGracefulTerminationSec: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							MaxNodeProvisionTime: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							MaxTotalUnreadyPercentage: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							NewPodScaleUpDelay: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							OkTotalUnreadyCount: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							ScanInterval: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							ScaleDownDelayAfterAdd: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							ScaleDownDelayAfterDelete: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							ScaleDownDelayAfterFailure: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							ScaleDownUnneededTime: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							ScaleDownUnreadyTime: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							ScaleDownUtilizationThreshold: ptr.To("invalid"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							SkipNodesWithLocalStorage: (*SkipNodesWithLocalStorage)(ptr.To(string(SkipNodesWithLocalStorageTrue))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							SkipNodesWithLocalStorage: (*SkipNodesWithLocalStorage)(ptr.To(string(SkipNodesWithLocalStorageFalse))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							SkipNodesWithSystemPods: (*SkipNodesWithSystemPods)(ptr.To(string(SkipNodesWithSystemPodsTrue))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &AutoScalerProfile{
							SkipNodesWithSystemPods: (*SkipNodesWithSystemPods)(ptr.To(string(SkipNodesWithSystemPodsFalse))),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						Identity: &Identity{
							Type: ManagedControlPlaneIdentityTypeSystemAssigned,
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "/resource/id",
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeSystemAssigned,
							UserAssignedIdentityResourceID: "/resource/id",
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						Identity: &Identity{
							Type: ManagedControlPlaneIdentityTypeUserAssigned,
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:           "v1.24.1",
						NetworkPlugin:     ptr.To("kubenet"),
						NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "overlay can be used with azure",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:           "v1.24.1",
						NetworkPlugin:     ptr.To("azure"),
						NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AKS Extension",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Extensions: []AKSExtension{
							{
								Name:          "extension1",
								ExtensionType: ptr.To("test-type"),
								Plan: &ExtensionPlan{
									Name:      "test-plan",
									Product:   "test-product",
									Publisher: "test-publisher",
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid AKS Extension: version given when AutoUpgradeMinorVersion is true",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Extensions: []AKSExtension{
							{
								Name:                    "extension1",
								ExtensionType:           ptr.To("test-type"),
								Version:                 ptr.To("1.0.0"),
								AutoUpgradeMinorVersion: ptr.To(true),
								Plan: &ExtensionPlan{
									Name:      "test-plan",
									Product:   "test-product",
									Publisher: "test-publisher",
								},
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AKS Extension: missing plan.product and plan.publisher",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Extensions: []AKSExtension{
							{
								Name:                    "extension1",
								ExtensionType:           ptr.To("test-type"),
								Version:                 ptr.To("1.0.0"),
								AutoUpgradeMinorVersion: ptr.To(true),
								Plan: &ExtensionPlan{
									Name: "test-plan",
								},
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Test invalid AzureKeyVaultKms",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPrivate),
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Valid NetworkDataplane: cilium",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:           "v1.17.8",
						NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
						NetworkDataplane:  ptr.To(NetworkDataplaneTypeCilium),
						NetworkPolicy:     ptr.To("cilium"),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid NetworkDataplane: cilium dataplane requires overlay network plugin mode",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:           "v1.17.8",
						NetworkPluginMode: nil,
						NetworkDataplane:  ptr.To(NetworkDataplaneTypeCilium),
						NetworkPolicy:     ptr.To("cilium"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Test valid AzureKeyVaultKms",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPrivate),
								KeyVaultResourceID:    ptr.To("0000-0000-0000-000"),
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Test valid AzureKeyVaultKms",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid NetworkDataplane: cilium dataplane requires network policy to be cilium",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:           "v1.17.8",
						NetworkPluginMode: nil,
						NetworkDataplane:  ptr.To(NetworkDataplaneTypeCilium),
						NetworkPolicy:     ptr.To("azure"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid NetworkPolicy: cilium network policy can only be used with cilium network dataplane",
			amcp: AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:           "v1.17.8",
						NetworkPluginMode: nil,
						NetworkDataplane:  ptr.To(NetworkDataplaneTypeAzure),
						NetworkPolicy:     ptr.To("cilium"),
					},
				},
			},
			expectErr: true,
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Testing inValid DNSPrefix with more then 54 characters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("thisisaverylong$^clusternameconsistingofmorethan54characterswhichshouldbeinvalid"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Testing inValid DNSPrefix with underscore",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("no_underscore"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Testing inValid DNSPrefix with special characters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("no-dollar$@%"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Testing Valid DNSPrefix with hyphen characters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("hyphen-allowed"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Testing Valid DNSPrefix with hyphen characters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("palette-test07"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Testing valid DNSPrefix ",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("thisisavlerylongclu7l0sternam3leconsistingofmorethan54"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.23.5",
					},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.23.5",
					},
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
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						AADProfile: &AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
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
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						AADProfile: &AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:              "v1.21.2",
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "DisableLocalAccounts can be set for AAD clusters",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						AADProfile: &AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](true),
					},
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
		name        string
		amcp        *AzureManagedControlPlane
		deferFunc   func()
		expectError bool
	}{
		{
			name:        "feature gate explicitly disabled",
			amcp:        getKnownValidAzureManagedControlPlane(),
			deferFunc:   utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, false),
			expectError: true,
		},
		{
			name:        "feature gate implicitly enabled",
			amcp:        getKnownValidAzureManagedControlPlane(),
			deferFunc:   func() {},
			expectError: false,
		},
	}
	client := mockClient{ReturnError: false}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			defer tc.deferFunc()
			mcpw := &azureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateCreate(context.Background(), tc.amcp)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AddonProfiles: []AddonProfile{
							{
								Name:    "first-addon-profile",
								Enabled: true,
							},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						AddonProfiles: []AddonProfile{
							{
								Name:    "first-addon-profile",
								Enabled: true,
							},
						},
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AddonProfiles: []AddonProfile{
							{
								Name:    "first-addon-profile",
								Enabled: false,
							},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						AddonProfiles: []AddonProfile{
							{
								Name:    "first-addon-profile",
								Enabled: true,
							},
						},
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane AddonProfiles cannot be completely removed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
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
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						AddonProfiles: []AddonProfile{
							{
								Name:    "first-addon-profile",
								Enabled: true,
							},
						},
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane invalid version downgrade change",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane invalid version downgrade change",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
				Status: AzureManagedControlPlaneStatus{
					AutoUpgradeVersion: "v1.18.3",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane Autoupgrade cannot be set to nil",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
						AutoUpgradeProfile: &ManagedClusterAutoUpgradeProfile{
							UpgradeChannel: ptr.To(UpgradeChannelStable),
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane Autoupgrade cannot be set to nil",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
						AutoUpgradeProfile: &ManagedClusterAutoUpgradeProfile{
							UpgradeChannel: ptr.To(UpgradeChannelStable),
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:       ptr.To("192.168.0.10"),
						SubscriptionID:     "212ec1q8",
						Version:            "v1.18.0",
						AutoUpgradeProfile: &ManagedClusterAutoUpgradeProfile{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane Autoupgrade is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
						AutoUpgradeProfile: &ManagedClusterAutoUpgradeProfile{
							UpgradeChannel: ptr.To(UpgradeChannelStable),
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
						AutoUpgradeProfile: &ManagedClusterAutoUpgradeProfile{
							UpgradeChannel: ptr.To(UpgradeChannelNone),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane SubscriptionID is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q9",
						Version:        "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane ResourceGroupName is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
					ResourceGroupName: "hello-1",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
					ResourceGroupName: "hello-2",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NodeResourceGroupName is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
					NodeResourceGroupName: "hello-1",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
					NodeResourceGroupName: "hello-2",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane Location is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Location:     "westeurope",
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Location:     "eastus",
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane SSHPublicKey is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSServiceIP is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.1.1"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSServiceIP is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPlugin is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPlugin: ptr.To("azure"),
						Version:       "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPlugin: ptr.To("kubenet"),
						Version:       "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPlugin is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPlugin: ptr.To("azure"),
						Version:       "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPolicy is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPolicy: ptr.To("azure"),
						Version:       "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPolicy: ptr.To("calico"),
						Version:       "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPolicy is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPolicy: ptr.To("azure"),
						Version:       "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPolicy is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:     ptr.To("192.168.0.10"),
						NetworkDataplane: ptr.To(NetworkDataplaneTypeCilium),
						Version:          "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:     ptr.To("192.168.0.10"),
						NetworkDataplane: ptr.To(NetworkDataplaneTypeAzure),
						Version:          "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkDataplane is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:     ptr.To("192.168.0.10"),
						NetworkDataplane: ptr.To(NetworkDataplaneTypeCilium),
						Version:          "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane LoadBalancerSKU is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:    ptr.To("192.168.0.10"),
						LoadBalancerSKU: ptr.To("Standard"),
						Version:         "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:    ptr.To("192.168.0.10"),
						LoadBalancerSKU: ptr.To(LoadBalancerSKUBasic),
						Version:         "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane LoadBalancerSKU is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP:    ptr.To("192.168.0.10"),
						LoadBalancerSKU: ptr.To(LoadBalancerSKUStandard),
						Version:         "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane ManagedAad can be set after cluster creation",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:    "v1.18.0",
						AADProfile: &AADProfile{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane managed field cannot set to false",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed: false,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane ManagedAad cannot be disabled",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane EnablePrivateCluster is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						APIServerAccessProfile: &APIServerAccessProfile{
							APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
								EnablePrivateCluster: ptr.To(true),
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane AuthorizedIPRanges is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						APIServerAccessProfile: &APIServerAccessProfile{
							AuthorizedIPRanges: []string{"192.168.0.1/32"},
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name:      "test-network",
								CIDRBlock: "10.0.0.0/8",
								Subnet: ManagedControlPlaneSubnet{
									Name:      "test-subnet",
									CIDRBlock: "10.0.2.0/24",
								},
							},
							ResourceGroup: "test-rg",
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name:      "test-network",
								CIDRBlock: "10.0.0.0/8",
								Subnet: ManagedControlPlaneSubnet{
									Name:      "test-subnet",
									CIDRBlock: "10.0.2.0/24",
								},
							},
							ResourceGroup: "test-rg",
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name:      "test-network",
								CIDRBlock: "10.0.0.0/8",
								Subnet: ManagedControlPlaneSubnet{
									Name:      "test-subnet",
									CIDRBlock: "10.0.2.0/24",
								},
							},
							ResourceGroup: "test-rg",
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name:      "test-network",
								CIDRBlock: "10.0.0.0/8",
								Subnet: ManagedControlPlaneSubnet{
									Name:      "test-subnet",
									CIDRBlock: "10.0.2.0/24",
								},
							},
							ResourceGroup: "test-rg",
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						OutboundType: (*ManagedControlPlaneOutboundType)(ptr.To(string(ManagedControlPlaneOutboundTypeUserDefinedRouting))),
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						OutboundType: (*ManagedControlPlaneOutboundType)(ptr.To(string(ManagedControlPlaneOutboundTypeLoadBalancer))),
					},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						HTTPProxyConfig: &HTTPProxyConfig{
							HTTPProxy:  ptr.To("http://1.2.3.4:8080"),
							HTTPSProxy: ptr.To("https://5.6.7.8:8443"),
							NoProxy:    []string{"endpoint1", "endpoint2"},
							TrustedCA:  ptr.To("ca"),
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						HTTPProxyConfig: &HTTPProxyConfig{
							HTTPProxy:  ptr.To("http://10.20.3.4:8080"),
							HTTPSProxy: ptr.To("https://5.6.7.8:8443"),
							NoProxy:    []string{"endpoint1", "endpoint2"},
							TrustedCA:  ptr.To("ca"),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     ptr.To("anything"),
						NetworkPluginMode: nil,
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     ptr.To("anything"),
						NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
					},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     nil,
						NetworkPluginMode: nil,
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:           "v0.0.0",
						NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
					},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     ptr.To("anything"),
						NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     ptr.To("anything"),
						Version:           "v0.0.0",
						NetworkPluginMode: ptr.To(NetworkPluginModeOverlay),
					},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v0.0.0",
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(false),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v0.0.0",
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v0.0.0",
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(false),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v0.0.0",
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable no error",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](true),
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](false),
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
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix can be updated from nil when resource name matches",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capz-aks",
				},
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "DisableLocalAccounts cannot be set for non AAD clusters",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:              "v1.18.0",
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable error nil -> empty",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To(""),
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable no error nil -> nil",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane AKSExtensions ConfigurationSettings and AutoUpgradeMinorVersion are mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Extensions: []AKSExtension{
							{
								Name:                    "extension1",
								AutoUpgradeMinorVersion: ptr.To(false),
								ConfigurationSettings: map[string]string{
									"key1": "value1",
								},
								Plan: &ExtensionPlan{
									Name:      "planName",
									Product:   "planProduct",
									Publisher: "planPublisher",
								},
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Extensions: []AKSExtension{
							{
								Name:                    "extension1",
								AutoUpgradeMinorVersion: ptr.To(true),
								ConfigurationSettings: map[string]string{
									"key1": "value1",
									"key2": "value2",
								},
								Plan: &ExtensionPlan{
									Name:      "planName",
									Product:   "planProduct",
									Publisher: "planPublisher",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane all other fields are immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Extensions: []AKSExtension{
							{
								Name:                    "extension1",
								AKSAssignedIdentityType: AKSAssignedIdentitySystemAssigned,
								ExtensionType:           ptr.To("extensionType"),
								Plan: &ExtensionPlan{
									Name:      "planName",
									Product:   "planProduct",
									Publisher: "planPublisher",
								},
								Scope: &ExtensionScope{
									ScopeType:        "Cluster",
									ReleaseNamespace: "default",
								},
								ReleaseTrain: ptr.To("releaseTrain"),
								Version:      ptr.To("v1.0.0"),
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Extensions: []AKSExtension{
							{
								Name:                    "extension2",
								AKSAssignedIdentityType: AKSAssignedIdentityUserAssigned,
								ExtensionType:           ptr.To("extensionType1"),
								Plan: &ExtensionPlan{
									Name:      "planName1",
									Product:   "planProduct1",
									Publisher: "planPublisher1",
								},
								Scope: &ExtensionScope{
									ScopeType:        "Namespace",
									ReleaseNamespace: "default",
								},
								ReleaseTrain: ptr.To("releaseTrain1"),
								Version:      ptr.To("v1.1.0"),
							},
						},
					},
				},
			},
			wantErr: true,
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
			AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
				DNSServiceIP: ptr.To(serviceIP),
				Version:      version,
			},
		},
	}
}

func getKnownValidAzureManagedControlPlane() *AzureManagedControlPlane {
	return &AzureManagedControlPlane{
		ObjectMeta: getAMCPMetaData(),
		Spec: AzureManagedControlPlaneSpec{
			AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
				DNSServiceIP: ptr.To("192.168.0.10"),
				Version:      "v1.18.0",
				AADProfile: &AADProfile{
					Managed: true,
					AdminGroupObjectIDs: []string{
						"616077a8-5db7-4c98-b856-b34619afg75h",
					},
				},
			},
			SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
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

func TestAzureManagedClusterSecurityProfileValidateCreate(t *testing.T) {
	testsCreate := []struct {
		name    string
		amcp    *AzureManagedControlPlane
		wantErr string
	}{
		{
			name: "Cannot enable Workload Identity without enabling OIDC issuer",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						SecurityProfile: &ManagedClusterSecurityProfile{
							WorkloadIdentity: &ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
						},
					},
				},
			},
			wantErr: "Spec.SecurityProfile.WorkloadIdentity: Invalid value: v1beta1.ManagedClusterSecurityProfileWorkloadIdentity{Enabled:true}: Spec.SecurityProfile.WorkloadIdentity cannot be enabled when Spec.OIDCIssuerProfile is disabled",
		},
		{
			name: "Cannot enable AzureKms without user assigned identity",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled: true,
							},
						},
					},
				},
			},
			wantErr: "Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID: Invalid value: \"null\": Spec.SecurityProfile.AzureKeyVaultKms can be set only when Spec.Identity.Type is UserAssigned",
		},
		{
			name: "When AzureKms.KeyVaultNetworkAccess is private AzureKeyVaultKms.KeyVaultResourceID cannot be empty",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						Version: "v1.17.8",
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "not empty",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPrivate),
							},
						},
					},
				},
			},
			wantErr: "Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID: Invalid value: \"null\": Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID cannot be empty when Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultNetworkAccess is Private",
		},
		{
			name: "When AzureKms.KeyVaultNetworkAccess is public AzureKeyVaultKms.KeyVaultResourceID should be empty",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "not empty",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPublic),
								KeyVaultResourceID:    ptr.To("not empty"),
							},
						},
					},
				},
			},
			wantErr: "Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID: Invalid value: \"not empty\": Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID should be empty when Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultNetworkAccess is Public",
		},
		{
			name: "Valid profile",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "not empty",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPublic),
							},
							Defender: &ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "not empty",
								SecurityMonitoring: ManagedClusterSecurityProfileDefenderSecurityMonitoring{
									Enabled: true,
								},
							},
							WorkloadIdentity: &ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
							ImageCleaner: &ManagedClusterSecurityProfileImageCleaner{
								Enabled:       true,
								IntervalHours: ptr.To(24),
							},
						},
					},
				},
			},
			wantErr: "",
		},
	}
	client := mockClient{ReturnError: false}
	for _, tc := range testsCreate {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mcpw := &azureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateCreate(context.Background(), tc.amcp)
			if tc.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tc.wantErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureClusterSecurityProfileValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldAMCP *AzureManagedControlPlane
		amcp    *AzureManagedControlPlane
		wantErr string
	}{
		{
			name: "AzureManagedControlPlane SecurityProfile.Defender is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							Defender: &ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "0000-0000-0000-0000",
								SecurityMonitoring: ManagedClusterSecurityProfileDefenderSecurityMonitoring{
									Enabled: true,
								},
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.Defender is mutable and cannot be unset",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							Defender: &ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "0000-0000-0000-0000",
								SecurityMonitoring: ManagedClusterSecurityProfileDefenderSecurityMonitoring{
									Enabled: true,
								},
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: "AzureManagedControlPlane.infrastructure.cluster.x-k8s.io \"\" is invalid: Spec.SecurityProfile.Defender: Invalid value: \"null\": cannot unset Spec.SecurityProfile.Defender, to disable defender please set Spec.SecurityProfile.Defender.SecurityMonitoring.Enabled to false",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.Defender is mutable and can be disabled",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							Defender: &ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "0000-0000-0000-0000",
								SecurityMonitoring: ManagedClusterSecurityProfileDefenderSecurityMonitoring{
									Enabled: true,
								},
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							Defender: &ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "0000-0000-0000-0000",
								SecurityMonitoring: ManagedClusterSecurityProfileDefenderSecurityMonitoring{
									Enabled: false,
								},
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.WorkloadIdentity is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							WorkloadIdentity: &ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.WorkloadIdentity cannot be enabled without OIDC issuer",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							WorkloadIdentity: &ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
						},
					},
				},
			},
			wantErr: "Spec.SecurityProfile.WorkloadIdentity: Invalid value: v1beta1.ManagedClusterSecurityProfileWorkloadIdentity{Enabled:true}: Spec.SecurityProfile.WorkloadIdentity cannot be enabled when Spec.OIDCIssuerProfile is disabled",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.WorkloadIdentity cannot unset values",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							WorkloadIdentity: &ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			wantErr: "AzureManagedControlPlane.infrastructure.cluster.x-k8s.io \"\" is invalid: Spec.SecurityProfile.WorkloadIdentity: Invalid value: \"null\": cannot unset Spec.SecurityProfile.WorkloadIdentity, to disable workloadIdentity please set Spec.SecurityProfile.WorkloadIdentity.Enabled to false",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.WorkloadIdentity can be disabled",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							WorkloadIdentity: &ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							WorkloadIdentity: &ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: false,
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.AzureKeyVaultKms is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPrivate),
								KeyVaultResourceID:    ptr.To("0000-0000-0000-0000"),
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.AzureKeyVaultKms.KeyVaultNetworkAccess can be updated when KMS is enabled",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPrivate),
								KeyVaultResourceID:    ptr.To("0000-0000-0000-0000"),
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.AzureKeyVaultKms.Enabled can be disabled",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               false,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.AzureKeyVaultKms cannot unset",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &Identity{
							Type:                           ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
					},
				},
			},
			wantErr: "AzureManagedControlPlane.infrastructure.cluster.x-k8s.io \"\" is invalid: Spec.SecurityProfile.AzureKeyVaultKms: Invalid value: \"null\": cannot unset Spec.SecurityProfile.AzureKeyVaultKms profile to disable the profile please set Spec.SecurityProfile.AzureKeyVaultKms.Enabled to false",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.AzureKeyVaultKms cannot be enabled without UserAssigned Identity",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(KeyVaultNetworkAccessTypesPrivate),
								KeyVaultResourceID:    ptr.To("0000-0000-0000-0000"),
							},
						},
					},
				},
			},
			wantErr: "Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID: Invalid value: \"0000-0000-0000-0000\": Spec.SecurityProfile.AzureKeyVaultKms can be set only when Spec.Identity.Type is UserAssigned",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.ImageCleaner is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							ImageCleaner: &ManagedClusterSecurityProfileImageCleaner{
								Enabled:       true,
								IntervalHours: ptr.To(28),
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.ImageCleaner cannot be unset",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							ImageCleaner: &ManagedClusterSecurityProfileImageCleaner{
								Enabled:       true,
								IntervalHours: ptr.To(48),
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version:         "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{},
					},
				},
			},
			wantErr: "AzureManagedControlPlane.infrastructure.cluster.x-k8s.io \"\" is invalid: Spec.SecurityProfile.ImageCleaner: Invalid value: \"null\": cannot unset Spec.SecurityProfile.ImageCleaner, to disable imageCleaner please set Spec.SecurityProfile.ImageCleaner.Enabled to false",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.ImageCleaner is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							ImageCleaner: &ManagedClusterSecurityProfileImageCleaner{
								Enabled: true,
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							ImageCleaner: &ManagedClusterSecurityProfileImageCleaner{
								IntervalHours: ptr.To(48),
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.ImageCleaner can be disabled",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							ImageCleaner: &ManagedClusterSecurityProfileImageCleaner{
								Enabled:       true,
								IntervalHours: ptr.To(48),
							},
						},
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &ManagedClusterSecurityProfile{
							ImageCleaner: &ManagedClusterSecurityProfileImageCleaner{
								Enabled:       false,
								IntervalHours: ptr.To(36),
							},
						},
					},
				},
			},
			wantErr: "",
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
			if tc.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tc.wantErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestValidateAPIServerAccessProfile(t *testing.T) {
	tests := []struct {
		name      string
		profile   *APIServerAccessProfile
		expectErr bool
	}{
		{
			name: "Testing valid PrivateDNSZone:System",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					PrivateDNSZone: ptr.To("System"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid PrivateDNSZone:None",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					PrivateDNSZone: ptr.To("None"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid PrivateDNSZone:With privatelink region",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid PrivateDNSZone:With private region",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/private.eastus.azmk8s.io"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid EnablePrivateCluster and valid PrivateDNSZone",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(false),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/private.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing valid PrivateDNSZone:With privatelink region and sub-region",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/sublocation2.privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid PrivateDNSZone:With private region and sub-region",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/sublocation2.private.eastus.azmk8s.io"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid PrivateDNSZone: privatelink region: len(sub-region) > 32 characters",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/thissublocationismorethan32characters.privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: private region: len(sub-region) > 32 characters",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/thissublocationismorethan32characters.private.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: random string",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("WrongPrivateDNSZone"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: subzone has an invalid char %",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/subzone%1.privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: subzone has an invalid char _",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/subzone_1.privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: region has invalid char",
			profile: &APIServerAccessProfile{
				APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/subzone1.privatelink.location@1.azmk8s.io"),
				},
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			errs := validateAPIServerAccessProfile(tc.profile, field.NewPath("profile"))
			if tc.expectErr {
				g.Expect(errs).To(HaveLen(1))
			} else {
				g.Expect(errs).To(BeEmpty())
			}
		})
	}
}

func TestValidateAMCPVirtualNetwork(t *testing.T) {
	tests := []struct {
		name    string
		amcp    *AzureManagedControlPlane
		wantErr string
	}{
		{
			name: "Testing valid VirtualNetwork in same resource group",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: AzureManagedControlPlaneSpec{
					ResourceGroupName: "rg1",
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg1",
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name:      "vnet1",
								CIDRBlock: defaultAKSVnetCIDR,
								Subnet: ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: defaultAKSNodeSubnetCIDR,
								},
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "Testing valid VirtualNetwork in different resource group",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: AzureManagedControlPlaneSpec{
					ResourceGroupName: "rg1",
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg2",
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name:      "vnet1",
								CIDRBlock: defaultAKSVnetCIDR,
								Subnet: ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: defaultAKSNodeSubnetCIDR,
								},
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "Testing invalid VirtualNetwork in different resource group: invalid subnet CIDR",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: AzureManagedControlPlaneSpec{
					ResourceGroupName: "rg1",
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg2",
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name:      "vnet1",
								CIDRBlock: "10.1.0.0/16",
								Subnet: ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: "10.0.0.0/24",
								},
							},
						},
					},
				},
			},
			wantErr: "pre-existing virtual networks CIDR block should contain the subnet CIDR block",
		},
		{
			name: "Testing invalid VirtualNetwork in different resource group: no subnet CIDR",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: AzureManagedControlPlaneSpec{
					ResourceGroupName: "rg1",
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg2",
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name:      "vnet1",
								CIDRBlock: "10.1.0.0/16",
								Subnet: ManagedControlPlaneSubnet{
									Name: "subnet1",
								},
							},
						},
					},
				},
			},
			wantErr: "pre-existing virtual networks CIDR block should contain the subnet CIDR block",
		},
		{
			name: "Testing invalid VirtualNetwork in different resource group: no VNet CIDR",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: AzureManagedControlPlaneSpec{
					ResourceGroupName: "rg1",
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg2",
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name: "vnet1",
								Subnet: ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: "11.0.0.0/24",
								},
							},
						},
					},
				},
			},
			wantErr: "pre-existing virtual networks CIDR block should contain the subnet CIDR block",
		},
		{
			name: "Testing invalid VirtualNetwork in different resource group: invalid VNet CIDR",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: AzureManagedControlPlaneSpec{
					ResourceGroupName: "rg1",
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg2",
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name:      "vnet1",
								CIDRBlock: "invalid_vnet_CIDR",
								Subnet: ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: "11.0.0.0/24",
								},
							},
						},
					},
				},
			},
			wantErr: "pre-existing virtual networks CIDR block is invalid",
		},
		{
			name: "Testing invalid VirtualNetwork in different resource group: invalid Subnet CIDR",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: AzureManagedControlPlaneSpec{
					ResourceGroupName: "rg1",
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						VirtualNetwork: ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg2",
							ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
								Name: "vnet1",
								Subnet: ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: "invalid_subnet_CIDR",
								},
							},
						},
					},
				},
			},
			wantErr: "pre-existing subnets CIDR block is invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mcpw := &azureManagedControlPlaneWebhook{}
			err := mcpw.Default(context.Background(), tc.amcp)
			g.Expect(err).NotTo(HaveOccurred())

			errs := validateAMCPVirtualNetwork(tc.amcp.Spec.VirtualNetwork, field.NewPath("spec", "VirtualNetwork"))
			if tc.wantErr != "" {
				g.Expect(errs).ToNot(BeEmpty())
				g.Expect(errs[0].Detail).To(Equal(tc.wantErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
