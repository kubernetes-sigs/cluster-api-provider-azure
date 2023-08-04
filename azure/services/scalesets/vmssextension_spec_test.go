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

package scalesets

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

var (
	fakeVMSSExtensionSpec = VMSSExtensionSpec{
		azure.ExtensionSpec{
			Name:              "my-vm-extension",
			VMName:            "my-vm",
			Publisher:         "my-publisher",
			Version:           "1.0",
			Settings:          map[string]string{"my-setting": "my-value"},
			ProtectedSettings: map[string]string{"my-protected-setting": "my-protected-value"},
		},
		"my-rg",
	}

	fakeVMSSExtensionParams = compute.VirtualMachineScaleSetExtension{
		Name: ptr.To("my-vm-extension"),
		VirtualMachineScaleSetExtensionProperties: &compute.VirtualMachineScaleSetExtensionProperties{
			Publisher:          ptr.To("my-publisher"),
			Type:               ptr.To("my-vm-extension"),
			TypeHandlerVersion: ptr.To("1.0"),
			Settings:           map[string]string{"my-setting": "my-value"},
			ProtectedSettings:  map[string]string{"my-protected-setting": "my-protected-value"},
		},
	}
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *VMSSExtensionSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "get parameters for vmextension",
			spec:     &fakeVMSSExtensionSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(Equal(fakeVMSSExtensionParams))
			},
			expectedError: "",
		},
		{
			name:     "vmextension that already exists",
			spec:     &fakeVMSSExtensionSpec,
			existing: fakeVMSSExtensionParams,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
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
