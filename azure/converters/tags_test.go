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

package converters

import (
	"reflect"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestMapToTags(t *testing.T) {
	tests := []struct {
		name   string
		srcMap map[string]*string
		want   v1beta1.Tags
	}{
		{
			name:   "convert nil map to tags",
			srcMap: map[string]*string{},
			want:   map[string]string{},
		},
		{
			name:   "convert empty map to tags",
			srcMap: map[string]*string{},
			want:   map[string]string{},
		},
		{
			name: "convert map to tags with values being nil",
			srcMap: map[string]*string{
				"key1": nil,
				"key2": nil,
			},
			want: map[string]string{
				"key1": "",
				"key2": "",
			},
		},
		{
			name: "convert map to tags",
			srcMap: map[string]*string{
				"key1": to.StringPtr("val1"),
				"key2": to.StringPtr("val2"),
				"key3": to.StringPtr("val3"),
			},
			want: map[string]string{
				"key1": "val1",
				"key2": "val2",
				"key3": "val3",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapToTags(tt.srcMap); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MapToTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTagsToMap(t *testing.T) {
	tests := []struct {
		name string
		tags v1beta1.Tags
		want map[string]*string
	}{
		{
			name: "convert nil tag to map",
			tags: nil,
			want: map[string]*string{},
		},
		{
			name: "convert empty tags to map",
			tags: map[string]string{},
			want: map[string]*string{},
		},
		{
			name: "convert tags to map",
			tags: map[string]string{
				"key1": "val1",
				"key2": "val2",
				"key3": "val3",
			},
			want: map[string]*string{
				"key1": to.StringPtr("val1"),
				"key2": to.StringPtr("val2"),
				"key3": to.StringPtr("val3"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagsToMap(tt.tags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TagsToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
