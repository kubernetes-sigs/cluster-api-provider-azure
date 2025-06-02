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

package scope

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
)

// fakeTokenCredential implements azcore.TokenCredential for testing
type fakeTokenCredential struct {
	tenantID string
}

func (f fakeTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "fake-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func TestNewAROControlPlaneScope(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	testCases := []struct {
		name        string
		params      AROControlPlaneScopeParams
		expectError bool
		setup       func(*gomock.Controller) AROControlPlaneScopeParams
	}{
		{
			name:        "nil control plane",
			expectError: true,
			setup: func(mockCtrl *gomock.Controller) AROControlPlaneScopeParams {
				return AROControlPlaneScopeParams{
					AzureClients:    AzureClients{},
					Client:          fake.NewClientBuilder().WithScheme(scheme).Build(),
					Cluster:         &clusterv1.Cluster{},
					ControlPlane:    nil,
					CredentialCache: azure.NewCredentialCache(),
				}
			},
		},
		{
			name:        "successful creation",
			expectError: false,
			setup: func(mockCtrl *gomock.Controller) AROControlPlaneScopeParams {
				credCacheMock := mock_azure.NewMockCredentialCache(mockCtrl)
				credCacheMock.EXPECT().GetOrStoreWorkloadIdentity(gomock.Any()).Return(fakeTokenCredential{tenantID: "fake"}, nil).AnyTimes()

				cluster := &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
				}

				controlPlane := &cplane.AROControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cp",
						Namespace: "default",
					},
					Spec: cplane.AROControlPlaneSpec{
						SubscriptionID:   "12345678-1234-1234-1234-123456789012",
						AzureEnvironment: "AzurePublicCloud",
						Resources: []runtime.RawExtension{
							{
								Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v1api20240812preview","kind":"HcpOpenShiftCluster","metadata":{"name":"test-cluster"},"spec":{"location":"eastus"}}`),
							},
						},
						IdentityRef: &corev1.ObjectReference{
							Name:      "test-identity",
							Namespace: "default",
							Kind:      "AzureClusterIdentity",
						},
					},
				}

				// Create a fake identity for the credential provider
				fakeIdentity := &infrav1.AzureClusterIdentity{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-identity",
						Namespace: "default",
					},
					Spec: infrav1.AzureClusterIdentitySpec{
						Type:     infrav1.WorkloadIdentity,
						ClientID: "fake-client-id",
						TenantID: "fake-tenant-id",
					},
				}

				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(cluster, controlPlane, fakeIdentity).Build()

				return AROControlPlaneScopeParams{
					AzureClients:    AzureClients{},
					Client:          fakeClient,
					Cluster:         cluster,
					ControlPlane:    controlPlane,
					CredentialCache: credCacheMock,
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			params := tc.setup(mockCtrl)
			_, err := NewAROControlPlaneScope(t.Context(), params)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAROControlPlaneScope_MakeEmptyKubeConfigSecret(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	controlPlane := &cplane.AROControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: "default",
		},
	}

	scope := &AROControlPlaneScope{
		Cluster:      cluster,
		ControlPlane: controlPlane,
	}

	result := scope.MakeEmptyKubeConfigSecret()

	g.Expect(result.Name).To(Equal(secret.Name(cluster.Name, secret.Kubeconfig)))
	g.Expect(result.Namespace).To(Equal(cluster.Namespace))
	g.Expect(result.OwnerReferences).To(HaveLen(1))
	g.Expect(result.Labels[clusterv1.ClusterNameLabel]).To(Equal(cluster.Name))
}

