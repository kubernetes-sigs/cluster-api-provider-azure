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
	"reflect"
	"regexp"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api-provider-azure/azure"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// log is for logging in this package.
var azuremanagedmachinepoollog = logf.Log.WithName("azuremanagedmachinepool-resource")

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedmachinepool,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools,verbs=create;update,versions=v1beta1,name=default.azuremanagedmachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) Default(client client.Client) {
	azuremanagedmachinepoollog.Info("default", "name", r.Name)

	if r.Labels == nil {
		r.Labels = make(map[string]string)
	}
	r.Labels[LabelAgentPoolMode] = r.Spec.Mode

	if r.Spec.Name == nil || *r.Spec.Name == "" {
		r.Spec.Name = &r.Name
	}
}

//+kubebuilder:webhook:verbs=update;delete,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedmachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools,versions=v1beta1,name=validation.azuremanagedmachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) ValidateCreate(client client.Client) error {
	azuremanagedmachinepoollog.Info("validate create", "name", r.Name)
	var allErrs field.ErrorList

	// If AutoScaling is enabled, both MinCount and MaxCount should be set.
	if r.Spec.AutoScaling != nil && (*r.Spec.AutoScaling.MaxCount < *r.Spec.AutoScaling.MinCount) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "AutoScaling"),
				r.Spec.AutoScaling,
				"MaxCount must be greater or equal to MinCount when AutoScaling is enabled"))
	}

	// NodeTaints should follow pattern key=value:effect.
	if r.Spec.NodeTaints != nil {
		re := regexp.MustCompile(`^.+=.+:.+$`)
		for _, v := range r.Spec.NodeTaints {
			if re.FindString(v) == "" {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("Spec", "NodeTaints"),
						r.Spec.NodeTaints,
						"Node taints should follow the pattern such as key=value:effect"))
			}
		}
	}

	// AllowedUnsafeSysctls should be one of "kernel.shm*", "kernel.msg*", "kernel.sem", "fs.mqueue.*", "net.*".
	if r.Spec.KubeletConfig != nil && len(*r.Spec.KubeletConfig.AllowedUnsafeSysctls) > 0 {
		for _, v := range *r.Spec.KubeletConfig.AllowedUnsafeSysctls {
			switch v {
			case "kernel.shm*", "kernel.msg*", "kernel.sem", "fs.mqueue.*", "net.*":
				continue
			}
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "KubeletConfig"),
					r.Spec.KubeletConfig,
					"AllowedUnsafeSysctls are \"kernel.shm*\", \"kernel.msg*\", \"kernel.sem\", \"fs.mqueue.*\", \"net.*\""))
		}
	}

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedMachinePool").GroupKind(), r.Name, allErrs)
	}
	return nil
}

