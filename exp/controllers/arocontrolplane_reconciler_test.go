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
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
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
	_ = cplane.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

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

	var initObjects []client.Object
	initObjects = append(initObjects, cluster, controlPlane, identity)

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

	// Set kubeconfig in scope if provided
	if kubeconfigData != nil {
		aroScope.SetKubeconfig(kubeconfigData, nil)
	}

	return aroScope, fakeClient
}

func TestNewAROControlPlaneService(t *testing.T) {
	g := NewWithT(t)

	aroScope, _ := createTestScope(t, nil)

	service, err := newAROControlPlaneService(aroScope)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(service).NotTo(BeNil())
	g.Expect(service.scope).To(Equal(aroScope))
	g.Expect(service.services).To(HaveLen(10)) // groups, networksecuritygroups, virtualnetworks, subnets, vaults, keyvaults, userassignedidentities, roleassignmentsaso, hcpopenshiftclusters, hcpopenshiftclustersexternalauth (ASO-based)
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

				// Create a service with mocked Azure services
				service := &aroControlPlaneService{
					scope:    aroScope,
					services: []azure.ServiceReconciler{},
					skuCache: resourceskus.NewStaticCache(nil, "eastus"),
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete
				service.kubeclient = aroScope.GetClient()

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

				service := &aroControlPlaneService{
					scope:    aroScope,
					services: []azure.ServiceReconciler{},
					skuCache: resourceskus.NewStaticCache(nil, "eastus"),
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete
				service.kubeclient = aroScope.GetClient()

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
			name:          "service reconcile error",
			expectedError: true,
			mockServices: func(t *testing.T) (*aroControlPlaneService, *scope.AROControlPlaneScope) {
				t.Helper()
				aroScope, _ := createTestScope(t, nil)

				mockService := mock_azure.NewMockServiceReconciler(gomock.NewController(t))
				mockService.EXPECT().Reconcile(gomock.Any()).Return(errors.New("service error"))
				mockService.EXPECT().Name().Return("test-service").AnyTimes()

				service := &aroControlPlaneService{
					scope:    aroScope,
					services: []azure.ServiceReconciler{mockService},
					skuCache: resourceskus.NewStaticCache(nil, "eastus"),
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete
				service.kubeclient = aroScope.GetClient()

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
					scope:    aroScope,
					services: []azure.ServiceReconciler{},
					skuCache: resourceskus.NewStaticCache(nil, "eastus"),
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete

				return service
			},
		},
		{
			name:          "pause with service error",
			expectedError: true,
			mockServices: func(t *testing.T) *aroControlPlaneService {
				t.Helper()
				aroScope, _ := createTestScope(t, nil)

				mockService := mock_azure.NewMockServiceReconciler(gomock.NewController(t))
				mockPauser := mock_azure.NewMockPauser(gomock.NewController(t))
				mockPauser.EXPECT().Pause(gomock.Any()).Return(errors.New("pause error"))
				mockService.EXPECT().Name().Return("test-service")

				// Create a service that implements both interfaces
				serviceWithPauser := struct {
					azure.ServiceReconciler
					azure.Pauser
				}{
					ServiceReconciler: mockService,
					Pauser:            mockPauser,
				}

				service := &aroControlPlaneService{
					scope:    aroScope,
					services: []azure.ServiceReconciler{serviceWithPauser},
					skuCache: resourceskus.NewStaticCache(nil, "eastus"),
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
					scope:    aroScope,
					services: []azure.ServiceReconciler{},
					skuCache: resourceskus.NewStaticCache(nil, "eastus"),
				}
				service.Reconcile = service.reconcile
				service.Pause = service.pause
				service.Delete = service.delete

				return service
			},
		},
		{
			name:          "delete with service error",
			expectedError: true,
			mockServices: func(t *testing.T) *aroControlPlaneService {
				t.Helper()
				aroScope, _ := createTestScope(t, nil)

				mockService := mock_azure.NewMockServiceReconciler(gomock.NewController(t))
				mockService.EXPECT().Delete(gomock.Any()).Return(errors.New("delete error"))
				mockService.EXPECT().Name().Return("test-service")

				service := &aroControlPlaneService{
					scope:    aroScope,
					services: []azure.ServiceReconciler{mockService},
					skuCache: resourceskus.NewStaticCache(nil, "eastus"),
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

func TestAROControlPlaneService_GetService(t *testing.T) {
	aroScope, _ := createTestScope(t, nil)

	mockService := mock_azure.NewMockServiceReconciler(gomock.NewController(t))
	mockService.EXPECT().Name().Return("test-service").AnyTimes()

	service := &aroControlPlaneService{
		scope:    aroScope,
		services: []azure.ServiceReconciler{mockService},
	}

	testCases := []struct {
		name          string
		serviceName   string
		expectedError bool
		expectedNil   bool
	}{
		{
			name:          "existing service",
			serviceName:   "test-service",
			expectedError: false,
			expectedNil:   false,
		},
		{
			name:          "non-existing service",
			serviceName:   "non-existing-service",
			expectedError: true,
			expectedNil:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := service.getService(tc.serviceName)

			if tc.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			if tc.expectedNil {
				g.Expect(result).To(BeNil())
			} else {
				g.Expect(result).NotTo(BeNil())
			}
		})
	}
}
