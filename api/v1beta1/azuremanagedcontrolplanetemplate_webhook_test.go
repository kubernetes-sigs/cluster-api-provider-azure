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
	"k8s.io/utils/ptr"
)

func TestControlPlaneTemplateDefaultingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing amcp defaulting webhook with no baseline")
	amcpt := getAzureManagedControlPlaneTemplate()
	mcptw := &azureManagedControlPlaneTemplateWebhook{}
	err := mcptw.Default(context.Background(), amcpt)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*amcpt.Spec.Template.Spec.NetworkPlugin).To(Equal("azure"))
	g.Expect(*amcpt.Spec.Template.Spec.LoadBalancerSKU).To(Equal("Standard"))
	g.Expect(amcpt.Spec.Template.Spec.Version).To(Equal("v1.17.5"))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.CIDRBlock).To(Equal(defaultAKSVnetCIDR))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooName"))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.Subnet.CIDRBlock).To(Equal(defaultAKSNodeSubnetCIDR))
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
	amcpt.Spec.Template.Spec.SKU.Tier = PaidManagedControlPlaneTier
	amcpt.Spec.Template.Spec.EnablePreviewFeatures = ptr.To(true)

	err = mcptw.Default(context.Background(), amcpt)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*amcpt.Spec.Template.Spec.NetworkPlugin).To(Equal(netPlug))
	g.Expect(*amcpt.Spec.Template.Spec.LoadBalancerSKU).To(Equal(lbSKU))
	g.Expect(*amcpt.Spec.Template.Spec.NetworkPolicy).To(Equal(netPol))
	g.Expect(amcpt.Spec.Template.Spec.Version).To(Equal("v9.99.99"))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.Name).To(Equal("fooVnetName"))
	g.Expect(amcpt.Spec.Template.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooSubnetName"))
	g.Expect(amcpt.Spec.Template.Spec.SKU.Tier).To(Equal(StandardManagedControlPlaneTier))
	g.Expect(*amcpt.Spec.Template.Spec.EnablePreviewFeatures).To(BeTrue())
}

