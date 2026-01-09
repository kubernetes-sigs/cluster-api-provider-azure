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
	"k8s.io/utils/ptr"
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
						Platform: cplane.AROPlatformProfileControlPlane{
							Location:               "eastus",
							ResourceGroup:          "test-rg",
							Subnet:                 "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
							NetworkSecurityGroupID: "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg",
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

func TestAROControlPlaneScope_SetAPIURL(t *testing.T) {
	testCases := []struct {
		name     string
		url      *string
		expected string
	}{
		{
			name:     "set API URL",
			url:      ptr.To("https://api.test.com"),
			expected: "https://api.test.com",
		},
		{
			name:     "nil URL",
			url:      nil,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := &AROControlPlaneScope{
				ControlPlane: &cplane.AROControlPlane{},
			}
			scope.SetAPIURL(tc.url)
			g.Expect(scope.ControlPlane.Status.APIURL).To(Equal(tc.expected))
		})
	}
}

func TestAROControlPlaneScope_SetConsoleURL(t *testing.T) {
	testCases := []struct {
		name     string
		url      *string
		expected string
	}{
		{
			name:     "set Console URL",
			url:      ptr.To("https://console.test.com"),
			expected: "https://console.test.com",
		},
		{
			name:     "nil URL",
			url:      nil,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := &AROControlPlaneScope{
				ControlPlane: &cplane.AROControlPlane{},
			}
			scope.SetConsoleURL(tc.url)
			g.Expect(scope.ControlPlane.Status.ConsoleURL).To(Equal(tc.expected))
		})
	}
}

func TestAROControlPlaneScope_SetKubeconfig(t *testing.T) {
	g := NewWithT(t)

	scope := &AROControlPlaneScope{}
	kubeconfig := "fake-kubeconfig"
	expirationTime := time.Now().Add(time.Hour)

	scope.SetKubeconfig(&kubeconfig, &expirationTime)

	g.Expect(scope.Kubeconfig).NotTo(BeNil())
	g.Expect(*scope.Kubeconfig).To(Equal(kubeconfig))
	g.Expect(scope.KubeonfigExpirationTimestamp).NotTo(BeNil())
	g.Expect(*scope.KubeonfigExpirationTimestamp).To(Equal(expirationTime))
}

func TestAROControlPlaneScope_GetAdminKubeconfigData(t *testing.T) {
	testCases := []struct {
		name       string
		kubeconfig *string
		expected   []byte
	}{
		{
			name:       "with kubeconfig",
			kubeconfig: ptr.To("fake-kubeconfig"),
			expected:   []byte("fake-kubeconfig"),
		},
		{
			name:       "nil kubeconfig",
			kubeconfig: nil,
			expected:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := &AROControlPlaneScope{
				Kubeconfig: tc.kubeconfig,
			}

			result := scope.GetAdminKubeconfigData()
			g.Expect(result).To(Equal(tc.expected))
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

func TestAROControlPlaneScope_SetProvisioningState(t *testing.T) {
	testCases := []struct {
		name                    string
		state                   string
		expectedReady           bool
		expectedConditionStatus metav1.ConditionStatus
		expectedConditionReason string
	}{
		{
			name:                    "empty state",
			state:                   "",
			expectedReady:           false,
			expectedConditionStatus: metav1.ConditionUnknown,
			expectedConditionReason: infrav1.CreatingReason,
		},
		{
			name:                    "succeeded state",
			state:                   ProvisioningStateSucceeded,
			expectedReady:           true,
			expectedConditionStatus: metav1.ConditionTrue,
		},
		{
			name:                    "accepted state",
			state:                   "Accepted",
			expectedReady:           false,
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: infrav1.CreatingReason,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scope := &AROControlPlaneScope{
				ControlPlane: &cplane.AROControlPlane{},
			}

			scope.SetProvisioningState(tc.state)

			condition := Get(cplane.AROControlPlaneReadyCondition, scope.ControlPlane.GetConditions())
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(tc.expectedConditionStatus))

			if tc.expectedConditionReason != "" {
				g.Expect(condition.Reason).To(Equal(tc.expectedConditionReason))
			}
		})
	}
}

func Get(conditionType clusterv1.ConditionType, conditions []metav1.Condition) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == string(conditionType) {
			return &conditions[i]
		}
	}
	return nil
}