func TestAROControlPlaneScope_GetterMethods(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	controlPlane := &cplane.AROControlPlane{
		Spec: cplane.AROControlPlaneSpec{
			SubscriptionID:   "12345678-1234-1234-1234-123456789012",
			AzureEnvironment: "AzurePublicCloud",
			Resources: []runtime.RawExtension{
				{
					Raw: []byte(`{"apiVersion":"redhatopenshift.azure.com/v1api20240812preview","kind":"HcpOpenShiftCluster","metadata":{"name":"test-cluster"},"spec":{"location":"eastus","properties":{"clusterProfile":{"resourceGroupId":"/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg"}}}}`),
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().Build()

	scope := &AROControlPlaneScope{
		Client:       fakeClient,
		Cluster:      cluster,
		ControlPlane: controlPlane,
	}

	g.Expect(scope.GetClient()).To(Equal(fakeClient))
	g.Expect(scope.ClusterName()).To(Equal("test-cluster"))
	g.Expect(scope.Namespace()).To(Equal("default"))
}

func TestAROControlPlaneScope_GetKubeconfigMaxAge(t *testing.T) {
	g := NewWithT(t)

	scope := &AROControlPlaneScope{}
	maxAge := scope.GetKubeconfigMaxAge()
	g.Expect(maxAge).To(Equal(60 * time.Minute))
}

func TestAROControlPlaneScope_Location(t *testing.T) {
	testCases := []struct {
		name          string
		resources     []runtime.RawExtension
		expectedLoc   string
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name:          "no resources defined",
			resources:     []runtime.RawExtension{},
			expectError:   true,
			errorContains: "no resources defined",
			description:   "should return error when no resources are defined",
		},
		{
			name: "location from HcpOpenShiftCluster",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster"},
						"spec": {"location": "eastus"}
					}`),
				},
			},
			expectedLoc: "eastus",
			expectError: false,
			description: "should extract location from HcpOpenShiftCluster",
		},
		{
			name: "location from ResourceGroup fallback",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster"},
						"spec": {}
					}`),
				},
				{
					Raw: []byte(`{
						"apiVersion": "resources.azure.com/v1api20200601",
						"kind": "ResourceGroup",
						"metadata": {"name": "test-rg"},
						"spec": {"location": "westus"}
					}`),
				},
			},
			expectedLoc: "westus",
			expectError: false,
			description: "should fallback to ResourceGroup location when HcpOpenShiftCluster has no location",
		},
		{
			name: "location from other Azure resource",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster"},
						"spec": {}
					}`),
				},
				{
					Raw: []byte(`{
						"apiVersion": "network.azure.com/v1api20201101",
						"kind": "VirtualNetwork",
						"metadata": {"name": "test-vnet"},
						"spec": {"location": "northeurope"}
					}`),
				},
			},
			expectedLoc: "northeurope",
			expectError: false,
			description: "should fallback to any Azure resource location as last resort",
		},
		{
			name: "HcpOpenShiftCluster takes priority over ResourceGroup",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "resources.azure.com/v1api20200601",
						"kind": "ResourceGroup",
						"metadata": {"name": "test-rg"},
						"spec": {"location": "westus"}
					}`),
				},
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster"},
						"spec": {"location": "eastus"}
					}`),
				},
			},
			expectedLoc: "eastus",
			expectError: false,
			description: "should prioritize HcpOpenShiftCluster location over ResourceGroup",
		},
		{
			name: "no location found anywhere",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster"},
						"spec": {}
					}`),
				},
				{
					Raw: []byte(`{
						"apiVersion": "resources.azure.com/v1api20200601",
						"kind": "ResourceGroup",
						"metadata": {"name": "test-rg"},
						"spec": {}
					}`),
				},
			},
			expectError:   true,
			errorContains: "no location found",
			description:   "should return error when no location is found in any resource",
		},
		{
			name: "malformed JSON in resources",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{invalid json`),
				},
				{
					Raw: []byte(`{
						"apiVersion": "resources.azure.com/v1api20200601",
						"kind": "ResourceGroup",
						"metadata": {"name": "test-rg"},
						"spec": {"location": "westus"}
					}`),
				},
			},
			expectedLoc: "westus",
			expectError: false,
			description: "should skip malformed JSON and continue to next resource",
		},
		{
			name: "empty location string ignored",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster"},
						"spec": {"location": ""}
					}`),
				},
				{
					Raw: []byte(`{
						"apiVersion": "resources.azure.com/v1api20200601",
						"kind": "ResourceGroup",
						"metadata": {"name": "test-rg"},
						"spec": {"location": "centralus"}
					}`),
				},
			},
			expectedLoc: "centralus",
			expectError: false,
			description: "should ignore empty location strings and fallback to next source",
		},
		{
			name: "location from first Azure resource with location",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster"},
						"spec": {}
					}`),
				},
				{
					Raw: []byte(`{
						"apiVersion": "network.azure.com/v1api20201101",
						"kind": "VirtualNetwork",
						"metadata": {"name": "test-vnet"},
						"spec": {"location": "northeurope"}
					}`),
				},
				{
					Raw: []byte(`{
						"apiVersion": "network.azure.com/v1api20201101",
						"kind": "Subnet",
						"metadata": {"name": "test-subnet"},
						"spec": {"location": "southcentralus"}
					}`),
				},
			},
			expectedLoc: "northeurope",
			expectError: false,
			description: "should use first Azure resource with location as fallback",
		},
		{
			name: "no HcpOpenShiftCluster but has ResourceGroup",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "resources.azure.com/v1api20200601",
						"kind": "ResourceGroup",
						"metadata": {"name": "test-rg"},
						"spec": {"location": "japaneast"}
					}`),
				},
			},
			expectedLoc: "japaneast",
			expectError: false,
			description: "should use ResourceGroup location when HcpOpenShiftCluster is missing",
		},
		{
			name: "multiple HcpOpenShiftClusters - first one wins",
			resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster-1"},
						"spec": {"location": "eastus"}
					}`),
				},
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240812preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster-2"},
						"spec": {"location": "westus"}
					}`),
				},
			},
			expectedLoc: "eastus",
			expectError: false,
			description: "should use first HcpOpenShiftCluster location when multiple exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scope := &AROControlPlaneScope{
				ControlPlane: &cplane.AROControlPlane{
					Spec: cplane.AROControlPlaneSpec{
						Resources: tc.resources,
					},
				},
			}

			location, err := scope.Location()

			if tc.expectError {
				g.Expect(err).To(HaveOccurred(), tc.description)
				if tc.errorContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errorContains), tc.description)
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred(), tc.description)
				g.Expect(location).To(Equal(tc.expectedLoc), tc.description)
			}
		})
	}
}

