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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capifeature "sigs.k8s.io/cluster-api/feature"
)

func TestAzureManagedCluster_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldAMC  *AzureManagedCluster
		amc     *AzureManagedCluster
		wantErr bool
	}{
		{
			name: "ControlPlaneEndpoint.Port update (AKS API-derived update scenario)",
			oldAMC: &AzureManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "aks-8622-h4h26c44.hcp.eastus.azmk8s.io",
					},
				},
			},
			amc: &AzureManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "aks-8622-h4h26c44.hcp.eastus.azmk8s.io",
						Port: 443,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ControlPlaneEndpoint.Host update (AKS API-derived update scenario)",
			oldAMC: &AzureManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Port: 443,
					},
				},
			},
			amc: &AzureManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "aks-8622-h4h26c44.hcp.eastus.azmk8s.io",
						Port: 443,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := tc.amc.ValidateUpdate(tc.oldAMC)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureManagedCluster_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		oldAMC  *AzureManagedCluster
		amc     *AzureManagedCluster
		wantErr bool
	}{
		{
			name: "can set Spec.ControlPlaneEndpoint.Host during create (clusterctl move scenario)",
			amc: &AzureManagedCluster{
				Spec: AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "my-host",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "can set Spec.ControlPlaneEndpoint.Port during create (clusterctl move scenario)",
			amc: &AzureManagedCluster{
				Spec: AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Port: 4443,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := tc.amc.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureManagedCluster_ValidateCreateFailure(t *testing.T) {
	tests := []struct {
		name        string
		amc         *AzureManagedCluster
		deferFunc   func()
		expectError bool
	}{
		{
			name:        "feature gate explicitly disabled",
			amc:         getKnownValidAzureManagedCluster(),
			deferFunc:   utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, false),
			expectError: true,
		},
		{
			name:        "feature gate implicitly enabled",
			amc:         getKnownValidAzureManagedCluster(),
			deferFunc:   func() {},
			expectError: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.deferFunc()
			g := NewWithT(t)
			_, err := tc.amc.ValidateCreate()
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getKnownValidAzureManagedCluster() *AzureManagedCluster {
	return &AzureManagedCluster{}
}
