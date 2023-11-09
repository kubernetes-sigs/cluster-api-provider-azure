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
	"regexp"

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
func (m *AzureManagedMachinePool) Default(client client.Client) {
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[LabelAgentPoolMode] = m.Spec.Mode

	if m.Spec.Name == nil || *m.Spec.Name == "" {
		m.Spec.Name = &m.Name
	}
}

//+kubebuilder:webhook:verbs=update;delete,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedmachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedmachinepools,versions=v1beta1,name=validation.azuremanagedmachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *AzureManagedMachinePool) ValidateCreate(client client.Client) error {
	validators := []func() *field.Error{
		m.validateMaxPods,
		m.validateKubeletConfig,
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
func (m *AzureManagedMachinePool) ValidateUpdate(oldRaw runtime.Object, client client.Client) error {
	old := oldRaw.(*AzureManagedMachinePool)
	var allErrs field.ErrorList

	if m.Spec.SKU != old.Spec.SKU {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "SKU"),
				m.Spec.SKU,
				"field is immutable"))
	}

	if old.Spec.OSDiskSizeGB != nil {
		// Prevent OSDiskSizeGB modification if it was already set to some value
		if m.Spec.OSDiskSizeGB == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OSDiskSizeGB"),
					m.Spec.OSDiskSizeGB,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.OSDiskSizeGB != *old.Spec.OSDiskSizeGB {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OSDiskSizeGB"),
					*m.Spec.OSDiskSizeGB,
					"field is immutable"))
		}
	}

	if !reflect.DeepEqual(m.Spec.Taints, old.Spec.Taints) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "Taints"),
				m.Spec.Taints,
				"field is immutable"))
	}

	// custom headers are immutable
	oldCustomHeaders := maps.FilterByKeyPrefix(old.ObjectMeta.Annotations, azure.CustomHeaderPrefix)
	newCustomHeaders := maps.FilterByKeyPrefix(m.ObjectMeta.Annotations, azure.CustomHeaderPrefix)
	if !reflect.DeepEqual(oldCustomHeaders, newCustomHeaders) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("metadata", "annotations"),
				m.ObjectMeta.Annotations,
				fmt.Sprintf("annotations with '%s' prefix are immutable", azure.CustomHeaderPrefix)))
	}

	if !ensureStringSlicesAreEqual(m.Spec.AvailabilityZones, old.Spec.AvailabilityZones) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "AvailabilityZones"),
				m.Spec.AvailabilityZones,
				"field is immutable"))
	}

	if m.Spec.Mode != string(NodePoolModeSystem) && old.Spec.Mode == string(NodePoolModeSystem) {
		// validate for last system node pool
		if err := m.validateLastSystemNodePool(client); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("Spec", "Mode"),
				m.Spec.Mode,
				"Last system node pool cannot be mutated to user node pool"))
		}
	}

	if old.Spec.MaxPods != nil {
		// Prevent MaxPods modification if it was already set to some value
		if m.Spec.MaxPods == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "MaxPods"),
					m.Spec.MaxPods,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.MaxPods != *old.Spec.MaxPods {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "MaxPods"),
					*m.Spec.MaxPods,
					"field is immutable"))
		}
	}

	if old.Spec.OsDiskType != nil {
		// Prevent OSDiskType modification if it was already set to some value
		if m.Spec.OsDiskType == nil || to.String(m.Spec.OsDiskType) == "" {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OsDiskType"),
					m.Spec.OsDiskType,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.OsDiskType != *old.Spec.OsDiskType {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "OsDiskType"),
					m.Spec.OsDiskType,
					"field is immutable"))
		}
	}

	if old.Spec.EnableUltraSSD != nil {
		// Prevent EnabledUltraSSD modification if it was already set to some value
		if m.Spec.EnableUltraSSD == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableUltraSSD"),
					m.Spec.EnableUltraSSD,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.EnableUltraSSD != *old.Spec.EnableUltraSSD {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableUltraSSD"),
					m.Spec.EnableUltraSSD,
					"field is immutable"))
		}
	} else {
		if m.Spec.EnableUltraSSD != nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableUltraSSD"),
					m.Spec.EnableUltraSSD,
					"field is immutable, unsetting is not allowed"))
		}
	}

	if old.Spec.VnetSubnetID == nil && m.Spec.VnetSubnetID != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "VnetSubnetID"),
				m.Spec.VnetSubnetID,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.Spec.VnetSubnetID != nil {
		if m.Spec.VnetSubnetID == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "VnetSubnetID"),
					m.Spec.VnetSubnetID,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.VnetSubnetID != *old.Spec.VnetSubnetID {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "VnetSubnetID"),
					m.Spec.VnetSubnetID,
					"field is immutable"))
		}
	}

	if old.Spec.EnableFIPS == nil && m.Spec.EnableFIPS != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "EnableFIPS"),
				m.Spec.EnableFIPS,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.Spec.EnableFIPS != nil {
		if m.Spec.EnableFIPS == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableFIPS"),
					m.Spec.EnableFIPS,
					"field is immutable, unsetting is not allowed"))
			// changing the field is not allowed
		} else if *m.Spec.EnableFIPS != *old.Spec.EnableFIPS {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableFIPS"),
					m.Spec.EnableFIPS,
					"field is immutable"))
		}
	}

	if old.Spec.EnableNodePublicIP == nil && m.Spec.EnableNodePublicIP != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "EnableNodePublicIP"),
				m.Spec.EnableNodePublicIP,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.Spec.EnableNodePublicIP != nil {
		if m.Spec.EnableNodePublicIP == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableNodePublicIP"),
					m.Spec.EnableNodePublicIP,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.EnableNodePublicIP != *old.Spec.EnableNodePublicIP {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableNodePublicIP"),
					m.Spec.EnableNodePublicIP,
					"field is immutable"))
		}
	}

	if old.Spec.ScaleSetPriority == nil && m.Spec.ScaleSetPriority != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "ScaleSetPriority"),
				m.Spec.ScaleSetPriority,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.Spec.ScaleSetPriority != nil {
		if m.Spec.ScaleSetPriority == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "ScaleSetPriority"),
					m.Spec.ScaleSetPriority,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.ScaleSetPriority != *old.Spec.ScaleSetPriority {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "ScaleSetPriority"),
					m.Spec.ScaleSetPriority,
					"field is immutable"))
		}
	}

	if old.Spec.EnableEncryptionAtHost == nil && m.Spec.EnableEncryptionAtHost != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "EnableEncryptionAtHost"),
				m.Spec.EnableEncryptionAtHost,
				"field is immutable, setting after creation is not allowed"))
	}

	if old.Spec.EnableEncryptionAtHost != nil {
		if m.Spec.EnableEncryptionAtHost == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableEncryptionAtHost"),
					m.Spec.EnableEncryptionAtHost,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.EnableEncryptionAtHost != *old.Spec.EnableEncryptionAtHost {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "EnableEncryptionAtHost"),
					m.Spec.EnableEncryptionAtHost,
					"field is immutable"))
		}
	}

	if kubeletConfigErr := m.validateKubeletConfig(); kubeletConfigErr != nil {
		allErrs = append(allErrs, kubeletConfigErr)
	}
	if len(allErrs) != 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedMachinePool").GroupKind(), m.Name, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *AzureManagedMachinePool) ValidateDelete(client client.Client) error {
	if m.Spec.Mode != string(NodePoolModeSystem) {
		return nil
	}

	return errors.Wrapf(m.validateLastSystemNodePool(client), "if the delete is triggered via owner MachinePool please refer to trouble shooting section in https://capz.sigs.k8s.io/topics/managedcluster.html")
}

