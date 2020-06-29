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

package controllers

import (
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"testing"

	. "github.com/onsi/gomega"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestGetControlPlaneMachines(t *testing.T) {
	g := NewWithT(t)

	controlPlaneMachine0 := clusterv1.Machine{
		ObjectMeta: v1.ObjectMeta{
			Name:   "control-plane-0",
			Labels: map[string]string{clusterv1.MachineControlPlaneLabelName: "cp-0"},
		},
	}

	controlPlaneMachine1 := clusterv1.Machine{
		ObjectMeta: v1.ObjectMeta{
			Name:   "control-plane-1",
			Labels: map[string]string{clusterv1.MachineControlPlaneLabelName: "cp-1"},
		},
	}

	agentMachine := clusterv1.Machine{
		ObjectMeta: v1.ObjectMeta{
			Name:   "machine-0",
			Labels: map[string]string{},
		},
	}

	cases := []struct {
		name        string
		machineList *clusterv1.MachineList
		expected    []*clusterv1.Machine
	}{
		{
			name: "empty",
			machineList: &clusterv1.MachineList{
				Items: []clusterv1.Machine{},
			},
			expected: []*clusterv1.Machine{},
		},
		{
			name: "one",
			machineList: &clusterv1.MachineList{
				Items: []clusterv1.Machine{
					controlPlaneMachine0,
				},
			},
			expected: []*clusterv1.Machine{&controlPlaneMachine0},
		},
		{
			name: "all",
			machineList: &clusterv1.MachineList{
				Items: []clusterv1.Machine{
					controlPlaneMachine0,
					controlPlaneMachine1,
				},
			},
			expected: []*clusterv1.Machine{&controlPlaneMachine0, &controlPlaneMachine1},
		},
		{
			name: "none",
			machineList: &clusterv1.MachineList{
				Items: []clusterv1.Machine{agentMachine},
			},
			expected: []*clusterv1.Machine{},
		},
		{
			name: "some",
			machineList: &clusterv1.MachineList{
				Items: []clusterv1.Machine{agentMachine, controlPlaneMachine0},
			},
			expected: []*clusterv1.Machine{&controlPlaneMachine0},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			actual := GetControlPlaneMachines(c.machineList)
			g.Expect(actual).To(HaveLen(len(c.expected)))

			for i, v := range c.expected {
				g.Expect(v.ObjectMeta.Name).To(Equal(actual[i].ObjectMeta.Name))
			}
		})
	}
}

func TestIsAvailabilityZoneSupported(t *testing.T) {
	g := NewWithT(t)

	clusterScope := &scope.ClusterScope{
		AzureCluster: &infrav1.AzureCluster{
			Spec: infrav1.AzureClusterSpec{
				Location: "",
			},
		},
	}

	s := azureMachineService{
		machineScope: &scope.MachineScope{
			Logger:       log.Log.Logger,
			ClusterScope: clusterScope,
		},
	}

	for _, l := range azure.SupportedAvailabilityZoneLocations {
		clusterScope.AzureCluster.Spec.Location = l
		g.Expect(s.isAvailabilityZoneSupported()).To(BeTrue())
	}

	unSupportedLocations := []string{
		"randomregion",
		"unsupportedregion",
	}

	for _, l := range unSupportedLocations {
		clusterScope.AzureCluster.Spec.Location = l
		g.Expect(s.isAvailabilityZoneSupported()).To(BeFalse())
	}
}
