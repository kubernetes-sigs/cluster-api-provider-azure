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
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
)

// SDKToVnet converts azure representation to internal representation
func SDKToVnet(vnet network.VirtualNetwork, managed string) *v1alpha1.VnetSpec {
	return &v1alpha1.VnetSpec{
		ID:        *vnet.ID,
		Name:      *vnet.Name,
		CidrBlock: (*vnet.AddressSpace.AddressPrefixes)[0],
		Managed:   strings.ToLower(managed),
	}
}
