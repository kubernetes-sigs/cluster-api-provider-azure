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

package azure

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	capifeature "sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestFindParentMachinePool(t *testing.T) {
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()
	g := NewWithT(t)
	client := mockClient{}

	tests := []struct {
		name    string
		mpName  string
		wantErr bool
	}{
		{
			name:    "valid",
			mpName:  "mock-machinepool-mp-0",
			wantErr: false,
		},
		{
			name:    "invalid",
			mpName:  "mock-machinepool-mp-1",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mp, err := FindParentMachinePool(tc.mpName, client)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(mp).NotTo(BeNil())
			}
		})
	}
}

func TestFindParentMachinePoolWithRetry(t *testing.T) {
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()
	g := NewWithT(t)
	client := mockClient{}

	tests := []struct {
		name        string
		mpName      string
		maxAttempts int
		wantErr     bool
	}{
		{
			name:        "valid",
			mpName:      "mock-machinepool-mp-0",
			maxAttempts: 1,
			wantErr:     false,
		},
		{
			name:        "valid with retries",
			mpName:      "mock-machinepool-mp-0",
			maxAttempts: 5,
			wantErr:     false,
		},
		{
			name:        "invalid",
			mpName:      "mock-machinepool-mp-1",
			maxAttempts: 1,
			wantErr:     true,
		},
		{
			name:        "invalid with retries",
			mpName:      "mock-machinepool-mp-1",
			maxAttempts: 5,
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mp, err := FindParentMachinePoolWithRetry(tc.mpName, client, tc.maxAttempts)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(mp).NotTo(BeNil())
			}
		})
	}
}

type mockClient struct {
	client.Client
}

func (m mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	mp := &expv1.MachinePool{}
	mp.Spec.Template.Spec.InfrastructureRef.Name = "mock-machinepool-mp-0"
	list.(*expv1.MachinePoolList).Items = []expv1.MachinePool{*mp}

	return nil
}
