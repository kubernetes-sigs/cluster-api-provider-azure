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
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestGetControlPlaneMachines(t *testing.T) {
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
			if len(actual) != len(c.expected) {
				t.Fatalf("Got unexpected GetControlPlaneMachines result. Expected length: %v. Got: %v.", len(actual), len(c.expected))
			}
			for i, v := range c.expected {
				if v.ObjectMeta.Name != actual[i].ObjectMeta.Name {
					t.Fatalf("Got unexpected GetControlPlaneMachines result. Expected Item: %v. Got: %v.", v, actual[i])

				}
			}
		})
	}
}

func TestIsAvailabilityZoneSupported(t *testing.T) {
	s := azureMachineService{
		machineScope: &scope.MachineScope{
			Logger: log.Log.Logger,
			AzureCluster: &v1alpha3.AzureCluster{
				Spec: v1alpha3.AzureClusterSpec{
					Location: "",
				},
			},
		},
	}

	for _, l := range azure.SupportedAvailabilityZoneLocations {
		s.machineScope.AzureCluster.Spec.Location = l
		if s.isAvailabilityZoneSupported() != true {
			t.Errorf("isAvailabilityZoneSupported should return true for supported region %s but returned %t", l, s.isAvailabilityZoneSupported())
		}
	}

	unSupportedLocations := []string{
		"randomregion",
		"unsupportedregion",
	}

	for _, l := range unSupportedLocations {
		s.machineScope.AzureCluster.Spec.Location = l
		if s.isAvailabilityZoneSupported() != false {
			t.Errorf("isAvailabilityZoneSupported should return false for unsupported region %s but returned %t", l, s.isAvailabilityZoneSupported())
		}
	}
}
