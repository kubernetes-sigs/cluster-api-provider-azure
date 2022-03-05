/*
Copyright 2021 The Kubernetes Authors.

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

package bastionhosts

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// AzureBastionSpec defines the specification for azure bastion feature.
type AzureBastionSpec struct {
	Name          string
	ResourceGroup string
	Location      string
	ClusterName   string
	SubnetID      string
	PublicIPID    string
}

// AzureBastionSpecInput defines the required inputs to construct an azure bastion spec.
type AzureBastionSpecInput struct {
	SubnetName   string
	PublicIPName string
	VNetName     string
}

// ResourceName returns the name of the bastion host.
func (s *AzureBastionSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *AzureBastionSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for bastion hosts.
func (s *AzureBastionSpec) OwnerResourceName() string {
	return ""
}

// Parameters returns the parameters for the bastion host.
func (s *AzureBastionSpec) Parameters(existing interface{}) (parameters interface{}, err error) {
	if existing != nil {
		if _, ok := existing.(network.BastionHost); !ok {
			return nil, errors.Errorf("%T is not a network.BastionHost", existing)
		}
		// bastion host already exists
		return nil, nil
	}

	bastionHostIPConfigName := fmt.Sprintf("%s-%s", s.Name, "bastionIP")

	return network.BastionHost{
		Name:     to.StringPtr(s.Name),
		Location: to.StringPtr(s.Location),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        to.StringPtr(s.Name),
			Role:        to.StringPtr("Bastion"),
		})),
		BastionHostPropertiesFormat: &network.BastionHostPropertiesFormat{
			DNSName: to.StringPtr(fmt.Sprintf("%s-bastion", strings.ToLower(s.Name))),
			IPConfigurations: &[]network.BastionHostIPConfiguration{
				{
					Name: to.StringPtr(bastionHostIPConfigName),
					BastionHostIPConfigurationPropertiesFormat: &network.BastionHostIPConfigurationPropertiesFormat{
						Subnet: &network.SubResource{
							ID: &s.SubnetID,
						},
						PublicIPAddress: &network.SubResource{
							ID: &s.PublicIPID,
						},
						PrivateIPAllocationMethod: network.IPAllocationMethodDynamic,
					},
				},
			},
		},
	}, nil
}
