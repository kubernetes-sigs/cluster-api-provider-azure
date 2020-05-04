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

package v1alpha3

import (
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestAzureCluster_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		cluster *AzureCluster
		updated *AzureCluster
		err     error
	}{
		{
			name:    "no change",
			cluster: createAzureCluster("westus2"),
			updated: createAzureCluster("westus2"),
			err:     nil,
		},
		{
			name:    "update location should throw an error",
			cluster: createAzureCluster("westus2"),
			updated: createAzureCluster("eastus"),
			err: apierrors.NewInvalid(
				GroupVersion.WithKind("AzureCluster").GroupKind(),
				"westus2", field.ErrorList{
					field.Invalid(field.NewPath("location"), "westus2", "AzureCluster Location is not mutable"),
				}),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cluster.ValidateUpdate(tc.updated)
			if tc.err != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(Equal(tc.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createAzureCluster(location string) *AzureCluster {
	return &AzureCluster{
		Spec: AzureClusterSpec{
			Location: location,
		},
	}
}
