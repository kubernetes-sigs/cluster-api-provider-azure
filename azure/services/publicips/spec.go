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

package publicips

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	// infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	// "sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// PublicIPSpec defines the specification for a Public IP.
type PublicIPSpec struct {
	Name          string
	DNSName       string
	IsIPv6        bool
	ResourceGroup string
}

// ResourceName returns the name of the public IP.
func (s PublicIPSpec) ResourceName() string {
	return s.Name
}

// OwnerResourceName is a no-op for public IPs.
func (s PublicIPSpec) OwnerResourceName() string {
	return ""
}

// ResourceGroupName returns the name of the resource group the public IP is in.
func (s PublicIPSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// Parameters returns the parameters for the route table.
func (s PublicIPSpec) Parameters(existing interface{}) (interface{}, error) {
	if existing != nil {
		// public IP already exists
		// TODO(karuppiah7890): handle update later
		return nil, nil
	}

	addressVersion := network.IPVersionIPv4
	if s.IsIPv6 {
		addressVersion = network.IPVersionIPv6
	}

	// only set DNS properties if there is a DNS name specified
	var dnsSettings *network.PublicIPAddressDNSSettings
	if s.DNSName != "" {
		dnsSettings = &network.PublicIPAddressDNSSettings{
			DomainNameLabel: to.StringPtr(strings.Split(s.DNSName, ".")[0]),
			Fqdn:            to.StringPtr(s.DNSName),
		}
	}

	return network.PublicIPAddress{
		// TODO(karuppiah7890): Add Tags with Cluster Name and AdditionalTags and other input
		// Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
		// 	ClusterName: s.Scope.ClusterName(),
		// 	Lifecycle: infrav1.ResourceLifecycleOwned,
		// 	Name:      to.StringPtr(s.Name),
		// 	Additional:  s.Scope.AdditionalTags(),
		// })),
		Sku:  &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
		Name: to.StringPtr(s.Name),
		// TODO(karuppiah7890): Add Location
		// Location: to.StringPtr(s.Scope.Location()),
		PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
			PublicIPAddressVersion:   addressVersion,
			PublicIPAllocationMethod: network.IPAllocationMethodStatic,
			DNSSettings:              dnsSettings,
		},
		// TODO(karuppiah7890): Add Zones
		// Zones: s.Zones()
	}, nil
}
