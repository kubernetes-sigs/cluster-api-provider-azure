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
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	apiinternal "sigs.k8s.io/cluster-api-provider-azure/internal/api"
)

func TestControlPlaneTemplateDefaultingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing amcp defaulting webhook with no baseline")
	amcpt := getAzureManagedControlPlaneTemplate()
	mcptw := &AzureManagedControlPlaneTemplateWebhook{}
	err := mcptw.Default(t.Context(), amcpt)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*amcpt.Spec.Template.Spec.NetworkPlugin).To(Equal("azure"))
	g.Expect(*amcpt.Spec.Template.Spec.LoadBalancerSKU).To(Equal("Standard"))
	g.Expect(amcpt.Spec.Template.Spec.Version).To(Equal("v1.17.5"))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.CIDRBlock).To(Equal(apiinternal.DefaultAKSVnetCIDR))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooName"))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.Subnet.CIDRBlock).To(Equal(apiinternal.DefaultAKSNodeSubnetCIDR))
	g.Expect(*amcpt.Spec.Template.Spec.EnablePreviewFeatures).To(BeFalse())

	t.Logf("Testing amcp defaulting webhook with baseline")
	netPlug := "kubenet"
	lbSKU := "Basic"
	netPol := "azure"
	amcpt.Spec.Template.Spec.NetworkPlugin = &netPlug
	amcpt.Spec.Template.Spec.LoadBalancerSKU = &lbSKU
	amcpt.Spec.Template.Spec.NetworkPolicy = &netPol
	amcpt.Spec.Template.Spec.Version = "9.99.99"
	amcpt.Spec.Template.Spec.VirtualNetwork.Name = "fooVnetName"
	amcpt.Spec.Template.Spec.VirtualNetwork.Subnet.Name = "fooSubnetName"
	amcpt.Spec.Template.Spec.SKU.Tier = infrav1.PaidManagedControlPlaneTier
	amcpt.Spec.Template.Spec.EnablePreviewFeatures = ptr.To(true)

	err = mcptw.Default(t.Context(), amcpt)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*amcpt.Spec.Template.Spec.NetworkPlugin).To(Equal(netPlug))
	g.Expect(*amcpt.Spec.Template.Spec.LoadBalancerSKU).To(Equal(lbSKU))
	g.Expect(*amcpt.Spec.Template.Spec.NetworkPolicy).To(Equal(netPol))
	g.Expect(amcpt.Spec.Template.Spec.Version).To(Equal("v9.99.99"))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.Name).To(Equal("fooVnetName"))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooSubnetName"))
	g.Expect(amcpt.Spec.Template.Spec.SKU.Tier).To(Equal(infrav1.StandardManagedControlPlaneTier))
	g.Expect(*amcpt.Spec.Template.Spec.EnablePreviewFeatures).To(BeTrue())
}

