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

package controllers

import (
	"context"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asoannotations "github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/common/config"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
)

type credentialParams struct {
	credentialType     azure.CredentialType
	tenantID           string
	clientID           string
	clientSecret       string
	clientCert         []byte
	clientCertPassword []byte

	authorityHost string
	armEndpoint   string
	armAudience   string
}

func TestAuthTokenForASOResource(t *testing.T) {
	tests := []struct {
		name           string
		resource       client.Object
		secret         *corev1.Secret
		expectedParams credentialParams
		expectedErr    error
	}{
		{
			name: "per-resource secret client secret",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Annotations: map[string]string{
						asoannotations.PerResourceSecret: "my-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "namespace",
				},
				Data: map[string][]byte{
					config.AzureTenantID:     []byte("tenant"),
					config.AzureClientID:     []byte("client"),
					config.AzureClientSecret: []byte("hunter2"),
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialTypeClientSecret,
				tenantID:       "tenant",
				clientID:       "client",
				clientSecret:   "hunter2",
			},
		},
		{
			name: "per-resource secret client cert",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Annotations: map[string]string{
						asoannotations.PerResourceSecret: "my-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "namespace",
				},
				Data: map[string][]byte{
					config.AzureTenantID:                  []byte("tenant"),
					config.AzureClientID:                  []byte("client"),
					config.AzureClientCertificate:         []byte("cert"),
					config.AzureClientCertificatePassword: []byte("hunter2"),
				},
			},
			expectedParams: credentialParams{
				credentialType:     azure.CredentialTypeClientCert,
				tenantID:           "tenant",
				clientID:           "client",
				clientCert:         []byte("cert"),
				clientCertPassword: []byte("hunter2"),
			},
		},
		{
			name: "per-resource secret managed identity",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Annotations: map[string]string{
						asoannotations.PerResourceSecret: "my-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "namespace",
				},
				Data: map[string][]byte{
					config.AzureClientID: []byte("client"),
					config.AuthMode:      []byte(config.PodIdentityAuthMode),
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialTypeManagedIdentity,
				clientID:       "client",
			},
		},
		{
			name: "per-resource secret workload identity",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Annotations: map[string]string{
						asoannotations.PerResourceSecret: "my-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "namespace",
				},
				Data: map[string][]byte{
					config.AzureTenantID: []byte("tenant"),
					config.AzureClientID: []byte("client"),
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialTypeWorkloadIdentity,
				tenantID:       "tenant",
				clientID:       "client",
			},
		},
		{
			name: "namespace secret client secret",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      asoNamespaceSecretName,
					Namespace: "namespace",
				},
				Data: map[string][]byte{
					config.AzureTenantID:     []byte("tenant"),
					config.AzureClientID:     []byte("client"),
					config.AzureClientSecret: []byte("hunter2"),
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialTypeClientSecret,
				tenantID:       "tenant",
				clientID:       "client",
				clientSecret:   "hunter2",
			},
		},
		{
			name: "namespace secret client cert",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      asoNamespaceSecretName,
					Namespace: "namespace",
				},
				Data: map[string][]byte{
					config.AzureTenantID:                  []byte("tenant"),
					config.AzureClientID:                  []byte("client"),
					config.AzureClientCertificate:         []byte("cert"),
					config.AzureClientCertificatePassword: []byte("hunter2"),
				},
			},
			expectedParams: credentialParams{
				credentialType:     azure.CredentialTypeClientCert,
				tenantID:           "tenant",
				clientID:           "client",
				clientCert:         []byte("cert"),
				clientCertPassword: []byte("hunter2"),
			},
		},
		{
			name: "namespace secret managed identity",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      asoNamespaceSecretName,
					Namespace: "namespace",
				},
				Data: map[string][]byte{
					config.AzureClientID: []byte("client"),
					config.AuthMode:      []byte(config.PodIdentityAuthMode),
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialTypeManagedIdentity,
				clientID:       "client",
			},
		},
		{
			name: "namespace secret workload identity",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      asoNamespaceSecretName,
					Namespace: "namespace",
				},
				Data: map[string][]byte{
					config.AzureTenantID: []byte("tenant"),
					config.AzureClientID: []byte("client"),
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialTypeWorkloadIdentity,
				tenantID:       "tenant",
				clientID:       "client",
			},
		},
		{
			name: "global secret client secret",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Annotations: map[string]string{
						asoNamespaceAnnotation: "aso-namespace",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      asoGlobalSecretName,
					Namespace: "aso-namespace",
				},
				Data: map[string][]byte{
					config.AzureTenantID:           []byte("tenant"),
					config.AzureClientID:           []byte("client"),
					config.AzureClientSecret:       []byte("hunter2"),
					config.AzureAuthorityHost:      []byte("auth host"),
					config.ResourceManagerEndpoint: []byte("arm endpoint"),
					config.ResourceManagerAudience: []byte("arm audience"),
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialTypeClientSecret,
				authorityHost:  "auth host",
				armEndpoint:    "arm endpoint",
				armAudience:    "arm audience",
				tenantID:       "tenant",
				clientID:       "client",
				clientSecret:   "hunter2",
			},
		},
		{
			name: "global secret client cert",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Annotations: map[string]string{
						asoNamespaceAnnotation: "aso-namespace",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      asoGlobalSecretName,
					Namespace: "aso-namespace",
				},
				Data: map[string][]byte{
					config.AzureTenantID:                  []byte("tenant"),
					config.AzureClientID:                  []byte("client"),
					config.AzureClientCertificate:         []byte("cert"),
					config.AzureClientCertificatePassword: []byte("hunter2"),
				},
			},
			expectedParams: credentialParams{
				credentialType:     azure.CredentialTypeClientCert,
				tenantID:           "tenant",
				clientID:           "client",
				clientCert:         []byte("cert"),
				clientCertPassword: []byte("hunter2"),
			},
		},
		{
			name: "global secret managed identity",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Annotations: map[string]string{
						asoNamespaceAnnotation: "aso-namespace",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      asoGlobalSecretName,
					Namespace: "aso-namespace",
				},
				Data: map[string][]byte{
					config.AzureClientID: []byte("client"),
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialTypeManagedIdentity,
				clientID:       "client",
			},
		},
		{
			name: "global secret workload identity",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Annotations: map[string]string{
						asoNamespaceAnnotation: "aso-namespace",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      asoGlobalSecretName,
					Namespace: "aso-namespace",
				},
				Data: map[string][]byte{
					config.UseWorkloadIdentityAuth: []byte("true"),
					config.AzureTenantID:           []byte("tenant"),
					config.AzureClientID:           []byte("client"),
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialTypeWorkloadIdentity,
				tenantID:       "tenant",
				clientID:       "client",
			},
		},
		{
			name: "secret not found",
			resource: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Annotations: map[string]string{
						asoannotations.PerResourceSecret: "my-secret",
					},
				},
			},
			expectedParams: credentialParams{
				credentialType: azure.CredentialType(-1), // don't expect any calls to the cache
			},
			secret:      nil,
			expectedErr: apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, asoGlobalSecretName), // When the per-resource secret isn't found, we try to get the global one
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			mockCtrl := gomock.NewController(t)

			var objs []client.Object
			if test.secret != nil {
				objs = append(objs, test.secret)
			}

			c := fakeclient.NewClientBuilder().
				WithObjects(objs...).
				Build()

			credCache := mock_azure.NewMockCredentialCache(mockCtrl)

			expectedClientOpts := azcore.ClientOptions{
				Cloud: cloud.Configuration{
					ActiveDirectoryAuthorityHost: test.expectedParams.authorityHost,
				},
			}
			if test.expectedParams.armAudience != "" ||
				test.expectedParams.armEndpoint != "" {
				expectedClientOpts.Cloud.Services = map[cloud.ServiceName]cloud.ServiceConfiguration{
					cloud.ResourceManager: {
						Audience: test.expectedParams.armAudience,
						Endpoint: test.expectedParams.armEndpoint,
					},
				}
			}

			switch test.expectedParams.credentialType {
			case azure.CredentialTypeClientSecret:
				credCache.EXPECT().GetOrStoreClientSecret(
					test.expectedParams.tenantID,
					test.expectedParams.clientID,
					test.expectedParams.clientSecret,
					gomock.Cond(func(opts *azidentity.ClientSecretCredentialOptions) bool {
						// ignore tracing provider
						return reflect.DeepEqual(expectedClientOpts.Cloud, opts.Cloud)
					}),
				).Return(nil, nil)
			case azure.CredentialTypeClientCert:
				credCache.EXPECT().GetOrStoreClientCert(
					test.expectedParams.tenantID,
					test.expectedParams.clientID,
					test.expectedParams.clientCert,
					test.expectedParams.clientCertPassword,
					gomock.Cond(func(opts *azidentity.ClientCertificateCredentialOptions) bool {
						// ignore tracing provider
						return reflect.DeepEqual(expectedClientOpts.Cloud, opts.Cloud)
					}),
				).Return(nil, nil)
			case azure.CredentialTypeManagedIdentity:
				credCache.EXPECT().GetOrStoreManagedIdentity(
					gomock.Cond(func(opts *azidentity.ManagedIdentityCredentialOptions) bool {
						// ignore tracing provider
						return reflect.DeepEqual(expectedClientOpts.Cloud, opts.Cloud) &&
							reflect.DeepEqual(azidentity.ClientID(test.expectedParams.clientID), opts.ID)
					}),
				).Return(nil, nil)
			case azure.CredentialTypeWorkloadIdentity:
				credCache.EXPECT().GetOrStoreWorkloadIdentity(
					gomock.Cond(func(opts *azidentity.WorkloadIdentityCredentialOptions) bool {
						// ignore tracing provider
						return reflect.DeepEqual(expectedClientOpts.Cloud, opts.Cloud) &&
							opts.TenantID == test.expectedParams.tenantID &&
							opts.ClientID == test.expectedParams.clientID
						// ignore token file path, it's always the same
					}),
				).Return(nil, nil)
			}

			asoCache := &asoCredentialCache{
				cache:  credCache,
				client: c,
			}
			_, err := asoCache.authTokenForASOResource(context.Background(), test.resource)
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
