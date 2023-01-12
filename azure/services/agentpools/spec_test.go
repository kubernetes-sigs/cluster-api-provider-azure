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

package agentpools

import (
	"reflect"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

var (
	fakeAgentPoolSpecWithAutoscaling = AgentPoolSpec{
		Name:              "fake-agent-pool-name",
		ResourceGroup:     "fake-rg",
		Cluster:           "fake-cluster",
		AvailabilityZones: []string{"fake-zone"},
		EnableAutoScaling: to.BoolPtr(true),
		EnableUltraSSD:    to.BoolPtr(true),
		MaxCount:          to.Int32Ptr(5),
		MaxPods:           to.Int32Ptr(10),
		MinCount:          to.Int32Ptr(1),
		Mode:              "fake-mode",
		NodeLabels:        map[string]*string{"fake-label": to.StringPtr("fake-value")},
		NodeTaints:        []string{"fake-taint"},
		OSDiskSizeGB:      2,
		OsDiskType:        to.StringPtr("fake-os-disk-type"),
		OSType:            to.StringPtr("fake-os-type"),
		Replicas:          1,
		SKU:               "fake-sku",
		Version:           to.StringPtr("fake-version"),
		VnetSubnetID:      "fake-vnet-subnet-id",
		Headers:           map[string]string{"fake-header": "fake-value"},
	}
	fakeAgentPoolSpecWithoutAutoscaling = AgentPoolSpec{
		Name:              "fake-agent-pool-name",
		ResourceGroup:     "fake-rg",
		Cluster:           "fake-cluster",
		AvailabilityZones: []string{"fake-zone"},
		EnableAutoScaling: to.BoolPtr(true),
		EnableUltraSSD:    to.BoolPtr(true),
		MaxCount:          to.Int32Ptr(5),
		MaxPods:           to.Int32Ptr(10),
		MinCount:          to.Int32Ptr(1),
		Mode:              "fake-mode",
		NodeLabels:        map[string]*string{"fake-label": to.StringPtr("fake-value")},
		NodeTaints:        []string{"fake-taint"},
		OSDiskSizeGB:      2,
		OsDiskType:        to.StringPtr("fake-os-disk-type"),
		OSType:            to.StringPtr("fake-os-type"),
		Replicas:          1,
		SKU:               "fake-sku",
		Version:           to.StringPtr("fake-version"),
		VnetSubnetID:      "fake-vnet-subnet-id",
		Headers:           map[string]string{"fake-header": "fake-value"},
	}
	fakeAgentPoolSpecWithZeroReplicas = AgentPoolSpec{
		Name:              "fake-agent-pool-name",
		ResourceGroup:     "fake-rg",
		Cluster:           "fake-cluster",
		AvailabilityZones: []string{"fake-zone"},
		EnableAutoScaling: to.BoolPtr(false),
		EnableUltraSSD:    to.BoolPtr(true),
		MaxCount:          to.Int32Ptr(5),
		MaxPods:           to.Int32Ptr(10),
		MinCount:          to.Int32Ptr(1),
		Mode:              "fake-mode",
		NodeLabels:        map[string]*string{"fake-label": to.StringPtr("fake-value")},
		NodeTaints:        []string{"fake-taint"},
		OSDiskSizeGB:      2,
		OsDiskType:        to.StringPtr("fake-os-disk-type"),
		OSType:            to.StringPtr("fake-os-type"),
		Replicas:          0,
		SKU:               "fake-sku",
		Version:           to.StringPtr("fake-version"),
		VnetSubnetID:      "fake-vnet-subnet-id",
		Headers:           map[string]string{"fake-header": "fake-value"},
	}

	fakeAgentPoolAutoScalingOutOfDate = containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:   &[]string{"fake-zone"},
			Count:               to.Int32Ptr(1),    // updates if changed
			EnableAutoScaling:   to.BoolPtr(false), // updates if changed
			EnableUltraSSD:      to.BoolPtr(true),
			MaxCount:            to.Int32Ptr(5), // updates if changed
			MaxPods:             to.Int32Ptr(10),
			MinCount:            to.Int32Ptr(1),                                               // updates if changed
			Mode:                containerservice.AgentPoolMode("fake-mode"),                  // updates if changed
			NodeLabels:          map[string]*string{"fake-label": to.StringPtr("fake-value")}, // updates if changed
			NodeTaints:          &[]string{"fake-taint"},                                      // updates if changed
			OrchestratorVersion: to.StringPtr("fake-version"),                                 // updates if changed
			OsDiskSizeGB:        to.Int32Ptr(2),
			OsDiskType:          containerservice.OSDiskType("fake-os-disk-type"),
			OsType:              containerservice.OSType("fake-os-type"),
			ProvisioningState:   to.StringPtr("Succeeded"),
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:              to.StringPtr("fake-sku"),
			VnetSubnetID:        to.StringPtr("fake-vnet-subnet-id"),
		},
	}

	fakeAgentPoolMaxCountOutOfDate = containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:   &[]string{"fake-zone"},
			Count:               to.Int32Ptr(1),   // updates if changed
			EnableAutoScaling:   to.BoolPtr(true), // updates if changed
			EnableUltraSSD:      to.BoolPtr(true),
			MaxCount:            to.Int32Ptr(3), // updates if changed
			MaxPods:             to.Int32Ptr(10),
			MinCount:            to.Int32Ptr(1),                                               // updates if changed
			Mode:                containerservice.AgentPoolMode("fake-mode"),                  // updates if changed
			NodeLabels:          map[string]*string{"fake-label": to.StringPtr("fake-value")}, // updates if changed
			NodeTaints:          &[]string{"fake-taint"},                                      // updates if changed
			OrchestratorVersion: to.StringPtr("fake-version"),                                 // updates if changed
			OsDiskSizeGB:        to.Int32Ptr(2),
			OsDiskType:          containerservice.OSDiskType("fake-os-disk-type"),
			OsType:              containerservice.OSType("fake-os-type"),
			ProvisioningState:   to.StringPtr("Succeeded"),
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:              to.StringPtr("fake-sku"),
			VnetSubnetID:        to.StringPtr("fake-vnet-subnet-id"),
		},
	}

	fakeAgentPoolMinCountOutOfDate = containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:   &[]string{"fake-zone"},
			Count:               to.Int32Ptr(1),   // updates if changed
			EnableAutoScaling:   to.BoolPtr(true), // updates if changed
			EnableUltraSSD:      to.BoolPtr(true),
			MaxCount:            to.Int32Ptr(5), // updates if changed
			MaxPods:             to.Int32Ptr(10),
			MinCount:            to.Int32Ptr(3),                                               // updates if changed
			Mode:                containerservice.AgentPoolMode("fake-mode"),                  // updates if changed
			NodeLabels:          map[string]*string{"fake-label": to.StringPtr("fake-value")}, // updates if changed
			NodeTaints:          &[]string{"fake-taint"},                                      // updates if changed
			OrchestratorVersion: to.StringPtr("fake-version"),                                 // updates if changed
			OsDiskSizeGB:        to.Int32Ptr(2),
			OsDiskType:          containerservice.OSDiskType("fake-os-disk-type"),
			OsType:              containerservice.OSType("fake-os-type"),
			ProvisioningState:   to.StringPtr("Succeeded"),
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:              to.StringPtr("fake-sku"),
			VnetSubnetID:        to.StringPtr("fake-vnet-subnet-id"),
		},
	}

	fakeAgentPoolModeOutOfDate = containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:   &[]string{"fake-zone"},
			Count:               to.Int32Ptr(1),   // updates if changed
			EnableAutoScaling:   to.BoolPtr(true), // updates if changed
			EnableUltraSSD:      to.BoolPtr(true),
			MaxCount:            to.Int32Ptr(5), // updates if changed
			MaxPods:             to.Int32Ptr(10),
			MinCount:            to.Int32Ptr(1),                                               // updates if changed
			Mode:                containerservice.AgentPoolMode("fake-old-mode"),              // updates if changed
			NodeLabels:          map[string]*string{"fake-label": to.StringPtr("fake-value")}, // updates if changed
			NodeTaints:          &[]string{"fake-taint"},                                      // updates if changed
			OrchestratorVersion: to.StringPtr("fake-version"),                                 // updates if changed
			OsDiskSizeGB:        to.Int32Ptr(2),
			OsDiskType:          containerservice.OSDiskType("fake-os-disk-type"),
			OsType:              containerservice.OSType("fake-os-type"),
			ProvisioningState:   to.StringPtr("Succeeded"),
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:              to.StringPtr("fake-sku"),
			VnetSubnetID:        to.StringPtr("fake-vnet-subnet-id"),
		},
	}

	fakeAgentPoolNodeLabelsOutOfDate = containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones: &[]string{"fake-zone"},
			Count:             to.Int32Ptr(1),   // updates if changed
			EnableAutoScaling: to.BoolPtr(true), // updates if changed
			EnableUltraSSD:    to.BoolPtr(true),
			MaxCount:          to.Int32Ptr(5), // updates if changed
			MaxPods:           to.Int32Ptr(10),
			MinCount:          to.Int32Ptr(1),                                  // updates if changed
			Mode:              containerservice.AgentPoolMode("fake-old-mode"), // updates if changed
			NodeLabels: map[string]*string{
				"fake-label":     to.StringPtr("fake-value"),
				"fake-old-label": to.StringPtr("fake-old-value")}, // updates if changed
			NodeTaints:          &[]string{"fake-taint"},      // updates if changed
			OrchestratorVersion: to.StringPtr("fake-version"), // updates if changed
			OsDiskSizeGB:        to.Int32Ptr(2),
			OsDiskType:          containerservice.OSDiskType("fake-os-disk-type"),
			OsType:              containerservice.OSType("fake-os-type"),
			ProvisioningState:   to.StringPtr("Succeeded"),
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:              to.StringPtr("fake-sku"),
			VnetSubnetID:        to.StringPtr("fake-vnet-subnet-id"),
		},
	}

	fakeAgentPoolNodeTaintsOutOfDate = containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:   &[]string{"fake-zone"},
			Count:               to.Int32Ptr(1),   // updates if changed
			EnableAutoScaling:   to.BoolPtr(true), // updates if changed
			EnableUltraSSD:      to.BoolPtr(true),
			MaxCount:            to.Int32Ptr(5), // updates if changed
			MaxPods:             to.Int32Ptr(10),
			MinCount:            to.Int32Ptr(1),                                               // updates if changed
			Mode:                containerservice.AgentPoolMode("fake-mode"),                  // updates if changed
			NodeLabels:          map[string]*string{"fake-label": to.StringPtr("fake-value")}, // updates if changed
			NodeTaints:          &[]string{"fake-old-taint"},                                  // updates if changed
			OrchestratorVersion: to.StringPtr("fake-version"),                                 // updates if changed
			OsDiskSizeGB:        to.Int32Ptr(2),
			OsDiskType:          containerservice.OSDiskType("fake-os-disk-type"),
			OsType:              containerservice.OSType("fake-os-type"),
			ProvisioningState:   to.StringPtr("Succeeded"),
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:              to.StringPtr("fake-sku"),
			VnetSubnetID:        to.StringPtr("fake-vnet-subnet-id"),
		},
	}
)