func TestControlPlaneTemplateUpdateWebhook(t *testing.T) {
	tests := []struct {
		name                    string
		oldControlPlaneTemplate *infrav1.AzureManagedControlPlaneTemplate
		controlPlaneTemplate    *infrav1.AzureManagedControlPlaneTemplate
		wantErr                 bool
	}{
		{
			name:                    "azuremanagedcontrolplanetemplate no changes - valid spec",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(),
			controlPlaneTemplate:    getAzureManagedControlPlaneTemplate(),
			wantErr:                 false,
		},
		{
			name: "azuremanagedcontrolplanetemplate subscriptionID is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.SubscriptionID = "foo"
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.SubscriptionID = "bar"
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate location is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Location = "foo"
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Location = "bar"
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate DNSServiceIP is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.DNSServiceIP = ptr.To("foo")
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.DNSServiceIP = ptr.To("bar")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate NetworkPlugin is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkPlugin = ptr.To("foo")
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkPlugin = ptr.To("bar")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate NetworkPolicy is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkPolicy = ptr.To("foo")
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkPolicy = ptr.To("bar")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate LoadBalancerSKU is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.LoadBalancerSKU = ptr.To("foo")
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.LoadBalancerSKU = ptr.To("bar")
			}),
			wantErr: true,
		},
		{
			name: "cannot disable AADProfile",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &infrav1.AADProfile{
					Managed: true,
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(),
			wantErr:              true,
		},
		{
			name: "cannot set AADProfile.Managed to false",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &infrav1.AADProfile{
					Managed: true,
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &infrav1.AADProfile{
					Managed: false,
				}
			}),
			wantErr: true,
		},
		{
			name: "length of AADProfile.AdminGroupObjectIDs cannot be zero",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &infrav1.AADProfile{
					AdminGroupObjectIDs: []string{"foo"},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &infrav1.AADProfile{
					AdminGroupObjectIDs: []string{},
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate OutboundType is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.OutboundType = (*infrav1.ManagedControlPlaneOutboundType)(ptr.To(string(infrav1.ManagedControlPlaneOutboundTypeLoadBalancer)))
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.OutboundType = (*infrav1.ManagedControlPlaneOutboundType)(ptr.To(string(infrav1.ManagedControlPlaneOutboundTypeManagedNATGateway)))
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate AKSExtension type and plan are immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Extensions = []infrav1.AKSExtension{
					{
						Name:          "foo",
						ExtensionType: ptr.To("foo-type"),
						Plan: &infrav1.ExtensionPlan{
							Name:      "foo-name",
							Product:   "foo-product",
							Publisher: "foo-publisher",
						},
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Extensions = []infrav1.AKSExtension{
					{
						Name:          "foo",
						ExtensionType: ptr.To("bar"),
						Plan: &infrav1.ExtensionPlan{
							Name:      "bar-name",
							Product:   "bar-product",
							Publisher: "bar-publisher",
						},
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate AKSExtension autoUpgradeMinorVersion is mutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Extensions = []infrav1.AKSExtension{
					{
						Name:                    "foo",
						ExtensionType:           ptr.To("foo"),
						AutoUpgradeMinorVersion: ptr.To(true),
						Plan: &infrav1.ExtensionPlan{
							Name:      "bar-name",
							Product:   "bar-product",
							Publisher: "bar-publisher",
						},
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Extensions = []infrav1.AKSExtension{
					{
						Name:                    "foo",
						ExtensionType:           ptr.To("foo"),
						AutoUpgradeMinorVersion: ptr.To(false),
						Plan: &infrav1.ExtensionPlan{
							Name:      "bar-name",
							Product:   "bar-product",
							Publisher: "bar-publisher",
						},
					},
				}
			}),
			wantErr: false,
		},
		{
			name: "azuremanagedcontrolplanetemplate networkDataplane is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkDataplane = ptr.To(infrav1.NetworkDataplaneTypeAzure)
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkDataplane = ptr.To(infrav1.NetworkDataplaneTypeCilium)
			}),
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane invalid version downgrade change",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Version = "v1.18.0"
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Version = "v1.17.0"
			}),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			cpw := &AzureManagedControlPlaneTemplateWebhook{}
			_, err := cpw.ValidateUpdate(t.Context(), tc.oldControlPlaneTemplate, tc.controlPlaneTemplate)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestValidateVirtualNetworkTemplateUpdate(t *testing.T) {
	tests := []struct {
		name                    string
		oldControlPlaneTemplate *infrav1.AzureManagedControlPlaneTemplate
		controlPlaneTemplate    *infrav1.AzureManagedControlPlaneTemplate
		wantErr                 bool
	}{
		{
			name:                    "azuremanagedcontrolplanetemplate no changes - valid spec",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(),
			controlPlaneTemplate:    getAzureManagedControlPlaneTemplate(),
			wantErr:                 false,
		},
		{
			name: "azuremanagedcontrolplanetemplate vnet is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.VirtualNetwork = infrav1.ManagedControlPlaneVirtualNetwork{
					ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
						CIDRBlock: "fooCIDRBlock",
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.VirtualNetwork = infrav1.ManagedControlPlaneVirtualNetwork{
					ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
						CIDRBlock: "barCIDRBlock",
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate networkCIDRBlock is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.VirtualNetwork = infrav1.ManagedControlPlaneVirtualNetwork{
					ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
						CIDRBlock: "fooCIDRBlock",
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.VirtualNetwork = infrav1.ManagedControlPlaneVirtualNetwork{
					ManagedControlPlaneVirtualNetworkClassSpec: infrav1.ManagedControlPlaneVirtualNetworkClassSpec{
						CIDRBlock: "barCIDRBlock",
					},
				}
			}),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			allErrs := validateAzureManagedControlPlaneTemplateVirtualNetworkTemplateUpdate(tc.controlPlaneTemplate, tc.oldControlPlaneTemplate)
			if tc.wantErr {
				g.Expect(allErrs).NotTo(BeEmpty())
			} else {
				g.Expect(allErrs).To(BeEmpty())
			}
		})
	}
}

func TestValidateAPIServerAccessProfileUpdate(t *testing.T) {
	tests := []struct {
		name                    string
		oldControlPlaneTemplate *infrav1.AzureManagedControlPlaneTemplate
		controlPlaneTemplate    *infrav1.AzureManagedControlPlaneTemplate
		wantErr                 bool
	}{
		{
			name:                    "azuremanagedcontrolplanetemplate no changes - valid spec",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(),
			controlPlaneTemplate:    getAzureManagedControlPlaneTemplate(),
			wantErr:                 false,
		},
		{
			name: "azuremanagedcontrolplanetemplate enablePrivateCluster is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &infrav1.APIServerAccessProfile{
					APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
						EnablePrivateCluster: ptr.To(true),
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &infrav1.APIServerAccessProfile{
					APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
						EnablePrivateCluster: ptr.To(false),
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate privateDNSZone is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &infrav1.APIServerAccessProfile{
					APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
						PrivateDNSZone: ptr.To("foo"),
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &infrav1.APIServerAccessProfile{
					APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
						PrivateDNSZone: ptr.To("bar"),
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate enablePrivateClusterPublicFQDN is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &infrav1.APIServerAccessProfile{
					APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
						EnablePrivateClusterPublicFQDN: ptr.To(true),
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *infrav1.AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &infrav1.APIServerAccessProfile{
					APIServerAccessProfileClassSpec: infrav1.APIServerAccessProfileClassSpec{
						EnablePrivateClusterPublicFQDN: ptr.To(false),
					},
				}
			}),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			allErrs := validateAzureManagedControlPlaneTemplateAPIServerAccessProfileTemplateUpdate(tc.controlPlaneTemplate, tc.oldControlPlaneTemplate)
			if tc.wantErr {
				g.Expect(allErrs).NotTo(BeEmpty())
			} else {
				g.Expect(allErrs).To(BeEmpty())
			}
		})
	}
}

func getAzureManagedControlPlaneTemplate(changes ...func(*infrav1.AzureManagedControlPlaneTemplate)) *infrav1.AzureManagedControlPlaneTemplate {
	input := &infrav1.AzureManagedControlPlaneTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
		},
		Spec: infrav1.AzureManagedControlPlaneTemplateSpec{
			Template: infrav1.AzureManagedControlPlaneTemplateResource{
				Spec: infrav1.AzureManagedControlPlaneTemplateResourceSpec{
					AzureManagedControlPlaneClassSpec: infrav1.AzureManagedControlPlaneClassSpec{
						Location: "fooLocation",
						Version:  "v1.17.5",
						SKU:      &infrav1.AKSSku{},
					},
				},
			},
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}
