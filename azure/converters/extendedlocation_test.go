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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestExtendedLocationToNetworkSDK(t *testing.T) {
	tests := []struct {
		name string
		args *infrav1.ExtendedLocationSpec
		want *network.ExtendedLocation
	}{
		{
			name: "normal extendedLocation instance",
			args: &infrav1.ExtendedLocationSpec{
				Name: "value",
				Type: "Edge",
			},
			want: &network.ExtendedLocation{
				Name: to.StringPtr("value"),
				Type: network.ExtendedLocationTypes("Edge"),
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

func TestExtendedLocationToComputeSDK(t *testing.T) {
	tests := []struct {
		name string
		args *infrav1.ExtendedLocationSpec
		want *compute.ExtendedLocation
	}{
		{
			name: "normal extendedLocation instance",
			args: &infrav1.ExtendedLocationSpec{
				Name: "value",
				Type: "Edge",
			},
			want: &compute.ExtendedLocation{
				Name: to.StringPtr("value"),
				Type: compute.ExtendedLocationTypes("Edge"),
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
