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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcehealth/armresourcehealth"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

func TestAzureAvailabilityStatusToCondition(t *testing.T) {
	tests := []struct {
		name     string
		avail    armresourcehealth.AvailabilityStatus
		expected *metav1.Condition
	}{
		{
			name:  "empty",
			avail: armresourcehealth.AvailabilityStatus{},
			expected: &metav1.Condition{
				Status: metav1.ConditionFalse,
				Reason: "Unknown",
			},
		},
		{
			name: "available",
			avail: armresourcehealth.AvailabilityStatus{
				Properties: &armresourcehealth.AvailabilityStatusProperties{
					AvailabilityState: ptr.To(armresourcehealth.AvailabilityStateValuesAvailable),
				},
			},
			expected: &metav1.Condition{
				Status: metav1.ConditionTrue,
				Reason: string(infrav1.AzureResourceAvailableCondition),
			},
		},
		{
			name: "unavailable",
			avail: armresourcehealth.AvailabilityStatus{
				Properties: &armresourcehealth.AvailabilityStatusProperties{
					AvailabilityState: ptr.To(armresourcehealth.AvailabilityStateValuesUnavailable),
					ReasonType:        ptr.To("this Is  a reason "),
					Summary:           ptr.To("The Summary"),
				},
			},
			expected: &metav1.Condition{
				Status:  metav1.ConditionFalse,
				Reason:  "ThisIsAReason",
				Message: "The Summary",
			},
		},
		{
			name: "degraded",
			avail: armresourcehealth.AvailabilityStatus{
				Properties: &armresourcehealth.AvailabilityStatusProperties{
					AvailabilityState: ptr.To(armresourcehealth.AvailabilityStateValuesDegraded),
					ReasonType:        ptr.To("TheReason"),
					Summary:           ptr.To("The Summary"),
				},
			},
			expected: &metav1.Condition{
				Status:  metav1.ConditionFalse,
				Reason:  "TheReason",
				Message: "The Summary",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			cond := SDKAvailabilityStatusToCondition(test.avail)

			g.Expect(cond.Status).To(Equal(test.expected.Status))
			g.Expect(cond.Reason).To(Equal(test.expected.Reason))
			g.Expect(cond.Message).To(Equal(test.expected.Message))
		})
	}
}
