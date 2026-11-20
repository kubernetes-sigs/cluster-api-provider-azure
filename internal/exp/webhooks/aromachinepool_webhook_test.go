/*
Copyright 2025 The Kubernetes Authors.

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

package webhooks

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

func TestAROMachinePoolWebhook_Default(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = infrav1exp.AddToScheme(scheme)

	machinePool := &infrav1exp.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pool",
			Namespace: "default",
		},
		Spec: infrav1exp.AROMachinePoolSpec{
			Resources: []runtime.RawExtension{},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	webhook := &AROMachinePoolWebhook{Client: fakeClient}

	err := webhook.Default(t.Context(), machinePool)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestAROMachinePoolWebhook_ValidateCreate(t *testing.T) {
	testCases := []struct {
		name          string
		machinePool   *infrav1exp.AROMachinePool
		expectError   bool
		errorContains string
	}{
		{
			name: "valid with resources",
			machinePool: &infrav1exp.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v20260901preview","kind":"HcpOpenShiftClustersNodePool","metadata":{"name":"test"}}`),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing resources",
			machinePool: &infrav1exp.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: []runtime.RawExtension{},
				},
			},
			expectError:   true,
			errorContains: "resources mode is required",
		},
		{
			name: "invalid JSON in resources",
			machinePool: &infrav1exp.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: []byte(`{invalid json`),
						},
					},
				},
			},
			expectError:   true,
			errorContains: "must be valid JSON",
		},
		{
			name: "resource missing apiVersion",
			machinePool: &infrav1exp.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: []byte(`{"kind":"HcpOpenShiftClustersNodePool","metadata":{"name":"test"}}`),
						},
					},
				},
			},
			expectError:   true,
			errorContains: "must have apiVersion",
		},
		{
			name: "resource missing kind",
			machinePool: &infrav1exp.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v20260901preview","metadata":{"name":"test"}}`),
						},
					},
				},
			},
			expectError:   true,
			errorContains: "must have kind",
		},
		{
			name: "resource missing metadata.name",
			machinePool: &infrav1exp.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v20260901preview","kind":"HcpOpenShiftClustersNodePool","metadata":{}}`),
						},
					},
				},
			},
			expectError:   true,
			errorContains: "must have metadata.name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			_ = infrav1exp.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			webhook := &AROMachinePoolWebhook{Client: fakeClient}

			_, err := webhook.ValidateCreate(t.Context(), tc.machinePool)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.errorContains))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAROMachinePoolWebhook_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = infrav1exp.AddToScheme(scheme)

	oldPool := &infrav1exp.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pool",
			Namespace: "default",
		},
		Spec: infrav1exp.AROMachinePoolSpec{
			Resources: []runtime.RawExtension{
				{
					Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v20260901preview","kind":"HcpOpenShiftClustersNodePool","metadata":{"name":"test"}}`),
				},
			},
		},
	}

	newPool := &infrav1exp.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pool",
			Namespace: "default",
		},
		Spec: infrav1exp.AROMachinePoolSpec{
			Resources: []runtime.RawExtension{
				{
					Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v20260901preview","kind":"HcpOpenShiftClustersNodePool","metadata":{"name":"test-updated"}}`),
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	webhook := &AROMachinePoolWebhook{Client: fakeClient}

	// ASO handles field immutability, so ValidateUpdate should succeed
	_, err := webhook.ValidateUpdate(t.Context(), oldPool, newPool)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestAROMachinePoolValidateResources(t *testing.T) {
	testCases := []struct {
		name          string
		resources     []runtime.RawExtension
		expectError   bool
		errorContains string
	}{
		{
			name:        "empty resources list",
			resources:   []runtime.RawExtension{},
			expectError: false, // Empty is allowed, validation happens in Validate()
		},
		{
			name: "valid resource",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v20260901preview",
						"kind": "HcpOpenShiftClustersNodePool",
						"metadata": {"name": "test-pool"}
					}`),
				},
			},
			expectError: false,
		},
		{
			name: "nil Raw data",
			resources: []runtime.RawExtension{
				{
					Raw: nil,
				},
			},
			expectError:   true,
			errorContains: "resource cannot be empty",
		},
		{
			name: "invalid JSON",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`not valid json{`),
				},
			},
			expectError:   true,
			errorContains: "must be valid JSON",
		},
		{
			name: "missing metadata",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v20260901preview",
						"kind": "HcpOpenShiftClustersNodePool"
					}`),
				},
			},
			expectError:   true,
			errorContains: "must have metadata",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			pool := &infrav1exp.AROMachinePool{
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: tc.resources,
				},
			}

			err := validateAROMachinePoolResources(pool)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.errorContains))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
