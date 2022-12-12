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

package azure

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestIsAzureSystemNodeLabelKey(t *testing.T) {
	tests := []struct {
		desc     string
		labelKey string
		expected bool
	}{
		{
			desc:     "system prefix as key should report an error",
			labelKey: AzureSystemNodeLabelPrefix,
			expected: true,
		},
		{
			desc:     "system prefix in key should report an error",
			labelKey: AzureSystemNodeLabelPrefix + "/foo",
			expected: true,
		},
		{
			desc:     "empty string should not report error",
			labelKey: "",
			expected: false,
		},
		{
			desc:     "string without system prefix should not report error",
			labelKey: "foo",
			expected: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ret := IsAzureSystemNodeLabelKey(test.labelKey)
			g.Expect(ret).To(Equal(test.expected))
		})
	}
}