func TestAROControlPlaneScope_ShouldReconcileKubeconfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	controlPlane := &cplane.AROControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: "default",
		},
	}

	testCases := []struct {
		name           string
		existingSecret *corev1.Secret
		expectedResult bool
		description    string
	}{
		{
			name:           "no secret exists",
			existingSecret: nil,
			expectedResult: true,
			description:    "should reconcile when kubeconfig secret doesn't exist",
		},
		{
			name: "secret exists with valid kubeconfig data",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:              secret.Name(cluster.Name, secret.Kubeconfig),
					Namespace:         cluster.Namespace,
					CreationTimestamp: metav1.Now(),
					Annotations: map[string]string{
						"aro.azure.com/kubeconfig-last-updated": time.Now().Format(time.RFC3339),
					},
				},
				Data: map[string][]byte{
					secret.KubeconfigDataName: []byte("fake-kubeconfig"),
				},
			},
			expectedResult: false,
			description:    "should not reconcile when valid kubeconfig exists and is recent",
		},
		{
			name: "secret exists but is old",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:              secret.Name(cluster.Name, secret.Kubeconfig),
					Namespace:         cluster.Namespace,
					CreationTimestamp: metav1.NewTime(time.Now().Add(-45 * time.Minute)), // Older than threshold
					Annotations: map[string]string{
						"aro.azure.com/kubeconfig-last-updated": time.Now().Add(-90 * time.Minute).Format(time.RFC3339),
					},
				},
				Data: map[string][]byte{
					secret.KubeconfigDataName: []byte("fake-kubeconfig"),
				},
			},
			expectedResult: true,
			description:    "should reconcile when kubeconfig is older than threshold",
		},
		{
			name: "secret exists with refresh annotation",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:              secret.Name(cluster.Name, secret.Kubeconfig),
					Namespace:         cluster.Namespace,
					CreationTimestamp: metav1.Now(),
					Annotations: map[string]string{
						"aro.azure.com/kubeconfig-refresh-needed": "true",
					},
				},
				Data: map[string][]byte{
					secret.KubeconfigDataName: []byte("fake-kubeconfig"),
				},
			},
			expectedResult: true,
			description:    "should reconcile when refresh annotation is present",
		},
		{
			name: "secret exists with empty kubeconfig data",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:              secret.Name(cluster.Name, secret.Kubeconfig),
					Namespace:         cluster.Namespace,
					CreationTimestamp: metav1.Now(),
				},
				Data: map[string][]byte{
					secret.KubeconfigDataName: []byte(""),
				},
			},
			expectedResult: true,
			description:    "should reconcile when kubeconfig data is empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			var fakeClient client.Client
			if tc.existingSecret != nil {
				fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tc.existingSecret).Build()
			} else {
				fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			}

			scope := &AROControlPlaneScope{
				Client:       fakeClient,
				Cluster:      cluster,
				ControlPlane: controlPlane,
			}

			result := scope.ShouldReconcileKubeconfig(t.Context())
			g.Expect(result).To(Equal(tc.expectedResult), tc.description)
		})
	}
}
