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
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func fakeAgentPool(changes ...func(*AgentPoolSpec)) AgentPoolSpec {
	pool := AgentPoolSpec{
		Name:              "fake-agent-pool-name",
		ResourceGroup:     "fake-rg",
		Cluster:           "fake-cluster",
		AvailabilityZones: []string{"fake-zone"},
		EnableAutoScaling: true,
		EnableUltraSSD:    ptr.To(true),
		KubeletDiskType:   (*infrav1.KubeletDiskType)(ptr.To("fake-kubelet-disk-type")),
		MaxCount:          ptr.To[int32](5),
		MaxPods:           ptr.To[int32](10),
		MinCount:          ptr.To[int32](1),
		Mode:              "fake-mode",
		NodeLabels:        map[string]*string{"fake-label": ptr.To("fake-value")},
		NodeTaints:        []string{"fake-taint"},
		OSDiskSizeGB:      2,
		OsDiskType:        ptr.To("fake-os-disk-type"),
		OSType:            ptr.To("fake-os-type"),
		Replicas:          1,
		SKU:               "fake-sku",
		Version:           ptr.To("fake-version"),
		VnetSubnetID:      "fake-vnet-subnet-id",
		Headers:           map[string]string{"fake-header": "fake-value"},
		AdditionalTags:    infrav1.Tags{"fake": "tag"},
	}

	for _, change := range changes {
		change(&pool)
	}

	return pool
}

func withReplicas(replicas int32) func(*AgentPoolSpec) {
	return func(pool *AgentPoolSpec) {
		pool.Replicas = replicas
	}
}

func withAutoscaling(enabled bool) func(*AgentPoolSpec) {
	return func(pool *AgentPoolSpec) {
		pool.EnableAutoScaling = enabled
	}
}

