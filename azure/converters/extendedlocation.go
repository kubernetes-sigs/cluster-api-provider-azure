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
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// MapToTags converts a map[string]*string into a infrav1.Tags.
func ExtendedLocationToSDK(src *infrav1.ExtendedLocationSpec) *network.ExtendedLocation {
	if src == nil {
		return nil
	} else {
		return &network.ExtendedLocation{
			Name: to.StringPtr(src.Name),
			Type: network.ExtendedLocationTypes(src.Type),
		}
	}
}
