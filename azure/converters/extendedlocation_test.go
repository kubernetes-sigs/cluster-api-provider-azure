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

package converters

import (
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestExtendedLocationToNetworkSDK(t *testing.T) {
	tests := []struct {
		name string
		args *infrav1.ExtendedLocationSpec
		want *armnetwork.ExtendedLocation
	}{
		{
			name: "normal extendedLocation instance",
			args: &infrav1.ExtendedLocationSpec{
				Name: "value",
				Type: "EdgeZone",
			},
			want: &armnetwork.ExtendedLocation{
				Name: ptr.To("value"),
				Type: ptr.To(armnetwork.ExtendedLocationTypesEdgeZone),
			},
		},
		{
			name: "nil extendedLocation properties",
			args: nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtendedLocationToNetworkSDK(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtendedLocationToNetworkSDK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtendedLocationToNetworkASO(t *testing.T) {
	tests := []struct {
		name string
		args *infrav1.ExtendedLocationSpec
		want *asonetworkv1.ExtendedLocation
	}{
		{
			name: "normal extendedLocation instance",
			args: &infrav1.ExtendedLocationSpec{
				Name: "value",
				Type: "EdgeZone",
			},
			want: &asonetworkv1.ExtendedLocation{
				Name: ptr.To("value"),
				Type: ptr.To(asonetworkv1.ExtendedLocationType_EdgeZone),
			},
		},
		{
			name: "nil extendedLocation properties",
			args: nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtendedLocationToNetworkASO(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtendedLocationToNetworkASO() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtendedLocationToComputeSDK(t *testing.T) {
	tests := []struct {
		name string
		args *infrav1.ExtendedLocationSpec
		want *armcompute.ExtendedLocation
	}{
		{
			name: "normal extendedLocation instance",
			args: &infrav1.ExtendedLocationSpec{
				Name: "value",
				Type: "Edge",
			},
			want: &armcompute.ExtendedLocation{
				Name: ptr.To("value"),
				Type: ptr.To(armcompute.ExtendedLocationTypes("Edge")),
			},
		},
		{
			name: "nil extendedLocation properties",
			args: nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtendedLocationToComputeSDK(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtendedLocationToComputeSDK() = %v, want %v", got, tt.want)
			}
		})
	}
}
