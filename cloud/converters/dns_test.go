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
	"github.com/Azure/azure-sdk-for-go/profiles/latest/privatedns/mgmt/privatedns"
	"github.com/onsi/gomega"
	"testing"
)

func Test_GetRecordType(t *testing.T) {
	cases := []struct {
		name   string
		ip     string
		expect privatedns.RecordType
	}{
		{
			name:   "ipv4",
			ip:     "10.0.0.4",
			expect: privatedns.A,
		},
		{
			name:   "ipv6",
			ip:     "2603:1030:805:2::b",
			expect: privatedns.AAAA,
		},
		{
			name:   "default",
			ip:     "",
			expect: privatedns.A,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			recordType := GetRecordType(c.ip)
			g.Expect(c.expect).To(gomega.BeEquivalentTo(recordType))
		})
	}
}