func TestAROControlPlaneScope_NetworkSpecInitialization(t *testing.T) {
	g := NewWithT(t)

	controlPlane := &cplane.AROControlPlane{
		Spec: cplane.AROControlPlaneSpec{
			Platform: cplane.AROPlatformProfileControlPlane{
				ResourceGroup:          "test-rg",
				Subnet:                 "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
				NetworkSecurityGroupID: "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg",
			},
		},
	}

	scope := &AROControlPlaneScope{
		ControlPlane: controlPlane,
	}

	scope.initNetworkSpec()

	g.Expect(scope.NetworkSpec).NotTo(BeNil())
	g.Expect(scope.NetworkSpec.Vnet.ResourceGroup).To(Equal("test-rg"))
	g.Expect(scope.NetworkSpec.Vnet.Name).To(Equal("test-vnet"))
	g.Expect(scope.NetworkSpec.Subnets).To(HaveLen(1))
	g.Expect(scope.NetworkSpec.Subnets[0].Name).To(Equal("test-subnet"))
	g.Expect(scope.NetworkSpec.Subnets[0].ID).To(Equal(controlPlane.Spec.Platform.Subnet))
	g.Expect(scope.NetworkSpec.Subnets[0].SecurityGroup.Name).To(Equal("test-nsg"))
}

func TestAROControlPlaneScope_RegexParsing(t *testing.T) {
	testCases := []struct {
		name                      string
		subnet                    string
		networkSecurityGroupID    string
		expectedVnetID            string
		expectedVnetName          string
		expectedSubnetName        string
		expectedSecurityGroupName string
	}{
		{
			name:                      "valid subnet and NSG",
			subnet:                    "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
			networkSecurityGroupID:    "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg",
			expectedVnetID:            "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet",
			expectedVnetName:          "test-vnet",
			expectedSubnetName:        "test-subnet",
			expectedSecurityGroupName: "test-nsg",
		},
		{
			name:                      "invalid subnet format",
			subnet:                    "invalid-subnet-format",
			networkSecurityGroupID:    "invalid-nsg-format",
			expectedVnetID:            "",
			expectedVnetName:          "",
			expectedSubnetName:        "",
			expectedSecurityGroupName: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scope := &AROControlPlaneScope{
				ControlPlane: &cplane.AROControlPlane{
					Spec: cplane.AROControlPlaneSpec{
						Platform: cplane.AROPlatformProfileControlPlane{
							Subnet:                 tc.subnet,
							NetworkSecurityGroupID: tc.networkSecurityGroupID,
						},
					},
				},
			}

			vnetID := scope.vnetID()
			vnetName := scope.vnetName()
			subnetName := scope.subnetName()
			securityGroupName := scope.securityGroupName()

			g.Expect(vnetID).To(Equal(tc.expectedVnetID))
			g.Expect(vnetName).To(Equal(tc.expectedVnetName))
			g.Expect(subnetName).To(Equal(tc.expectedSubnetName))
			g.Expect(securityGroupName).To(Equal(tc.expectedSecurityGroupName))
		})
	}
}

