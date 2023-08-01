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

package privatedns

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// LinkSpec defines the specification for a virtual network link in a private DNS zone.
type LinkSpec struct {
	Name              string
	ZoneName          string
	SubscriptionID    string
	VNetResourceGroup string
	VNetName          string
	ResourceGroup     string
	ClusterName       string
	AdditionalTags    infrav1.Tags
}

// ResourceName returns the name of the virtual network link.
func (s LinkSpec) ResourceName() string {
	return s.Name
}

// OwnerResourceName returns the zone name of the virtual network link.
func (s LinkSpec) OwnerResourceName() string {
	return s.ZoneName
}

// ResourceGroupName returns the name of the resource group of the virtual network link.
func (s LinkSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// Parameters returns the parameters for the virtual network link.
func (s LinkSpec) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	if existing != nil {
		_, ok := existing.(privatedns.VirtualNetworkLink)
		if !ok {
			return nil, errors.Errorf("%T is not a privatedns.VirtualNetworkLink", existing)
		}
		return nil, nil
	}

	return privatedns.VirtualNetworkLink{
		VirtualNetworkLinkProperties: &privatedns.VirtualNetworkLinkProperties{
			VirtualNetwork: &privatedns.SubResource{
				ID: ptr.To(azure.VNetID(s.SubscriptionID, s.VNetResourceGroup, s.VNetName)),
			},
			RegistrationEnabled: ptr.To(false),
		},
		Location: ptr.To(azure.Global),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Additional:  s.AdditionalTags,
		})),
	}, nil
}
