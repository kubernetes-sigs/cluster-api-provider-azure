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

package azure

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

func TestStringSlice(t *testing.T) {
	cases := []struct {
		Name     string
		Arg      *[]string
		Expected []string
	}{
		{
			Name:     "Should return nil if the pointer is nil",
			Arg:      nil,
			Expected: nil,
		},
		{
			Name:     "Should return string slice value for the passed string slice pointer",
			Arg:      &[]string{"foo", "bar"},
			Expected: []string{"foo", "bar"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			g := NewWithT(t)
			actual := StringSlice(tc.Arg)
			g.Expect(tc.Expected).To(Equal(actual))
		})
	}
}

func TestStringMapPtr(t *testing.T) {
	cases := []struct {
		Name     string
		Arg      map[string]string
		Expected map[string]*string
	}{
		{
			Name:     "Should return nil if the map is nil",
			Arg:      nil,
			Expected: nil,
		},
		{
			Name:     "Should convert to a map[string]*string",
			Arg:      map[string]string{"foo": "baz", "bar": "qux"},
			Expected: map[string]*string{"foo": ptr.To("baz"), "bar": ptr.To("qux")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := StringMapPtr(tc.Arg)
			if !cmp.Equal(tc.Expected, actual) {
				t.Errorf("Got difference between expected result and result %v", cmp.Diff(tc.Expected, actual))
			}
		})
	}
}

func TestPtrSlice(t *testing.T) {
	cases := []struct {
		Name     string
		Arg      *[]string
		Expected []*string
	}{
		{
			Name:     "Should return nil if the pointer is nil",
			Arg:      nil,
			Expected: nil,
		},
		{
			Name:     "Should return nil if the slice pointed to is empty",
			Arg:      &[]string{},
			Expected: nil,
		},
		{
			Name:     "Should return slice of pointers from a pointer to a slice",
			Arg:      &[]string{"foo", "bar"},
			Expected: []*string{ptr.To("foo"), ptr.To("bar")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			g := NewWithT(t)
			actual := PtrSlice(tc.Arg)
			g.Expect(tc.Expected).To(Equal(actual))
		})
	}
}

func TestAliasOrNil(t *testing.T) {
	type TestAlias string
	cases := []struct {
		Name     string
		Arg      *string
		Expected *string
	}{
		{
			Name: "Should return nil if the pointer is nil",
		},
		{
			Name: "Should return nil for an empty string pointer",
			Arg:  ptr.To(""),
		},
		{
			Name:     "Should return a string pointer",
			Arg:      ptr.To("foo"),
			Expected: ptr.To("foo"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			g := NewWithT(t)

			// Test with string
			actual := AliasOrNil[string](tc.Arg)
			g.Expect(tc.Expected).To(Equal(actual))

			// Test with type alias
			aliasActual := AliasOrNil[TestAlias](tc.Arg)
			if tc.Expected == nil {
				g.Expect(aliasActual).To(BeNil())
			} else {
				g.Expect(aliasActual).To(Equal(ptr.To(TestAlias(*tc.Expected))))
			}
		})
	}
}
