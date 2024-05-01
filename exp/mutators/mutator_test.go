/*
Copyright 2024 The Kubernetes Authors.

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

package mutators

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestApplyMutators(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		resources   []runtime.RawExtension
		mutators    []ResourcesMutator
		expected    []*unstructured.Unstructured
		expectedErr error
	}{
		{
			name: "no mutators",
			resources: []runtime.RawExtension{
				{Raw: []byte(`{"apiVersion": "v1", "kind": "SomeObject"}`)},
			},
			expected: []*unstructured.Unstructured{
				{Object: map[string]interface{}{"apiVersion": "v1", "kind": "SomeObject"}},
			},
		},
		{
			name: "mutators apply in order",
			resources: []runtime.RawExtension{
				{Raw: []byte(`{"apiVersion": "v1", "kind": "SomeObject"}`)},
			},
			mutators: []ResourcesMutator{
				func(_ context.Context, us []*unstructured.Unstructured) error {
					us[0].Object["f1"] = "3"
					us[0].Object["f2"] = "3"
					us[0].Object["f3"] = "3"
					return nil
				},
				func(_ context.Context, us []*unstructured.Unstructured) error {
					us[0].Object["f1"] = "2"
					us[0].Object["f2"] = "2"
					return nil
				},
				func(_ context.Context, us []*unstructured.Unstructured) error {
					us[0].Object["f1"] = "1"
					return nil
				},
			},
			expected: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "SomeObject",
						"f1":         "1",
						"f2":         "2",
						"f3":         "3",
					},
				},
			},
		},
		{
			name:      "error",
			resources: []runtime.RawExtension{},
			mutators: []ResourcesMutator{
				func(_ context.Context, us []*unstructured.Unstructured) error {
					return errors.New("mutator err")
				},
			},
			expectedErr: errors.New("mutator err"),
		},
		{
			name:      "incompatible is terminal",
			resources: []runtime.RawExtension{},
			mutators: []ResourcesMutator{
				func(_ context.Context, us []*unstructured.Unstructured) error {
					return Incompatible{}
				},
			},
			expectedErr: reconcile.TerminalError(Incompatible{}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			actual, err := ApplyMutators(ctx, test.resources, test.mutators...)
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(actual).To(Equal(test.expected))
		})
	}
}
