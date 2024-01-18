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

package fleetsmembers

import (
	"context"
	"testing"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20230315preview"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

var (
	fakeAzureFleetsMember = asocontainerservicev1.FleetsMember{
		Spec: asocontainerservicev1.Fleets_Member_Spec{
			AzureName: fakeAzureFleetsMemberSpec.Name,
			Owner: &genruntime.KnownResourceReference{
				ARMID: azure.FleetID(fakeAzureFleetsMemberSpec.SubscriptionID, fakeAzureFleetsMemberSpec.ManagerResourceGroup, fakeAzureFleetsMemberSpec.ManagerName),
			},
			Group: ptr.To(fakeAzureFleetsMemberSpec.Group),
			ClusterResourceReference: &genruntime.ResourceReference{
				ARMID: azure.ManagedClusterID(fakeAzureFleetsMemberSpec.SubscriptionID, fakeAzureFleetsMemberSpec.ClusterResourceGroup, fakeAzureFleetsMemberSpec.ClusterName),
			},
		},
	}
	fakeAzureFleetsMemberSpec = AzureFleetsMemberSpec{
		Name:                 "fake-name",
		Namespace:            "fake-namespace",
		ClusterName:          "fake-cluster-name",
		ClusterResourceGroup: "fake-cluster-resource-group",
		Group:                "fake-group",
		SubscriptionID:       "fake-subscription-id",
		ManagerName:          "fake-manager-name",
		ManagerResourceGroup: "fake-manager-resource-group",
	}
	fakeFleetsMemberStatus = asocontainerservicev1.Fleets_Member_STATUS{
		Name:              ptr.To(fakeAzureFleetsMemberSpec.Name),
		ProvisioningState: ptr.To(asocontainerservicev1.FleetMemberProvisioningState_STATUS_Succeeded),
	}
)

func getASOFleetsMember(changes ...func(*asocontainerservicev1.FleetsMember)) *asocontainerservicev1.FleetsMember {
	fleetsMember := fakeAzureFleetsMember.DeepCopy()
	for _, change := range changes {
		change(fleetsMember)
	}
	return fleetsMember
}

func TestAzureFleetsMemberSpec_Parameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *AzureFleetsMemberSpec
		existing      *asocontainerservicev1.FleetsMember
		expect        func(g *WithT, result asocontainerservicev1.FleetsMember)
		expectedError string
	}{
		{
			name:     "Creating a new FleetsMember",
			spec:     &fakeAzureFleetsMemberSpec,
			existing: nil,
			expect: func(g *WithT, result asocontainerservicev1.FleetsMember) {
				g.Expect(result).To(Not(BeNil()))

				// ObjectMeta is populated later in the codeflow
				g.Expect(result.ObjectMeta).To(Equal(metav1.ObjectMeta{}))

				// Spec is populated from the spec passed in
				g.Expect(result.Spec).To(Equal(getASOFleetsMember().Spec))
			},
		},
		{
			name: "User updates to a FleetsMember group should be overwritten",
			spec: &fakeAzureFleetsMemberSpec,
			existing: getASOFleetsMember(
				// user added group which should be overwritten by capz
				func(fleetsMember *asocontainerservicev1.FleetsMember) {
					fleetsMember.Spec.Group = ptr.To("fake-group-2")
				},
				// user added Status
				func(fleetsMember *asocontainerservicev1.FleetsMember) {
					fleetsMember.Status = fakeFleetsMemberStatus
				},
			),
			expect: func(g *WithT, result asocontainerservicev1.FleetsMember) {
				g.Expect(result).To(Not(BeNil()))
				resultantASOFleetsMember := getASOFleetsMember()

				// ObjectMeta should be carried over from existing fleets member.
				g.Expect(result.ObjectMeta).To(Equal(resultantASOFleetsMember.ObjectMeta))

				// Group addition is accepted.
				g.Expect(result.Spec).To(Equal(resultantASOFleetsMember.Spec))

				// Status should be carried over.
				g.Expect(result.Status).To(Equal(fakeFleetsMemberStatus))
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
