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
)

func TestAzureCluster_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		cluster *AzureCluster
		updated *AzureCluster
		wantErr bool
	}{
		{
			name:    "no change",
			cluster: createAzureCluster(t, "westus2"),
			updated: createAzureCluster(t, "westus2"),
			wantErr: false,
		},
		{
			name:    "no change",
			cluster: createAzureCluster(t, "westus2"),
			updated: createAzureCluster(t, "eastus"),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cluster.ValidateUpdate(tc.updated)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createAzureCluster(t *testing.T, location string) *AzureCluster {
	return &AzureCluster{
		Spec: AzureClusterSpec{
			Location: location,
		},
	}
}
