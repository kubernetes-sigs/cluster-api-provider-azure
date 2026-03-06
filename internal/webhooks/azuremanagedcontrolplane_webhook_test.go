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

package webhooks

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capifeature "sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	apiinternal "sigs.k8s.io/cluster-api-provider-azure/internal/api"
)

func init() {
	format.MaxLength = 0
	format.TruncatedDiff = false
}

func TestDefaultingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing amcp defaulting webhook with no baseline")
	amcp := &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "fooCluster",
			},
		},
		Spec: infrav1.AzureManagedControlPlaneSpec{
			AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
				Location: "fooLocation",
				Version:  "1.17.5",
				Extensions: []infrav1.AKSExtension{
					{
						Name: "test-extension",
						Plan: &infrav1.ExtensionPlan{
							Product:   "test-product",
							Publisher: "test-publisher",
						},
					},
				},
				SKU: &infrav1.AKSSku{},
			},
			SSHPublicKey: ptr.To(""),
		},
	}
	mcpw := &AzureManagedControlPlaneWebhook{}
	err := mcpw.Default(t.Context(), amcp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(amcp.Spec.ResourceGroupName).To(Equal("fooCluster"))
	g.Expect(amcp.Spec.Version).To(Equal("v1.17.5"))
	g.Expect(*amcp.Spec.SSHPublicKey).NotTo(BeEmpty())
	g.Expect(amcp.Spec.NodeResourceGroupName).To(Equal("MC_fooCluster_fooName_fooLocation"))
	g.Expect(amcp.Spec.VirtualNetwork.Name).To(Equal("fooName"))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooName"))
	g.Expect(amcp.Spec.DNSPrefix).NotTo(BeNil())
	g.Expect(*amcp.Spec.DNSPrefix).To(Equal(amcp.Name))
	g.Expect(amcp.Spec.Extensions[0].Plan.Name).To(Equal("fooName-test-product"))

	t.Logf("Testing amcp defaulting webhook with baseline")
	netPlug := "kubenet"
	netPol := "azure"
	amcp.Spec.NetworkPlugin = &netPlug
	amcp.Spec.NetworkPolicy = &netPol
	amcp.Spec.Version = "9.99.99"
	amcp.Spec.SSHPublicKey = nil
	amcp.Spec.ResourceGroupName = "fooRg"
	amcp.Spec.NodeResourceGroupName = "fooNodeRg"
	amcp.Spec.VirtualNetwork.Name = "fooVnetName"
	amcp.Spec.VirtualNetwork.Subnet.Name = "fooSubnetName"
	amcp.Spec.SKU.Tier = infrav1.PaidManagedControlPlaneTier
	amcp.Spec.OIDCIssuerProfile = &infrav1.OIDCIssuerProfile{
		Enabled: ptr.To(true),
	}
	amcp.Spec.DNSPrefix = ptr.To("test-prefix")
	amcp.Spec.FleetsMember = &infrav1.FleetsMember{}
	amcp.Spec.AutoUpgradeProfile = &infrav1.ManagedClusterAutoUpgradeProfile{
		UpgradeChannel: ptr.To(infrav1.UpgradeChannelPatch),
	}
	amcp.Spec.SecurityProfile = &infrav1.ManagedClusterSecurityProfile{
		AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
			Enabled: true,
		},
		ImageCleaner: &infrav1.ManagedClusterSecurityProfileImageCleaner{
			Enabled:       true,
			IntervalHours: ptr.To(48),
		},
	}
	amcp.Spec.EnablePreviewFeatures = ptr.To(true)

	err = mcpw.Default(t.Context(), amcp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*amcp.Spec.NetworkPlugin).To(Equal(netPlug))
	g.Expect(*amcp.Spec.NetworkPolicy).To(Equal(netPol))
	g.Expect(amcp.Spec.Version).To(Equal("v9.99.99"))
	g.Expect(amcp.Spec.SSHPublicKey).To(BeNil())
	g.Expect(amcp.Spec.ResourceGroupName).To(Equal("fooRg"))
	g.Expect(amcp.Spec.NodeResourceGroupName).To(Equal("fooNodeRg"))
	g.Expect(amcp.Spec.VirtualNetwork.Name).To(Equal("fooVnetName"))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooSubnetName"))
	g.Expect(amcp.Spec.SKU.Tier).To(Equal(infrav1.StandardManagedControlPlaneTier))
	g.Expect(*amcp.Spec.OIDCIssuerProfile.Enabled).To(BeTrue())
	g.Expect(amcp.Spec.DNSPrefix).NotTo(BeNil())
	g.Expect(*amcp.Spec.DNSPrefix).To(Equal("test-prefix"))
	g.Expect(amcp.Spec.FleetsMember.Name).To(Equal("fooCluster"))
	g.Expect(amcp.Spec.AutoUpgradeProfile).NotTo(BeNil())
	g.Expect(amcp.Spec.AutoUpgradeProfile.UpgradeChannel).NotTo(BeNil())
	g.Expect(*amcp.Spec.AutoUpgradeProfile.UpgradeChannel).To(Equal(infrav1.UpgradeChannelPatch))
	g.Expect(amcp.Spec.SecurityProfile).NotTo(BeNil())
	g.Expect(amcp.Spec.SecurityProfile.AzureKeyVaultKms).NotTo(BeNil())
	g.Expect(amcp.Spec.SecurityProfile.ImageCleaner).NotTo(BeNil())
	g.Expect(amcp.Spec.SecurityProfile.ImageCleaner.IntervalHours).NotTo(BeNil())
	g.Expect(*amcp.Spec.SecurityProfile.ImageCleaner.IntervalHours).To(Equal(48))
	g.Expect(amcp.Spec.EnablePreviewFeatures).NotTo(BeNil())
	g.Expect(*amcp.Spec.EnablePreviewFeatures).To(BeTrue())

	t.Logf("Testing amcp defaulting webhook with overlay")
	amcp = &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
		},
		Spec: infrav1.AzureManagedControlPlaneSpec{
			AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
				ResourceGroupName: "fooRg",
				Location:          "fooLocation",
				Version:           "1.17.5",
				NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
				AutoUpgradeProfile: &infrav1.ManagedClusterAutoUpgradeProfile{
					UpgradeChannel: ptr.To(infrav1.UpgradeChannelRapid),
				},
				SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
					Defender: &infrav1.ManagedClusterSecurityProfileDefender{
						LogAnalyticsWorkspaceResourceID: "not empty",
						SecurityMonitoring: infrav1.ManagedClusterSecurityProfileDefenderSecurityMonitoring{
							Enabled: true,
						},
					},
					WorkloadIdentity: &infrav1.ManagedClusterSecurityProfileWorkloadIdentity{
						Enabled: true,
					},
				},
				SKU: &infrav1.AKSSku{},
			},
			SSHPublicKey: ptr.To(""),
		},
	}
	err = mcpw.Default(t.Context(), amcp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(amcp.Spec.VirtualNetwork.CIDRBlock).To(Equal(apiinternal.DefaultAKSVnetCIDRForOverlay))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.CIDRBlock).To(Equal(apiinternal.DefaultAKSNodeSubnetCIDRForOverlay))
	g.Expect(amcp.Spec.AutoUpgradeProfile).NotTo(BeNil())
	g.Expect(amcp.Spec.AutoUpgradeProfile.UpgradeChannel).NotTo(BeNil())
	g.Expect(*amcp.Spec.AutoUpgradeProfile.UpgradeChannel).To(Equal(infrav1.UpgradeChannelRapid))
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
		profile     *infrav1.LoadBalancerProfile
		expectedErr field.Error
	}{
		{
			name: "Valid LoadBalancerProfile",
			profile: &infrav1.LoadBalancerProfile{
				ManagedOutboundIPs:     ptr.To(10),
				AllocatedOutboundPorts: ptr.To(1000),
				IdleTimeoutInMinutes:   ptr.To(60),
			},
		},
		{
			name: "Invalid LoadBalancerProfile.ManagedOutboundIPs",
			profile: &infrav1.LoadBalancerProfile{
				ManagedOutboundIPs: ptr.To(200),
			},
			expectedErr: field.Error{
				Type:     field.ErrorTypeInvalid,
				Field:    "spec.loadBalancerProfile.ManagedOutboundIPs",
				BadValue: ptr.To(200),
				Detail:   "value should be in between 1 and 100",
			},
		},
		{
			name: "Invalid LoadBalancerProfile.IdleTimeoutInMinutes",
			profile: &infrav1.LoadBalancerProfile{
				IdleTimeoutInMinutes: ptr.To(600),
			},
			expectedErr: field.Error{
				Type:     field.ErrorTypeInvalid,
				Field:    "spec.loadBalancerProfile.IdleTimeoutInMinutes",
				BadValue: ptr.To(600),
				Detail:   "value should be in between 4 and 120",
			},
		},
		{
			name: "LoadBalancerProfile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs",
			profile: &infrav1.LoadBalancerProfile{
				ManagedOutboundIPs: ptr.To(1),
				OutboundIPs: []string{
					"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/foo-bar/providers/Microsoft.Network/publicIPAddresses/my-public-ip",
				},
			},
			expectedErr: field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    "spec.loadBalancerProfile",
				BadValue: ptr.To(2),
				Detail:   "load balancer profile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			allErrs := validateLoadBalancerProfile(tt.profile, field.NewPath("spec").Child("loadBalancerProfile"))
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
		profile   *infrav1.AutoScalerProfile
		expectErr bool
	}{
		{
			name: "Valid AutoScalerProfile",
			profile: &infrav1.AutoScalerProfile{
				BalanceSimilarNodeGroups:      (*infrav1.BalanceSimilarNodeGroups)(ptr.To(string(infrav1.BalanceSimilarNodeGroupsFalse))),
				Expander:                      (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderRandom))),
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
				SkipNodesWithLocalStorage:     (*infrav1.SkipNodesWithLocalStorage)(ptr.To(string(infrav1.SkipNodesWithLocalStorageTrue))),
				SkipNodesWithSystemPods:       (*infrav1.SkipNodesWithSystemPods)(ptr.To(string(infrav1.SkipNodesWithSystemPodsTrue))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderRandom",
			profile: &infrav1.AutoScalerProfile{
				Expander: (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderRandom))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderLeastWaste",
			profile: &infrav1.AutoScalerProfile{
				Expander: (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderLeastWaste))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderMostPods",
			profile: &infrav1.AutoScalerProfile{
				Expander: (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderMostPods))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderPriority",
			profile: &infrav1.AutoScalerProfile{
				Expander: (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderPriority))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsTrue",
			profile: &infrav1.AutoScalerProfile{
				BalanceSimilarNodeGroups: (*infrav1.BalanceSimilarNodeGroups)(ptr.To(string(infrav1.BalanceSimilarNodeGroupsTrue))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsFalse",
			profile: &infrav1.AutoScalerProfile{
				BalanceSimilarNodeGroups: (*infrav1.BalanceSimilarNodeGroups)(ptr.To(string(infrav1.BalanceSimilarNodeGroupsFalse))),
			},
			expectErr: false,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxEmptyBulkDelete",
			profile: &infrav1.AutoScalerProfile{
				MaxEmptyBulkDelete: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxGracefulTerminationSec",
			profile: &infrav1.AutoScalerProfile{
				MaxGracefulTerminationSec: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxNodeProvisionTime",
			profile: &infrav1.AutoScalerProfile{
				MaxNodeProvisionTime: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxTotalUnreadyPercentage",
			profile: &infrav1.AutoScalerProfile{
				MaxTotalUnreadyPercentage: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.NewPodScaleUpDelay",
			profile: &infrav1.AutoScalerProfile{
				NewPodScaleUpDelay: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.OkTotalUnreadyCount",
			profile: &infrav1.AutoScalerProfile{
				OkTotalUnreadyCount: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScanInterval",
			profile: &infrav1.AutoScalerProfile{
				ScanInterval: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterAdd",
			profile: &infrav1.AutoScalerProfile{
				ScaleDownDelayAfterAdd: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterDelete",
			profile: &infrav1.AutoScalerProfile{
				ScaleDownDelayAfterDelete: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterFailure",
			profile: &infrav1.AutoScalerProfile{
				ScaleDownDelayAfterFailure: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnneededTime",
			profile: &infrav1.AutoScalerProfile{
				ScaleDownUnneededTime: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnreadyTime",
			profile: &infrav1.AutoScalerProfile{
				ScaleDownUnreadyTime: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUtilizationThreshold",
			profile: &infrav1.AutoScalerProfile{
				ScaleDownUtilizationThreshold: ptr.To("invalid"),
			},
			expectErr: true,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageTrue",
			profile: &infrav1.AutoScalerProfile{
				SkipNodesWithLocalStorage: (*infrav1.SkipNodesWithLocalStorage)(ptr.To(string(infrav1.SkipNodesWithLocalStorageTrue))),
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageFalse",
			profile: &infrav1.AutoScalerProfile{
				SkipNodesWithSystemPods: (*infrav1.SkipNodesWithSystemPods)(ptr.To(string(infrav1.SkipNodesWithSystemPodsFalse))),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			allErrs := validateAutoScalerProfile(tt.profile, field.NewPath("spec").Child("autoScalerProfile"))
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
		amcp      infrav1.AzureManagedControlPlane
		expectErr bool
	}{
		{
			name: "Testing valid DNSServiceIP",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.17.8",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid DNSServiceIP",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10.3"),
						Version:      "v1.17.8",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid DNSServiceIP",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.11"),
						Version:      "v1.17.8",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing empty DNSServiceIP",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid Version",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "honk",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "not following the Kubernetes Version pattern",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "1.19.0",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Version not set",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Valid Version",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.17.8",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Valid Managed AADProfile",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						AADProfile: &infrav1.AADProfile{
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
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &infrav1.LoadBalancerProfile{
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
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &infrav1.LoadBalancerProfile{
							ManagedOutboundIPs: ptr.To(200),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid LoadBalancerProfile.AllocatedOutboundPorts",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &infrav1.LoadBalancerProfile{
							AllocatedOutboundPorts: ptr.To(80000),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid LoadBalancerProfile.IdleTimeoutInMinutes",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &infrav1.LoadBalancerProfile{
							IdleTimeoutInMinutes: ptr.To(600),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "LoadBalancerProfile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						LoadBalancerProfile: &infrav1.LoadBalancerProfile{
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
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						APIServerAccessProfile: &infrav1.APIServerAccessProfile{
							AuthorizedIPRanges: []string{"1.2.3.400/32"},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing valid AutoScalerProfile",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							BalanceSimilarNodeGroups:      (*infrav1.BalanceSimilarNodeGroups)(ptr.To(string(infrav1.BalanceSimilarNodeGroupsFalse))),
							Expander:                      (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderRandom))),
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
							SkipNodesWithLocalStorage:     (*infrav1.SkipNodesWithLocalStorage)(ptr.To(string(infrav1.SkipNodesWithLocalStorageTrue))),
							SkipNodesWithSystemPods:       (*infrav1.SkipNodesWithSystemPods)(ptr.To(string(infrav1.SkipNodesWithSystemPodsTrue))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderRandom",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							Expander: (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderRandom))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderLeastWaste",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							Expander: (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderLeastWaste))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderMostPods",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							Expander: (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderMostPods))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderPriority",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							Expander: (*infrav1.Expander)(ptr.To(string(infrav1.ExpanderPriority))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsTrue",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							BalanceSimilarNodeGroups: (*infrav1.BalanceSimilarNodeGroups)(ptr.To(string(infrav1.BalanceSimilarNodeGroupsTrue))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsFalse",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							BalanceSimilarNodeGroups: (*infrav1.BalanceSimilarNodeGroups)(ptr.To(string(infrav1.BalanceSimilarNodeGroupsFalse))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxEmptyBulkDelete",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							MaxEmptyBulkDelete: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxGracefulTerminationSec",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							MaxGracefulTerminationSec: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxNodeProvisionTime",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							MaxNodeProvisionTime: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxTotalUnreadyPercentage",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							MaxTotalUnreadyPercentage: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.NewPodScaleUpDelay",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							NewPodScaleUpDelay: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.OkTotalUnreadyCount",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							OkTotalUnreadyCount: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScanInterval",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							ScanInterval: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterAdd",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							ScaleDownDelayAfterAdd: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterDelete",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							ScaleDownDelayAfterDelete: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterFailure",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							ScaleDownDelayAfterFailure: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnneededTime",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							ScaleDownUnneededTime: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnreadyTime",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							ScaleDownUnreadyTime: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUtilizationThreshold",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							ScaleDownUtilizationThreshold: ptr.To("invalid"),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageTrue",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							SkipNodesWithLocalStorage: (*infrav1.SkipNodesWithLocalStorage)(ptr.To(string(infrav1.SkipNodesWithLocalStorageTrue))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageFalse",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							SkipNodesWithLocalStorage: (*infrav1.SkipNodesWithLocalStorage)(ptr.To(string(infrav1.SkipNodesWithLocalStorageFalse))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithSystemPodsTrue",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							SkipNodesWithSystemPods: (*infrav1.SkipNodesWithSystemPods)(ptr.To(string(infrav1.SkipNodesWithSystemPodsTrue))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithSystemPodsFalse",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						AutoScalerProfile: &infrav1.AutoScalerProfile{
							SkipNodesWithSystemPods: (*infrav1.SkipNodesWithSystemPods)(ptr.To(string(infrav1.SkipNodesWithSystemPodsFalse))),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid Identity: SystemAssigned",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						Identity: &infrav1.Identity{
							Type: infrav1.ManagedControlPlaneIdentityTypeSystemAssigned,
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid Identity: UserAssigned",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "/resource/id",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid Identity: SystemAssigned with UserAssigned values",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeSystemAssigned,
							UserAssignedIdentityResourceID: "/resource/id",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid Identity: UserAssigned with missing properties",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.24.1",
						Identity: &infrav1.Identity{
							Type: infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "overlay cannot be used with kubenet",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:           "v1.24.1",
						NetworkPlugin:     ptr.To("kubenet"),
						NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "overlay can be used with azure",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:           "v1.24.1",
						NetworkPlugin:     ptr.To("azure"),
						NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AKS Extension",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Extensions: []infrav1.AKSExtension{
							{
								Name:          "extension1",
								ExtensionType: ptr.To("test-type"),
								Plan: &infrav1.ExtensionPlan{
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
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Extensions: []infrav1.AKSExtension{
							{
								Name:                    "extension1",
								ExtensionType:           ptr.To("test-type"),
								Version:                 ptr.To("1.0.0"),
								AutoUpgradeMinorVersion: ptr.To(true),
								Plan: &infrav1.ExtensionPlan{
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
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Extensions: []infrav1.AKSExtension{
							{
								Name:                    "extension1",
								ExtensionType:           ptr.To("test-type"),
								Version:                 ptr.To("1.0.0"),
								AutoUpgradeMinorVersion: ptr.To(true),
								Plan: &infrav1.ExtensionPlan{
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
			amcp: infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPrivate),
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Valid NetworkDataplane: cilium",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:           "v1.17.8",
						NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
						NetworkDataplane:  ptr.To(infrav1.NetworkDataplaneTypeCilium),
						NetworkPolicy:     ptr.To("cilium"),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid NetworkDataplane: cilium dataplane requires overlay network plugin mode",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:           "v1.17.8",
						NetworkPluginMode: nil,
						NetworkDataplane:  ptr.To(infrav1.NetworkDataplaneTypeCilium),
						NetworkPolicy:     ptr.To("cilium"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Test valid AzureKeyVaultKms",
			amcp: infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPrivate),
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
			amcp: infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid NetworkDataplane: cilium dataplane requires network policy to be cilium",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:           "v1.17.8",
						NetworkPluginMode: nil,
						NetworkDataplane:  ptr.To(infrav1.NetworkDataplaneTypeCilium),
						NetworkPolicy:     ptr.To("azure"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid NetworkPolicy: cilium network policy can only be used with cilium network dataplane",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:           "v1.17.8",
						NetworkPluginMode: nil,
						NetworkDataplane:  ptr.To(infrav1.NetworkDataplaneTypeAzure),
						NetworkPolicy:     ptr.To("cilium"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing valid FleetsMember",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					FleetsMember: &infrav1.FleetsMember{
						Name: "fleetmember1",
					},
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid FleetsMember: Fleets member name cannot contain capital letters",
			amcp: infrav1.AzureManagedControlPlane{
				ObjectMeta: getAMCPMetaData(),
				Spec: infrav1.AzureManagedControlPlaneSpec{
					FleetsMember: &infrav1.FleetsMember{
						Name: "FleetMember1",
					},
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		// client is used to fetch the AzureManagedControlPlane, we do not want to return an error on client.Get
		client := mockClient{ReturnError: false}
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			mcpw := &AzureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateCreate(t.Context(), &tt.amcp)
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
		amcp     *infrav1.AzureManagedControlPlane
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
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("-thisi$"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Testing inValid DNSPrefix with more then 54 characters",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("thisisaverylong$^clusternameconsistingofmorethan54characterswhichshouldbeinvalid"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Testing inValid DNSPrefix with underscore",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("no_underscore"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Testing inValid DNSPrefix with special characters",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("no-dollar$@%"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Testing Valid DNSPrefix with hyphen characters",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("hyphen-allowed"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Testing Valid DNSPrefix with hyphen characters",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("palette-test07"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Testing valid DNSPrefix ",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("thisisavlerylongclu7l0sternam3leconsistingofmorethan54"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid name with microsoft",
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "microsoft-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
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
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "a-windows-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
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
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
						Host: "my-host",
					},
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						AADProfile: &infrav1.AADProfile{
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
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
						Port: 444,
					},
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						AADProfile: &infrav1.AADProfile{
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
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:              "v1.21.2",
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "DisableLocalAccounts can be set for AAD clusters",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.21.2",
						AADProfile: &infrav1.AADProfile{
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
			mcpw := &AzureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateCreate(t.Context(), tc.amcp)
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
		name               string
		amcp               *infrav1.AzureManagedControlPlane
		featureGateEnabled *bool
		expectError        bool
	}{
		{
			name:               "feature gate implicitly enabled",
			amcp:               getKnownValidAzureManagedControlPlane(),
			featureGateEnabled: nil,
			expectError:        false,
		},
	}
	client := mockClient{ReturnError: false}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			if tc.featureGateEnabled != nil {
				utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, *tc.featureGateEnabled)
			}
			mcpw := &AzureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateCreate(t.Context(), tc.amcp)
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
		oldAMCP *infrav1.AzureManagedControlPlane
		amcp    *infrav1.AzureManagedControlPlane
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AddonProfiles: []infrav1.AddonProfile{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						AddonProfiles: []infrav1.AddonProfile{
							{
								Name:    "first-addon-profile",
								Enabled: true,
							},
						},
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AddonProfiles: []infrav1.AddonProfile{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						AddonProfiles: []infrav1.AddonProfile{
							{
								Name:    "first-addon-profile",
								Enabled: true,
							},
						},
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane AddonProfiles cannot be completely removed",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						AddonProfiles: []infrav1.AddonProfile{
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
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						AddonProfiles: []infrav1.AddonProfile{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane invalid version downgrade change",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
				Status: infrav1.AzureManagedControlPlaneStatus{
					AutoUpgradeVersion: "v1.18.3",
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane Autoupgrade cannot be set to nil",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
						AutoUpgradeProfile: &infrav1.ManagedClusterAutoUpgradeProfile{
							UpgradeChannel: ptr.To(infrav1.UpgradeChannelStable),
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
						AutoUpgradeProfile: &infrav1.ManagedClusterAutoUpgradeProfile{
							UpgradeChannel: ptr.To(infrav1.UpgradeChannelStable),
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:       ptr.To("192.168.0.10"),
						SubscriptionID:     "212ec1q8",
						Version:            "v1.18.0",
						AutoUpgradeProfile: &infrav1.ManagedClusterAutoUpgradeProfile{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane Autoupgrade is mutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
						AutoUpgradeProfile: &infrav1.ManagedClusterAutoUpgradeProfile{
							UpgradeChannel: ptr.To(infrav1.UpgradeChannelStable),
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
						AutoUpgradeProfile: &infrav1.ManagedClusterAutoUpgradeProfile{
							UpgradeChannel: ptr.To(infrav1.UpgradeChannelNone),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane SubscriptionID is immutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:   ptr.To("192.168.0.10"),
						SubscriptionID: "212ec1q8",
						Version:        "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:      ptr.To("192.168.0.10"),
						Version:           "v1.18.0",
						ResourceGroupName: "hello-1",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:      ptr.To("192.168.0.10"),
						Version:           "v1.18.0",
						ResourceGroupName: "hello-2",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NodeResourceGroupName is immutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
					NodeResourceGroupName: "hello-1",
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Location:     "westeurope",
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
					SSHPublicKey: ptr.To(generateSSHPublicKey(true)),
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.1.1"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSServiceIP is immutable, unsetting is not allowed",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPlugin is immutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPlugin: ptr.To("azure"),
						Version:       "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPlugin: ptr.To("azure"),
						Version:       "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPolicy is immutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPolicy: ptr.To("azure"),
						Version:       "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:  ptr.To("192.168.0.10"),
						NetworkPolicy: ptr.To("azure"),
						Version:       "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPolicy is immutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:     ptr.To("192.168.0.10"),
						NetworkDataplane: ptr.To(infrav1.NetworkDataplaneTypeCilium),
						Version:          "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:     ptr.To("192.168.0.10"),
						NetworkDataplane: ptr.To(infrav1.NetworkDataplaneTypeAzure),
						Version:          "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkDataplane is immutable, unsetting is not allowed",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:     ptr.To("192.168.0.10"),
						NetworkDataplane: ptr.To(infrav1.NetworkDataplaneTypeCilium),
						Version:          "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane LoadBalancerSKU is immutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:    ptr.To("192.168.0.10"),
						LoadBalancerSKU: ptr.To("Standard"),
						Version:         "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:    ptr.To("192.168.0.10"),
						LoadBalancerSKU: ptr.To(infrav1.LoadBalancerSKUBasic),
						Version:         "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane LoadBalancerSKU is immutable, unsetting is not allowed",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP:    ptr.To("192.168.0.10"),
						LoadBalancerSKU: ptr.To(infrav1.LoadBalancerSKUStandard),
						Version:         "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane ManagedAad can be set after cluster creation",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:    "v1.18.0",
						AADProfile: &infrav1.AADProfile{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane managed field cannot set to false",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
							Managed: true,
							AdminGroupObjectIDs: []string{
								"616077a8-5db7-4c98-b856-b34619afg75h",
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane EnablePrivateCluster is immutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						APIServerAccessProfile: &infrav1.APIServerAccessProfile{
							APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						APIServerAccessProfile: &infrav1.APIServerAccessProfile{
							AuthorizedIPRanges: []string{"192.168.0.1/32"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane.VirtualNetwork Name is mutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							Name: "test-network",
							ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
								CIDRBlock: "10.0.0.0/8",
								Subnet: infrav1.ManagedControlPlaneSubnet{
									Name:      "test-subnet",
									CIDRBlock: "10.0.2.0/24",
								},
							},
							ResourceGroup: "test-rg",
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane.VirtualNetwork Name is mutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							Name: "test-network",
							ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
								CIDRBlock: "10.0.0.0/8",
								Subnet: infrav1.ManagedControlPlaneSubnet{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							Name: "test-network",
							ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
								CIDRBlock: "10.0.0.0/8",
								Subnet: infrav1.ManagedControlPlaneSubnet{
									Name:      "test-subnet",
									CIDRBlock: "10.0.2.0/24",
								},
							},
							ResourceGroup: "test-rg",
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						DNSServiceIP: ptr.To("192.168.0.10"),
						Version:      "v1.18.0",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							Name: "test-network",
							ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
								CIDRBlock: "10.0.0.0/8",
								Subnet: infrav1.ManagedControlPlaneSubnet{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						OutboundType: (*infrav1.ManagedControlPlaneOutboundType)(ptr.To(string(infrav1.ManagedControlPlaneOutboundTypeUserDefinedRouting))),
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						OutboundType: (*infrav1.ManagedControlPlaneOutboundType)(ptr.To(string(infrav1.ManagedControlPlaneOutboundTypeLoadBalancer))),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane HTTPProxyConfig is immutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						HTTPProxyConfig: &infrav1.HTTPProxyConfig{
							HTTPProxy:  ptr.To("http://1.2.3.4:8080"),
							HTTPSProxy: ptr.To("https://5.6.7.8:8443"),
							NoProxy:    []string{"endpoint1", "endpoint2"},
							TrustedCA:  ptr.To("ca"),
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						HTTPProxyConfig: &infrav1.HTTPProxyConfig{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     ptr.To("anything"),
						NetworkPluginMode: nil,
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     ptr.To("anything"),
						NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "NetworkPluginMode can change to \"overlay\" when NetworkPolicy is not set",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     nil,
						NetworkPluginMode: nil,
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:           "v0.0.0",
						NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "NetworkPolicy is allowed when NetworkPluginMode is not changed",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     ptr.To("anything"),
						NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						NetworkPolicy:     ptr.To("anything"),
						Version:           "v0.0.0",
						NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane OIDCIssuerProfile.Enabled false -> false OK",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v0.0.0",
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane OIDCIssuerProfile.Enabled false -> true OK",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v0.0.0",
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane OIDCIssuerProfile.Enabled true -> false err",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v0.0.0",
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane OIDCIssuerProfile.Enabled true -> true OK",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v0.0.0",
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable error",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks-1"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable no error",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "DisableLocalAccounts can be set only for AAD enabled clusters",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
							Managed:             true,
							AdminGroupObjectIDs: []string{"00000000-0000-0000-0000-000000000000"},
						},
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						AADProfile: &infrav1.AADProfile{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix can be updated from nil when resource name matches",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capz-aks",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To("capz-aks"),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "DisableLocalAccounts cannot be set for non AAD clusters",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:              "v1.18.0",
						DisableLocalAccounts: ptr.To[bool](true),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable error nil -> empty",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: ptr.To(""),
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSPrefix is immutable no error nil -> nil",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					DNSPrefix: nil,
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedControlPlane AKSExtensions ConfigurationSettings and AutoUpgradeMinorVersion are mutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Extensions: []infrav1.AKSExtension{
							{
								Name:                    "extension1",
								AutoUpgradeMinorVersion: ptr.To(false),
								ConfigurationSettings: map[string]string{
									"key1": "value1",
								},
								Plan: &infrav1.ExtensionPlan{
									Name:      "planName",
									Product:   "planProduct",
									Publisher: "planPublisher",
								},
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Extensions: []infrav1.AKSExtension{
							{
								Name:                    "extension1",
								AutoUpgradeMinorVersion: ptr.To(true),
								ConfigurationSettings: map[string]string{
									"key1": "value1",
									"key2": "value2",
								},
								Plan: &infrav1.ExtensionPlan{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Extensions: []infrav1.AKSExtension{
							{
								Name:                    "extension1",
								AKSAssignedIdentityType: infrav1.AKSAssignedIdentitySystemAssigned,
								ExtensionType:           ptr.To("extensionType"),
								Plan: &infrav1.ExtensionPlan{
									Name:      "planName",
									Product:   "planProduct",
									Publisher: "planPublisher",
								},
								Scope: &infrav1.ExtensionScope{
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
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Extensions: []infrav1.AKSExtension{
							{
								Name:                    "extension2",
								AKSAssignedIdentityType: infrav1.AKSAssignedIdentityUserAssigned,
								ExtensionType:           ptr.To("extensionType1"),
								Plan: &infrav1.ExtensionPlan{
									Name:      "planName1",
									Product:   "planProduct1",
									Publisher: "planPublisher1",
								},
								Scope: &infrav1.ExtensionScope{
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
			mcpw := &AzureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateUpdate(t.Context(), tc.oldAMCP, tc.amcp)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createAzureManagedControlPlane(serviceIP, version, sshKey string) *infrav1.AzureManagedControlPlane {
	return &infrav1.AzureManagedControlPlane{
		ObjectMeta: getAMCPMetaData(),
		Spec: infrav1.AzureManagedControlPlaneSpec{
			SSHPublicKey: &sshKey,
			AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
				DNSServiceIP: ptr.To(serviceIP),
				Version:      version,
			},
		},
	}
}

func getKnownValidAzureManagedControlPlane() *infrav1.AzureManagedControlPlane {
	return &infrav1.AzureManagedControlPlane{
		ObjectMeta: getAMCPMetaData(),
		Spec: infrav1.AzureManagedControlPlaneSpec{
			AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
				DNSServiceIP: ptr.To("192.168.0.10"),
				Version:      "v1.18.0",
				AADProfile: &infrav1.AADProfile{
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
		amcp    *infrav1.AzureManagedControlPlane
		wantErr string
	}{
		{
			name: "Cannot enable Workload Identity without enabling OIDC issuer",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							WorkloadIdentity: &infrav1.ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
						},
					},
				},
			},
			wantErr: "spec.securityProfile.workloadIdentity: Invalid value: {\"enabled\":true}: Spec.SecurityProfile.WorkloadIdentity cannot be enabled when Spec.OIDCIssuerProfile is disabled",
		},
		{
			name: "Cannot enable AzureKms without user assigned identity",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled: true,
							},
						},
					},
				},
			},
			wantErr: "spec.securityProfile.azureKeyVaultKms.keyVaultResourceID: Invalid value: null: Spec.SecurityProfile.AzureKeyVaultKms can be set only when Spec.Identity.Type is UserAssigned",
		},
		{
			name: "When AzureKms.KeyVaultNetworkAccess is private AzureKeyVaultKms.KeyVaultResourceID cannot be empty",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						Version: "v1.17.8",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "not empty",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPrivate),
							},
						},
					},
				},
			},
			wantErr: "spec.securityProfile.azureKeyVaultKms.keyVaultResourceID: Invalid value: null: Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID cannot be empty when Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultNetworkAccess is Private",
		},
		{
			name: "When AzureKms.KeyVaultNetworkAccess is public AzureKeyVaultKms.KeyVaultResourceID should be empty",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "not empty",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPublic),
								KeyVaultResourceID:    ptr.To("not empty"),
							},
						},
					},
				},
			},
			wantErr: "spec.securityProfile.azureKeyVaultKms.keyVaultResourceID: Invalid value: \"not empty\": Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID should be empty when Spec.SecurityProfile.AzureKeyVaultKms.KeyVaultNetworkAccess is Public",
		},
		{
			name: "Valid profile",
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.17.8",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "not empty",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPublic),
							},
							Defender: &infrav1.ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "not empty",
								SecurityMonitoring: infrav1.ManagedClusterSecurityProfileDefenderSecurityMonitoring{
									Enabled: true,
								},
							},
							WorkloadIdentity: &infrav1.ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
							ImageCleaner: &infrav1.ManagedClusterSecurityProfileImageCleaner{
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
			mcpw := &AzureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateCreate(t.Context(), tc.amcp)
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
		oldAMCP *infrav1.AzureManagedControlPlane
		amcp    *infrav1.AzureManagedControlPlane
		wantErr string
	}{
		{
			name: "AzureManagedControlPlane SecurityProfile.Defender is mutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							Defender: &infrav1.ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "0000-0000-0000-0000",
								SecurityMonitoring: infrav1.ManagedClusterSecurityProfileDefenderSecurityMonitoring{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							Defender: &infrav1.ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "0000-0000-0000-0000",
								SecurityMonitoring: infrav1.ManagedClusterSecurityProfileDefenderSecurityMonitoring{
									Enabled: true,
								},
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			wantErr: "AzureManagedControlPlane.infrastructure.cluster.x-k8s.io \"\" is invalid: spec.securityProfile.defender: Invalid value: null: cannot unset Spec.SecurityProfile.Defender, to disable defender please set Spec.SecurityProfile.Defender.SecurityMonitoring.Enabled to false",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.Defender is mutable and can be disabled",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							Defender: &infrav1.ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "0000-0000-0000-0000",
								SecurityMonitoring: infrav1.ManagedClusterSecurityProfileDefenderSecurityMonitoring{
									Enabled: true,
								},
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							Defender: &infrav1.ManagedClusterSecurityProfileDefender{
								LogAnalyticsWorkspaceResourceID: "0000-0000-0000-0000",
								SecurityMonitoring: infrav1.ManagedClusterSecurityProfileDefenderSecurityMonitoring{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							WorkloadIdentity: &infrav1.ManagedClusterSecurityProfileWorkloadIdentity{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							WorkloadIdentity: &infrav1.ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
						},
					},
				},
			},
			wantErr: "spec.securityProfile.workloadIdentity: Invalid value: {\"enabled\":true}: Spec.SecurityProfile.WorkloadIdentity cannot be enabled when Spec.OIDCIssuerProfile is disabled",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.WorkloadIdentity cannot unset values",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							WorkloadIdentity: &infrav1.ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			wantErr: "AzureManagedControlPlane.infrastructure.cluster.x-k8s.io \"\" is invalid: spec.securityProfile.workloadIdentity: Invalid value: null: cannot unset Spec.SecurityProfile.WorkloadIdentity, to disable workloadIdentity please set Spec.SecurityProfile.WorkloadIdentity.Enabled to false",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.WorkloadIdentity can be disabled",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							WorkloadIdentity: &infrav1.ManagedClusterSecurityProfileWorkloadIdentity{
								Enabled: true,
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						OIDCIssuerProfile: &infrav1.OIDCIssuerProfile{
							Enabled: ptr.To(true),
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							WorkloadIdentity: &infrav1.ManagedClusterSecurityProfileWorkloadIdentity{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPrivate),
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPrivate),
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               false,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.AzureKeyVaultKms cannot unset",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPublic),
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						Identity: &infrav1.Identity{
							Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
							UserAssignedIdentityResourceID: "not empty",
						},
					},
				},
			},
			wantErr: "AzureManagedControlPlane.infrastructure.cluster.x-k8s.io \"\" is invalid: spec.securityProfile.azureKeyVaultKms: Invalid value: null: cannot unset Spec.SecurityProfile.AzureKeyVaultKms profile to disable the profile please set Spec.SecurityProfile.AzureKeyVaultKms.Enabled to false",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.AzureKeyVaultKms cannot be enabled without UserAssigned Identity",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							AzureKeyVaultKms: &infrav1.AzureKeyVaultKms{
								Enabled:               true,
								KeyID:                 "0000-0000-0000-0000",
								KeyVaultNetworkAccess: ptr.To(infrav1.KeyVaultNetworkAccessTypesPrivate),
								KeyVaultResourceID:    ptr.To("0000-0000-0000-0000"),
							},
						},
					},
				},
			},
			wantErr: "spec.securityProfile.azureKeyVaultKms.keyVaultResourceID: Invalid value: \"0000-0000-0000-0000\": Spec.SecurityProfile.AzureKeyVaultKms can be set only when Spec.Identity.Type is UserAssigned",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.ImageCleaner is mutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							ImageCleaner: &infrav1.ManagedClusterSecurityProfileImageCleaner{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							ImageCleaner: &infrav1.ManagedClusterSecurityProfileImageCleaner{
								Enabled:       true,
								IntervalHours: ptr.To(48),
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version:         "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{},
					},
				},
			},
			wantErr: "AzureManagedControlPlane.infrastructure.cluster.x-k8s.io \"\" is invalid: spec.securityProfile.imageCleaner: Invalid value: null: cannot unset Spec.SecurityProfile.ImageCleaner, to disable imageCleaner please set Spec.SecurityProfile.ImageCleaner.Enabled to false",
		},
		{
			name: "AzureManagedControlPlane SecurityProfile.ImageCleaner is mutable",
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							ImageCleaner: &infrav1.ManagedClusterSecurityProfileImageCleaner{
								Enabled: true,
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							ImageCleaner: &infrav1.ManagedClusterSecurityProfileImageCleaner{
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
			oldAMCP: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							ImageCleaner: &infrav1.ManagedClusterSecurityProfileImageCleaner{
								Enabled:       true,
								IntervalHours: ptr.To(48),
							},
						},
					},
				},
			},
			amcp: &infrav1.AzureManagedControlPlane{
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Version: "v1.18.0",
						SecurityProfile: &infrav1.ManagedClusterSecurityProfile{
							ImageCleaner: &infrav1.ManagedClusterSecurityProfileImageCleaner{
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
			mcpw := &AzureManagedControlPlaneWebhook{
				Client: client,
			}
			_, err := mcpw.ValidateUpdate(t.Context(), tc.oldAMCP, tc.amcp)
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
		profile   *infrav1.APIServerAccessProfile
		expectErr bool
	}{
		{
			name: "Testing valid PrivateDNSZone:System",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					PrivateDNSZone: ptr.To("System"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid PrivateDNSZone:None",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					PrivateDNSZone: ptr.To("None"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid PrivateDNSZone:With privatelink region",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid PrivateDNSZone:With private region",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/private.eastus.azmk8s.io"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid EnablePrivateCluster and valid PrivateDNSZone",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(false),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/private.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing valid PrivateDNSZone:With privatelink region and sub-region",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/sublocation2.privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid PrivateDNSZone:With private region and sub-region",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/sublocation2.private.eastus.azmk8s.io"),
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid PrivateDNSZone: privatelink region: len(sub-region) > 32 characters",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/thissublocationismorethan32characters.privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: private region: len(sub-region) > 32 characters",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/thissublocationismorethan32characters.private.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: random string",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("WrongPrivateDNSZone"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: subzone has an invalid char %",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/subzone%1.privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: subzone has an invalid char _",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
					EnablePrivateCluster: ptr.To(true),
					PrivateDNSZone:       ptr.To("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg1/providers/Microsoft.Network/privateDnsZones/subzone_1.privatelink.eastus.azmk8s.io"),
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid PrivateDNSZone: region has invalid char",
			profile: &infrav1.APIServerAccessProfile{
				APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
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
		amcp    *infrav1.AzureManagedControlPlane
		wantErr string
	}{
		{
			name: "Testing valid VirtualNetwork in same resource group",
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						ResourceGroupName: "rg1",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg1",
							Name:          "vnet1",
							ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
								CIDRBlock: apiinternal.DefaultAKSVnetCIDR,
								Subnet: infrav1.ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: apiinternal.DefaultAKSNodeSubnetCIDR,
								},
							},
						},
						SKU: &infrav1.AKSSku{},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "Testing valid VirtualNetwork in different resource group",
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						ResourceGroupName: "rg1",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg2",
							Name:          "vnet1",
							ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
								CIDRBlock: apiinternal.DefaultAKSVnetCIDR,
								Subnet: infrav1.ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: apiinternal.DefaultAKSNodeSubnetCIDR,
								},
							},
						},
						SKU: &infrav1.AKSSku{},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "Testing invalid VirtualNetwork in different resource group: invalid subnet CIDR",
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						ResourceGroupName: "rg1",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg2",
							Name:          "vnet1",
							ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
								CIDRBlock: "10.1.0.0/16",
								Subnet: infrav1.ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: "10.0.0.0/24",
								},
							},
						},
						SKU: &infrav1.AKSSku{},
					},
				},
			},
			wantErr: "pre-existing virtual networks CIDR block should contain the subnet CIDR block",
		},
		{
			name: "Testing invalid VirtualNetwork in different resource group: invalid VNet CIDR",
			amcp: &infrav1.AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooName",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "fooCluster",
					},
				},
				Spec: infrav1.AzureManagedControlPlaneSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						ResourceGroupName: "rg1",
						VirtualNetwork: infrav1.ManagedControlPlaneVirtualNetwork{
							ResourceGroup: "rg2",
							Name:          "vnet1",
							ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
								CIDRBlock: "invalid_vnet_CIDR",
								Subnet: infrav1.ManagedControlPlaneSubnet{
									Name:      "subnet1",
									CIDRBlock: "11.0.0.0/24",
								},
							},
						},
						SKU: &infrav1.AKSSku{},
					},
				},
			},
			wantErr: "pre-existing virtual networks CIDR block is invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mcpw := &AzureManagedControlPlaneWebhook{}
			err := mcpw.Default(t.Context(), tc.amcp)
			g.Expect(err).NotTo(HaveOccurred())

			errs := validateAMCPVirtualNetwork(tc.amcp.Spec.VirtualNetwork, field.NewPath("spec", "virtualNetwork"))
			if tc.wantErr != "" {
				g.Expect(errs).ToNot(BeEmpty())
				g.Expect(errs[0].Detail).To(Equal(tc.wantErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

type mockClient struct {
	client.Client
	ReturnError bool
}

func (m mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if m.ReturnError {
		return errors.New("AzureCluster not found: failed to find owner cluster for test-cluster")
	}
	// Check if we're calling Get on an AzureCluster or a Cluster
	switch obj := obj.(type) {
	case *infrav1.AzureCluster:
		obj.Spec.SubscriptionID = "test-subscription-id"
	case *clusterv1.Cluster:
		obj.Namespace = "default"
		obj.Spec = clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				Kind: infrav1.AzureClusterKind,
				Name: "test-cluster",
			},
			ClusterNetwork: clusterv1.ClusterNetwork{
				Services: clusterv1.NetworkRanges{
					CIDRBlocks: []string{"192.168.0.0/26"},
				},
			},
		}
	default:
		return errors.New("unexpected object type")
	}

	return nil
}