func fakeAgentPoolWithProvisioningState(provisioningState string) containerservice.AgentPool {
	return fakeAgentPoolWithProvisioningStateAndCountAndAutoscaling(provisioningState, 1, true)
}

func fakeAgentPoolWithProvisioningStateAndCountAndAutoscaling(provisioningState string, count int32, autoscaling bool) containerservice.AgentPool {
	var state *string
	if provisioningState != "" {
		state = to.StringPtr(provisioningState)
	}
	return containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:   &[]string{"fake-zone"},
			Count:               to.Int32Ptr(count),
			EnableAutoScaling:   to.BoolPtr(autoscaling),
			EnableUltraSSD:      to.BoolPtr(true),
			MaxCount:            to.Int32Ptr(5),
			MaxPods:             to.Int32Ptr(10),
			MinCount:            to.Int32Ptr(1),
			Mode:                containerservice.AgentPoolMode("fake-mode"),
			NodeLabels:          map[string]*string{"fake-label": to.StringPtr("fake-value")},
			NodeTaints:          &[]string{"fake-taint"},
			OrchestratorVersion: to.StringPtr("fake-version"),
			OsDiskSizeGB:        to.Int32Ptr(2),
			OsDiskType:          containerservice.OSDiskType("fake-os-disk-type"),
			OsType:              containerservice.OSType("fake-os-type"),
			ProvisioningState:   state,
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:              to.StringPtr("fake-sku"),
			VnetSubnetID:        to.StringPtr("fake-vnet-subnet-id"),
		},
	}
}

