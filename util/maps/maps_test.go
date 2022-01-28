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

func TestFilterByKeyPrefix(t *testing.T) {
	cases := []struct {
		Name     string
		Input    map[string]string
		Prefix   string
		Expected map[string]string
	}{
		{
			Name: "TestMixed",
			Input: map[string]string{
				"prefix-key1": "value1",
				"prefix-key2": "value2",
				"PrEfIx-key3": "value3",
				"prefix-":     "value4",
				"":            "value5",
				"foobar":      "value6",
				"prefix-key4": "",
			},
			Prefix: "prefix-",
			Expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key4": "",
			},
		},
		{
			Name:     "WithEmptyInput",
			Input:    map[string]string{},
			Prefix:   "prefix-",
			Expected: map[string]string{},
		},
		{
			Name:     "WithNilInput",
			Input:    nil,
			Prefix:   "prefix-",
			Expected: map[string]string{},
		},
		{
			Name: "WithEmptyPrefix",
			Input: map[string]string{
				"prefix-key1": "value1",
			},
			Prefix: "",
			Expected: map[string]string{
				"prefix-key1": "value1",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			g.Expect(FilterByKeyPrefix(c.Input, c.Prefix)).To(gomega.Equal(c.Expected))
		})
	}
}