func TestControlPlaneTemplateUpdateWebhook(t *testing.T) {
	tests := []struct {
		name                    string
		oldControlPlaneTemplate *AzureManagedControlPlaneTemplate
		controlPlaneTemplate    *AzureManagedControlPlaneTemplate
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
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.SubscriptionID = "foo"
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.SubscriptionID = "bar"
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate location is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Location = "foo"
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Location = "bar"
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate DNSServiceIP is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.DNSServiceIP = ptr.To("foo")
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.DNSServiceIP = ptr.To("bar")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate NetworkPlugin is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkPlugin = ptr.To("foo")
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkPlugin = ptr.To("bar")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate NetworkPolicy is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkPolicy = ptr.To("foo")
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkPolicy = ptr.To("bar")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate LoadBalancerSKU is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.LoadBalancerSKU = ptr.To("foo")
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.LoadBalancerSKU = ptr.To("bar")
			}),
			wantErr: true,
		},
		{
			name: "cannot disable AADProfile",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &AADProfile{
					Managed: true,
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(),
			wantErr:              true,
		},
		{
			name: "cannot set AADProfile.Managed to false",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &AADProfile{
					Managed: true,
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &AADProfile{
					Managed: false,
				}
			}),
			wantErr: true,
		},
		{
			name: "length of AADProfile.AdminGroupObjectIDs cannot be zero",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &AADProfile{
					AdminGroupObjectIDs: []string{"foo"},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.AADProfile = &AADProfile{
					AdminGroupObjectIDs: []string{},
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate OutboundType is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.OutboundType = (*ManagedControlPlaneOutboundType)(ptr.To(string(ManagedControlPlaneOutboundTypeLoadBalancer)))
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.OutboundType = (*ManagedControlPlaneOutboundType)(ptr.To(string(ManagedControlPlaneOutboundTypeManagedNATGateway)))
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate AKSExtension type and plan are immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Extensions = []AKSExtension{
					{
						Name:          "foo",
						ExtensionType: ptr.To("foo-type"),
						Plan: &ExtensionPlan{
							Name:      "foo-name",
							Product:   "foo-product",
							Publisher: "foo-publisher",
						},
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Extensions = []AKSExtension{
					{
						Name:          "foo",
						ExtensionType: ptr.To("bar"),
						Plan: &ExtensionPlan{
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
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Extensions = []AKSExtension{
					{
						Name:                    "foo",
						ExtensionType:           ptr.To("foo"),
						AutoUpgradeMinorVersion: ptr.To(true),
						Plan: &ExtensionPlan{
							Name:      "bar-name",
							Product:   "bar-product",
							Publisher: "bar-publisher",
						},
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Extensions = []AKSExtension{
					{
						Name:                    "foo",
						ExtensionType:           ptr.To("foo"),
						AutoUpgradeMinorVersion: ptr.To(false),
						Plan: &ExtensionPlan{
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
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkDataplane = ptr.To(NetworkDataplaneTypeAzure)
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.NetworkDataplane = ptr.To(NetworkDataplaneTypeCilium)
			}),
			wantErr: true,
		},
		{
			name: "AzureManagedControlPlane invalid version downgrade change",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Version = "v1.18.0"
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.Version = "v1.17.0"
			}),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			cpw := &azureManagedControlPlaneTemplateWebhook{}
			_, err := cpw.ValidateUpdate(context.Background(), tc.oldControlPlaneTemplate, tc.controlPlaneTemplate)
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
		oldControlPlaneTemplate *AzureManagedControlPlaneTemplate
		controlPlaneTemplate    *AzureManagedControlPlaneTemplate
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
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.VirtualNetwork = ManagedControlPlaneVirtualNetwork{
					ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
						CIDRBlock: "fooCIDRBlock",
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.VirtualNetwork = ManagedControlPlaneVirtualNetwork{
					ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
						CIDRBlock: "barCIDRBlock",
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate networkCIDRBlock is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.VirtualNetwork = ManagedControlPlaneVirtualNetwork{
					ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
						CIDRBlock: "fooCIDRBlock",
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.VirtualNetwork = ManagedControlPlaneVirtualNetwork{
					ManagedControlPlaneVirtualNetworkClassSpec: ManagedControlPlaneVirtualNetworkClassSpec{
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
			allErrs := tc.controlPlaneTemplate.validateVirtualNetworkTemplateUpdate(tc.oldControlPlaneTemplate)
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
		oldControlPlaneTemplate *AzureManagedControlPlaneTemplate
		controlPlaneTemplate    *AzureManagedControlPlaneTemplate
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
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &APIServerAccessProfile{
					APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
						EnablePrivateCluster: ptr.To(true),
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &APIServerAccessProfile{
					APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
						EnablePrivateCluster: ptr.To(false),
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate privateDNSZone is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &APIServerAccessProfile{
					APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
						PrivateDNSZone: ptr.To("foo"),
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &APIServerAccessProfile{
					APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
						PrivateDNSZone: ptr.To("bar"),
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedcontrolplanetemplate enablePrivateClusterPublicFQDN is immutable",
			oldControlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &APIServerAccessProfile{
					APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
						EnablePrivateClusterPublicFQDN: ptr.To(true),
					},
				}
			}),
			controlPlaneTemplate: getAzureManagedControlPlaneTemplate(func(cpt *AzureManagedControlPlaneTemplate) {
				cpt.Spec.Template.Spec.APIServerAccessProfile = &APIServerAccessProfile{
					APIServerAccessProfileClassSpec: APIServerAccessProfileClassSpec{
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
			allErrs := tc.controlPlaneTemplate.validateAPIServerAccessProfileTemplateUpdate(tc.oldControlPlaneTemplate)
			if tc.wantErr {
				g.Expect(allErrs).NotTo(BeEmpty())
			} else {
				g.Expect(allErrs).To(BeEmpty())
			}
		})
	}
}

func getAzureManagedControlPlaneTemplate(changes ...func(*AzureManagedControlPlaneTemplate)) *AzureManagedControlPlaneTemplate {
	input := &AzureManagedControlPlaneTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
		},
		Spec: AzureManagedControlPlaneTemplateSpec{
			Template: AzureManagedControlPlaneTemplateResource{
				Spec: AzureManagedControlPlaneTemplateResourceSpec{
					AzureManagedControlPlaneClassSpec: AzureManagedControlPlaneClassSpec{
						Location: "fooLocation",
						Version:  "v1.17.5",
						SKU:      &AKSSku{},
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
