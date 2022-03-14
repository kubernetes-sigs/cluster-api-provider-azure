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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestSKUtoSDK(t *testing.T) {
	cases := []struct {
		name   string
		SKU    infrav1.SKU
		expect network.LoadBalancerSkuName
	}{
		{
			name:   "should return Azure load balancer Standard SKU name",
			SKU:    infrav1.SKUStandard,
			expect: network.LoadBalancerSkuNameStandard,
		},
		{
			name:   "should return empty",
			SKU:    "",
			expect: "",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			loadBalancerSkuName := SKUtoSDK(c.SKU)
			g.Expect(c.expect).To(gomega.BeEquivalentTo(loadBalancerSkuName))
		})
	}
}
