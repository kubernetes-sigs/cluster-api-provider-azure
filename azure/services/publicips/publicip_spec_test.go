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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
)

func TestParameters(t *testing.T) {
	testCases := []struct {
		name                    string
		existingPublicIPAddress *network.PublicIPAddress
		publicIPSpec            PublicIPSpec
		expectedPublicIPAddress network.PublicIPAddress
	}{
		{
			name:                    "ipv4 public ip address with dns",
			existingPublicIPAddress: nil,
			publicIPSpec: PublicIPSpec{
				Name:    "my-publicip",
				DNSName: "fakedns.mydomain.io",
			},
			expectedPublicIPAddress: network.PublicIPAddress{
				Name:     to.StringPtr("my-publicip"),
				Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
				Location: to.StringPtr("testlocation"),
				Tags: map[string]*string{
					"Name": to.StringPtr("my-publicip"),
					"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
				},
				PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
					PublicIPAddressVersion:   network.IPVersionIPv4,
					PublicIPAllocationMethod: network.IPAllocationMethodStatic,
					DNSSettings: &network.PublicIPAddressDNSSettings{
						DomainNameLabel: to.StringPtr("fakedns"),
						Fqdn:            to.StringPtr("fakedns.mydomain.io"),
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			publicIPAddress, err := testCase.publicIPSpec.Parameters(testCase.existingPublicIPAddress)

			g.Expect(err).To(BeNil())
			g.Expect(publicIPAddress).To(Equal(testCase.expectedPublicIPAddress))
		})
	}
}
