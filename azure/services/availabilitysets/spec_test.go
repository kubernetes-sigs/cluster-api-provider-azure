/*
Copyright 2021 The Kubernetes Authors.

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

package availabilitysets

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
)

var (
	fakeSetSpecMissingCap = AvailabilitySetSpec{
		Name:           "test-as",
		ResourceGroup:  "test-rg",
		ClusterName:    "test-cluster",
		Location:       "test-location",
		SKU:            &resourceskus.SKU{},
		AdditionalTags: map[string]string{},
	}
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *AvailabilitySetSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "error when no SKU is present",
			spec:     &fakeSetSpecMissing,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "unable to get required availability set SKU from machine cache",
		},
		{
			name:     "error when SKU capability is missing",
			spec:     &fakeSetSpecMissingCap,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "unable to get required availability set SKU capability MaximumPlatformFaultDomainCount",
		},
		{
			name:     "get parameters when all values are present",
			spec:     &fakeSetSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcompute.AvailabilitySet{}))
				g.Expect(result.(armcompute.AvailabilitySet).Properties.PlatformFaultDomainCount).To(Equal(ptr.To[int32](int32(fakeFaultDomainCount))))
			},
			expectedError: "",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(t.Context(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, result)
		})
	}
}
