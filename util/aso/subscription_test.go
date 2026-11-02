/*
Copyright 2026 The Kubernetes Authors.

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

package aso

import (
	"strings"
	"testing"

	asoconfig "github.com/Azure/azure-service-operator/v2/pkg/common/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateAzureCLISubscription(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	namespaceCredential := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: asoNamespacedCredentialSecret, Namespace: "ns"},
		Data: map[string][]byte{
			asoconfig.AzureSubscriptionID: []byte("secret-sub"),
			asoconfig.AzureTenantID:       []byte("tenant"),
			asoconfig.AzureClientID:       []byte("client"),
		},
	}
	subscriptionOnly := namespaceCredential.DeepCopy()
	delete(subscriptionOnly.Data, asoconfig.AzureTenantID)
	delete(subscriptionOnly.Data, asoconfig.AzureClientID)

	tests := []struct {
		name    string
		env     string
		objects []runtime.Object
		specSub string
		wantErr string
	}{
		{
			name:    "no namespace credential allows the ASO global credential",
			env:     "other-sub",
			specSub: "cluster-sub",
		},
		{
			name: "no namespace credential allows the ASO global credential when the cluster subscription is empty",
			env:  "cluster-sub",
		},
		{
			name:    "namespace credential subscription matches",
			objects: []runtime.Object{namespaceCredential},
			specSub: "secret-sub",
		},
		{
			name:    "namespace credential subscription differs",
			objects: []runtime.Object{namespaceCredential},
			specSub: "cluster-sub",
			wantErr: "does not match",
		},
		{
			name:    "namespace credential missing tenant and client id",
			objects: []runtime.Object{subscriptionOnly},
			specSub: "secret-sub",
			wantErr: "does not contain key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AZURE_SUBSCRIPTION_ID", tt.env)
			c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objects...).Build()
			err := ValidateAzureCLISubscription(t.Context(), c, "ns", tt.specSub)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
