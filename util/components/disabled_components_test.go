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

package components

import (
	"testing"

	. "github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestIsValidDisableComponent(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name     string
		value    string
		expected bool
	}{
		{
			name:     "Valid component",
			value:    string(infrav1.DisableASOSecretController),
			expected: true,
		},
		{
			name:     "Invalid component",
			value:    "InvalidComponent",
			expected: false,
		},
		{
			name:     "Empty string",
			value:    "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsValidDisableComponent(tc.value)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestIsComponentDisabled(t *testing.T) {
	g := NewGomegaWithT(t)

	testCases := []struct {
		name               string
		disabledComponents []string
		component          infrav1.DisableComponent
		expectedResult     bool
	}{
		{
			name:               "When DisableASOSecretController is in the list, expect true",
			disabledComponents: []string{"DisableASOSecretController", "component2"},
			component:          infrav1.DisableASOSecretController,
			expectedResult:     true,
		},
		{
			name:               "When DisableASOSecretController is not in the list, expect false",
			disabledComponents: []string{"component", "component2"},
			component:          infrav1.DisableASOSecretController,
			expectedResult:     false,
		},
		{
			name:               "When the list is empty, expect false",
			disabledComponents: []string{},
			component:          infrav1.DisableComponent("component"),
			expectedResult:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsComponentDisabled(tc.disabledComponents, tc.component)
			g.Expect(result).To(Equal(tc.expectedResult))
		})
	}
}
