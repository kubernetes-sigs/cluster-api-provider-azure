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

package v1beta2

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (r *AROCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(aroClusterWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(w).
		Complete()
}

type aroClusterWebhook struct{}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-arocluster,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=aroclusters,versions=v1beta2,name=validation.aroclusters.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

var _ webhook.CustomValidator = &aroClusterWebhook{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *aroClusterWebhook) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *aroClusterWebhook) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	// Note: AROClusterSpec currently only has ControlPlaneEndpoint field
	// The endpoint itself can be updated as it may change during cluster lifecycle
	// If more fields are added to AROClusterSpec in the future that should be immutable,
	// they should be added here following the pattern from other ARO webhooks
	// with proper validation using webhookutils.ValidateImmutable()

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *aroClusterWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
