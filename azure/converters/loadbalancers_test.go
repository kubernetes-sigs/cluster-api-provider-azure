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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/google/go-cmp/cmp"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestSKUtoSDK(t *testing.T) {
	tests := []struct {
		name string
		sku  infrav1.SKU
		want armnetwork.LoadBalancerSKUName
	}{
		{
			name: "standard sku",
			sku:  infrav1.SKUStandard,
			want: armnetwork.LoadBalancerSKUNameStandard,
		},
		{
			name: "unknown",
			sku:  "unknown",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SKUtoSDK(tt.sku)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SKUtoSDK(%s) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
