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
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestGetSpotVMOptions(t *testing.T) {
	type resultParams struct {
		vmPriorityTypes       compute.VirtualMachinePriorityTypes
		vmEvictionPolicyTypes compute.VirtualMachineEvictionPolicyTypes
		billingProfile        *compute.BillingProfile
	}
	tests := []struct {
		name string
		spot *infrav1.SpotVMOptions
		want resultParams
	}{
		{
			name: "nil spot",
			spot: nil,
			want: resultParams{
				vmPriorityTypes:       "",
				vmEvictionPolicyTypes: "",
				billingProfile:        nil,
			},
		},
		{
			name: "spot with nil max price",
			spot: &infrav1.SpotVMOptions{
				MaxPrice: nil,
			},
			want: resultParams{
				vmPriorityTypes:       compute.VirtualMachinePriorityTypesSpot,
				vmEvictionPolicyTypes: compute.VirtualMachineEvictionPolicyTypesDeallocate,
				billingProfile:        nil,
			},
		},
		{
			name: "spot with max price",
			spot: &infrav1.SpotVMOptions{
				MaxPrice: func(price string) *resource.Quantity {
					p := resource.MustParse(price)
					return &p
				}("1000"),
			},
			want: resultParams{
				vmPriorityTypes:       compute.VirtualMachinePriorityTypesSpot,
				vmEvictionPolicyTypes: compute.VirtualMachineEvictionPolicyTypesDeallocate,
				billingProfile: &compute.BillingProfile{
					MaxPrice: to.Float64Ptr(1000),
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result := resultParams{}
			var err error
			result.vmPriorityTypes, result.vmEvictionPolicyTypes, result.billingProfile, err = GetSpotVMOptions(tt.spot)
			g.Expect(result.vmPriorityTypes).To(Equal(tt.want.vmPriorityTypes), fmt.Sprintf("got: %v, want: %v", result.vmPriorityTypes, tt.want.vmPriorityTypes))
			g.Expect(result.vmEvictionPolicyTypes).To(Equal(tt.want.vmEvictionPolicyTypes), fmt.Sprintf("got: %v, want: %v", result.vmEvictionPolicyTypes, tt.want.vmEvictionPolicyTypes))
			g.Expect(result.billingProfile).To(Equal(tt.want.billingProfile), fmt.Sprintf("got: %v, want: %v", result.billingProfile, tt.want.billingProfile))
			g.Expect(err).To(BeNil())
		})
	}
}
