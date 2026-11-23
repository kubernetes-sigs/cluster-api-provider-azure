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

package webhooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	clusterctlv1alpha3 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (mw *AROMachinePoolWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	mw.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr, &infrav1exp.AROMachinePool{}).
		WithDefaulter(mw).
		WithValidator(mw).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-aromachinepool,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools,verbs=create;update,versions=v1beta2,name=default.aromachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

// AROMachinePoolWebhook implements a validating and defaulting webhook for AROMachinePool.
type AROMachinePoolWebhook struct {
	Client client.Client
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (mw *AROMachinePoolWebhook) Default(_ context.Context, _ *infrav1exp.AROMachinePool) error {
	return nil
}

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-aromachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools,versions=v1beta2,name=validation.aromachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (mw *AROMachinePoolWebhook) ValidateCreate(_ context.Context, m *infrav1exp.AROMachinePool) (admission.Warnings, error) {
	return nil, validateAROMachinePool(m)
}

// validateAROMachinePool validates the ARO Machine Pool and returns an aggregate error.
func validateAROMachinePool(m *infrav1exp.AROMachinePool) error {
	var errs []error

	// Validate that resources mode is used
	if len(m.Spec.Resources) == 0 {
		errs = append(errs, field.Required(
			field.NewPath("spec", "resources"),
			"resources mode is required; field-based configuration is no longer supported"))
	}

	// Validate resources if specified
	if resourcesErr := validateAROMachinePoolResources(m); resourcesErr != nil {
		errs = append(errs, resourcesErr)
	}

	return kerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (mw *AROMachinePoolWebhook) ValidateUpdate(_ context.Context, _, m *infrav1exp.AROMachinePool) (admission.Warnings, error) {
	return nil, validateAROMachinePool(m)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (mw *AROMachinePoolWebhook) ValidateDelete(_ context.Context, m *infrav1exp.AROMachinePool) (admission.Warnings, error) {
	return nil, errors.Wrapf(validateLastSystemNodePool(mw.Client, m.Labels, m.Namespace, m.Annotations), "if the delete is triggered via owner MachinePool please refer to trouble shooting section in https://capz.sigs.k8s.io/topics/managedcluster.html")
}

// validateLastSystemNodePool is used to check if the existing system node pool is the last system node pool.
// If it is a last system node pool it cannot be deleted or mutated to user node pool as AKS expects min 1 system node pool.
func validateLastSystemNodePool(cli client.Client, labels map[string]string, namespace string, annotations map[string]string) error {
	ctx := context.Background()

	// Fetch the Cluster.
	clusterName, ok := labels[clusterv1.ClusterNameLabel]
	if !ok {
		return nil
	}

	ownerCluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      clusterName,
	}

	if err := cli.Get(ctx, key, ownerCluster); err != nil {
		// If the cluster doesn't exist, allow deletion
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !ownerCluster.DeletionTimestamp.IsZero() {
		return nil
	}

	// checking if this AROMachinePool is going to be deleted for clusterctl move operation
	if _, ok := annotations[clusterctlv1alpha3.DeleteForMoveAnnotation]; ok {
		return nil
	}

	opt1 := client.InNamespace(namespace)
	opt2 := client.MatchingLabels(map[string]string{
		clusterv1.ClusterNameLabel: clusterName,
		//LabelAgentPoolMode:         string(NodePoolModeSystem),
	})

	ammpList := &infrav1exp.AROMachinePoolList{}
	if err := cli.List(ctx, ammpList, opt1, opt2); err != nil {
		// If listing fails, allow deletion (cluster might be in deletion state)
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if len(ammpList.Items) <= 1 {
		return errors.New("ARO Cluster must have at least one system pool")
	}
	return nil
}

// validateResources validates the resources field configuration.
func validateAROMachinePoolResources(m *infrav1exp.AROMachinePool) error {
	if len(m.Spec.Resources) == 0 {
		return nil // Resources is optional
	}

	basePath := field.NewPath("spec", "resources")
	var allErrs field.ErrorList

	// Validate that each resource can be unmarshaled
	for i := range m.Spec.Resources {
		resourcePath := basePath.Index(i)
		raw := &m.Spec.Resources[i]

		if raw.Raw == nil {
			allErrs = append(allErrs, field.Required(resourcePath, "resource cannot be empty"))
			continue
		}

		// Basic validation: check that it's valid JSON
		var obj map[string]interface{}
		if err := json.Unmarshal(raw.Raw, &obj); err != nil {
			allErrs = append(allErrs, field.Invalid(resourcePath, string(raw.Raw), fmt.Sprintf("resource must be valid JSON: %v", err)))
			continue
		}

		// Validate that required fields exist
		apiVersion, ok := obj["apiVersion"].(string)
		if !ok || apiVersion == "" {
			allErrs = append(allErrs, field.Required(resourcePath.Child("apiVersion"), "resource must have apiVersion"))
		}

		kind, ok := obj["kind"].(string)
		if !ok || kind == "" {
			allErrs = append(allErrs, field.Required(resourcePath.Child("kind"), "resource must have kind"))
		}

		// Check if metadata exists
		metadata, ok := obj["metadata"].(map[string]interface{})
		if !ok {
			allErrs = append(allErrs, field.Required(resourcePath.Child("metadata"), "resource must have metadata"))
			continue
		}

		// Validate name exists in metadata
		name, ok := metadata["name"].(string)
		if !ok || name == "" {
			allErrs = append(allErrs, field.Required(resourcePath.Child("metadata", "name"), "resource must have metadata.name"))
		}
	}

	return allErrs.ToAggregate()
}
