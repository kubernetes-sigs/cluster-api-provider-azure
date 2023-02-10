/*
Copyright 2020 The Kubernetes Authors.

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

package converters_test

import (
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

func Test_SDKToVMSS(t *testing.T) {
	cases := []struct {
		Name           string
		SubjectFactory func(*gomega.GomegaWithT) (compute.VirtualMachineScaleSet, []compute.VirtualMachineScaleSetVM)
		Expect         func(*gomega.GomegaWithT, *azure.VMSS)
	}{
		{
			Name: "ShouldPopulateWithData",
			SubjectFactory: func(g *gomega.GomegaWithT) (compute.VirtualMachineScaleSet, []compute.VirtualMachineScaleSetVM) {
				tags := map[string]*string{
					"foo": to.StringPtr("bazz"),
				}
				zones := []string{"zone0", "zone1"}
				return compute.VirtualMachineScaleSet{
						Sku: &compute.Sku{
							Name:     to.StringPtr("skuName"),
							Tier:     to.StringPtr("skuTier"),
							Capacity: to.Int64Ptr(2),
						},
						Zones:    to.StringSlicePtr(zones),
						ID:       to.StringPtr("vmssID"),
						Name:     to.StringPtr("vmssName"),
						Location: to.StringPtr("westus2"),
						Tags:     tags,
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							SinglePlacementGroup: to.BoolPtr(false),
							ProvisioningState:    to.StringPtr(string(compute.ProvisioningState1Succeeded)),
						},
					},
					[]compute.VirtualMachineScaleSetVM{
						{
							InstanceID: to.StringPtr("0"),
							ID:         to.StringPtr("vm/0"),
							Name:       to.StringPtr("vm0"),
							Zones:      to.StringSlicePtr([]string{"zone0"}),
							VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
								ProvisioningState: to.StringPtr(string(compute.ProvisioningState1Succeeded)),
								OsProfile: &compute.OSProfile{
									ComputerName: to.StringPtr("instance-000000"),
								},
							},
						},
						{
							InstanceID: to.StringPtr("1"),
							ID:         to.StringPtr("vm/1"),
							Name:       to.StringPtr("vm1"),
							Zones:      to.StringSlicePtr([]string{"zone1"}),
							VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
								ProvisioningState: to.StringPtr(string(compute.ProvisioningState1Succeeded)),
								OsProfile: &compute.OSProfile{
									ComputerName: to.StringPtr("instance-000001"),
								},
							},
						},
					}
			},
			Expect: func(g *gomega.GomegaWithT, actual *azure.VMSS) {
				expected := azure.VMSS{
					ID:       "vmssID",
					Name:     "vmssName",
					Sku:      "skuName",
					Capacity: 2,
					Zones:    []string{"zone0", "zone1"},
					State:    "Succeeded",
					Tags: map[string]string{
						"foo": "bazz",
					},
					Instances: make([]azure.VMSSVM, 2),
				}

				for i := 0; i < 2; i++ {
					expected.Instances[i] = azure.VMSSVM{
						ID:               fmt.Sprintf("vm/%d", i),
						InstanceID:       fmt.Sprintf("%d", i),
						Name:             fmt.Sprintf("instance-00000%d", i),
						AvailabilityZone: fmt.Sprintf("zone%d", i),
						State:            "Succeeded",
					}
				}
				g.Expect(actual).To(gomega.Equal(&expected))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			vmss, instances := c.SubjectFactory(g)
			subject := converters.SDKToVMSS(vmss, instances)
			c.Expect(g, subject)
		})
	}
}

func Test_SDKToVMSSVM(t *testing.T) {
	cases := []struct {
		Name        string
		SDKInstance compute.VirtualMachineScaleSetVM
		VMSSVM      *azure.VMSSVM
	}{
		{
			Name: "minimal VM",
			SDKInstance: compute.VirtualMachineScaleSetVM{
				ID: to.StringPtr("vm/0"),
			},
			VMSSVM: &azure.VMSSVM{
				ID: "vm/0",
			},
		},
		{
			Name: "VM with nil properties",
			SDKInstance: compute.VirtualMachineScaleSetVM{
				ID:                                 to.StringPtr("vm/0.5"),
				VirtualMachineScaleSetVMProperties: nil,
			},
			VMSSVM: &azure.VMSSVM{
				ID: "vm/0.5",
			},
		},
		{
			Name: "VM with state",
			SDKInstance: compute.VirtualMachineScaleSetVM{
				ID: to.StringPtr("/subscriptions/foo/resourceGroups/MY_RESOURCE_GROUP/providers/bar"),
				VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
					ProvisioningState: to.StringPtr(string(compute.ProvisioningState1Succeeded)),
					OsProfile:         &compute.OSProfile{ComputerName: to.StringPtr("instance-000000")},
				},
			},
			VMSSVM: &azure.VMSSVM{
				ID:    "/subscriptions/foo/resourceGroups/my_resource_group/providers/bar",
				Name:  "instance-000000",
				State: "Succeeded",
			},
		},
		{
			Name: "VM with storage",
			SDKInstance: compute.VirtualMachineScaleSetVM{
				ID: to.StringPtr("/subscriptions/foo/resourceGroups/MY_RESOURCE_GROUP/providers/bar"),
				VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
					OsProfile: &compute.OSProfile{ComputerName: to.StringPtr("instance-000001")},
					StorageProfile: &compute.StorageProfile{
						ImageReference: &compute.ImageReference{
							ID: to.StringPtr("imageID"),
						},
					},
				},
			},
			VMSSVM: &azure.VMSSVM{
				ID:   "/subscriptions/foo/resourceGroups/my_resource_group/providers/bar",
				Name: "instance-000001",
				Image: infrav1.Image{
					ID:          to.StringPtr("imageID"),
					Marketplace: &infrav1.AzureMarketplaceImage{},
				},
				State: "Creating",
			},
		},
		{
			Name: "VM with zones",
			SDKInstance: compute.VirtualMachineScaleSetVM{
				ID: to.StringPtr("/subscriptions/foo/resourceGroups/MY_RESOURCE_GROUP/providers/bar"),
				VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
					OsProfile: &compute.OSProfile{ComputerName: to.StringPtr("instance-000002")},
				},
				Zones: &[]string{"zone0", "zone1"},
			},
			VMSSVM: &azure.VMSSVM{
				ID:               "/subscriptions/foo/resourceGroups/my_resource_group/providers/bar",
				Name:             "instance-000002",
				AvailabilityZone: "zone0",
				State:            "Creating",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			g.Expect(converters.SDKToVMSSVM(c.SDKInstance)).To(gomega.Equal(c.VMSSVM))
		})
	}
}

func Test_SDKImageToImage(t *testing.T) {
	cases := []struct {
		Name         string
		SDKImageRef  *compute.ImageReference
		IsThirdParty bool
		Image        infrav1.Image
	}{
		{
			Name: "minimal image",
			SDKImageRef: &compute.ImageReference{
				ID: to.StringPtr("imageID"),
			},
			IsThirdParty: false,
			Image: infrav1.Image{
				ID:          to.StringPtr("imageID"),
				Marketplace: &infrav1.AzureMarketplaceImage{},
			},
		},
		{
			Name: "marketplace image",
			SDKImageRef: &compute.ImageReference{
				ID:        to.StringPtr("imageID"),
				Publisher: to.StringPtr("publisher"),
				Offer:     to.StringPtr("offer"),
				Sku:       to.StringPtr("sku"),
				Version:   to.StringPtr("version"),
			},
			IsThirdParty: true,
			Image: infrav1.Image{
				ID: to.StringPtr("imageID"),
				Marketplace: &infrav1.AzureMarketplaceImage{
					ImagePlan: infrav1.ImagePlan{
						Publisher: "publisher",
						Offer:     "offer",
						SKU:       "sku",
					},
					Version:         "version",
					ThirdPartyImage: true,
				},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			g.Expect(converters.SDKImageToImage(c.SDKImageRef, c.IsThirdParty)).To(gomega.Equal(c.Image))
		})
	}
}

func Test_SDKVMToVMSSVM(t *testing.T) {
	cases := []struct {
		Name     string
		Subject  compute.VirtualMachine
		Expected *azure.VMSSVM
	}{
		{
			Name: "minimal VM",
			Subject: compute.VirtualMachine{
				ID: to.StringPtr("vmID1"),
			},
			Expected: &azure.VMSSVM{
				ID: "vmID1",
			},
		},
		{
			Name: "VM with zones",
			Subject: compute.VirtualMachine{
				ID: to.StringPtr("vmID2"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					OsProfile: &compute.OSProfile{
						ComputerName: to.StringPtr("vmwithzones"),
					},
				},
				Zones: to.StringSlicePtr([]string{"zone0", "zone1"}),
			},
			Expected: &azure.VMSSVM{
				ID:               "vmID2",
				Name:             "vmwithzones",
				State:            "Creating",
				AvailabilityZone: "zone0",
			},
		},
		{
			Name: "VM with storage",
			Subject: compute.VirtualMachine{
				ID: to.StringPtr("vmID3"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					OsProfile: &compute.OSProfile{
						ComputerName: to.StringPtr("vmwithstorage"),
					},
					StorageProfile: &compute.StorageProfile{
						ImageReference: &compute.ImageReference{
							ID: to.StringPtr("imageID"),
						},
					},
				},
			},
			Expected: &azure.VMSSVM{
				ID: "vmID3",
				Image: infrav1.Image{
					ID:          to.StringPtr("imageID"),
					Marketplace: &infrav1.AzureMarketplaceImage{},
				},
				Name:  "vmwithstorage",
				State: "Creating",
			},
		},
		{
			Name: "VM with provisioning state",
			Subject: compute.VirtualMachine{
				ID: to.StringPtr("vmID4"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					OsProfile: &compute.OSProfile{
						ComputerName: to.StringPtr("vmwithstate"),
					},
					ProvisioningState: to.StringPtr("Succeeded"),
				},
			},
			Expected: &azure.VMSSVM{
				ID:    "vmID4",
				Name:  "vmwithstate",
				State: "Succeeded",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			subject := converters.SDKVMToVMSSVM(c.Subject, "")
			g.Expect(subject).To(gomega.Equal(c.Expected))
		})
	}
}

func Test_GetOrchestrationMode(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	g.Expect(converters.GetOrchestrationMode(infrav1.FlexibleOrchestrationMode)).
		To(gomega.Equal(compute.OrchestrationModeFlexible))
	g.Expect(converters.GetOrchestrationMode(infrav1.UniformOrchestrationMode)).
		To(gomega.Equal(compute.OrchestrationModeUniform))
	g.Expect(converters.GetOrchestrationMode("invalid")).
		To(gomega.Equal(compute.OrchestrationModeUniform))
}
