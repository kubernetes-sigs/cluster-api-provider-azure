/*
Copyright 2023 The Kubernetes Authors.

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

package versions

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetHigherK8sVersion(t *testing.T) {
	cases := []struct {
		name   string
		a      string
		b      string
		output string
	}{
		{
			name:   "b is greater than a",
			a:      "v1.17.8",
			b:      "v1.18.8",
			output: "v1.18.8",
		},
		{
			name:   "a is greater than b",
			a:      "v1.18.9",
			b:      "v1.18.8",
			output: "v1.18.9",
		},
		{
			name:   "b is greater than a",
			a:      "v1.18",
			b:      "v1.18.8",
			output: "v1.18.8",
		},
		{
			name:   "a is equal to b",
			a:      "v1.18.8",
			b:      "v1.18.8",
			output: "v1.18.8",
		},
		{
			name:   "a is greater than b and a is major.minor",
			a:      "v1.18",
			b:      "v1.17.8",
			output: "v1.18",
		},
		{
			name:   "a is greater than b and a is major.minor",
			a:      "1.18",
			b:      "1.17.8",
			output: "1.18",
		},
		{
			name:   "a is invalid",
			a:      "1.18.",
			b:      "v1.17.8",
			output: "v1.17.8",
		},
		{
			name:   "b is invalid",
			a:      "1.18.1",
			b:      "v1.17.8.",
			output: "1.18.1",
		},
		{
			name:   "b is invalid",
			a:      "9.99.9999",
			b:      "v1.17.8.",
			output: "9.99.9999",
		},
		{
			name:   "a is higher",
			a:      "1.20.3",
			b:      "v1.15.3",
			output: "1.20.3",
		},
		{
			name:   "a & b is invalid",
			a:      "",
			b:      "v1.17.8.",
			output: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := NewWithT(t)
			output := GetHigherK8sVersion(c.a, c.b)
			g.Expect(output).To(Equal(c.output))
		})
	}
}
