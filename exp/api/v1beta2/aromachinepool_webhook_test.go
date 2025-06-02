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

package v1beta2

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAROMachinePoolWebhook_Default(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = AddToScheme(scheme)

	machinePool := &AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pool",
			Namespace: "default",
		},
		Spec: AROMachinePoolSpec{
			Resources: []runtime.RawExtension{},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	webhook := &aroMachinePoolWebhook{Client: fakeClient}

	err := webhook.Default(t.Context(), machinePool)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestAROMachinePoolWebhook_ValidateCreate(t *testing.T) {
	testCases := []struct {
		name          string
		machinePool   *AROMachinePool
		expectError   bool
		errorContains string
	}{
		{
			name: "valid with resources",
			machinePool: &AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v1api20240610preview","kind":"HcpOpenShiftClustersNodePool","metadata":{"name":"test"}}`),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing resources",
			machinePool: &AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: AROMachinePoolSpec{
					Resources: []runtime.RawExtension{},
				},
			},
			expectError:   true,
			errorContains: "resources mode is required",
		},
		{
			name: "invalid JSON in resources",
			machinePool: &AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: AROMachinePoolSpec{
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
			machinePool: &AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: AROMachinePoolSpec{
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
			machinePool: &AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v1api20240610preview","metadata":{"name":"test"}}`),
						},
					},
				},
			},
			expectError:   true,
			errorContains: "must have kind",
		},
		{
			name: "resource missing metadata.name",
			machinePool: &AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v1api20240610preview","kind":"HcpOpenShiftClustersNodePool","metadata":{}}`),
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
			_ = AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			webhook := &aroMachinePoolWebhook{Client: fakeClient}

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
	_ = AddToScheme(scheme)

	oldPool := &AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pool",
			Namespace: "default",
		},
		Spec: AROMachinePoolSpec{
			Resources: []runtime.RawExtension{
				{
					Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v1api20240610preview","kind":"HcpOpenShiftClustersNodePool","metadata":{"name":"test"}}`),
				},
			},
		},
	}

	newPool := &AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pool",
			Namespace: "default",
		},
		Spec: AROMachinePoolSpec{
			Resources: []runtime.RawExtension{
				{
					Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v1api20240610preview","kind":"HcpOpenShiftClustersNodePool","metadata":{"name":"test-updated"}}`),
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	webhook := &aroMachinePoolWebhook{Client: fakeClient}

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
						"apiVersion": "redhatopenshift.azure.com/v1api20240610preview",
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
						"apiVersion": "redhatopenshift.azure.com/v1api20240610preview",
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

			pool := &AROMachinePool{
				Spec: AROMachinePoolSpec{
					Resources: tc.resources,
				},
			}

			err := pool.validateResources()

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.errorContains))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