// validateLastSystemNodePool is used to check if the existing system node pool is the last system node pool.
// If it is a last system node pool it cannot be deleted or mutated to user node pool as AKS expects min 1 system node pool.
func (m *AzureManagedMachinePool) validateLastSystemNodePool(cli client.Client) error {
	ctx := context.Background()

	// Fetch the Cluster.
	clusterName, ok := m.Labels[clusterv1.ClusterLabelName]
	if !ok {
		return nil
	}

	ownerCluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Namespace: m.Namespace,
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

	opt1 := client.InNamespace(m.Namespace)
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

func (m *AzureManagedMachinePool) validateMaxPods() *field.Error {
	if m.Spec.MaxPods != nil {
		if to.Int32(m.Spec.MaxPods) < 10 || to.Int32(m.Spec.MaxPods) > 250 {
			return field.Invalid(
				field.NewPath("Spec", "MaxPods"),
				m.Spec.MaxPods,
				"MaxPods must be between 10 and 250")
		}
	}

	return nil
}

func (m *AzureManagedMachinePool) validateKubeletConfig() *field.Error {
	// AllowedUnsafeSysctls should be one of "kernel.shm*", "kernel.msg*", "kernel.sem", "fs.mqueue.*", "net.*".
	allowedUnsafeSysctlPatterns := []string{"kernel.shm*", "kernel.msg*", "kernel.sem", "fs.mqueue.*", "net.*"}
	if m.Spec.KubeletConfig != nil && m.Spec.KubeletConfig.AllowedUnsafeSysctls != nil && len(*m.Spec.KubeletConfig.AllowedUnsafeSysctls) > 0 {
		for _, v := range *m.Spec.KubeletConfig.AllowedUnsafeSysctls {
			foundMatch := false
			for _, pattern := range allowedUnsafeSysctlPatterns {
				match, _ := regexp.MatchString(pattern, v)
				if match {
					foundMatch = true
					break
				}
			}
			if !foundMatch {
				return field.Invalid(
					field.NewPath("Spec", "KubeletConfig"),
					m.Spec.KubeletConfig,
					"AllowedUnsafeSysctls are \"kernel.shm*\", \"kernel.msg*\", \"kernel.sem\", \"fs.mqueue.*\", \"net.*\"")
			}
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
