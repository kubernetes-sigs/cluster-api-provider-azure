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

package publicips

import (
	"context"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	fakePublicIPSpecWithDNS = PublicIPSpec{
		Name:        "my-publicip",
		DNSName:     "fakedns.mydomain.io",
		Location:    "centralIndia",
		ClusterName: "my-cluster",
		AdditionalTags: infrav1.Tags{
			"foo": "bar",
		},
		FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
	}

	fakePublicIPSpecWithoutDNS = PublicIPSpec{
		Name:        "my-publicip-2",
		Location:    "centralIndia",
		ClusterName: "my-cluster",
		AdditionalTags: infrav1.Tags{
			"foo": "bar",
		},
		FailureDomains: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
	}

	fakePublicIPWithDNS = armnetwork.PublicIPAddress{
		Name:     ptr.To("my-publicip"),
		SKU:      &armnetwork.PublicIPAddressSKU{Name: ptr.To(armnetwork.PublicIPAddressSKUNameStandard)},
		Location: ptr.To("centralIndia"),
		Tags: map[string]*string{
			"Name": ptr.To("my-publicip"),
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			"foo": ptr.To("bar"),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAddressVersion:   ptr.To(armnetwork.IPVersionIPv4),
			PublicIPAllocationMethod: ptr.To(armnetwork.IPAllocationMethodStatic),
			DNSSettings: &armnetwork.PublicIPAddressDNSSettings{
				DomainNameLabel: ptr.To("fakedns"),
				Fqdn:            ptr.To("fakedns.mydomain.io"),
			},
		},
		Zones: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
	}

	fakePublicIPWithoutDNS = armnetwork.PublicIPAddress{
		Name:     ptr.To("my-publicip-2"),
		SKU:      &armnetwork.PublicIPAddressSKU{Name: ptr.To(armnetwork.PublicIPAddressSKUNameStandard)},
		Location: ptr.To("centralIndia"),
		Tags: map[string]*string{
			"Name": ptr.To("my-publicip-2"),
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			"foo": ptr.To("bar"),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAddressVersion:   ptr.To(armnetwork.IPVersionIPv4),
			PublicIPAllocationMethod: ptr.To(armnetwork.IPAllocationMethodStatic),
		},
		Zones: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
	}

	fakePublicIPIpv6 = armnetwork.PublicIPAddress{
		Name:     ptr.To("my-publicip-ipv6"),
		SKU:      &armnetwork.PublicIPAddressSKU{Name: ptr.To(armnetwork.PublicIPAddressSKUNameStandard)},
		Location: ptr.To("centralIndia"),
		Tags: map[string]*string{
			"Name": ptr.To("my-publicip-ipv6"),
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			"foo": ptr.To("bar"),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAddressVersion:   ptr.To(armnetwork.IPVersionIPv6),
			PublicIPAllocationMethod: ptr.To(armnetwork.IPAllocationMethodStatic),
			DNSSettings: &armnetwork.PublicIPAddressDNSSettings{
				DomainNameLabel: ptr.To("fakename"),
				Fqdn:            ptr.To("fakename.mydomain.io"),
			},
		},
		Zones: []*string{ptr.To("failure-domain-id-1"), ptr.To("failure-domain-id-2"), ptr.To("failure-domain-id-3")},
	}
)

func TestParameters(t *testing.T) {
	testCases := []struct {
		name          string
		existing      interface{}
		spec          PublicIPSpec
		expected      interface{}
		expectedError string
	}{
		{
			name:          "noop if public IP exists",
			existing:      fakePublicIPWithDNS,
			spec:          fakePublicIPSpecWithDNS,
			expected:      nil,
			expectedError: "",
		},
		{
			name:          "public ipv4 address with dns",
			existing:      nil,
			spec:          fakePublicIPSpecWithDNS,
			expected:      fakePublicIPWithDNS,
			expectedError: "",
		},
		{
			name:          "public ipv4 address without dns",
			existing:      nil,
			spec:          fakePublicIPSpecWithoutDNS,
			expected:      fakePublicIPWithoutDNS,
			expectedError: "",
		},
		{
			name:          "public ipv6 address with dns",
			existing:      nil,
			spec:          fakePublicIPSpecIpv6, // In publicips_test.go
			expected:      fakePublicIPIpv6,
			expectedError: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Diff between expected result and actual result:\n%s", cmp.Diff(tc.expected, result))
			}
		})
	}
}
