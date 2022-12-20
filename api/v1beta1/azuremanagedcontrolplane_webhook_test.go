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
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/pointer"
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
		},
	}
	amcp.Default(nil)
	g.Expect(*amcp.Spec.NetworkPlugin).To(Equal("azure"))
	g.Expect(*amcp.Spec.LoadBalancerSKU).To(Equal("Standard"))
	g.Expect(amcp.Spec.Version).To(Equal("v1.17.5"))
	g.Expect(amcp.Spec.SSHPublicKey).NotTo(BeEmpty())
	g.Expect(amcp.Spec.NodeResourceGroupName).To(Equal("MC_fooRg_fooName_fooLocation"))
	g.Expect(amcp.Spec.VirtualNetwork.Name).To(Equal("fooName"))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooName"))
	g.Expect(amcp.Spec.SKU.Tier).To(Equal(FreeManagedControlPlaneTier))

	t.Logf("Testing amcp defaulting webhook with baseline")
	netPlug := "kubenet"
	lbSKU := "Basic"
	netPol := "azure"
	amcp.Spec.NetworkPlugin = &netPlug
	amcp.Spec.LoadBalancerSKU = &lbSKU
	amcp.Spec.NetworkPolicy = &netPol
	amcp.Spec.Version = "9.99.99"
	amcp.Spec.SSHPublicKey = ""
	amcp.Spec.NodeResourceGroupName = "fooNodeRg"
	amcp.Spec.VirtualNetwork.Name = "fooVnetName"
	amcp.Spec.VirtualNetwork.Subnet.Name = "fooSubnetName"
	amcp.Spec.SKU.Tier = PaidManagedControlPlaneTier

	amcp.Default(nil)
	g.Expect(*amcp.Spec.NetworkPlugin).To(Equal(netPlug))
	g.Expect(*amcp.Spec.LoadBalancerSKU).To(Equal(lbSKU))
	g.Expect(*amcp.Spec.NetworkPolicy).To(Equal(netPol))
	g.Expect(amcp.Spec.Version).To(Equal("v9.99.99"))
	g.Expect(amcp.Spec.SSHPublicKey).NotTo(BeEmpty())
	g.Expect(amcp.Spec.NodeResourceGroupName).To(Equal("fooNodeRg"))
	g.Expect(amcp.Spec.VirtualNetwork.Name).To(Equal("fooVnetName"))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooSubnetName"))
	g.Expect(amcp.Spec.SKU.Tier).To(Equal(PaidManagedControlPlaneTier))
}

