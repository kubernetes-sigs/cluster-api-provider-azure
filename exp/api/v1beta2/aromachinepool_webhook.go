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
	"fmt"
	"regexp"
	"unicode"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1alpha3 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	webhookutils "sigs.k8s.io/cluster-api-provider-azure/util/webhook"
)

var (
	ocpSemver               = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$`)
	validNodePublicPrefixID = regexp.MustCompile(`(?i)^/?subscriptions/[0-9a-f]{8}-([0-9a-f]{4}-){3}[0-9a-f]{12}/resourcegroups/[^/]+/providers/microsoft\.network/publicipprefixes/[^/]+$`) //nolint:unused // used by unused functions kept for future reference
)

// SetupAROMachinePoolWebhookWithManager sets up and registers the webhook with the manager.
func SetupAROMachinePoolWebhookWithManager(mgr ctrl.Manager) error {
	mw := &aroMachinePoolWebhook{Client: mgr.GetClient()}
	return ctrl.NewWebhookManagedBy(mgr).
		For(&AROMachinePool{}).
		WithDefaulter(mw).
		WithValidator(mw).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-aromachinepool,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools,verbs=create;update,versions=v1beta2,name=default.aromachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

// aroMachinePoolWebhook implements a validating and defaulting webhook for AROMachinePool.
type aroMachinePoolWebhook struct {
	Client client.Client
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (mw *aroMachinePoolWebhook) Default(_ context.Context, obj runtime.Object) error {
	m, ok := obj.(*AROMachinePool)
	if !ok {
		return apierrors.NewBadRequest("expected an AROMachinePool")
	}
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}

	if m.Spec.NodePoolName == "" {
		m.Spec.NodePoolName = m.Name
	}

	return nil
}

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-aromachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools,versions=v1beta2,name=validation.aromachinepools.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (mw *aroMachinePoolWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m, ok := obj.(*AROMachinePool)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AROMachinePool")
	}

	return nil, m.Validate(mw.Client)
}

// Validate the Azure Machine Pool and return an aggregate error.
func (m *AROMachinePool) Validate(_ client.Client) error {
	var errs []error

	errs = append(errs, validateOCPVersion(
		m.Spec.Version,
		field.NewPath("spec").Child("version")))

	errs = append(errs, validateNodePoolName(
		m.Spec.NodePoolName,
		field.NewPath("spec").Child("nodePoolName")))

	if m.Spec.Autoscaling != nil {
		errs = append(errs, validateMinReplicas(
			&m.Spec.Autoscaling.MinReplicas,
			field.NewPath("spec", "autoscaling", "minReplicas")))

		errs = append(errs, validateMaxReplicas(
			&m.Spec.Autoscaling.MaxReplicas, &m.Spec.Autoscaling.MinReplicas,
			field.NewPath("spec", "autoscaling", "maxReplicas")))
	}
	return kerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (mw *aroMachinePoolWebhook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	old, ok := oldObj.(*AROMachinePool)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AROMachinePool")
	}
	m, ok := newObj.(*AROMachinePool)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AROMachinePool")
	}
	var allErrs field.ErrorList

	// Based on TypeSpec models from ARO-HCP repository
	// Fields without Lifecycle.Update in @visibility decorator are immutable

	// NodePool name is identity - always immutable
	if err := webhookutils.ValidateImmutable(
		field.NewPath("spec", "nodePollName"),
		old.Spec.NodePoolName,
		m.Spec.NodePoolName); err != nil {
		allErrs = append(allErrs, err)
	}

	// Labels validation (labels are mutable per TypeSpec: @visibility(Lifecycle.Read, Lifecycle.Create, Lifecycle.Update))
	if err := validateLabels(m.Spec.Labels, field.NewPath("spec", "labels")); err != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "labels"),
				m.Spec.Labels,
				err.Error()))
	}

	// platform.osDisk: @visibility(Lifecycle.Read, Lifecycle.Create) - immutable
	if err := webhookutils.ValidateImmutable(
		field.NewPath("spec", "platform", "diskSizeGiB"),
		old.Spec.Platform.DiskSizeGiB,
		m.Spec.Platform.DiskSizeGiB); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := webhookutils.ValidateImmutable(
		field.NewPath("spec", "platform", "diskStorageAccountType"),
		old.Spec.Platform.DiskStorageAccountType,
		m.Spec.Platform.DiskStorageAccountType); err != nil {
		allErrs = append(allErrs, err)
	}

	// platform.vmSize: @visibility(Lifecycle.Read, Lifecycle.Create) - immutable
	if err := webhookutils.ValidateImmutable(
		field.NewPath("spec", "platform", "VMSize"),
		old.Spec.Platform.VMSize,
		m.Spec.Platform.VMSize); err != nil {
		allErrs = append(allErrs, err)
	}

	// platform.subnetId: @visibility(Lifecycle.Read, Lifecycle.Create) - immutable
	if err := webhookutils.ValidateImmutable(
		field.NewPath("spec", "platform", "subnet"),
		old.Spec.Platform.Subnet,
		m.Spec.Platform.Subnet); err != nil && old.Spec.Platform.Subnet != "" {
		allErrs = append(allErrs, err)
	}

	// platform.availabilityZone: @visibility(Lifecycle.Read, Lifecycle.Create) - immutable
	if err := webhookutils.ValidateImmutable(
		field.NewPath("spec", "platform", "availabilityZone"),
		old.Spec.Platform.AvailabilityZone,
		m.Spec.Platform.AvailabilityZone); err != nil {
		allErrs = append(allErrs, err)
	}

	// autoRepair: @visibility(Lifecycle.Read, Lifecycle.Create) - immutable
	if err := webhookutils.ValidateImmutable(
		field.NewPath("spec", "autoRepair"),
		old.Spec.AutoRepair,
		m.Spec.AutoRepair); err != nil {
		allErrs = append(allErrs, err)
	}

	// Note: autoScaling has @visibility(Lifecycle.Read, Lifecycle.Create, Lifecycle.Update)
	// so it is mutable and should not be validated as immutable here
	// Note: version has @visibility(Lifecycle.Read, Lifecycle.Create, Lifecycle.Update)
	// so it is mutable and should not be validated as immutable here
	// Note: replicas has @visibility(Lifecycle.Read, Lifecycle.Create, Lifecycle.Update)
	// so it is mutable and should not be validated as immutable here
	// Note: taints have @visibility(Lifecycle.Read, Lifecycle.Create, Lifecycle.Update)
	// so they are mutable and should not be validated as immutable here

	if len(allErrs) == 0 {
		return nil, m.Validate(mw.Client)
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind(AROMachinePoolKind).GroupKind(), m.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (mw *aroMachinePoolWebhook) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m, ok := obj.(*AROMachinePool)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AROMachinePool")
	}

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

	ammpList := &AROMachinePoolList{}
	if err := cli.List(ctx, ammpList, opt1, opt2); err != nil {
		return err
	}

	if len(ammpList.Items) <= 1 {
		return errors.New("ARO Cluster must have at least one system pool")
	}
	return nil
}

func validateMaxReplicas(maxReplicas *int, minReplicas *int, fldPath *field.Path) error {
	if maxReplicas != nil {
		maxReplicasMin := 2
		maxReplicasMax := 250
		if ptr.Deref(maxReplicas, 0) < maxReplicasMin || ptr.Deref(maxReplicas, 0) > maxReplicasMax {
			return field.Invalid(
				fldPath,
				maxReplicas,
				fmt.Sprintf("MaxReplicas must be between %d and %d", maxReplicasMin, maxReplicasMax))
		}
		if ptr.Deref(maxReplicas, 0) < ptr.Deref(minReplicas, 0) {
			return field.Invalid(
				fldPath,
				maxReplicas,
				fmt.Sprintf("MaxReplicas must be at least the value of MinReplicas(=%d)", ptr.Deref(minReplicas, 0)))
		}
	}

	return nil
}

func validateMinReplicas(minReplicas *int, fldPath *field.Path) error {
	if minReplicas != nil {
		minReplicasMin := 2
		minReplicasMax := 250
		if ptr.Deref(minReplicas, 0) < minReplicasMin || ptr.Deref(minReplicas, 0) > minReplicasMax {
			return field.Invalid(
				fldPath,
				minReplicas,
				fmt.Sprintf("MinReplicas must be between %d and %d", minReplicasMin, minReplicasMax))
		}
	}

	return nil
}

//nolint:unused // kept for future use
func validateMPName(mpName string, specName *string, fldPath *field.Path) error {
	var name *string
	var fieldNameMessage string
	if specName == nil || *specName == "" {
		name = &mpName
		fieldNameMessage = "when spec.name is empty, metadata.name"
	} else {
		name = specName
		fieldNameMessage = "spec.name"
	}

	if err := validateNameLength(name, fieldNameMessage, fldPath); err != nil {
		return err
	}
	return validateNamePattern(name, fieldNameMessage, fldPath)
}

//nolint:unused // kept for future use
func validateNameLength(name *string, fieldNameMessage string, fldPath *field.Path) error {
	maxNameLen := 12
	if name != nil && len(*name) > maxNameLen {
		return field.Invalid(
			fldPath,
			"Linux",
			fmt.Sprintf("For OSType Linux, %s can not be longer than %d characters.", fieldNameMessage, maxNameLen))
	}
	return nil
}

//nolint:unused // kept for future use
func validateNamePattern(name *string, fieldNameMessage string, fldPath *field.Path) error {
	if name == nil || *name == "" {
		return nil
	}

	if !unicode.IsLower(rune((*name)[0])) {
		return field.Invalid(
			fldPath,
			name,
			fmt.Sprintf("%s must begin with a lowercase letter.", fieldNameMessage))
	}

	for _, char := range *name {
		if !unicode.IsLower(char) && !unicode.IsNumber(char) {
			return field.Invalid(
				fldPath,
				name,
				fmt.Sprintf("%s may only contain lowercase alphanumeric characters.", fieldNameMessage))
		}
	}
	return nil
}

func validateLabels(nodeLabels map[string]string, fldPath *field.Path) error {
	for key := range nodeLabels {
		if azureutil.IsAzureSystemNodeLabelKey(key) {
			return field.Invalid(
				fldPath,
				key,
				fmt.Sprintf("Node pool label key must not start with %s", azureutil.AzureSystemNodeLabelPrefix))
		}
	}

	return nil
}

//nolint:unused // kept for future use
func validateNodePublicIPPrefixID(nodePublicIPPrefixID *string, fldPath *field.Path) error {
	if nodePublicIPPrefixID != nil && !validNodePublicPrefixID.MatchString(*nodePublicIPPrefixID) {
		return field.Invalid(
			fldPath,
			nodePublicIPPrefixID,
			fmt.Sprintf("resource ID must match %q", validNodePublicPrefixID.String()))
	}
	return nil
}

//nolint:unused // kept for future use
func validateEnableNodePublicIP(enableNodePublicIP *bool, nodePublicIPPrefixID *string, fldPath *field.Path) error {
	if (enableNodePublicIP == nil || !*enableNodePublicIP) &&
		nodePublicIPPrefixID != nil {
		return field.Invalid(
			fldPath,
			enableNodePublicIP,
			"must be set to true when NodePublicIPPrefixID is set")
	}
	return nil
}

//nolint:unused // kept for future use
func validateMPSubnetName(subnetName *string, fldPath *field.Path) error {
	if subnetName != nil {
		subnetRegex := "^[a-zA-Z0-9][a-zA-Z0-9._-]{0,78}[a-zA-Z0-9]$"
		regex := regexp.MustCompile(subnetRegex)
		if success := regex.MatchString(ptr.Deref(subnetName, "")); !success {
			return field.Invalid(fldPath, subnetName,
				fmt.Sprintf("name of subnet doesn't match regex %s", subnetRegex))
		}
	}
	return nil
}

// validateOCPVersion validates the Kubernetes version.
func validateOCPVersion(version string, fldPath *field.Path) error {
	if !ocpSemver.MatchString(version) {
		return field.Invalid(fldPath, version, "must be a <valid semantic version>")
	}
	return nil
}

// validateNodePoolName validates the node pool name against Azure ARO HCP requirements.
// Azure requires: ^[a-zA-Z][-a-zA-Z0-9]{1,13}[a-zA-Z0-9]$.
// This means: starts with letter, 1-13 chars of letters/numbers/hyphens, ends with letter or number.
// Total length: 3-15 characters.
func validateNodePoolName(name string, fldPath *field.Path) error {
	if name == "" {
		return field.Required(fldPath, "nodePoolName must be specified")
	}

	// Check length (3-15 characters)
	if len(name) < 3 || len(name) > 15 {
		return field.Invalid(fldPath, name, "nodePoolName must be between 3 and 15 characters long")
	}

	// Validate against Azure pattern: ^[a-zA-Z][-a-zA-Z0-9]{1,13}[a-zA-Z0-9]$
	nodePoolNamePattern := regexp.MustCompile(`^[a-zA-Z][-a-zA-Z0-9]{1,13}[a-zA-Z0-9]$`)
	if !nodePoolNamePattern.MatchString(name) {
		return field.Invalid(fldPath, name, "nodePoolName must start with a letter, contain only letters, numbers, and hyphens, and end with a letter or number (3-15 characters total)")
	}

	return nil
}
