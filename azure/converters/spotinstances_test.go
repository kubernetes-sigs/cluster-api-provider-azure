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
	"k8s.io/apimachinery/pkg/api/resource"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func Test_GetSpotVMOptions(t *testing.T) {
	type resultParams struct {
		vmPriorityTypes       compute.VirtualMachinePriorityTypes
		vmEvictionPolicyTypes compute.VirtualMachineEvictionPolicyTypes
		billingProfile        *compute.BillingProfile
	}
	cases := []struct {
		name          string
		spotVMOptions *infrav1.SpotVMOptions
		expectErr     bool
		expect        func(*GomegaWithT, resultParams)
	}{
		{
			name:          "should return empty vm priority, empty eviction policy and nil billing profile for nil spotVMOptions",
			spotVMOptions: nil,
			expectErr:     false,
			expect: func(g *GomegaWithT, result resultParams) {
				g.Expect(result.vmPriorityTypes).To(Equal(compute.VirtualMachinePriorityTypes("")))
				g.Expect(result.vmEvictionPolicyTypes).To(Equal(compute.VirtualMachineEvictionPolicyTypes("")))
				g.Expect(result.billingProfile).Should(BeNil())
			},
		},

		{
			name:          "should return vm priority, eviction policy and nil billing profile for spotVMOptions with nil MaxPrice",
			spotVMOptions: &infrav1.SpotVMOptions{},
			expectErr:     false,
			expect: func(g *GomegaWithT, result resultParams) {
				g.Expect(result.vmPriorityTypes).To(Equal(compute.VirtualMachinePriorityTypesSpot))
				g.Expect(result.vmEvictionPolicyTypes).To(Equal(compute.VirtualMachineEvictionPolicyTypesDeallocate))
				g.Expect(result.billingProfile).Should(BeNil())
			},
		},
		{
			name: "should return vm priority, eviction policy and billing profile for spotVMOptions with set MaxPrice",
			spotVMOptions: &infrav1.SpotVMOptions{
				MaxPrice: func(price string) *resource.Quantity {
					p := resource.MustParse(price)
					return &p
				}("1000"),
			},
			expectErr: false,
			expect: func(g *GomegaWithT, result resultParams) {
				g.Expect(result.vmPriorityTypes).To(Equal(compute.VirtualMachinePriorityTypesSpot))
				g.Expect(result.vmEvictionPolicyTypes).To(Equal(compute.VirtualMachineEvictionPolicyTypesDeallocate))
				g.Expect(result.billingProfile.MaxPrice).To(Equal(to.Float64Ptr(1000)))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result := resultParams{}
			var err error
			result.vmPriorityTypes, result.vmEvictionPolicyTypes, result.billingProfile, err = GetSpotVMOptions(c.spotVMOptions)
			if c.expectErr {
				g.Expect(err).ShouldNot(BeNil())
			} else {
				g.Expect(err).Should(BeNil())
			}
			c.expect(g, result)
		})
	}
}
