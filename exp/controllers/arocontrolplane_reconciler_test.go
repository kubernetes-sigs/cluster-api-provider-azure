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

package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav1beta2 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

const (
	TestKubeconfigWithValidCA = `apiVersion: v1
clusters:
- cluster:
    server: https://test-api.example.com:443
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURYVENDQWtXZ0F3SUJBZ0lKQUtZRkhSS3RJSDd4TUEwR0NTcUdTSWIzRFFFQkN3VUFNRTh4Q3pBSkJnTlYKQkFZVEFsVlRNUk13RVFZRFZRUUlEQXBUYjIxbExWTjBZWFJsTVJFd0R3WURWUVFIREFoVGIyMWxMVU5wZEhreApHREFXQmdOVkJBb01EMDlzWkNCU2FXVnVaRFZoYm1rd0hoY05NalF3TVRBek1USXdPREE0V2hjTk16UXdNVEF4Ck1USXdPREE0V2pCUE1Rc3dDUVlEVlFRR0V3SlZVekVUTUJFR0ExVUVDQXdLVTI5dFpTMVRkR0YwWlRFUk1BOEcKQTFVRUJ3d0lVMjkwWlMxRGFYUjVNUmd3RmdZRFZRUUtEQTlQYkdRZ1VtbGxibVExWVc1ck1JSUJJakFOQmdrcQpoa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZ0tDQVFFQXUzRFZBcllLZ3VLNHlsOVNBczJHcTRrN0pJWDBQTFBKCnBYV0pGUnZBbTVJUGs1SzNhRVR0b0FGN2EvVmZ6b1FaMXJObEI4QVZGR3BUaHVrR3ZTUWtGWVYyOElaZDNQODEKaWc1ejVQY2UzQWhrSWVLdGYrbVhTRWQ0eEV5d3FYRFMyVXdXdEtrcEszTnkzVDBQM3FyVzFNUHAyWWNGbmc5OQpUcnR3YjYvUWRPNDNmaE5weGVWYVFuNm9ReDFZQStSUzVxYUdJZmJDa1FDMC9lZlVHMzFrQlNrUFFCRUtjTW1JCnJPaXhxOVpGQmNLNmR1S3k3dkI3TjJHMWJZTHZSSEFTQzA0U3FHTnlDNGh6VjI5RUtrSFl4UVNLejdwSDBiZmEKU0ppdTN6OUpBS1dNVGlFRFZHN2UxWE1MTU1aTEF6TmEvUTNjNnEra0I4dDhTaWo1bndJ
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
preferences: {}
users:
- name: test-user
  user:
    token: test-token`
)

func createTestScope(t *testing.T, kubeconfigData *string) (*scope.AROControlPlaneScope, client.Client) {
	t.Helper()
	return createTestScopeWithOptions(t, kubeconfigData, true)
}

