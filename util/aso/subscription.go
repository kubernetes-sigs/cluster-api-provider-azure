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
	"context"
	"os"
	"strings"

	asoconfig "github.com/Azure/azure-service-operator/v2/pkg/common/config"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// asoNamespacedCredentialSecret is the namespace-scoped credential ASO prefers
// over its global credential. It matches identity.NamespacedSecretName in ASO.
const asoNamespacedCredentialSecret = "aso-credential" //nolint:gosec // Secret name, not a credential.

// ValidateAzureCLISubscription returns an error when a namespace aso-credential
// Secret would select a different subscription than the cluster once the
// per-resource credential annotation is removed.
//
// AzureCLI has no client ID or client secret, and ASO cannot use Azure CLI
// auth at namespace or resource scope. A per-resource Secret with only
// AZURE_SUBSCRIPTION_ID is not a valid ASO credential. With no namespace
// aso-credential Secret, ASO uses its global credential and this check
// succeeds. CAPZ cannot read that global credential. When the namespace Secret
// exists, ASO uses it and does not fall back to the global credential, so its
// subscription must match the cluster.
func ValidateAzureCLISubscription(ctx context.Context, c client.Client, namespace, specSubscription string) error {
	asoSubscription, found, err := subscriptionASOWouldUse(ctx, c, namespace)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	clusterSubscription := specSubscription
	if clusterSubscription == "" {
		clusterSubscription = os.Getenv("AZURE_SUBSCRIPTION_ID")
	}
	if !strings.EqualFold(clusterSubscription, asoSubscription) {
		return errors.Errorf("AzureCLI cluster subscription %q does not match the subscription %q ASO would use", clusterSubscription, asoSubscription)
	}
	return nil
}

// subscriptionASOWouldUse reports the subscription ASO selects from a
// namespace-scoped credential. The global credential is configured in ASO's
// process and cannot be inspected by CAPZ.
func subscriptionASOWouldUse(ctx context.Context, c client.Client, namespace string) (string, bool, error) {
	secret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{Name: asoNamespacedCredentialSecret, Namespace: namespace}, secret)
	if apierrors.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, errors.Wrapf(err, "failed to get %s/%s", namespace, asoNamespacedCredentialSecret)
	}
	subscription, ok := secret.Data[asoconfig.AzureSubscriptionID]
	if !ok || len(subscription) == 0 {
		return "", false, errors.Errorf("credential secret %s/%s does not contain key %q", namespace, asoNamespacedCredentialSecret, asoconfig.AzureSubscriptionID)
	}
	// ASO requires these keys and does not fall back to the global credential
	// when the namespace secret exists but cannot be parsed.
	for _, key := range []string{asoconfig.AzureTenantID, asoconfig.AzureClientID} {
		value, exists := secret.Data[key]
		if !exists || len(value) == 0 {
			return "", false, errors.Errorf("credential secret %s/%s does not contain key %q", namespace, asoNamespacedCredentialSecret, key)
		}
	}
	return string(subscription), true, nil
}
