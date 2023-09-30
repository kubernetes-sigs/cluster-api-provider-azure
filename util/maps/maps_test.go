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

package maps

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestMerge(t *testing.T) {
	tests := []struct {
		name      string
		base      map[string]string
		overrides map[string]string
		expected  map[string]string
	}{
		{
			name:      "nil base",
			base:      nil,
			overrides: map[string]string{"key": "value"},
			expected:  map[string]string{"key": "value"},
		},
		{
			name:      "nil overrides",
			base:      map[string]string{"key": "value"},
			overrides: nil,
			expected:  map[string]string{"key": "value"},
		},
		{
			name:      "overrides takes precedence",
			base:      map[string]string{"key": "base"},
			overrides: map[string]string{"key": "overrides"},
			expected:  map[string]string{"key": "overrides"},
		},
		{
			name:      "non-overlapping keys are all preserved",
			base:      map[string]string{"base": "value"},
			overrides: map[string]string{"overrides": "value"},
			expected:  map[string]string{"base": "value", "overrides": "value"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			g.Expect(Merge(test.base, test.overrides)).To(gomega.Equal(test.expected))
		})
	}
}