func TestAROControlPlaneScope_AdditionalTags(t *testing.T) {
	testCases := []struct {
		name     string
		tags     infrav1.Tags
		expected infrav1.Tags
	}{
		{
			name:     "nil tags",
			tags:     nil,
			expected: infrav1.Tags{},
		},
		{
			name: "with tags",
			tags: infrav1.Tags{
				"key1": "value1",
				"key2": "value2",
			},
			expected: infrav1.Tags{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scope := &AROControlPlaneScope{
				ControlPlane: &cplane.AROControlPlane{
					Spec: cplane.AROControlPlaneSpec{
						AdditionalTags: tc.tags,
					},
				},
			}

			result := scope.AdditionalTags()
			g.Expect(result).To(Equal(tc.expected))
		})
	}
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
			Platform: cplane.AROPlatformProfileControlPlane{
				Location:      "eastus",
				ResourceGroup: "test-rg",
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
	g.Expect(scope.Location()).To(Equal("eastus"))
	g.Expect(scope.ResourceGroup()).To(Equal("test-rg"))
	g.Expect(scope.ClusterName()).To(Equal("test-cluster"))
	g.Expect(scope.Namespace()).To(Equal("default"))
}

func TestAROControlPlaneScope_SetStatusVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	testCases := []struct {
		name            string
		versionID       string
		expectedVersion string
		description     string
	}{
		{
			name:            "empty version ID",
			versionID:       "",
			expectedVersion: "",
			description:     "should handle empty version ID gracefully",
		},
		{
			name:            "valid version ID",
			versionID:       "4.14.5",
			expectedVersion: "4.14.5",
			description:     "should set version when ID is provided",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			controlPlane := &cplane.AROControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cp",
					Namespace: "default",
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			scope := &AROControlPlaneScope{
				Client:       fakeClient,
				Cluster:      cluster,
				ControlPlane: controlPlane,
			}

			// Call SetStatusVersion
			scope.SetStatusVersion(tc.versionID)

			// Verify the result
			g.Expect(scope.ControlPlane.Status.Version).To(Equal(tc.expectedVersion), tc.description)
		})
	}
}

func TestAROControlPlaneScope_AnnotateKubeconfigInvalid(t *testing.T) {
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
		expectedError  bool
		description    string
	}{
		{
			name:           "no existing secret",
			existingSecret: nil,
			expectedError:  false,
			description:    "should handle case when secret doesn't exist",
		},
		{
			name: "existing secret gets annotated",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secret.Name(cluster.Name, secret.Kubeconfig),
					Namespace: cluster.Namespace,
				},
				Data: map[string][]byte{
					secret.KubeconfigDataName: []byte("fake-kubeconfig"),
				},
			},
			expectedError: false,
			description:   "should annotate existing secret",
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

			err := scope.AnnotateKubeconfigInvalid(t.Context())

			if tc.expectedError {
				g.Expect(err).To(HaveOccurred(), tc.description)
			} else {
				g.Expect(err).NotTo(HaveOccurred(), tc.description)

				if tc.existingSecret != nil {
					// Verify the annotation was added
					updatedSecret := &corev1.Secret{}
					key := client.ObjectKey{
						Name:      secret.Name(cluster.Name, secret.Kubeconfig),
						Namespace: cluster.Namespace,
					}
					err := fakeClient.Get(t.Context(), key, updatedSecret)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(updatedSecret.Annotations).To(HaveKey("aro.azure.com/kubeconfig-refresh-needed"))
					g.Expect(updatedSecret.Annotations["aro.azure.com/kubeconfig-refresh-needed"]).To(Equal("true"))
				}
			}
		})
	}
}

func TestAROControlPlaneScope_GetKubeconfigMaxAge(t *testing.T) {
	g := NewWithT(t)

	scope := &AROControlPlaneScope{}
	maxAge := scope.GetKubeconfigMaxAge()
	g.Expect(maxAge).To(Equal(60 * time.Minute))
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
		expirationTime *time.Time
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
			name: "token expired",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:              secret.Name(cluster.Name, secret.Kubeconfig),
					Namespace:         cluster.Namespace,
					CreationTimestamp: metav1.Now(),
				},
				Data: map[string][]byte{
					secret.KubeconfigDataName: []byte("fake-kubeconfig"),
				},
			},
			expirationTime: ptr.To(time.Now().Add(-1 * time.Hour)), // Expired 1 hour ago
			expectedResult: true,
			description:    "should reconcile when token is expired",
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
				Client:                       fakeClient,
				Cluster:                      cluster,
				ControlPlane:                 controlPlane,
				KubeonfigExpirationTimestamp: tc.expirationTime,
			}

			result := scope.ShouldReconcileKubeconfig(t.Context())
			g.Expect(result).To(Equal(tc.expectedResult), tc.description)
		})
	}
}