func withSpotMaxPrice(spotMaxPrice string) func(*AgentPoolSpec) {
	quantity := resource.MustParse(spotMaxPrice)
	return func(pool *AgentPoolSpec) {
		pool.SpotMaxPrice = &quantity
	}
}
func sdkFakeAgentPool(changes ...func(*armcontainerservice.AgentPool)) armcontainerservice.AgentPool {
	pool := armcontainerservice.AgentPool{
		Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{
			AvailabilityZones:   []*string{ptr.To("fake-zone")},
			Count:               ptr.To[int32](1), // updates if changed
			EnableAutoScaling:   ptr.To(true),     // updates if changed
			EnableUltraSSD:      ptr.To(true),
			KubeletDiskType:     ptr.To(armcontainerservice.KubeletDiskType("fake-kubelet-disk-type")),
			MaxCount:            ptr.To[int32](5), // updates if changed
			MaxPods:             ptr.To[int32](10),
			MinCount:            ptr.To[int32](1),                                       // updates if changed
			Mode:                ptr.To(armcontainerservice.AgentPoolMode("fake-mode")), // updates if changed
			NodeLabels:          map[string]*string{"fake-label": ptr.To("fake-value")}, // updates if changed
			NodeTaints:          []*string{ptr.To("fake-taint")},                        // updates if changed
			OrchestratorVersion: ptr.To("fake-version"),                                 // updates if changed
			OSDiskSizeGB:        ptr.To[int32](2),
			OSDiskType:          ptr.To(armcontainerservice.OSDiskType("fake-os-disk-type")),
			OSType:              ptr.To(armcontainerservice.OSType("fake-os-type")),
			Tags:                map[string]*string{"fake": ptr.To("tag")},
			Type:                ptr.To(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
			VMSize:              ptr.To("fake-sku"),
			VnetSubnetID:        ptr.To("fake-vnet-subnet-id"),
		},
	}

	for _, change := range changes {
		change(&pool)
	}

	return pool
}

func sdkWithAutoscaling(enableAutoscaling bool) func(*armcontainerservice.AgentPool) {
	return func(pool *armcontainerservice.AgentPool) {
		pool.Properties.EnableAutoScaling = ptr.To(enableAutoscaling)
	}
}

func sdkWithCount(count int32) func(*armcontainerservice.AgentPool) {
	return func(pool *armcontainerservice.AgentPool) {
		pool.Properties.Count = ptr.To[int32](count)
	}
}

func sdkWithProvisioningState(state string) func(*armcontainerservice.AgentPool) {
	return func(pool *armcontainerservice.AgentPool) {
		pool.Properties.ProvisioningState = ptr.To(state)
	}
}

func sdkWithScaleDownMode(scaleDownMode armcontainerservice.ScaleDownMode) func(*armcontainerservice.AgentPool) {
	return func(pool *armcontainerservice.AgentPool) {
		if scaleDownMode == "" {
			pool.Properties.ScaleDownMode = nil
		} else {
			pool.Properties.ScaleDownMode = ptr.To(scaleDownMode)
		}
	}
}

func sdkWithSpotMaxPrice(spotMaxPrice float32) func(*armcontainerservice.AgentPool) {
	return func(pool *armcontainerservice.AgentPool) {
		pool.Properties.SpotMaxPrice = &spotMaxPrice
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
			spec:          fakeAgentPool(),
			existing:      nil,
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name:          "existing agent pool up to date with provisioning state `Succeeded` without error",
			spec:          fakeAgentPool(),
			existing:      sdkFakeAgentPool(sdkWithProvisioningState("Succeeded")),
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "existing agent pool up to date with provisioning state `Canceled` without error",
			spec:          fakeAgentPool(),
			existing:      sdkFakeAgentPool(sdkWithProvisioningState("Canceled")),
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "existing agent pool up to date with provisioning state `Failed` without error",
			spec:          fakeAgentPool(),
			existing:      sdkFakeAgentPool(sdkWithProvisioningState("Failed")),
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "existing agent pool up to date with non-terminal provisioning state `Deleting`",
			spec:          fakeAgentPool(),
			existing:      sdkFakeAgentPool(sdkWithProvisioningState("Deleting")),
			expected:      nil,
			expectedError: azure.WithTransientError(errors.New("Unable to update existing agent pool in non terminal state. Agent pool must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: Deleting"), 20*time.Second),
		},
		{
			name:          "existing agent pool up to date with non-terminal provisioning state `InProgress`",
			spec:          fakeAgentPool(),
			existing:      sdkFakeAgentPool(sdkWithProvisioningState("InProgress")),
			expected:      nil,
			expectedError: azure.WithTransientError(errors.New("Unable to update existing agent pool in non terminal state. Agent pool must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: InProgress"), 20*time.Second),
		},
		{
			name:          "existing agent pool up to date with non-terminal provisioning state `randomString`",
			spec:          fakeAgentPool(),
			existing:      sdkFakeAgentPool(sdkWithProvisioningState("randomString")),
			expected:      nil,
			expectedError: azure.WithTransientError(errors.New("Unable to update existing agent pool in non terminal state. Agent pool must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: randomString"), 20*time.Second),
		},
		{
			name: "parameters with an existing agent pool, update when count is out of date when enableAutoScaling is false",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				sdkWithAutoscaling(false),
				sdkWithCount(5),
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool, do not update when count is out of date and enableAutoScaling is true",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				sdkWithAutoscaling(true),
				sdkWithCount(5),
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      nil,
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool and update needed on autoscaling",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				sdkWithAutoscaling(false),
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool and update needed on scale down mode",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				sdkWithScaleDownMode(armcontainerservice.ScaleDownModeDeallocate),
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool and update needed on spot max price",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				sdkWithSpotMaxPrice(123.456),
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool and update needed on spot max price",
			spec: fakeAgentPool(
				withSpotMaxPrice("789.12345"),
			),
			existing: sdkFakeAgentPool(
				sdkWithProvisioningState("Succeeded"),
			),
			expected: sdkFakeAgentPool(
				sdkWithSpotMaxPrice(789.12345),
			),
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool and update needed on max count",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				func(pool *armcontainerservice.AgentPool) { pool.Properties.MaxCount = ptr.To[int32](3) },
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool and update needed on min count",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				func(pool *armcontainerservice.AgentPool) { pool.Properties.MinCount = ptr.To[int32](3) },
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool and update needed on mode",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				func(pool *armcontainerservice.AgentPool) {
					pool.Properties.Mode = ptr.To(armcontainerservice.AgentPoolMode("fake-old-mode"))
				},
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool and update needed on node labels",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				func(pool *armcontainerservice.AgentPool) {
					pool.Properties.NodeLabels = map[string]*string{
						"fake-label":     ptr.To("fake-value"),
						"fake-old-label": ptr.To("fake-old-value"),
					}
				},
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name: "difference in system node labels shouldn't trigger update",
			spec: fakeAgentPool(
				func(pool *AgentPoolSpec) {
					pool.NodeLabels = map[string]*string{
						"fake-label": ptr.To("fake-value"),
					}
				},
			),
			existing: sdkFakeAgentPool(
				func(pool *armcontainerservice.AgentPool) {
					pool.Properties.NodeLabels = map[string]*string{
						"fake-label":                            ptr.To("fake-value"),
						"kubernetes.azure.com/scalesetpriority": ptr.To("spot"),
					}
				},
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      nil,
			expectedError: nil,
		},
		{
			name: "difference in system node labels with empty labels shouldn't trigger update",
			spec: fakeAgentPool(
				func(pool *AgentPoolSpec) {
					pool.NodeLabels = nil
				},
			),
			existing: sdkFakeAgentPool(
				func(pool *armcontainerservice.AgentPool) {
					pool.Properties.NodeLabels = map[string]*string{
						"kubernetes.azure.com/scalesetpriority": ptr.To("spot"),
					}
				},
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      nil,
			expectedError: nil,
		},
		{
			name: "parameters with an existing agent pool and update needed on node taints",
			spec: fakeAgentPool(),
			existing: sdkFakeAgentPool(
				func(pool *armcontainerservice.AgentPool) {
					pool.Properties.NodeTaints = []*string{ptr.To("fake-old-taint")}
				},
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      sdkFakeAgentPool(),
			expectedError: nil,
		},
		{
			name: "scale to zero",
			spec: fakeAgentPool(
				withReplicas(0),
				withAutoscaling(false),
			),
			existing: sdkFakeAgentPool(
				sdkWithAutoscaling(false),
				sdkWithCount(1),
				sdkWithProvisioningState("Succeeded"),
			),
			expected: sdkFakeAgentPool(
				sdkWithAutoscaling(false),
				sdkWithCount(0),
			),
			expectedError: nil,
		},
		{
			name: "empty node taints should not trigger an update",
			spec: fakeAgentPool(
				func(pool *AgentPoolSpec) { pool.NodeTaints = nil },
			),
			existing: sdkFakeAgentPool(
				func(pool *armcontainerservice.AgentPool) { pool.Properties.NodeTaints = nil },
				sdkWithProvisioningState("Succeeded"),
			),
			expected:      nil,
			expectedError: nil,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
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
				"foo": ptr.To("bar"),
			},
			aksLabels: map[string]*string{
				"foo": ptr.To("baz"),
			},
			expected: map[string]*string{
				"foo": ptr.To("bar"),
			},
		},
		{
			name:       "delete labels",
			capzLabels: map[string]*string{},
			aksLabels: map[string]*string{
				"foo":   ptr.To("bar"),
				"hello": ptr.To("world"),
			},
			expected: map[string]*string{},
		},
		{
			name:       "delete labels from nil",
			capzLabels: nil,
			aksLabels: map[string]*string{
				"foo":   ptr.To("bar"),
				"hello": ptr.To("world"),
			},
			expected: nil,
		},
		{
			name: "delete one label",
			capzLabels: map[string]*string{
				"foo": ptr.To("bar"),
			},
			aksLabels: map[string]*string{
				"foo":   ptr.To("bar"),
				"hello": ptr.To("world"),
			},
			expected: map[string]*string{
				"foo": ptr.To("bar"),
			},
		},
		{
			name: "retain system label during update",
			capzLabels: map[string]*string{
				"foo": ptr.To("bar"),
			},
			aksLabels: map[string]*string{
				"kubernetes.azure.com/scalesetpriority": ptr.To("spot"),
			},
			expected: map[string]*string{
				"foo":                                   ptr.To("bar"),
				"kubernetes.azure.com/scalesetpriority": ptr.To("spot"),
			},
		},
		{
			name:       "retain system label during delete",
			capzLabels: map[string]*string{},
			aksLabels: map[string]*string{
				"kubernetes.azure.com/scalesetpriority": ptr.To("spot"),
			},
			expected: map[string]*string{
				"kubernetes.azure.com/scalesetpriority": ptr.To("spot"),
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
