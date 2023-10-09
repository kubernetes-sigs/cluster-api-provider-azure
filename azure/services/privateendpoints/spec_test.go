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

package privateendpoints

import (
	"context"
	"testing"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	fakePrivateEndpoint = PrivateEndpointSpec{
		Name:                       "test_private_endpoint_1",
		Namespace:                  "test_ns",
		ResourceGroup:              "test_rg",
		Location:                   "test_location",
		CustomNetworkInterfaceName: "test_if_name",
		PrivateIPAddresses:         []string{"1.2.3.4", "5.6.7.8"},
		SubnetID:                   "test_subnet_id",
		ApplicationSecurityGroups:  []string{"test_asg1", "test_asg2"},
		ManualApproval:             false,
		PrivateLinkServiceConnections: []PrivateLinkServiceConnection{
			{
				Name:                 "test_plsc_1",
				PrivateLinkServiceID: "test_pl",
				RequestMessage:       "Please approve my connection.",
				GroupIDs:             []string{"aa", "bb"}},
		},
		AdditionalTags: infrav1.Tags{"test_tag1": "test_value1", "test_tag2": "test_value2"},
		ClusterName:    "test_cluster",
	}

	fakeExtendedLocation = asonetworkv1.ExtendedLocation{
		Name: ptr.To("extended_location_name"),
		Type: ptr.To(asonetworkv1.ExtendedLocationType_EdgeZone),
	}

	fakeASOPrivateEndpoint = asonetworkv1.PrivateEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fakePrivateEndpoint.Name,
			Namespace: fakePrivateEndpoint.Namespace,
		},
		Spec: asonetworkv1.PrivateEndpoint_Spec{
			ApplicationSecurityGroups: []asonetworkv1.ApplicationSecurityGroupSpec_PrivateEndpoint_SubResourceEmbedded{
				{
					Reference: &genruntime.ResourceReference{
						ARMID: fakePrivateEndpoint.ApplicationSecurityGroups[0],
					},
				},
				{
					Reference: &genruntime.ResourceReference{
						ARMID: fakePrivateEndpoint.ApplicationSecurityGroups[1],
					},
				},
			},
			AzureName: fakePrivateEndpoint.Name,
			PrivateLinkServiceConnections: []asonetworkv1.PrivateLinkServiceConnection{{
				Name: ptr.To(fakePrivateEndpoint.PrivateLinkServiceConnections[0].Name),
				PrivateLinkServiceReference: &genruntime.ResourceReference{
					ARMID: fakePrivateEndpoint.PrivateLinkServiceConnections[0].PrivateLinkServiceID,
				},
				GroupIds:       fakePrivateEndpoint.PrivateLinkServiceConnections[0].GroupIDs,
				RequestMessage: ptr.To(fakePrivateEndpoint.PrivateLinkServiceConnections[0].RequestMessage),
			}},
			Owner: &genruntime.KnownResourceReference{
				Name: fakePrivateEndpoint.ResourceGroup,
			},
			Subnet: &asonetworkv1.Subnet_PrivateEndpoint_SubResourceEmbedded{
				Reference: &genruntime.ResourceReference{
					ARMID: fakePrivateEndpoint.SubnetID,
				},
			},
			IpConfigurations: []asonetworkv1.PrivateEndpointIPConfiguration{
				{
					PrivateIPAddress: ptr.To(fakePrivateEndpoint.PrivateIPAddresses[0]),
				},
				{
					PrivateIPAddress: ptr.To(fakePrivateEndpoint.PrivateIPAddresses[1]),
				},
			},
			CustomNetworkInterfaceName: ptr.To(fakePrivateEndpoint.CustomNetworkInterfaceName),
			Location:                   ptr.To(fakePrivateEndpoint.Location),
			Tags:                       map[string]string{"sigs.k8s.io_cluster-api-provider-azure_cluster_test_cluster": "owned", "Name": "test_private_endpoint_1", "test_tag1": "test_value1", "test_tag2": "test_value2"},
		},
	}

	fakeASOPrivateEndpointsStatus = asonetworkv1.PrivateEndpoint_STATUS_PrivateEndpoint_SubResourceEmbedded{
		ApplicationSecurityGroups: []asonetworkv1.ApplicationSecurityGroup_STATUS_PrivateEndpoint_SubResourceEmbedded{
			{
				Id: ptr.To(fakePrivateEndpoint.ApplicationSecurityGroups[0]),
			},
			{
				Id: ptr.To(fakePrivateEndpoint.ApplicationSecurityGroups[1]),
			},
		},
		Name: ptr.To(fakePrivateEndpoint.Name),
		// ... other fields truncated for brevity
	}
)

