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

package v1beta1

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/maps"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedmachinepool,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools,verbs=create;update,versions=v1beta1,name=default.azuremanagedmachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) Default(client client.Client) {
	if r.Labels == nil {
		r.Labels = make(map[string]string)
	}
	r.Labels[LabelAgentPoolMode] = r.Spec.Mode

	if r.Spec.Name == nil || *r.Spec.Name == "" {
		r.Spec.Name = &r.Name
	}

	if r.Spec.ScaleSetPriority == nil {
		r.Spec.ScaleSetPriority = to.StringPtr(DefaultScaleSetPriority)
	}
}

//+kubebuilder:webhook:verbs=update;delete,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedmachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools,versions=v1beta1,name=validation.azuremanagedmachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) ValidateCreate(client client.Client) error {
	validators := []func() error{
		r.validateMaxPods,
		r.ValidateSpotNodePool,
	}

	var errs []error
	for _, validator := range validators {
		if err := validator(); err != nil {
			errs = append(errs, err)
		}
	}

	return kerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) ValidateUpdate(oldRaw runtime.Object, client client.Client) error {
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

	if old.Spec.ScaleSetPriority != nil {
		// Prevent ScaleSetPriority modification if it was already set to some value
		if r.Spec.ScaleSetPriority == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "ScaleSetPriority"),
					r.Spec.ScaleSetPriority,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.ScaleSetPriority != *old.Spec.ScaleSetPriority {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "ScaleSetPriority"),
					*r.Spec.ScaleSetPriority,
					"field is immutable"))
		}
	}

	if !reflect.DeepEqual(r.Spec.Taints, old.Spec.Taints) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "Taints"),
				r.Spec.Taints,
				"field is immutable"))
	}

	// custom headers are immutable
	oldCustomHeaders := maps.FilterByKeyPrefix(old.ObjectMeta.Annotations, azure.CustomHeaderPrefix)
	newCustomHeaders := maps.FilterByKeyPrefix(r.ObjectMeta.Annotations, azure.CustomHeaderPrefix)
	if !reflect.DeepEqual(oldCustomHeaders, newCustomHeaders) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("metadata", "annotations"),
				r.ObjectMeta.Annotations,
				fmt.Sprintf("annotations with '%s' prefix are immutable", azure.CustomHeaderPrefix)))
	}

	if !ensureStringSlicesAreEqual(r.Spec.AvailabilityZones, old.Spec.AvailabilityZones) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "AvailabilityZones"),
				r.Spec.AvailabilityZones,
				"field is immutable"))
	}

	if r.Spec.Mode != string(NodePoolModeSystem) && old.Spec.Mode == string(NodePoolModeSystem) {
		// validate for last system node pool
		if err := r.validateLastSystemNodePool(client); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("Spec", "Mode"),
				r.Spec.Mode,
				"Last system node pool cannot be mutated to user node pool"))
		}
	}

	if old.Spec.MaxPods != nil {
		// Prevent MaxPods modification if it was already set to some value
		if r.Spec.MaxPods == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "MaxPods"),
					r.Spec.MaxPods,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.MaxPods != *old.Spec.MaxPods {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "MaxPods"),
					*r.Spec.MaxPods,
					"field is immutable"))
		}
	}

	if old.Spec.OsDiskType != nil {
		// Prevent OSDiskType modification if it was already set to some value
		if r.Spec.OsDiskType == nil || to.String(r.Spec.OsDiskType) == "" {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OsDiskType"),
					r.Spec.OsDiskType,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.OsDiskType != *old.Spec.OsDiskType {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OsDiskType"),
					r.Spec.OsDiskType,
					"field is immutable"))
		}
	}

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedMachinePool").GroupKind(), r.Name, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) ValidateDelete(client client.Client) error {
	if r.Spec.Mode != string(NodePoolModeSystem) {
		return nil
	}

	return errors.Wrapf(r.validateLastSystemNodePool(client), "if the delete is triggered via owner MachinePool please refer to trouble shooting section in https://capz.sigs.k8s.io/topics/managedcluster.html")
}

// validateLastSystemNodePool is used to check if the existing system node pool is the last system node pool.
// If it is a last system node pool it cannot be deleted or mutated to user node pool as AKS expects min 1 system node pool.
func (r *AzureManagedMachinePool) validateLastSystemNodePool(cli client.Client) error {
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

	if err := cli.Get(ctx, key, ownerCluster); err != nil {
		if azure.ResourceNotFound(err) {
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
		LabelAgentPoolMode:         string(NodePoolModeSystem),
	})

	ammpList := &AzureManagedMachinePoolList{}
	if err := cli.List(ctx, ammpList, opt1, opt2); err != nil {
		return err
	}

	if len(ammpList.Items) <= 1 {
		return errors.New("AKS Cluster must have at least one system pool")
	}
	return nil
}

func (r *AzureManagedMachinePool) validateMaxPods() error {
	if r.Spec.MaxPods != nil {
		if to.Int32(r.Spec.MaxPods) < 10 || to.Int32(r.Spec.MaxPods) > 250 {
			return field.Invalid(
				field.NewPath("Spec", "MaxPods"),
				r.Spec.MaxPods,
				"MaxPods must be between 10 and 250")
		}
	}

	return nil
}

func (r *AzureManagedMachinePool) ValidateSpotNodePool() error {
	if r.Spec.ScaleSetPriority != nil && *r.Spec.ScaleSetPriority == "Spot" {
		if r.Spec.Mode != string(NodePoolModeUser) {
			return field.Forbidden(
				field.NewPath("Spec", "ScaleSetPriority"),
				"Spot ScaleSetPriority requires AzureManagedMachinePool mode User")
		}
	}

	return nil
}

func ensureStringSlicesAreEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	m := map[string]bool{}
	for _, v := range a {
		m[v] = true
	}

	for _, v := range b {
		if _, ok := m[v]; !ok {
			return false
		}
	}
	return true
}
