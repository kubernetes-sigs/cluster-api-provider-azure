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

package controllers

import "testing"

func Test_ExtractVnetFromSubnetID(t *testing.T) {
	cases := map[string]struct {
		subnetID  string
		expect    string
		expectErr string
	}{
		"should work with valid resource ID": {
			subnetID: "/subscriptions/fooSub/resourceGroups/fooGroup/providers/Microsoft.Network/virtualNetworks/fooNet/subnets/aks-subnet",
			expect:   "fooNet",
		},
		"should fail with too few tokens": {
			subnetID:  "/subscriptions/fooSub",
			expectErr: "expected 11 tokens but found 3",
		},
		"should fail with too many tokens": {
			subnetID:  "/subscriptions/fooSub/resourceGroups/fooGroup/providers/Microsoft.Network/virtualNetworks/fooNet/subnets/aks-subnet/mininet/subthing",
			expectErr: "expected 11 tokens but found 13",
		},
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got, err := extractVnetFromSubnetID(tc.subnetID)
			if tc.expect != got {
				t.Errorf("expected value '%s' but got value '%s'", tc.expect, got)
			}
			if tc.expectErr != "" {
				if err == nil {
					t.Errorf("expected error '%s' but got no error", tc.expectErr)
				}
				if err != nil && err.Error() != tc.expectErr {
					t.Errorf("expected error '%s' but got error '%s'", tc.expectErr, err)
				}
			}
		})
	}
}
