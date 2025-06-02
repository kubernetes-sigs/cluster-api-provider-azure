/*
Copyright 2025 The Kubernetes Authors.

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

package v1beta2

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestAROMAchinePoolWebhook_Default(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	testCases := []struct {
		name                 string
		inputNodePoolName    string
		expectedNodePoolName string
		description          string
	}{
		{
			name:                 "empty nodepoolname uses metadata name",
			inputNodePoolName:    "",
			expectedNodePoolName: "test-pool",
			description:          "should use metadata name when nodepoolname is empty",
		},
		{
			name:                 "existing nodepoolname unchanged",
			inputNodePoolName:    "custom-pool",
			expectedNodePoolName: "custom-pool",
			description:          "should leave existing nodepoolname unchanged",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			machinePool := &AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: AROMachinePoolSpec{
					NodePoolName: tc.inputNodePoolName,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			webhook := &aroMachinePoolWebhook{Client: fakeClient}

			err := webhook.Default(t.Context(), machinePool)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(machinePool.Spec.NodePoolName).To(Equal(tc.expectedNodePoolName), tc.description)
		})
	}
}

func TestValidateOCPVersionAROMachinePool(t *testing.T) {
	testCases := []struct {
		name        string
		version     string
		expectError bool
		description string
	}{
		{
			name:        "valid semantic version",
			version:     "4.14.5",
			expectError: false,
			description: "should accept valid semantic version",
		},
		{
			name:        "valid semantic version with pre-release",
			version:     "4.14.5-rc.1",
			expectError: false,
			description: "should accept semantic version with pre-release",
		},
		{
			name:        "valid semantic version with build metadata",
			version:     "4.14.5+build.1",
			expectError: false,
			description: "should accept semantic version with build metadata",
		},
		{
			name:        "invalid version format X.Y only",
			version:     "4.14",
			expectError: true,
			description: "should reject X.Y format without patch version",
		},
		{
			name:        "invalid version format with letters",
			version:     "4.14.abc",
			expectError: true,
			description: "should reject version with letters in patch",
		},
		{
			name:        "empty version",
			version:     "",
			expectError: true,
			description: "should reject empty version",
		},
		{
			name:        "invalid version format single number",
			version:     "4",
			expectError: true,
			description: "should reject incomplete version with single number",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			machinePool := &AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: AROMachinePoolSpec{
					NodePoolName: "test-pool",
					Version:      tc.version,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
			err := machinePool.Validate(fakeClient)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred(), tc.description)
				g.Expect(err.Error()).To(ContainSubstring("must be a <valid semantic version>"), "error message should match expected format")
			} else {
				g.Expect(err).NotTo(HaveOccurred(), tc.description)
			}
		})
	}
}

func TestValidateNodePoolName(t *testing.T) {
	testCases := []struct {
		name         string
		nodePoolName string
		expectError  bool
		description  string
	}{
		{
			name:         "valid short name",
			nodePoolName: "abc",
			expectError:  false,
			description:  "should accept 3 character name",
		},
		{
			name:         "valid name with hyphens",
			nodePoolName: "w-uksouth-0",
			expectError:  false,
			description:  "should accept name with hyphens",
		},
		{
			name:         "valid max length name",
			nodePoolName: "a123456789012bc",
			expectError:  false,
			description:  "should accept 15 character name",
		},
		{
			name:         "too long name",
			nodePoolName: "mveber2-int-mp-0",
			expectError:  true,
			description:  "should reject name longer than 15 characters",
		},
		{
			name:         "too short name",
			nodePoolName: "ab",
			expectError:  true,
			description:  "should reject name shorter than 3 characters",
		},
		{
			name:         "starts with number",
			nodePoolName: "0pool",
			expectError:  true,
			description:  "should reject name starting with number",
		},
		{
			name:         "ends with hyphen",
			nodePoolName: "pool-",
			expectError:  true,
			description:  "should reject name ending with hyphen",
		},
		{
			name:         "empty name",
			nodePoolName: "",
			expectError:  true,
			description:  "should reject empty name",
		},
		{
			name:         "contains underscore",
			nodePoolName: "pool_name",
			expectError:  true,
			description:  "should reject name with underscore",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			machinePool := &AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: AROMachinePoolSpec{
					NodePoolName: tc.nodePoolName,
					Version:      "4.19.0",
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
			err := machinePool.Validate(fakeClient)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred(), tc.description)
			} else {
				g.Expect(err).NotTo(HaveOccurred(), tc.description)
			}
		})
	}
}
