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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestAROControlPlaneWebhook_ValidateCreate_ResourcesRequired(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	testCases := []struct {
		name          string
		controlPlane  *AROControlPlane
		expectError   bool
		errorContains string
	}{
		{
			name: "missing resources should fail",
			controlPlane: &AROControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "default",
				},
				Spec: AROControlPlaneSpec{
					// No resources specified
				},
			},
			expectError:   true,
			errorContains: "resources mode is required",
		},
		{
			name: "with resources should pass",
			controlPlane: &AROControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "default",
				},
				Spec: AROControlPlaneSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: []byte(`{
								"apiVersion": "redhatopenshift.azure.com/v1api20240610preview",
								"kind": "HcpOpenShiftCluster",
								"metadata": {"name": "test-cluster"}
							}`),
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			webhook := &aroControlPlaneWebhook{Client: fakeClient}

			_, err := webhook.ValidateCreate(t.Context(), tc.controlPlane)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				if tc.errorContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errorContains))
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
