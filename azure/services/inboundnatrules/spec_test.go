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

package inboundnatrules

import (
	"context"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name     string
		spec     InboundNatSpec
		existing interface{}
		expected interface{}
		errorMsg string
	}{
		{
			name:     "no existing InboundNatRule",
			spec:     fakeInboundNatSpec(true),
			existing: nil,
			expected: fakeNatRule(),
		},
		{
			name:     "no existing InboundNatRule and FrontendIPConfigurationID not set",
			spec:     fakeInboundNatSpec(false),
			existing: nil,
			errorMsg: "FrontendIPConfigurationID is not set",
		},
		{
			name:     "existing is not an InboundNatRule",
			spec:     fakeInboundNatSpec(true),
			existing: context.TODO(),
			errorMsg: "*context.emptyCtx is not an armnetwork.InboundNatRule",
		},
		{
			name:     "existing InboundNatRule",
			spec:     fakeInboundNatSpec(false),
			existing: fakeNatRule(),
			expected: nil,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.Background(), tc.existing)
			if tc.errorMsg != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.errorMsg))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Got difference between expected result and computed result:\n%s", cmp.Diff(tc.expected, result))
			}
		})
	}
}

func fakeInboundNatSpec(frontendIPConfigID bool) InboundNatSpec {
	spec := InboundNatSpec{
		Name:             "my-machine-1",
		LoadBalancerName: "my-lb-1",
		ResourceGroup:    fakeGroupName,
	}
	if frontendIPConfigID {
		spec.FrontendIPConfigurationID = ptr.To("frontend-ip-config-id-1")
	}
	return spec
}

// fakeNatRule returns a fake InboundNatRule, associated with `fakeInboundNatSpec()`.
func fakeNatRule() armnetwork.InboundNatRule {
	return armnetwork.InboundNatRule{
		Name: ptr.To("my-machine-1"),
		Properties: &armnetwork.InboundNatRulePropertiesFormat{
			BackendPort:      ptr.To[int32](22),
			EnableFloatingIP: ptr.To(false),
			FrontendIPConfiguration: &armnetwork.SubResource{
				ID: ptr.To("frontend-ip-config-id-1"),
			},
			IdleTimeoutInMinutes: ptr.To[int32](4),
			Protocol:             ptr.To(armnetwork.TransportProtocolTCP),
		},
	}
}

func TestGetAvailablePort(t *testing.T) {
	testcases := []struct {
		name               string
		portsInput         map[int32]struct{}
		expectedError      string
		expectedPortResult int32
	}{
		{
			name:               "Empty ports",
			portsInput:         map[int32]struct{}{},
			expectedError:      "",
			expectedPortResult: 22,
		},
		{
			name: "22 taken",
			portsInput: map[int32]struct{}{
				22: {},
			},
			expectedError:      "",
			expectedPortResult: 2201,
		},
		{
			name: "Existing ports",
			portsInput: map[int32]struct{}{
				22:   {},
				2201: {},
				2202: {},
				2204: {},
			},
			expectedError:      "",
			expectedPortResult: 2203,
		},
		{
			name:               "No ports available",
			portsInput:         getFullPortsMap(),
			expectedError:      "No available SSH Frontend ports",
			expectedPortResult: 0,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			res, err := getAvailableSSHFrontendPort(tc.portsInput)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).To(Equal(tc.expectedPortResult))
			}
		})
	}
}

func getFullPortsMap() map[int32]struct{} {
	res := map[int32]struct{}{
		22: {},
	}
	for i := 2201; i < 2220; i++ {
		res[int32(i)] = struct{}{}
	}
	return res
}