func getASOPrivateEndpoint(changes ...func(*asonetworkv1.PrivateEndpoint)) *asonetworkv1.PrivateEndpoint {
	privateEndpoint := fakeASOPrivateEndpoint.DeepCopy()
	for _, change := range changes {
		change(privateEndpoint)
	}
	return privateEndpoint
}

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *PrivateEndpointSpec
		existing      *asonetworkv1.PrivateEndpoint
		expect        func(g *WithT, result asonetworkv1.PrivateEndpoint)
		expectedError string
	}{
		{
			name:     "Creating a new PrivateEndpoint",
			spec:     ptr.To(fakePrivateEndpoint),
			existing: nil,
			expect: func(g *WithT, result asonetworkv1.PrivateEndpoint) {
				g.Expect(result).To(Not(BeNil()))

				// ObjectMeta is populated later in the codeflow
				g.Expect(result.ObjectMeta).To(Equal(metav1.ObjectMeta{}))

				// Spec is populated from the spec passed in
				g.Expect(result.Spec).To(Equal(getASOPrivateEndpoint().Spec))
			},
		},
		{
			name: "user updates to private endpoints Extended Location should be accepted",
			spec: ptr.To(fakePrivateEndpoint),
			existing: getASOPrivateEndpoint(
				// user added ExtendedLocation
				func(endpoint *asonetworkv1.PrivateEndpoint) {
					endpoint.Spec.ExtendedLocation = fakeExtendedLocation.DeepCopy()
				},
				// user added Status
				func(endpoint *asonetworkv1.PrivateEndpoint) {
					endpoint.Status = fakeASOPrivateEndpointsStatus
				},
			),
			expect: func(g *WithT, result asonetworkv1.PrivateEndpoint) {
				g.Expect(result).To(Not(BeNil()))
				resultantASOPrivateEndpoint := getASOPrivateEndpoint(
					func(endpoint *asonetworkv1.PrivateEndpoint) {
						endpoint.Spec.ExtendedLocation = fakeExtendedLocation.DeepCopy()
					},
				)

				// ObjectMeta should be carried over from existing private endpoint.
				g.Expect(result.ObjectMeta).To(Equal(resultantASOPrivateEndpoint.ObjectMeta))

				// Extended location addition is accepted.
				g.Expect(result.Spec).To(Equal(resultantASOPrivateEndpoint.Spec))

				// Status should be carried over.
				g.Expect(result.Status).To(Equal(fakeASOPrivateEndpointsStatus))
			},
		},
		{
			name: "user updates ASO's private endpoint resource and capz should overwrite it",
			spec: ptr.To(fakePrivateEndpoint),
			existing: getASOPrivateEndpoint(
				// add ExtendedLocation
				func(endpoint *asonetworkv1.PrivateEndpoint) {
					endpoint.Spec.ExtendedLocation = fakeExtendedLocation.DeepCopy()
				},

				// User also updates private IP addresses and location.
				// This change should be overwritten by CAPZ.
				func(endpoint *asonetworkv1.PrivateEndpoint) {
					endpoint.Spec.IpConfigurations = []asonetworkv1.PrivateEndpointIPConfiguration{
						{
							PrivateIPAddress: ptr.To("9.9.9.9"),
						},
					}
					endpoint.Spec.Location = ptr.To("new_location")
				},
				// add Status
				func(endpoint *asonetworkv1.PrivateEndpoint) {
					endpoint.Status = fakeASOPrivateEndpointsStatus
				},
			),
			expect: func(g *WithT, result asonetworkv1.PrivateEndpoint) {
				g.Expect(result).NotTo(BeNil())
				resultantASOPrivateEndpoint := getASOPrivateEndpoint(
					func(endpoint *asonetworkv1.PrivateEndpoint) {
						endpoint.Spec.ExtendedLocation = fakeExtendedLocation.DeepCopy()
					},
				)

				// user changes except ExtendedLocation should be overwritten by CAPZ.
				g.Expect(result.ObjectMeta).To(Equal(resultantASOPrivateEndpoint.ObjectMeta))

				// Extended location addition is accepted.
				g.Expect(result.Spec).To(Equal(resultantASOPrivateEndpoint.Spec))

				// Status should be carried over.
				g.Expect(result.Status).To(Equal(fakeASOPrivateEndpointsStatus))
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
			}
			tc.expect(g, *result)
		})
	}
}