func fakeAgentPoolWithAutoscalingAndCount(enableAutoScaling bool, count int32) containerservice.AgentPool {
	return containerservice.AgentPool{
		ManagedClusterAgentPoolProfileProperties: &containerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:   &[]string{"fake-zone"},
			Count:               to.Int32Ptr(count),
			EnableAutoScaling:   to.BoolPtr(enableAutoScaling),
			EnableUltraSSD:      to.BoolPtr(true),
			MaxCount:            to.Int32Ptr(5),
			MaxPods:             to.Int32Ptr(10),
			MinCount:            to.Int32Ptr(1),
			Mode:                containerservice.AgentPoolMode("fake-mode"),
			NodeLabels:          map[string]*string{"fake-label": to.StringPtr("fake-value")},
			NodeTaints:          &[]string{"fake-taint"},
			OrchestratorVersion: to.StringPtr("fake-version"),
			OsDiskSizeGB:        to.Int32Ptr(2),
			OsDiskType:          containerservice.OSDiskType("fake-os-disk-type"),
			OsType:              containerservice.OSType("fake-os-type"),
			ProvisioningState:   to.StringPtr("Succeeded"),
			Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
			VMSize:              to.StringPtr("fake-sku"),
			VnetSubnetID:        to.StringPtr("fake-vnet-subnet-id"),
		},
	}
}

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          AgentPoolSpec
		existing      interface{}
		expected      interface{}
		expectedError error
	}{
		{
			name:          "parameters without an existing agent pool",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      nil,
			expected:      fakeAgentPoolWithProvisioningState(""),
			expectedError: nil,
		},
		{
			name:          "existing agent pool up to date with provisioning state `Succeeded` without error",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolWithProvisioningState("Succeeded"),
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "existing agent pool up to date with provisioning state `Canceled` without error",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolWithProvisioningState("Canceled"),
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "existing agent pool up to date with provisioning state `Failed` without error",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolWithProvisioningState("Failed"),
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "existing agent pool up to date with non-terminal provisioning state `Deleting`",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolWithProvisioningState("Deleting"),
			expected:      nil,
			expectedError: azure.WithTransientError(errors.New("Unable to update existing agent pool in non terminal state. Agent pool must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: Deleting"), 20*time.Second),
		},
		{
			name:          "existing agent pool up to date with non-terminal provisioning state `InProgress`",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolWithProvisioningState("InProgress"),
			expected:      nil,
			expectedError: azure.WithTransientError(errors.New("Unable to update existing agent pool in non terminal state. Agent pool must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: InProgress"), 20*time.Second),
		},
		{
			name:          "existing agent pool up to date with non-terminal provisioning state `randomString`",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolWithProvisioningState("randomString"),
			expected:      nil,
			expectedError: azure.WithTransientError(errors.New("Unable to update existing agent pool in non terminal state. Agent pool must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: randomString"), 20*time.Second),
		},
		{
			name:          "parameters with an existing agent pool, update when count is out of date when enableAutoScaling is false",
			spec:          fakeAgentPoolSpecWithoutAutoscaling,
			existing:      fakeAgentPoolWithAutoscalingAndCount(false, 5),
			expected:      fakeAgentPoolWithProvisioningState(""),
			expectedError: nil,
		},
		{
			name:          "parameters with an existing agent pool, do not update when count is out of date and enableAutoScaling is true",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolWithAutoscalingAndCount(true, 5),
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "parameters with an existing agent pool and update needed on autoscaling",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolAutoScalingOutOfDate,
			expected:      fakeAgentPoolWithProvisioningState(""),
			expectedError: nil,
		},
		{
			name:          "parameters with an existing agent pool and update needed on max count",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolMaxCountOutOfDate,
			expected:      fakeAgentPoolWithProvisioningState(""),
			expectedError: nil,
		},
		{
			name:          "parameters with an existing agent pool and update needed on min count",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolMinCountOutOfDate,
			expected:      fakeAgentPoolWithProvisioningState(""),
			expectedError: nil,
		},
		{
			name:          "parameters with an existing agent pool and update needed on mode",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolModeOutOfDate,
			expected:      fakeAgentPoolWithProvisioningState(""),
			expectedError: nil,
		},
		{
			name:          "parameters with an existing agent pool and update needed on node labels",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolNodeLabelsOutOfDate,
			expected:      fakeAgentPoolWithProvisioningState(""),
			expectedError: nil,
		},
		{
			name:          "parameters with an existing agent pool and update needed on node taints",
			spec:          fakeAgentPoolSpecWithAutoscaling,
			existing:      fakeAgentPoolNodeTaintsOutOfDate,
			expected:      fakeAgentPoolWithProvisioningState(""),
			expectedError: nil,
		},
		{
			name:          "scale to zero",
			spec:          fakeAgentPoolSpecWithZeroReplicas,
			existing:      fakeAgentPoolWithAutoscalingAndCount(false, 1),
			expected:      fakeAgentPoolWithProvisioningStateAndCountAndAutoscaling("", 0, false),
			expectedError: nil,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(tc.existing)
			if tc.expectedError != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Got difference between expected result and computed result:\n%s", cmp.Diff(tc.expected, result))
			}
		})
	}
}