func TestValidatingWebhook(t *testing.T) {
	// NOTE: AzureManageControlPlane is behind AKS feature gate flag; the webhook
	// must prevent creating new objects in case the feature flag is disabled.
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()
	g := NewWithT(t)
	tests := []struct {
		name      string
		amcp      AzureManagedControlPlane
		expectErr bool
	}{
		{
			name: "Testing valid DNSServiceIP",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "v1.17.8",
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid DNSServiceIP",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0.3"),
					Version:      "v1.17.8",
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid Version",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "honk",
				},
			},
			expectErr: true,
		},
		{
			name: "not following the Kubernetes Version pattern",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "1.19.0",
				},
			},
			expectErr: true,
		},
		{
			name: "Version not set",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "",
				},
			},
			expectErr: true,
		},
		{
			name: "Valid Version",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "v1.17.8",
				},
			},
			expectErr: false,
		},
		{
			name: "Valid Managed AADProfile",
			amcp: AzureManagedControlPlane{
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
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						ManagedOutboundIPs:     to.Int32Ptr(10),
						AllocatedOutboundPorts: to.Int32Ptr(1000),
						IdleTimeoutInMinutes:   to.Int32Ptr(60),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid LoadBalancerProfile.ManagedOutboundIPs",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						ManagedOutboundIPs: to.Int32Ptr(200),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid LoadBalancerProfile.AllocatedOutboundPorts",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						AllocatedOutboundPorts: to.Int32Ptr(80000),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid LoadBalancerProfile.IdleTimeoutInMinutes",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						IdleTimeoutInMinutes: to.Int32Ptr(600),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "LoadBalancerProfile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.21.2",
					LoadBalancerProfile: &LoadBalancerProfile{
						ManagedOutboundIPs: to.Int32Ptr(1),
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
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						BalanceSimilarNodeGroups:      (*BalanceSimilarNodeGroups)(to.StringPtr(string(BalanceSimilarNodeGroupsFalse))),
						Expander:                      (*Expander)(to.StringPtr(string(ExpanderRandom))),
						MaxEmptyBulkDelete:            to.StringPtr("10"),
						MaxGracefulTerminationSec:     to.StringPtr("600"),
						MaxNodeProvisionTime:          to.StringPtr("10m"),
						MaxTotalUnreadyPercentage:     to.StringPtr("45"),
						NewPodScaleUpDelay:            to.StringPtr("10m"),
						OkTotalUnreadyCount:           to.StringPtr("3"),
						ScanInterval:                  to.StringPtr("60s"),
						ScaleDownDelayAfterAdd:        to.StringPtr("10m"),
						ScaleDownDelayAfterDelete:     to.StringPtr("10s"),
						ScaleDownDelayAfterFailure:    to.StringPtr("10m"),
						ScaleDownUnneededTime:         to.StringPtr("10m"),
						ScaleDownUnreadyTime:          to.StringPtr("10m"),
						ScaleDownUtilizationThreshold: to.StringPtr("0.5"),
						SkipNodesWithLocalStorage:     (*SkipNodesWithLocalStorage)(to.StringPtr(string(SkipNodesWithLocalStorageTrue))),
						SkipNodesWithSystemPods:       (*SkipNodesWithSystemPods)(to.StringPtr(string(SkipNodesWithSystemPodsTrue))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderRandom",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						Expander: (*Expander)(to.StringPtr(string(ExpanderRandom))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderLeastWaste",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						Expander: (*Expander)(to.StringPtr(string(ExpanderLeastWaste))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderMostPods",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						Expander: (*Expander)(to.StringPtr(string(ExpanderMostPods))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.ExpanderPriority",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						Expander: (*Expander)(to.StringPtr(string(ExpanderPriority))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsTrue",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						BalanceSimilarNodeGroups: (*BalanceSimilarNodeGroups)(to.StringPtr(string(BalanceSimilarNodeGroupsTrue))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.BalanceSimilarNodeGroupsFalse",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						BalanceSimilarNodeGroups: (*BalanceSimilarNodeGroups)(to.StringPtr(string(BalanceSimilarNodeGroupsFalse))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxEmptyBulkDelete",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						MaxEmptyBulkDelete: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxGracefulTerminationSec",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						MaxGracefulTerminationSec: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxNodeProvisionTime",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						MaxNodeProvisionTime: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.MaxTotalUnreadyPercentage",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						MaxTotalUnreadyPercentage: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.NewPodScaleUpDelay",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						NewPodScaleUpDelay: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.OkTotalUnreadyCount",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						OkTotalUnreadyCount: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScanInterval",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScanInterval: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterAdd",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownDelayAfterAdd: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterDelete",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownDelayAfterDelete: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownDelayAfterFailure",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownDelayAfterFailure: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnneededTime",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownUnneededTime: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUnreadyTime",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownUnreadyTime: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing invalid AutoScalerProfile.ScaleDownUtilizationThreshold",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						ScaleDownUtilizationThreshold: to.StringPtr("invalid"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageTrue",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						SkipNodesWithLocalStorage: (*SkipNodesWithLocalStorage)(to.StringPtr(string(SkipNodesWithLocalStorageTrue))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithLocalStorageFalse",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						SkipNodesWithLocalStorage: (*SkipNodesWithLocalStorage)(to.StringPtr(string(SkipNodesWithLocalStorageFalse))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithSystemPodsTrue",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						SkipNodesWithSystemPods: (*SkipNodesWithSystemPods)(to.StringPtr(string(SkipNodesWithSystemPodsTrue))),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Testing valid AutoScalerProfile.SkipNodesWithSystemPodsFalse",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					Version: "v1.24.1",
					AutoScalerProfile: &AutoScalerProfile{
						SkipNodesWithSystemPods: (*SkipNodesWithSystemPods)(to.StringPtr(string(SkipNodesWithSystemPodsFalse))),
					},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectErr {
				g.Expect(tt.amcp.ValidateCreate(nil)).NotTo(Succeed())
			} else {
				g.Expect(tt.amcp.ValidateCreate(nil)).To(Succeed())
			}
		})
	}
}

func TestAzureManagedControlPlane_ValidateCreate(t *testing.T) {
	// NOTE: AzureManageControlPlane is behind AKS feature gate flag; the webhook
	// must prevent creating new objects in case the feature flag is disabled.
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()
	g := NewWithT(t)

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
			amcp:     createAzureManagedControlPlane("192.168.0.0.3", "v1.18.0", generateSSHPublicKey(true)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name:     "invalid sshKey",
			amcp:     createAzureManagedControlPlane("192.168.0.0", "v1.18.0", generateSSHPublicKey(false)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name:     "invalid sshKey with a simple text and invalid DNSServiceIP",
			amcp:     createAzureManagedControlPlane("192.168.0.0.3", "v1.18.0", "invalid_sshkey_honk"),
			wantErr:  true,
			errorLen: 2,
		},
		{
			name:     "invalid version",
			amcp:     createAzureManagedControlPlane("192.168.0.0", "honk.version", generateSSHPublicKey(true)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "invalid name with microsoft",
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "microsoft-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					SSHPublicKey: generateSSHPublicKey(true),
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
					SSHPublicKey: generateSSHPublicKey(true),
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.23.5",
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "can't set Spec.ControlPlaneEndpoint.Host during create",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "my-host",
					},
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.18.0",
					SSHPublicKey: generateSSHPublicKey(true),
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "can't set Spec.ControlPlaneEndpoint.Port during create",
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Port: 444,
					},
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.18.0",
					SSHPublicKey: generateSSHPublicKey(true),
					AADProfile: &AADProfile{
						Managed: true,
						AdminGroupObjectIDs: []string{
							"616077a8-5db7-4c98-b856-b34619afg75h",
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.amcp.ValidateCreate(nil)
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
	g := NewWithT(t)

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
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.deferFunc()
			err := tc.amcp.ValidateCreate(nil)
			g.Expect(err).To(HaveOccurred())
		})
	}
}

func TestAzureManagedControlPlane_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)
	commonSSHKey := generateSSHPublicKey(true)

	tests := []struct {
		name    string
		oldAMCP *AzureManagedControlPlane
		amcp    *AzureManagedControlPlane
		wantErr bool
	}{
		{
			name:    "can't add a SSHPublicKey to an existing AzureManagedControlPlane",
			oldAMCP: createAzureManagedControlPlane("192.168.0.0", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane("192.168.0.0", "v1.18.0", generateSSHPublicKey(true)),
			wantErr: true,
		},
		{
			name:    "same SSHPublicKey is valid",
			oldAMCP: createAzureManagedControlPlane("192.168.0.0", "v1.18.0", commonSSHKey),
			amcp:    createAzureManagedControlPlane("192.168.0.0", "v1.18.0", commonSSHKey),
			wantErr: false,
		},
		{
			name:    "AzureManagedControlPlane with invalid serviceIP",
			oldAMCP: createAzureManagedControlPlane("", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane("192.168.0.0.3", "v1.18.0", generateSSHPublicKey(true)),
			wantErr: true,
		},
		{
			name:    "AzureManagedControlPlane with invalid version",
			oldAMCP: createAzureManagedControlPlane("", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane("192.168.0.0", "1.999.9", generateSSHPublicKey(true)),
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane SubscriptionID is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:   to.StringPtr("192.168.0.0"),
					SubscriptionID: "212ec1q8",
					Version:        "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:   to.StringPtr("192.168.0.0"),
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
					DNSServiceIP:      to.StringPtr("192.168.0.0"),
					ResourceGroupName: "hello-1",
					Version:           "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:      to.StringPtr("192.168.0.0"),
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
					DNSServiceIP:          to.StringPtr("192.168.0.0"),
					NodeResourceGroupName: "hello-1",
					Version:               "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:          to.StringPtr("192.168.0.0"),
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
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Location:     "westeurope",
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					SSHPublicKey: generateSSHPublicKey(true),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					SSHPublicKey: generateSSHPublicKey(true),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSServiceIP is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.1"),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane DNSServiceIP is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
					DNSServiceIP:  to.StringPtr("192.168.0.0"),
					NetworkPlugin: to.StringPtr("azure"),
					Version:       "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  to.StringPtr("192.168.0.0"),
					NetworkPlugin: to.StringPtr("kubenet"),
					Version:       "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPlugin is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  to.StringPtr("192.168.0.0"),
					NetworkPlugin: to.StringPtr("azure"),
					Version:       "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPolicy is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  to.StringPtr("192.168.0.0"),
					NetworkPolicy: to.StringPtr("azure"),
					Version:       "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  to.StringPtr("192.168.0.0"),
					NetworkPolicy: to.StringPtr("calico"),
					Version:       "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane NetworkPolicy is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:  to.StringPtr("192.168.0.0"),
					NetworkPolicy: to.StringPtr("azure"),
					Version:       "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane LoadBalancerSKU is immutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:    to.StringPtr("192.168.0.0"),
					LoadBalancerSKU: to.StringPtr("Standard"),
					Version:         "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:    to.StringPtr("192.168.0.0"),
					LoadBalancerSKU: to.StringPtr("Basic"),
					Version:         "v1.18.0",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane LoadBalancerSKU is immutable, unsetting is not allowed",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP:    to.StringPtr("192.168.0.0"),
					LoadBalancerSKU: to.StringPtr("Standard"),
					Version:         "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.18.0",
					APIServerAccessProfile: &APIServerAccessProfile{
						EnablePrivateCluster: to.BoolPtr(true),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane AuthorizedIPRanges is mutable",
			oldAMCP: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
					DNSServiceIP: to.StringPtr("192.168.0.0"),
					Version:      "v1.18.0",
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
					DNSServiceIP: to.StringPtr("192.168.0.0"),
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
			name: "AzureManagedControlPlane ControlPlaneEndpoint.Port is immutable",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "aks-8622-h4h26c44.hcp.eastus.azmk8s.io",
						Port: 443,
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "aks-8622-h4h26c44.hcp.eastus.azmk8s.io",
						Port: 444,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane ControlPlaneEndpoint.Host is immutable",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "aks-8622-h4h26c44.hcp.eastus.azmk8s.io",
						Port: 443,
					},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "this-is-not-allowed",
						Port: 443,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "ControlPlaneEndpoint update from zero values are allowed",
			oldAMCP: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{},
				},
			},
			amcp: &AzureManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: AzureManagedControlPlaneSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "aks-8622-h4h26c44.hcp.eastus.azmk8s.io",
						Port: 443,
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.amcp.ValidateUpdate(tc.oldAMCP, nil)
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
		Spec: AzureManagedControlPlaneSpec{
			SSHPublicKey: sshKey,
			DNSServiceIP: to.StringPtr(serviceIP),
			Version:      version,
		},
	}
}

func getKnownValidAzureManagedControlPlane() *AzureManagedControlPlane {
	return &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			DNSServiceIP: to.StringPtr("192.168.0.0"),
			Version:      "v1.18.0",
			SSHPublicKey: generateSSHPublicKey(true),
			AADProfile: &AADProfile{
				Managed: true,
				AdminGroupObjectIDs: []string{
					"616077a8-5db7-4c98-b856-b34619afg75h",
				},
			},
		},
	}
}
