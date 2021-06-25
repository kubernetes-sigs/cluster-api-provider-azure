/*
Copyright 2021 The Kubernetes Authors.

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

package v1alpha4

import (
	"context"
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api-provider-azure/azure"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var azuremanagedmachinepoollog = logf.Log.WithName("azuremanagedmachinepool-resource")

var webhookClient client.Client

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (r *AzureManagedMachinePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// NewAzureManagedMachinePoolWebhook returns a new AzureManagedMachinePool instance.
func NewAzureManagedMachinePoolWebhook(client client.Client) *AzureManagedMachinePool {
	webhookClient = client
	return &AzureManagedMachinePool{}
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha4-azuremanagedmachinepool,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools,verbs=create;update,versions=v1alpha4,name=default.azuremanagedmachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

var _ webhook.Defaulter = &AzureManagedMachinePool{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) Default() {
	azuremanagedmachinepoollog.Info("default", "name", r.Name)

	if r.Labels == nil {
		r.Labels = make(map[string]string)
	}
	r.Labels[LabelAgentPoolMode] = r.Spec.Mode
}

//+kubebuilder:webhook:verbs=update;delete,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha4-azuremanagedmachinepool,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools,versions=v1alpha4,name=validation.azuremanagedmachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

var _ webhook.Validator = &AzureManagedMachinePool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) ValidateCreate() error {
	azuremanagedmachinepoollog.Info("validate create", "name", r.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) ValidateUpdate(oldRaw runtime.Object) error {
	old := oldRaw.(*AzureManagedMachinePool)
	var allErrs field.ErrorList

	if r.Spec.SKU != old.Spec.SKU {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "SKU"),
				r.Spec.SKU,
				"field is immutable"))
	}

	if old.Spec.OSDiskSizeGB != nil {
		// Prevent OSDiskSizeGB modification if it was already set to some value
		if r.Spec.OSDiskSizeGB == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OSDiskSizeGB"),
					r.Spec.OSDiskSizeGB,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.OSDiskSizeGB != *old.Spec.OSDiskSizeGB {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OSDiskSizeGB"),
					*r.Spec.OSDiskSizeGB,
					"field is immutable"))
		}
	}

	if r.Spec.Mode != SystemNodePool && old.Spec.Mode == SystemNodePool {
		// validate for last system node pool
		if err := r.validateLastSystemNodePool(); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("Spec", "Mode"),
				r.Spec.Mode,
				"Last system node pool cannot be mutated to user node pool"))
		}
	}

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedMachinePool").GroupKind(), r.Name, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) ValidateDelete() error {
	azuremanagedmachinepoollog.Info("validate delete", "name", r.Name)

	if r.Spec.Mode != SystemNodePool {
		return nil
	}

	return r.validateLastSystemNodePool()
}

func (r *AzureManagedMachinePool) validateLastSystemNodePool() error {
	ctx := context.Background()

	// Fetch the Cluster.
	clusterName, ok := r.Labels[clusterv1.ClusterLabelName]
	if !ok {
		return nil
	}

	ownerCluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Namespace: r.Namespace,
		Name:      clusterName,
	}

	if err := webhookClient.Get(ctx, key, ownerCluster); err != nil {
		if !azure.ResourceNotFound(err) {
			return nil
		}
		return err
	}

	if !ownerCluster.DeletionTimestamp.IsZero() {
		return nil
	}

	opt1 := client.InNamespace(r.Namespace)
	opt2 := client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: clusterName,
		LabelAgentPoolMode:         SystemNodePool,
	})

	ammpList := &AzureManagedMachinePoolList{}
	if err := webhookClient.List(ctx, ammpList, opt1, opt2); err != nil {
		return err
	}

	if len(ammpList.Items) <= 1 {
		return errors.New("Aks Cluster must have at least one system pool")
	}
	return nil
}
