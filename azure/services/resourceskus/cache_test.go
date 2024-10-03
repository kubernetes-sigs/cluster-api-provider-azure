/*
Copyright 2020 The Kubernetes Authors.

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

package resourceskus

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/utils/ptr"
)

func TestCacheGet(t *testing.T) {
	cases := map[string]struct {
		sku          string
		location     string
		resourceType ResourceType
		have         []armcompute.ResourceSKU
		err          string
	}{
		"should find": {
			sku:          "foo",
			location:     "test",
			resourceType: "bar",
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("other"),
					ResourceType: ptr.To("baz"),
				},
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To("bar"),
				},
			},
		},
		"should not find": {
			sku:          "foo",
			location:     "test",
			resourceType: "bar",
			have: []armcompute.ResourceSKU{
				{
					Name: ptr.To("other"),
				},
			},
			err: "reconcile error that cannot be recovered occurred: resource sku with name 'foo' and category 'bar' not found in location 'test'. Object will not be requeued",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache := &Cache{
				data:     tc.have,
				location: tc.location,
			}

			val, err := cache.Get(context.Background(), tc.sku, tc.resourceType)
			if tc.err != "" {
				if err == nil {
					t.Fatalf("expected cache.get to fail with error %s, but actual error was nil", tc.err)
					return
				}
				if err.Error() != tc.err {
					t.Fatalf("expected cache.get to fail with error %s, but actual error was %s", tc.err, err.Error())
					return
				}
			} else {
				if val.Name == nil {
					t.Fatalf("expected name to be %s, but was nil", tc.sku)
					return
				}
				if *val.Name != tc.sku {
					t.Fatalf("expected name to be %s, but was %s", tc.sku, *val.Name)
				}
				if val.ResourceType == nil {
					t.Fatalf("expected name to be %s, but was nil", tc.sku)
					return
				}
				if *val.ResourceType != string(tc.resourceType) {
					t.Fatalf("expected kind to be %s, but was %s", tc.resourceType, *val.ResourceType)
				}
			}
		})
	}
}

func TestCacheGetZones(t *testing.T) {
	cases := map[string]struct {
		have []armcompute.ResourceSKU
		want []string
	}{
		"should find 1 result": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("1")},
						},
					},
				},
			},
			want: []string{"1"},
		},
		"should find 2 results": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("1")},
						},
					},
				},
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("2")},
						},
					},
				},
			},
			want: []string{"1", "2"},
		},
		"should not find due to location mismatch": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("foobar"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("foobar"),
							Zones:    []*string{ptr.To("1")},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to location restriction": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("1")},
						},
					},
					Restrictions: []*armcompute.ResourceSKURestrictions{
						{
							Type:   ptr.To(armcompute.ResourceSKURestrictionsTypeLocation),
							Values: []*string{ptr.To("baz")},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to zone restriction": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("1")},
						},
					},
					Restrictions: []*armcompute.ResourceSKURestrictions{
						{
							Type: ptr.To(armcompute.ResourceSKURestrictionsTypeZone),
							RestrictionInfo: &armcompute.ResourceSKURestrictionInfo{
								Zones: []*string{ptr.To("1")},
							},
						},
					},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache := &Cache{
				data: tc.have,
			}

			zones, err := cache.GetZones(context.Background(), "baz")
			if err != nil {
				t.Error(err)
			}
			if diff := cmp.Diff(zones, tc.want, []cmp.Option{cmpopts.EquateEmpty()}...); diff != "" {
				t.Errorf("%s", diff)
			}
		})
	}
}

func TestCacheGetZonesWithVMSize(t *testing.T) {
	cases := map[string]struct {
		have []armcompute.ResourceSKU
		want []string
	}{
		"should find 1 result": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("1")},
						},
					},
				},
			},
			want: []string{"1"},
		},
		"should find 2 results": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("1"), ptr.To("2")},
						},
					},
				},
			},
			want: []string{"1", "2"},
		},
		"should not find due to size mismatch": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foobar"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("1")},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to location mismatch": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("foobar"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("foobar"),
							Zones:    []*string{ptr.To("1")},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to location restriction": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("1")},
						},
					},
					Restrictions: []*armcompute.ResourceSKURestrictions{
						{
							Type:   ptr.To(armcompute.ResourceSKURestrictionsTypeLocation),
							Values: []*string{ptr.To("baz")},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to zone restriction": {
			have: []armcompute.ResourceSKU{
				{
					Name:         ptr.To("foo"),
					ResourceType: ptr.To(string(VirtualMachines)),
					Locations: []*string{
						ptr.To("baz"),
					},
					LocationInfo: []*armcompute.ResourceSKULocationInfo{
						{
							Location: ptr.To("baz"),
							Zones:    []*string{ptr.To("1")},
						},
					},
					Restrictions: []*armcompute.ResourceSKURestrictions{
						{
							Type: ptr.To(armcompute.ResourceSKURestrictionsTypeZone),
							RestrictionInfo: &armcompute.ResourceSKURestrictionInfo{
								Zones: []*string{ptr.To("1")},
							},
						},
					},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache := &Cache{
				data: tc.have,
			}

			zones, err := cache.GetZonesWithVMSize(context.Background(), "foo", "baz")
			if err != nil {
				t.Error(err)
			}
			if diff := cmp.Diff(zones, tc.want, []cmp.Option{cmpopts.EquateEmpty()}...); diff != "" {
				t.Fatalf("%s", diff)
			}
		})
	}
}
