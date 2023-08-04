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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	recordSpec = RecordSpec{
		Record:        infrav1.AddressRecord{Hostname: "privatednsHostname", IP: "10.0.0.8"},
		ZoneName:      "my-zone",
		ResourceGroup: "my-rg",
	}

	recordSpecIpv6 = RecordSpec{
		Record:        infrav1.AddressRecord{Hostname: "privatednsHostname", IP: "2603:1030:805:2::b"},
		ZoneName:      "my-zone",
		ResourceGroup: "my-rg",
	}
)

func TestRecordSpec_ResourceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(recordSpec.ResourceName()).Should(Equal("privatednsHostname"))
}

func TestRecordSpec_ResourceGroupName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(recordSpec.ResourceGroupName()).Should(Equal("my-rg"))
}

func TestRecordSpec_OwnerResourceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(recordSpec.OwnerResourceName()).Should(Equal("my-zone"))
}

func TestRecordSpec_Parameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          RecordSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:          "new private dns record for ipv4",
			expectedError: "",
			spec:          recordSpec,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(Equal(privatedns.RecordSet{
					RecordSetProperties: &privatedns.RecordSetProperties{
						TTL: ptr.To[int64](300),
						ARecords: &[]privatedns.ARecord{
							{
								Ipv4Address: ptr.To("10.0.0.8"),
							},
						},
					},
				}))
			},
		},
		{
			name:          "new private dns record for ipv6",
			expectedError: "",
			spec:          recordSpecIpv6,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(Equal(privatedns.RecordSet{
					RecordSetProperties: &privatedns.RecordSetProperties{
						TTL: ptr.To[int64](300),
						AaaaRecords: &[]privatedns.AaaaRecord{
							{
								Ipv6Address: ptr.To("2603:1030:805:2::b"),
							},
						},
					},
				}))
			},
		},
	}

	for _, tc := range testcases {
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
				tc.expect(g, result)
			}
		})
	}
}
