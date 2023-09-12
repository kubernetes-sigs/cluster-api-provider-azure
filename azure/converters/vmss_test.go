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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

func Test_SDKToVMSS(t *testing.T) {
	cases := []struct {
		Name           string
		SubjectFactory func(*gomega.GomegaWithT) (armcompute.VirtualMachineScaleSet, []compute.VirtualMachineScaleSetVM)
		Expect         func(*gomega.GomegaWithT, azure.VMSS)
	}{
		{
			Name: "ShouldPopulateWithData",
			SubjectFactory: func(g *gomega.GomegaWithT) (armcompute.VirtualMachineScaleSet, []compute.VirtualMachineScaleSetVM) {
				tags := map[string]*string{
					"foo": ptr.To("bazz"),
				}
				zones := []string{"zone0", "zone1"}
				return armcompute.VirtualMachineScaleSet{
						SKU: &armcompute.SKU{
							Name:     ptr.To("skuName"),
							Tier:     ptr.To("skuTier"),
							Capacity: ptr.To[int64](2),
						},
						Zones:    azure.PtrSlice(&zones),
						ID:       ptr.To("vmssID"),
						Name:     ptr.To("vmssName"),
						Location: ptr.To("westus2"),
						Tags:     tags,
						Properties: &armcompute.VirtualMachineScaleSetProperties{
							SinglePlacementGroup: ptr.To(false),
							ProvisioningState:    ptr.To(string(compute.ProvisioningState1Succeeded)),
						},
					},
					[]compute.VirtualMachineScaleSetVM{
						{
							InstanceID: ptr.To("0"),
							ID:         ptr.To("vm/0"),
							Name:       ptr.To("vm0"),
							Zones:      &[]string{"zone0"},
							VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
								ProvisioningState: ptr.To(string(compute.ProvisioningState1Succeeded)),
								OsProfile: &compute.OSProfile{
									ComputerName: ptr.To("instance-000000"),
								},
							},
						},
						{
							InstanceID: ptr.To("1"),
							ID:         ptr.To("vm/1"),
							Name:       ptr.To("vm1"),
							Zones:      &[]string{"zone1"},
							VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
								ProvisioningState: ptr.To(string(compute.ProvisioningState1Succeeded)),
								OsProfile: &compute.OSProfile{
									ComputerName: ptr.To("instance-000001"),
								},
							},
						},
					}
			},
			Expect: func(g *gomega.GomegaWithT, actual azure.VMSS) {
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
				g.Expect(actual).To(gomega.Equal(expected))
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
				ID: ptr.To("vm/0"),
			},
			VMSSVM: &azure.VMSSVM{
				ID: "vm/0",
			},
		},
		{
			Name: "VM with nil properties",
			SDKInstance: compute.VirtualMachineScaleSetVM{
				ID:                                 ptr.To("vm/0.5"),
				VirtualMachineScaleSetVMProperties: nil,
			},
			VMSSVM: &azure.VMSSVM{
				ID: "vm/0.5",
			},
		},
		{
			Name: "VM with state",
			SDKInstance: compute.VirtualMachineScaleSetVM{
				ID: ptr.To("/subscriptions/foo/resourceGroups/MY_RESOURCE_GROUP/providers/bar"),
				VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
					ProvisioningState: ptr.To(string(compute.ProvisioningState1Succeeded)),
					OsProfile:         &compute.OSProfile{ComputerName: ptr.To("instance-000000")},
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
				ID: ptr.To("/subscriptions/foo/resourceGroups/MY_RESOURCE_GROUP/providers/bar"),
				VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
					OsProfile: &compute.OSProfile{ComputerName: ptr.To("instance-000001")},
					StorageProfile: &compute.StorageProfile{
						ImageReference: &compute.ImageReference{
							ID: ptr.To("imageID"),
						},
					},
				},
			},
			VMSSVM: &azure.VMSSVM{
				ID:   "/subscriptions/foo/resourceGroups/my_resource_group/providers/bar",
				Name: "instance-000001",
				Image: infrav1.Image{
					ID: ptr.To("imageID"),
				},
				State: "Creating",
			},
		},
		{
			Name: "VM with zones",
			SDKInstance: compute.VirtualMachineScaleSetVM{
				ID: ptr.To("/subscriptions/foo/resourceGroups/MY_RESOURCE_GROUP/providers/bar"),
				VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
					OsProfile: &compute.OSProfile{ComputerName: ptr.To("instance-000002")},
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
			Name: "id image",
			SDKImageRef: &compute.ImageReference{
				ID: ptr.To("imageID"),
			},
			IsThirdParty: false,
			Image: infrav1.Image{
				ID: ptr.To("imageID"),
			},
		},
		{
			Name: "marketplace image",
			SDKImageRef: &compute.ImageReference{
				Publisher: ptr.To("publisher"),
				Offer:     ptr.To("offer"),
				Sku:       ptr.To("sku"),
				Version:   ptr.To("version"),
			},
			IsThirdParty: true,
			Image: infrav1.Image{
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
		{
			Name: "shared gallery image",
			SDKImageRef: &compute.ImageReference{
				SharedGalleryImageID: ptr.To("/subscriptions/subscription/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery/images/image/versions/version"),
			},
			Image: infrav1.Image{
				SharedGallery: &infrav1.AzureSharedGalleryImage{
					SubscriptionID: "subscription",
					ResourceGroup:  "rg",
					Gallery:        "gallery",
					Name:           "image",
					Version:        "version",
				},
			},
		},
		{
			Name: "community gallery image",
			SDKImageRef: &compute.ImageReference{
				CommunityGalleryImageID: ptr.To("/CommunityGalleries/gallery/Images/image/Versions/version"),
			},
			Image: infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: "gallery",
					Name:    "image",
					Version: "version",
				},
			},
		},
		{
			Name: "compute gallery image",
			SDKImageRef: &compute.ImageReference{
				ID: ptr.To("/subscriptions/subscription/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery/images/image/versions/version"),
			},
			Image: infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery:        "gallery",
					Name:           "image",
					Version:        "version",
					SubscriptionID: ptr.To("subscription"),
					ResourceGroup:  ptr.To("rg"),
				},
			},
		},
		{
			Name: "compute gallery image not formatted as expected",
			SDKImageRef: &compute.ImageReference{
				ID: ptr.To("/compute/gallery/not/formatted/as/expected"),
			},
			Image: infrav1.Image{
				ID: ptr.To("/compute/gallery/not/formatted/as/expected"),
			},
		},
		{
			Name: "community gallery image not formatted as expected",
			SDKImageRef: &compute.ImageReference{
				CommunityGalleryImageID: ptr.To("/community/gallery/not/formatted/as/expected"),
			},
			Image: infrav1.Image{},
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

func Test_SDKImageToImageV2(t *testing.T) {
	cases := []struct {
		Name         string
		SDKImageRef  *armcompute.ImageReference
		IsThirdParty bool
		Image        infrav1.Image
	}{
		{
			Name: "id image",
			SDKImageRef: &armcompute.ImageReference{
				ID: ptr.To("imageID"),
			},
			IsThirdParty: false,
			Image: infrav1.Image{
				ID: ptr.To("imageID"),
			},
		},
		{
			Name: "marketplace image",
			SDKImageRef: &armcompute.ImageReference{
				Publisher: ptr.To("publisher"),
				Offer:     ptr.To("offer"),
				SKU:       ptr.To("sku"),
				Version:   ptr.To("version"),
			},
			IsThirdParty: true,
			Image: infrav1.Image{
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
		{
			Name: "shared gallery image",
			SDKImageRef: &armcompute.ImageReference{
				SharedGalleryImageID: ptr.To("/subscriptions/subscription/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery/images/image/versions/version"),
			},
			Image: infrav1.Image{
				SharedGallery: &infrav1.AzureSharedGalleryImage{
					SubscriptionID: "subscription",
					ResourceGroup:  "rg",
					Gallery:        "gallery",
					Name:           "image",
					Version:        "version",
				},
			},
		},
		{
			Name: "community gallery image",
			SDKImageRef: &armcompute.ImageReference{
				CommunityGalleryImageID: ptr.To("/CommunityGalleries/gallery/Images/image/Versions/version"),
			},
			Image: infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: "gallery",
					Name:    "image",
					Version: "version",
				},
			},
		},
		{
			Name: "compute gallery image",
			SDKImageRef: &armcompute.ImageReference{
				ID: ptr.To("/subscriptions/subscription/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery/images/image/versions/version"),
			},
			Image: infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery:        "gallery",
					Name:           "image",
					Version:        "version",
					SubscriptionID: ptr.To("subscription"),
					ResourceGroup:  ptr.To("rg"),
				},
			},
		},
		{
			Name: "compute gallery image not formatted as expected",
			SDKImageRef: &armcompute.ImageReference{
				ID: ptr.To("/compute/gallery/not/formatted/as/expected"),
			},
			Image: infrav1.Image{
				ID: ptr.To("/compute/gallery/not/formatted/as/expected"),
			},
		},
		{
			Name: "community gallery image not formatted as expected",
			SDKImageRef: &armcompute.ImageReference{
				CommunityGalleryImageID: ptr.To("/community/gallery/not/formatted/as/expected"),
			},
			Image: infrav1.Image{},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			g.Expect(converters.SDKImageToImageV2(c.SDKImageRef, c.IsThirdParty)).To(gomega.Equal(c.Image))
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
				ID: ptr.To("vmID1"),
			},
			Expected: &azure.VMSSVM{
				ID: "vmID1",
			},
		},
		{
			Name: "VM with zones",
			Subject: compute.VirtualMachine{
				ID: ptr.To("vmID2"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					OsProfile: &compute.OSProfile{
						ComputerName: ptr.To("vmwithzones"),
					},
				},
				Zones: &[]string{"zone0", "zone1"},
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
				ID: ptr.To("vmID3"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					OsProfile: &compute.OSProfile{
						ComputerName: ptr.To("vmwithstorage"),
					},
					StorageProfile: &compute.StorageProfile{
						ImageReference: &compute.ImageReference{
							ID: ptr.To("imageID"),
						},
					},
				},
			},
			Expected: &azure.VMSSVM{
				ID: "vmID3",
				Image: infrav1.Image{
					ID: ptr.To("imageID"),
				},
				Name:  "vmwithstorage",
				State: "Creating",
			},
		},
		{
			Name: "VM with provisioning state",
			Subject: compute.VirtualMachine{
				ID: ptr.To("vmID4"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					OsProfile: &compute.OSProfile{
						ComputerName: ptr.To("vmwithstate"),
					},
					ProvisioningState: ptr.To("Succeeded"),
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
		To(gomega.Equal(armcompute.OrchestrationModeFlexible))
	g.Expect(converters.GetOrchestrationMode(infrav1.UniformOrchestrationMode)).
		To(gomega.Equal(armcompute.OrchestrationModeUniform))
	g.Expect(converters.GetOrchestrationMode("invalid")).
		To(gomega.Equal(armcompute.OrchestrationModeUniform))
}
