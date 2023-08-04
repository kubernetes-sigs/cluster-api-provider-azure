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
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// ExtendedLocationToNetworkSDK converts infrav1.ExtendedLocationSpec to network.ExtendedLocation.
func ExtendedLocationToNetworkSDK(src *infrav1.ExtendedLocationSpec) *network.ExtendedLocation {
	if src == nil {
		return nil
	}
	return &network.ExtendedLocation{
		Name: ptr.To(src.Name),
		Type: network.ExtendedLocationTypes(src.Type),
	}
}

// ExtendedLocationToComputeSDK converts infrav1.ExtendedLocationSpec to compute.ExtendedLocation.
func ExtendedLocationToComputeSDK(src *infrav1.ExtendedLocationSpec) *compute.ExtendedLocation {
	if src == nil {
		return nil
	}
	return &compute.ExtendedLocation{
		Name: ptr.To(src.Name),
		Type: compute.ExtendedLocationTypes(src.Type),
	}
}
