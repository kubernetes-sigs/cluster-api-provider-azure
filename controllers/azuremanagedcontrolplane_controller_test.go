/*
Copyright 2023 The Kubernetes Authors.

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

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestClusterToAzureManagedControlPlane(t *testing.T) {
	tests := []struct {
		name            string
		controlPlaneRef *corev1.ObjectReference
		expected        []ctrl.Request
	}{
		{
			name:            "nil",
			controlPlaneRef: nil,
			expected:        nil,
		},
		{
			name: "bad kind",
			controlPlaneRef: &corev1.ObjectReference{
				Kind: "NotAzureManagedControlPlane",
			},
			expected: nil,
		},
		{
			name: "ok",
			controlPlaneRef: &corev1.ObjectReference{
				Kind:      "AzureManagedControlPlane",
				Name:      "name",
				Namespace: "namespace",
			},
			expected: []ctrl.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "name",
						Namespace: "namespace",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			actual := (&AzureManagedControlPlaneReconciler{}).ClusterToAzureManagedControlPlane(&clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ControlPlaneRef: test.controlPlaneRef,
				},
			})
			if test.expected == nil {
				g.Expect(actual).To(BeNil())
			} else {
				g.Expect(actual).To(Equal(test.expected))
			}
		})
	}
}
