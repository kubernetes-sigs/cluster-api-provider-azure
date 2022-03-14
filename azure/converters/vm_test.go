/*
Copyright 2022 The Kubernetes Authors.

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

package converters

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-04-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func Test_SDKToVM(t *testing.T) {
	cases := []struct {
		name   string
		sdkVM  compute.VirtualMachine
		expect func(*GomegaWithT, *VM)
	}{
		{
			name: "ShouldPopulateWithData",
			sdkVM: compute.VirtualMachine{
				ID:   to.StringPtr("fake-vm-id-1"),
				Name: to.StringPtr("fake-vm-1"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: to.StringPtr("Provisioning"),
				},
			},
			expect: func(g *GomegaWithT, resultVM *VM) {
				g.Expect(resultVM).To(Equal(&VM{
					ID:    "fake-vm-id-1",
					Name:  "fake-vm-1",
					State: infrav1.ProvisioningState("Provisioning"),
				}))
			},
		},
		{
			name: "ShouldPopulateWithData with VMSize",
			sdkVM: compute.VirtualMachine{
				ID:   to.StringPtr("fake-vm-id-1"),
				Name: to.StringPtr("fake-vm-1"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: to.StringPtr("Provisioning"),
					HardwareProfile: &compute.HardwareProfile{
						VMSize: "fake-vm-size",
					},
				},
			},
			expect: func(g *GomegaWithT, resultVM *VM) {
				g.Expect(resultVM).To(Equal(&VM{
					ID:     "fake-vm-id-1",
					Name:   "fake-vm-1",
					State:  infrav1.ProvisioningState("Provisioning"),
					VMSize: "fake-vm-size",
				}))
			},
		},
		{
			name: "ShouldPopulateWithData with availability zones",
			sdkVM: compute.VirtualMachine{
				ID:   to.StringPtr("fake-vm-id-1"),
				Name: to.StringPtr("fake-vm-1"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: to.StringPtr("Provisioning"),
				},
				Zones: &[]string{
					"fake-az-1",
				},
			},
			expect: func(g *GomegaWithT, resultVM *VM) {
				g.Expect(resultVM).To(Equal(&VM{
					ID:               "fake-vm-id-1",
					Name:             "fake-vm-1",
					State:            infrav1.ProvisioningState("Provisioning"),
					AvailabilityZone: "fake-az-1",
				}))
			},
		},
		{
			name: "ShouldPopulateWithData with tags",
			sdkVM: compute.VirtualMachine{
				ID:   to.StringPtr("fake-vm-id-1"),
				Name: to.StringPtr("fake-vm-1"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: to.StringPtr("Provisioning"),
				},
				Tags: map[string]*string{
					"key-1": to.StringPtr("val-1"),
					"key-2": to.StringPtr("val-2"),
					"key-3": to.StringPtr("val-3"),
				},
			},
			expect: func(g *GomegaWithT, resultVM *VM) {
				g.Expect(resultVM).To(Equal(&VM{
					ID:    "fake-vm-id-1",
					Name:  "fake-vm-1",
					State: infrav1.ProvisioningState("Provisioning"),
					Tags: map[string]string{
						"key-1": "val-1",
						"key-2": "val-2",
						"key-3": "val-3",
					},
				}))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			subject := SDKToVM(c.sdkVM)
			c.expect(g, subject)
		})
	}
}
