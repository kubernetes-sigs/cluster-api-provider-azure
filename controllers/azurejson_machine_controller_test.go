/*
Copyright 2020 The Kubernetes Authors.

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

package controllers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestUnclonedMachinesPredicate(t *testing.T) {
	cases := map[string]struct {
		expected bool
		labels   map[string]string
	}{
		"uncloned worker node should return true": {
			expected: true,
			labels:   nil,
		},
		"control plane node should return true": {
			expected: true,
			labels: map[string]string{
				clusterv1.MachineControlPlaneLabelName: "",
			},
		},
		"machineset node should return false": {
			expected: false,
			labels: map[string]string{
				clusterv1.MachineSetLabelName: "",
			},
		},
		"machinedeployment node should return false": {
			expected: false,
			labels: map[string]string{
				clusterv1.MachineDeploymentLabelName: "",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			machine := &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tc.labels,
				},
			}
			e := event.GenericEvent{
				Meta:   machine,
				Object: machine,
			}
			filter := filterUnclonedMachinesPredicate{}
			if filter.Generic(e) != tc.expected {
				t.Errorf("expected: %t, got %t", tc.expected, filter.Generic(e))
			}
		})
	}
}
