/*
Copyright The Kubernetes Authors.

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
	utilfeature "k8s.io/component-base/featuregate/testing"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capifeature "sigs.k8s.io/cluster-api/feature"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
)

func TestAzureManagedCluster_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldAMC  *infrav1.AzureManagedCluster
		amc     *infrav1.AzureManagedCluster
		wantErr bool
	}{
		{
			name: "ControlPlaneEndpoint.Port update (AKS API-derived update scenario)",
			oldAMC: &infrav1.AzureManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: infrav1.AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
						Host: "aks-8622-h4h26c44.hcp.eastus.azmk8s.io",
					},
				},
			},
			amc: &infrav1.AzureManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: infrav1.AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
						Host: "aks-8622-h4h26c44.hcp.eastus.azmk8s.io",
						Port: 443,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ControlPlaneEndpoint.Host update (AKS API-derived update scenario)",
			oldAMC: &infrav1.AzureManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: infrav1.AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
						Port: 443,
					},
				},
			},
			amc: &infrav1.AzureManagedCluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: infrav1.AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
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
			_, err := (&AzureManagedClusterWebhook{}).ValidateUpdate(t.Context(), tc.oldAMC, tc.amc)
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
		oldAMC  *infrav1.AzureManagedCluster
		amc     *infrav1.AzureManagedCluster
		wantErr bool
	}{
		{
			name: "can set Spec.ControlPlaneEndpoint.Host during create (clusterctl move scenario)",
			amc: &infrav1.AzureManagedCluster{
				Spec: infrav1.AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
						Host: "my-host",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "can set Spec.ControlPlaneEndpoint.Port during create (clusterctl move scenario)",
			amc: &infrav1.AzureManagedCluster{
				Spec: infrav1.AzureManagedClusterSpec{
					ControlPlaneEndpoint: clusterv1beta1.APIEndpoint{
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
			_, err := (&AzureManagedClusterWebhook{}).ValidateCreate(t.Context(), tc.amc)
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
		name               string
		amc                *infrav1.AzureManagedCluster
		featureGateEnabled *bool
		expectError        bool
	}{
		{
			name:               "feature gate implicitly enabled",
			amc:                getKnownValidAzureManagedCluster(),
			featureGateEnabled: nil,
			expectError:        false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.featureGateEnabled != nil {
				utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, *tc.featureGateEnabled)
			}
			g := NewWithT(t)
			_, err := (&AzureManagedClusterWebhook{}).ValidateCreate(t.Context(), tc.amc)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getKnownValidAzureManagedCluster() *infrav1.AzureManagedCluster {
	return &infrav1.AzureManagedCluster{}
}
