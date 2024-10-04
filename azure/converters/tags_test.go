/*
Copyright 2019 The Kubernetes Authors.

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

package converters

import (
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func Test_TagsToMap(t *testing.T) {
	cases := []struct {
		tags   infrav1.Tags
		expect map[string]*string
	}{
		{
			tags:   nil,
			expect: nil,
		},
		{
			tags:   infrav1.Tags{},
			expect: map[string]*string{},
		},
		{
			tags:   infrav1.Tags{"env": "prod"},
			expect: map[string]*string{"env": ptr.To("prod")},
		},
	}

	for _, c := range cases {
		t.Run("name", func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			result := TagsToMap(c.tags)
			g.Expect(c.expect).To(gomega.BeEquivalentTo(result))
		})
	}
}

func Test_MapToTags(t *testing.T) {
	cases := []struct {
		tags   map[string]*string
		expect infrav1.Tags
	}{
		{
			tags:   nil,
			expect: nil,
		},
		{
			tags:   map[string]*string{},
			expect: infrav1.Tags{},
		},
		{
			tags:   map[string]*string{"env": ptr.To("prod")},
			expect: infrav1.Tags{"env": "prod"},
		},
	}

	for _, c := range cases {
		t.Run("name", func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			result := MapToTags(c.tags)
			g.Expect(c.expect).To(gomega.BeEquivalentTo(result))
		})
	}
}

// convert the value to map then back to tags
// and make sure that the value we get is strictly equal
// to the original value (ie. round trip conversion).
func Test_TagsToMapRoundTrip(t *testing.T) {
	cases := []struct {
		tags   infrav1.Tags
		expect infrav1.Tags
	}{
		{
			tags:   nil,
			expect: nil,
		},
		{
			tags:   infrav1.Tags{},
			expect: infrav1.Tags{},
		},
		{
			tags:   infrav1.Tags{"env": "prod"},
			expect: infrav1.Tags{"env": "prod"},
		},
	}

	for _, c := range cases {
		t.Run("name", func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			result := MapToTags(TagsToMap(c.tags))
			g.Expect(c.expect).To(gomega.BeEquivalentTo(result))
		})
	}
}

// convert the value to tags then back to map
// and make sure that the value we get is strictly equal
// to the original value (ie. round trip conversion).
func Test_MapToTagsMapRoundTrip(t *testing.T) {
	cases := []struct {
		tags   map[string]*string
		expect map[string]*string
	}{
		{
			tags:   nil,
			expect: nil,
		},
		{
			tags:   map[string]*string{},
			expect: map[string]*string{},
		},
		{
			tags:   map[string]*string{"env": ptr.To("prod")},
			expect: map[string]*string{"env": ptr.To("prod")},
		},
	}

	for _, c := range cases {
		t.Run("name", func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			result := TagsToMap(MapToTags(c.tags))
			g.Expect(c.expect).To(gomega.BeEquivalentTo(result))
		})
	}
}
