/*
Copyright 2023 The Kubernetes Authors.

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

package routetables

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var (
	fakeRouteTable = armnetwork.RouteTable{
		ID:       ptr.To("fake-id"),
		Location: ptr.To("fake-location"),
		Name:     ptr.To("fake-name"),
	}
	fakeRouteTableSpec = RouteTableSpec{
		Name:        "test-rt-1",
		Location:    "fake-location",
		ClusterName: "cluster",
		AdditionalTags: map[string]string{
			"foo": "bar",
		},
	}
	fakeRouteTableTags = map[string]*string{
		"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster": ptr.To("owned"),
		"foo":  ptr.To("bar"),
		"Name": ptr.To("test-rt-1"),
	}
)

func TestRouteTableSpec_Parameters(t *testing.T) {
	testCases := []struct {
		name          string
		spec          *RouteTableSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "error when existing is not of RouteTable type",
			spec:     &RouteTableSpec{},
			existing: struct{}{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "struct {} is not an armnetwork.RouteTable",
		},
		{
			name:     "get result as nil when existing RouteTable is present",
			spec:     &fakeRouteTableSpec,
			existing: fakeRouteTable,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get result as nil when existing RouteTable is present with empty data",
			spec:     &fakeRouteTableSpec,
			existing: armnetwork.RouteTable{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get RouteTable when all values are present",
			spec:     &fakeRouteTableSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.RouteTable{}))
				g.Expect(result.(armnetwork.RouteTable).Location).To(Equal(ptr.To[string](fakeRouteTableSpec.Location)))
				g.Expect(result.(armnetwork.RouteTable).Tags).To(Equal(fakeRouteTableTags))
			},
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
			tc.expect(g, result)
		})
	}
}