func createTestScopeWithOptions(t *testing.T, kubeconfigData *string, createKubeconfigSecret bool) (*scope.AROControlPlaneScope, client.Client) {
	t.Helper()
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = infrav1beta2.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = asoredhatopenshiftv1.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
				Name: "test-cp",
			},
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
			IdentityRef: &corev1.ObjectReference{
				Name:      "test-identity",
				Namespace: "default",
				Kind:      "AzureClusterIdentity",
			},
			// Add test resources for mutator
			Resources: []runtime.RawExtension{
				{
					Raw: []byte(`{
						"apiVersion": "redhatopenshift.azure.com/v1api20240610preview",
						"kind": "HcpOpenShiftCluster",
						"metadata": {"name": "test-cluster"},
						"spec": {"location": "eastus", "properties": {"spec": {"version": {"id": "4.14"}}}}
					}`),
				},
			},
		},
		Status: cplane.AROControlPlaneStatus{
			APIURL: "https://test-api.example.com",
		},
	}

	// Create the required identity object
	identity := &infrav1.AzureClusterIdentity{
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

	// Create AROCluster with infrastructure ready
	aroCluster := &infrav1beta2.AROCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-arocluster",
			Namespace: "default",
		},
		Status: infrav1beta2.AROClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   string(infrav1beta2.ResourcesReadyCondition),
					Status: metav1.ConditionTrue,
					Reason: "InfrastructureReady",
				},
			},
		},
	}

	// Set the cluster's InfrastructureRef to point to the AROCluster
	cluster.Spec.InfrastructureRef = clusterv1.ContractVersionedObjectReference{
		Name:     aroCluster.Name,
		APIGroup: infrav1beta2.GroupVersion.Group,
		Kind:     infrav1beta2.AROClusterKind,
	}

	var initObjects []client.Object
	initObjects = append(initObjects, cluster, controlPlane, identity, aroCluster)

	// Add kubeconfig secret if kubeconfig data is provided and requested
	if kubeconfigData != nil && createKubeconfigSecret {
		kubeconfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name(cluster.Name, secret.Kubeconfig),
				Namespace: cluster.Namespace,
			},
			Data: map[string][]byte{
				secret.KubeconfigDataName: []byte(*kubeconfigData),
			},
		}
		initObjects = append(initObjects, kubeconfigSecret)
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjects...).
		Build()

	credCacheMock := mock_azure.NewMockCredentialCache(gomock.NewController(t))
	credCacheMock.EXPECT().GetOrStoreWorkloadIdentity(gomock.Any()).
		Return(fakeTokenCredential{tenantID: "fake"}, nil).AnyTimes()

	// Create timeouts for the AsyncReconciler
	timeouts := reconciler.Timeouts{
		Loop:                  90 * time.Minute,
		AzureServiceReconcile: 12 * time.Second,
		AzureCall:             2 * time.Second,
		Requeue:               15 * time.Second,
	}

	scopeParams := scope.AROControlPlaneScopeParams{
		AzureClients:    scope.AzureClients{}, // Empty but not nil
		Client:          fakeClient,
		Cluster:         cluster,
		ControlPlane:    controlPlane,
		CredentialCache: credCacheMock,
		Timeouts:        timeouts,
	}

	aroScope, err := scope.NewAROControlPlaneScope(t.Context(), scopeParams)
	g.Expect(err).NotTo(HaveOccurred())

	return aroScope, fakeClient
}

func TestNewAROControlPlaneService(t *testing.T) {
	g := NewWithT(t)

	aroScope, _ := createTestScope(t, nil)

	service, err := newAROControlPlaneService(aroScope)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(service).NotTo(BeNil())
	g.Expect(service.scope).To(Equal(aroScope))
	g.Expect(service.kubeclient).NotTo(BeNil())
	g.Expect(service.newResourceReconciler).NotTo(BeNil())
	g.Expect(service.keyVaultSvc).NotTo(BeNil())
	g.Expect(service.skuCache).NotTo(BeNil())
	g.Expect(service.Reconcile).NotTo(BeNil())
	g.Expect(service.Pause).NotTo(BeNil())
	g.Expect(service.Delete).NotTo(BeNil())
}

