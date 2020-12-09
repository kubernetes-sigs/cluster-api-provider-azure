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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCacheGet(t *testing.T) {
	cases := map[string]struct {
		sku          string
		resourceType ResourceType
		have         []compute.ResourceSku
		err          string
	}{
		"should find": {
			sku:          "foo",
			resourceType: "bar",
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("other"),
					ResourceType: to.StringPtr("baz"),
				},
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr("bar"),
				},
			},
		},
		"should not find": {
			sku:          "foo",
			resourceType: "bar",
			have: []compute.ResourceSku{
				{
					Name: to.StringPtr("other"),
				},
			},
			err: "resource sku with name 'foo' and category 'bar' not found",
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache := &Cache{
				data: tc.have,
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
		have []compute.ResourceSku
		want []string
	}{
		"should find 1 result": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"1"},
						},
					},
				},
			},
			want: []string{"1"},
		},
		"should find 2 results": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"1"},
						},
					},
				},
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"2"},
						},
					},
				},
			},
			want: []string{"1", "2"},
		},
		"should not find due to location mismatch": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"foobar",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("foobar"),
							Zones:    &[]string{"1"},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to location restriction": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"1"},
						},
					},
					Restrictions: &[]compute.ResourceSkuRestrictions{
						{
							Type:   compute.Location,
							Values: &[]string{"baz"},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to zone restriction": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"1"},
						},
					},
					Restrictions: &[]compute.ResourceSkuRestrictions{
						{
							Type: compute.Zone,
							RestrictionInfo: &compute.ResourceSkuRestrictionInfo{
								Zones: &[]string{"1"},
							},
						},
					},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		tc := tc
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
				t.Errorf(diff)
			}
		})
	}
}

func TestCacheGetZonesWithVMSize(t *testing.T) {
	cases := map[string]struct {
		have []compute.ResourceSku
		want []string
	}{
		"should find 1 result": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"1"},
						},
					},
				},
			},
			want: []string{"1"},
		},
		"should find 2 results": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"1", "2"},
						},
					},
				},
			},
			want: []string{"1", "2"},
		},
		"should not find due to size mismatch": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foobar"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"1"},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to location mismatch": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"foobar",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("foobar"),
							Zones:    &[]string{"1"},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to location restriction": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"1"},
						},
					},
					Restrictions: &[]compute.ResourceSkuRestrictions{
						{
							Type:   compute.Location,
							Values: &[]string{"baz"},
						},
					},
				},
			},
			want: nil,
		},
		"should not find due to zone restriction": {
			have: []compute.ResourceSku{
				{
					Name:         to.StringPtr("foo"),
					ResourceType: to.StringPtr(string(VirtualMachines)),
					Locations: &[]string{
						"baz",
					},
					LocationInfo: &[]compute.ResourceSkuLocationInfo{
						{
							Location: to.StringPtr("baz"),
							Zones:    &[]string{"1"},
						},
					},
					Restrictions: &[]compute.ResourceSkuRestrictions{
						{
							Type: compute.Zone,
							RestrictionInfo: &compute.ResourceSkuRestrictionInfo{
								Zones: &[]string{"1"},
							},
						},
					},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		tc := tc
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
				t.Fatalf(diff)
			}
		})
	}
}