func TestMergeSystemNodeLabels(t *testing.T) {
	testcases := []struct {
		name       string
		capzLabels map[string]*string
		aksLabels  map[string]*string
		expected   map[string]*string
	}{
		{
			name: "update an existing label",
			capzLabels: map[string]*string{
				"foo": to.StringPtr("bar"),
			},
			aksLabels: map[string]*string{
				"foo": to.StringPtr("baz"),
			},
			expected: map[string]*string{
				"foo": to.StringPtr("bar"),
			},
		},
		{
			name:       "delete labels",
			capzLabels: map[string]*string{},
			aksLabels: map[string]*string{
				"foo":   to.StringPtr("bar"),
				"hello": to.StringPtr("world"),
			},
			expected: map[string]*string{},
		},
		{
			name: "delete one label",
			capzLabels: map[string]*string{
				"foo": to.StringPtr("bar"),
			},
			aksLabels: map[string]*string{
				"foo":   to.StringPtr("bar"),
				"hello": to.StringPtr("world"),
			},
			expected: map[string]*string{
				"foo": to.StringPtr("bar"),
			},
		},
		{
			name: "retain system label during update",
			capzLabels: map[string]*string{
				"foo": to.StringPtr("bar"),
			},
			aksLabels: map[string]*string{
				"kubernetes.azure.com/scalesetpriority": to.StringPtr("spot"),
			},
			expected: map[string]*string{
				"foo":                                   to.StringPtr("bar"),
				"kubernetes.azure.com/scalesetpriority": to.StringPtr("spot"),
			},
		},
		{
			name:       "retain system label during delete",
			capzLabels: map[string]*string{},
			aksLabels: map[string]*string{
				"kubernetes.azure.com/scalesetpriority": to.StringPtr("spot"),
			},
			expected: map[string]*string{
				"kubernetes.azure.com/scalesetpriority": to.StringPtr("spot"),
			},
		},
	}

	for _, tc := range testcases {
		t.Logf("Testing " + tc.name)
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			ret := mergeSystemNodeLabels(tc.capzLabels, tc.aksLabels)
			g.Expect(ret).To(Equal(tc.expected))
		})
	}
}