// ValidateUpdateHelper is a helper function to reduce cyclomatic complexity.
func ValidateUpdateHelper(old AzureManagedMachinePoolSpec, r AzureManagedMachinePoolSpec) field.ErrorList {
	var updateErrs field.ErrorList
	if r.SKU != old.SKU {
		updateErrs = append(updateErrs,
			field.Invalid(
				field.NewPath("Spec", "SKU"),
				r.SKU,
				"field is immutable"))
	}

	if old.OSDiskSizeGB != nil {
		// Prevent OSDiskSizeGB modification if it was already set to some value
		if r.OSDiskSizeGB == nil {
			// unsetting the field is not allowed
			updateErrs = append(updateErrs,
				field.Invalid(
					field.NewPath("Spec", "OSDiskSizeGB"),
					r.OSDiskSizeGB,
					"field is immutable, unsetting is not allowed"))
		} else if *r.OSDiskSizeGB != *old.OSDiskSizeGB {
			// changing the field is not allowed
			updateErrs = append(updateErrs,
				field.Invalid(
					field.NewPath("Spec", "OSDiskSizeGB"),
					*r.OSDiskSizeGB,
					"field is immutable"))
		}
	}
	if !reflect.DeepEqual(r.NodeTaints, old.NodeTaints) {
		updateErrs = append(updateErrs,
			field.Invalid(
				field.NewPath("Spec", "NodeTaints"),
				r.NodeTaints,
				"field is immutable"))
	}

	if old.VnetSubnetID == nil && r.VnetSubnetID != nil {
		updateErrs = append(updateErrs,
			field.Invalid(
				field.NewPath("Spec", "VnetSubnetID"),
				r.VnetSubnetID,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.VnetSubnetID != nil {
		if r.VnetSubnetID == nil {
			updateErrs = append(updateErrs,
				field.Invalid(
					field.NewPath("Spec", "VnetSubnetID"),
					r.VnetSubnetID,
					"field is immutable, unsetting is not allowed"))
		} else if *r.VnetSubnetID != *old.VnetSubnetID {
			updateErrs = append(updateErrs,
				field.Invalid(
					field.NewPath("Spec", "VnetSubnetID"),
					r.VnetSubnetID,
					"field is immutable"))
		}
	}

	if old.MaxPods == nil && r.MaxPods != nil {
		updateErrs = append(updateErrs,
			field.Invalid(
				field.NewPath("Spec", "MaxPods"),
				r.MaxPods,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.MaxPods != nil {
		if r.MaxPods == nil {
			updateErrs = append(updateErrs,
				field.Invalid(
					field.NewPath("Spec", "MaxPods"),
					r.MaxPods,
					"field is immutable, unsetting is not allowed"))
		} else if *r.MaxPods != *old.MaxPods {
			updateErrs = append(updateErrs,
				field.Invalid(
					field.NewPath("Spec", "MaxPods"),
					r.MaxPods,
					"field is immutable"))
		}
	}

	return updateErrs
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) ValidateUpdate(oldRaw runtime.Object, client client.Client) error {
	old := oldRaw.(*AzureManagedMachinePool)
	var allErrs field.ErrorList

	var updateErrs = ValidateUpdateHelper(old.Spec, r.Spec)
	allErrs = append(allErrs, updateErrs...)

	if old.Spec.EnableFIPS == nil && r.Spec.EnableFIPS != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "EnableFIPS"),
				r.Spec.EnableFIPS,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.Spec.EnableFIPS != nil {
		if r.Spec.EnableFIPS == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableFIPS"),
					r.Spec.EnableFIPS,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.EnableFIPS != *old.Spec.EnableFIPS {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableFIPS"),
					r.Spec.EnableFIPS,
					"field is immutable"))
		}
	}

	if old.Spec.EnableNodePublicIP == nil && r.Spec.EnableNodePublicIP != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "EnableNodePublicIP"),
				r.Spec.EnableNodePublicIP,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.Spec.EnableNodePublicIP != nil {
		if r.Spec.EnableNodePublicIP == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableNodePublicIP"),
					r.Spec.EnableNodePublicIP,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.EnableNodePublicIP != *old.Spec.EnableNodePublicIP {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableNodePublicIP"),
					r.Spec.EnableNodePublicIP,
					"field is immutable"))
		}
	}

	if old.Spec.OsDiskType == nil && r.Spec.OsDiskType != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "OsDiskType"),
				r.Spec.OsDiskType,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.Spec.OsDiskType != nil {
		if r.Spec.OsDiskType == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OsDiskType"),
					r.Spec.OsDiskType,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.OsDiskType != *old.Spec.OsDiskType {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OsDiskType"),
					r.Spec.OsDiskType,
					"field is immutable"))
		}
	}

	if old.Spec.ScaleSetPriority == nil && r.Spec.ScaleSetPriority != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "ScaleSetPriority"),
				r.Spec.ScaleSetPriority,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.Spec.ScaleSetPriority != nil {
		if r.Spec.ScaleSetPriority == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "ScaleSetPriority"),
					r.Spec.ScaleSetPriority,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.ScaleSetPriority != *old.Spec.ScaleSetPriority {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "ScaleSetPriority"),
					r.Spec.ScaleSetPriority,
					"field is immutable"))
		}
	}

	if !reflect.DeepEqual(r.Spec.AvailabilityZones, old.Spec.AvailabilityZones) {
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

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedMachinePool").GroupKind(), r.Name, allErrs)
	}

	return r.ValidateCreate(client)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedMachinePool) ValidateDelete(client client.Client) error {
	azuremanagedmachinepoollog.Info("validate delete", "name", r.Name)

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
