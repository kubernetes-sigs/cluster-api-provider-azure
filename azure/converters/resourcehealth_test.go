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

	"github.com/Azure/azure-sdk-for-go/services/resourcehealth/mgmt/2020-05-01/resourcehealth"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestAzureAvailabilityStatusToCondition(t *testing.T) {
	tests := []struct {
		name     string
		avail    resourcehealth.AvailabilityStatus
		expected *clusterv1.Condition
	}{
		{
			name:  "empty",
			avail: resourcehealth.AvailabilityStatus{},
			expected: &clusterv1.Condition{
				Status: corev1.ConditionFalse,
			},
		},
		{
			name: "available",
			avail: resourcehealth.AvailabilityStatus{
				Properties: &resourcehealth.AvailabilityStatusProperties{
					AvailabilityState: resourcehealth.AvailabilityStateValuesAvailable,
				},
			},
			expected: &clusterv1.Condition{
				Status: corev1.ConditionTrue,
			},
		},
		{
			name: "unavailable",
			avail: resourcehealth.AvailabilityStatus{
				Properties: &resourcehealth.AvailabilityStatusProperties{
					AvailabilityState: resourcehealth.AvailabilityStateValuesUnavailable,
					ReasonType:        ptr.To("this Is  a reason "),
					Summary:           ptr.To("The Summary"),
				},
			},
			expected: &clusterv1.Condition{
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityError,
				Reason:   "ThisIsAReason",
				Message:  "The Summary",
			},
		},
		{
			name: "degraded",
			avail: resourcehealth.AvailabilityStatus{
				Properties: &resourcehealth.AvailabilityStatusProperties{
					AvailabilityState: resourcehealth.AvailabilityStateValuesDegraded,
					ReasonType:        ptr.To("TheReason"),
					Summary:           ptr.To("The Summary"),
				},
			},
			expected: &clusterv1.Condition{
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityWarning,
				Reason:   "TheReason",
				Message:  "The Summary",
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			cond := SDKAvailabilityStatusToCondition(test.avail)

			g.Expect(cond.Status).To(Equal(test.expected.Status))
			g.Expect(cond.Severity).To(Equal(test.expected.Severity))
			g.Expect(cond.Reason).To(Equal(test.expected.Reason))
			g.Expect(cond.Message).To(Equal(test.expected.Message))
		})
	}
}
