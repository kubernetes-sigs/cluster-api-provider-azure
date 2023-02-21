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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestIPTagsToSDK(t *testing.T) {
	tests := []struct {
		name   string
		ipTags []infrav1.IPTag
		want   *[]network.IPTag
	}{
		{
			name:   "empty",
			ipTags: []infrav1.IPTag{},
			want:   nil,
		},
		{
			name: "list of tags",
			ipTags: []infrav1.IPTag{
				{
					Type: "tag",
					Tag:  "value",
				},
				{
					Type: "internal",
					Tag:  "foo",
				},
			},
			want: &[]network.IPTag{
				{
					IPTagType: pointer.String("tag"),
					Tag:       pointer.String("value"),
				},
				{
					IPTagType: pointer.String("internal"),
					Tag:       pointer.String("foo"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IPTagsToSDK(tt.ipTags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IPTagsToSDK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIPTagsToSDKv2(t *testing.T) {
	tests := []struct {
		name   string
		ipTags []infrav1.IPTag
		want   []*armnetwork.IPTag
	}{
		{
			name:   "empty",
			ipTags: []infrav1.IPTag{},
			want:   nil,
		},
		{
			name: "list of tags",
			ipTags: []infrav1.IPTag{
				{
					Type: "tag",
					Tag:  "value",
				},
				{
					Type: "internal",
					Tag:  "foo",
				},
			},
			want: []*armnetwork.IPTag{
				{
					IPTagType: pointer.String("tag"),
					Tag:       pointer.String("value"),
				},
				{
					IPTagType: pointer.String("internal"),
					Tag:       pointer.String("foo"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IPTagsToSDKv2(tt.ipTags); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IPTagsToSDK() = %v, want %v", got, tt.want)
			}
		})
	}
}
