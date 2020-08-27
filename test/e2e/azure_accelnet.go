// +build e2e

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

package e2e

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// AzureAcceleratedNetworkingSpecInput is the input for AzureAcceleratedNetworkingSpec.
type AzureAcceleratedNetworkingSpecInput struct {
	ClusterName string
}

// AzureAcceleratedNetworkingSpec implements a test that verifies Azure VMs in a workload
// cluster provisioned by CAPZ have accelerated networking enabled if they're capable of it.
func AzureAcceleratedNetworkingSpec(ctx context.Context, inputGetter func() AzureAcceleratedNetworkingSpecInput) {
	var (
		specName = "azure-accelerated-networking"
		input    AzureAcceleratedNetworkingSpecInput
	)

	input = inputGetter()
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	By("creating Azure clients with the workload cluster's subscription")
	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	authorizer, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	vmsClient := compute.NewVirtualMachinesClient(subscriptionID)
	vmsClient.Authorizer = authorizer
	nicsClient := network.NewInterfacesClient(subscriptionID)
	nicsClient.Authorizer = authorizer

	By("verifying EnableAcceleratedNetworking for the primary NIC of each VM")
	// NOTE: add SKUs being tested to this lookup table.
	acceleratedNetworking := map[compute.VirtualMachineSizeTypes]bool{
		compute.VirtualMachineSizeTypesStandardB2ms:  false,
		compute.VirtualMachineSizeTypesStandardD2V2:  true,
		compute.VirtualMachineSizeTypesStandardD2V3:  false,
		compute.VirtualMachineSizeTypesStandardD2sV3: false,
		compute.VirtualMachineSizeTypesStandardD4V2:  true,
		compute.VirtualMachineSizeTypesStandardD4V3:  true,
		compute.VirtualMachineSizeTypesStandardD8sV3: true,
	}
	rgName := input.ClusterName
	page, err := vmsClient.List(ctx, rgName)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(page.Values())).To(BeNumerically(">", 0))
	for page.NotDone() {
		for _, vm := range page.Values() {
			sku := vm.HardwareProfile.VMSize
			for _, nic := range *vm.NetworkProfile.NetworkInterfaces {
				if nic.Primary != nil && *nic.Primary {
					// verify that accelerated networking is enabled if the SKU is capable
					nicInfo, err := nicsClient.Get(ctx, rgName, filepath.Base(*nic.ID), "")
					Expect(err).NotTo(HaveOccurred())
					capable, found := acceleratedNetworking[sku]
					if !found {
						fmt.Fprintf(GinkgoWriter, "SKU %s was not found, please add to the acceleratedNetworking lookup table.\n", sku)
					} else {
						Expect(capable).To(Equal(*nicInfo.EnableAcceleratedNetworking))
					}
					break
				}
			}
		}
		err = page.NextWithContext(ctx)
		Expect(err).NotTo(HaveOccurred())
	}
}