func TestAROControlPlaneService_Reconcile(t *testing.T) {
	testCases := []struct {
		name           string
		kubeconfigData *string
		expectedError  bool
		mockServices   func(t *testing.T) (*aroControlPlaneService, *scope.AROControlPlaneScope)
		validateResult func(*WithT, *scope.AROControlPlaneScope, client.Client)
	}{
		{
			name:           "successful reconcile",
			kubeconfigData: ptr.To(TestKubeconfigWithValidCA),
			expectedError:  false,
			mockServices: func(t *testing.T) (*aroControlPlaneService, *scope.AROControlPlaneScope) {
				t.Helper()
				aroScope, _ := createTestScope(t, ptr.To(TestKubeconfigWithValidCA))

				// Create mock keyVault service that succeeds
				mockKeyVaultSvc := &mockServiceReconciler{}

				// Create a service with Resources mode
				service := &aroControlPlaneService{
					scope:       aroScope,
					kubeclient:  aroScope.GetClient(),
					keyVaultSvc: mockKeyVaultSvc,
					newResourceReconciler: func(cp *cplane.AROControlPlane, resources []*unstructured.Unstructured) resourceReconciler {
						return &mockResourceReconciler{}
					},
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete

				return service, aroScope
			},
		},
		{
			name:           "reconcile with existing kubeconfig",
			kubeconfigData: ptr.To(TestKubeconfigWithValidCA),
			expectedError:  false,
			mockServices: func(t *testing.T) (*aroControlPlaneService, *scope.AROControlPlaneScope) {
				t.Helper()
				aroScope, _ := createTestScope(t, ptr.To(TestKubeconfigWithValidCA))

				// Create mock keyVault service that succeeds
				mockKeyVaultSvc := &mockServiceReconciler{}

				service := &aroControlPlaneService{
					scope:       aroScope,
					kubeclient:  aroScope.GetClient(),
					keyVaultSvc: mockKeyVaultSvc,
					newResourceReconciler: func(cp *cplane.AROControlPlane, resources []*unstructured.Unstructured) resourceReconciler {
						return &mockResourceReconciler{}
					},
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete

				return service, aroScope
			},
			validateResult: func(g *WithT, scope *scope.AROControlPlaneScope, client client.Client) {
				// Check that kubeconfig secret exists
				kubeconfigSecret := &corev1.Secret{}
				key := types.NamespacedName{
					Name:      secret.Name(scope.Cluster.Name, secret.Kubeconfig),
					Namespace: scope.Cluster.Namespace,
				}
				err := client.Get(t.Context(), key, kubeconfigSecret)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(kubeconfigSecret.Data).To(HaveKey(secret.KubeconfigDataName))
			},
		},
		{
			name:          "resource reconcile error",
			expectedError: true,
			mockServices: func(t *testing.T) (*aroControlPlaneService, *scope.AROControlPlaneScope) {
				t.Helper()
				aroScope, _ := createTestScope(t, nil)

				// Create mock keyVault service that succeeds
				mockKeyVaultSvc := &mockServiceReconciler{}

				service := &aroControlPlaneService{
					scope:       aroScope,
					kubeclient:  aroScope.GetClient(),
					keyVaultSvc: mockKeyVaultSvc,
					newResourceReconciler: func(cp *cplane.AROControlPlane, resources []*unstructured.Unstructured) resourceReconciler {
						return &mockResourceReconciler{reconcileErr: errors.New("resource reconcile error")}
					},
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete

				return service, aroScope
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			service, aroScope := tc.mockServices(t)

			err := service.Reconcile(t.Context())

			if tc.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				if tc.validateResult != nil {
					tc.validateResult(g, aroScope, service.kubeclient)
				}
			}
		})
	}
}

func TestAROControlPlaneService_Pause(t *testing.T) {
	testCases := []struct {
		name          string
		expectedError bool
		mockServices  func(t *testing.T) *aroControlPlaneService
	}{
		{
			name:          "successful pause",
			expectedError: false,
			mockServices: func(t *testing.T) *aroControlPlaneService {
				t.Helper()
				aroScope, _ := createTestScope(t, nil)

				service := &aroControlPlaneService{
					scope:      aroScope,
					kubeclient: aroScope.GetClient(),
					newResourceReconciler: func(cp *cplane.AROControlPlane, resources []*unstructured.Unstructured) resourceReconciler {
						return &mockResourceReconciler{}
					},
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete

				return service
			},
		},
		{
			name:          "pause with resource error",
			expectedError: true,
			mockServices: func(t *testing.T) *aroControlPlaneService {
				t.Helper()
				aroScope, _ := createTestScope(t, nil)

				service := &aroControlPlaneService{
					scope:      aroScope,
					kubeclient: aroScope.GetClient(),
					newResourceReconciler: func(cp *cplane.AROControlPlane, resources []*unstructured.Unstructured) resourceReconciler {
						return &mockResourceReconciler{pauseErr: errors.New("pause error")}
					},
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete

				return service
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			service := tc.mockServices(t)

			err := service.Pause(t.Context())

			if tc.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAROControlPlaneService_Delete(t *testing.T) {
	testCases := []struct {
		name          string
		expectedError bool
		mockServices  func(t *testing.T) *aroControlPlaneService
	}{
		{
			name:          "successful delete",
			expectedError: false,
			mockServices: func(t *testing.T) *aroControlPlaneService {
				t.Helper()
				aroScope, _ := createTestScope(t, nil)

				service := &aroControlPlaneService{
					scope:      aroScope,
					kubeclient: aroScope.GetClient(),
					newResourceReconciler: func(cp *cplane.AROControlPlane, resources []*unstructured.Unstructured) resourceReconciler {
						return &mockResourceReconciler{}
					},
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete

				return service
			},
		},
		{
			name:          "delete with resource error",
			expectedError: true,
			mockServices: func(t *testing.T) *aroControlPlaneService {
				t.Helper()
				aroScope, _ := createTestScope(t, nil)

				service := &aroControlPlaneService{
					scope:      aroScope,
					kubeclient: aroScope.GetClient(),
					newResourceReconciler: func(cp *cplane.AROControlPlane, resources []*unstructured.Unstructured) resourceReconciler {
						return &mockResourceReconciler{deleteErr: errors.New("delete error")}
					},
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete

				return service
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			service := tc.mockServices(t)

			err := service.Delete(t.Context())

			if tc.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAROControlPlaneService_ReconcileKubeconfig(t *testing.T) {
	testCases := []struct {
		name            string
		kubeconfigData  string
		expectedError   bool
		validateSecrets func(*WithT, client.Client, string)
	}{
		{
			name:           "reconcile kubeconfig with CA data",
			kubeconfigData: TestKubeconfigWithValidCA,
			expectedError:  false,
			validateSecrets: func(g *WithT, client client.Client, clusterName string) {
				// Check kubeconfig secret
				kubeconfigSecret := &corev1.Secret{}
				kubeconfigKey := types.NamespacedName{
					Name:      secret.Name(clusterName, secret.Kubeconfig),
					Namespace: "default",
				}
				err := client.Get(t.Context(), kubeconfigKey, kubeconfigSecret)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(kubeconfigSecret.Data).To(HaveKey(secret.KubeconfigDataName))

				// Check cluster CA secret
				caSecret := &corev1.Secret{}
				caKey := types.NamespacedName{
					Name:      secret.Name(clusterName, secret.ClusterCA),
					Namespace: "default",
				}
				err = client.Get(t.Context(), caKey, caSecret)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(caSecret.Data).To(HaveKey(secret.TLSCrtDataName))
			},
		},
		{
			name: "reconcile kubeconfig without CA data",
			kubeconfigData: `apiVersion: v1
clusters:
- cluster:
    server: https://test-api.example.com:443
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
preferences: {}
users:
- name: test-user
  user:
    token: test-token`,
			expectedError: false,
			validateSecrets: func(g *WithT, client client.Client, clusterName string) {
				// Check kubeconfig secret
				kubeconfigSecret := &corev1.Secret{}
				kubeconfigKey := types.NamespacedName{
					Name:      secret.Name(clusterName, secret.Kubeconfig),
					Namespace: "default",
				}
				err := client.Get(t.Context(), kubeconfigKey, kubeconfigSecret)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(kubeconfigSecret.Data).To(HaveKey(secret.KubeconfigDataName))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			aroScope, fakeClient := createTestScopeWithOptions(t, &tc.kubeconfigData, true)

			service := &aroControlPlaneService{
				scope:      aroScope,
				kubeclient: fakeClient,
			}

			err := service.reconcileKubeconfig(t.Context())

			if tc.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				if tc.validateSecrets != nil {
					tc.validateSecrets(g, fakeClient, aroScope.ClusterName())
				}
			}
		})
	}
}

// mockResourceReconciler is a mock implementation of resourceReconciler for testing
type mockResourceReconciler struct {
	reconcileErr error
	pauseErr     error
	deleteErr    error
}

func (m *mockResourceReconciler) Reconcile(ctx context.Context) error {
	if m.reconcileErr != nil {
		return m.reconcileErr
	}
	return nil
}

func (m *mockResourceReconciler) Pause(ctx context.Context) error {
	if m.pauseErr != nil {
		return m.pauseErr
	}
	return nil
}

func (m *mockResourceReconciler) Delete(ctx context.Context) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

// mockServiceReconciler is a mock implementation of azure.ServiceReconciler for testing.
type mockServiceReconciler struct {
	reconcileErr error
	pauseErr     error
	deleteErr    error
}

func (m *mockServiceReconciler) Reconcile(ctx context.Context) error {
	if m.reconcileErr != nil {
		return m.reconcileErr
	}
	return nil
}

func (m *mockServiceReconciler) Pause(ctx context.Context) error {
	if m.pauseErr != nil {
		return m.pauseErr
	}
	return nil
}

func (m *mockServiceReconciler) Delete(ctx context.Context) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

func (m *mockServiceReconciler) Name() string {
	return "mockServiceReconciler"
}
