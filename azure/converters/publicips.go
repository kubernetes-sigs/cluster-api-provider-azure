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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// IPTagsToSDK converts a CAPZ IP tag to an Azure SDK IP tag.
func IPTagsToSDK(ipTags []infrav1.IPTag) *[]network.IPTag {
	if len(ipTags) == 0 {
		return nil
	}
	skdIPTags := make([]network.IPTag, len(ipTags))
	for i, ipTag := range ipTags {
		skdIPTags[i] = network.IPTag{
			IPTagType: pointer.String(ipTag.Type),
			Tag:       pointer.String(ipTag.Tag),
		}
	}
	return &skdIPTags
}

// IPTagsToSDKv2 converts CAPZ IP tags to a slice of pointers to Azure SDKv2 IP tags.
func IPTagsToSDKv2(ipTags []infrav1.IPTag) []*armnetwork.IPTag {
	if len(ipTags) == 0 {
		return nil
	}
	sdkIPTags := make([]*armnetwork.IPTag, len(ipTags))
	for i, ipTag := range ipTags {
		sdkIPTags[i] = &armnetwork.IPTag{
			IPTagType: pointer.String(ipTag.Type),
			Tag:       pointer.String(ipTag.Tag),
		}
	}
	return sdkIPTags
}
