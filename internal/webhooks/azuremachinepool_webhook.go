/*
Copyright The Kubernetes Authors.

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
	"encoding/base64"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/blang/semver"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	apiinternal "sigs.k8s.io/cluster-api-provider-azure/internal/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	utilSSH "sigs.k8s.io/cluster-api-provider-azure/util/ssh"
)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (ampw *AzureMachinePoolWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1exp.AzureMachinePool{}).
		WithDefaulter(ampw).
		WithValidator(ampw).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-azuremachinepool,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepools,verbs=create;update,versions=v1beta2,name=default.azuremachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-azuremachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepools,versions=v1beta2,name=validation.azuremachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// AzureMachinePoolWebhook implements a validating and defaulting webhook for AzureMachinePool.
type AzureMachinePoolWebhook struct {
	Client client.Client
}

var _ admission.Validator[*infrav1exp.AzureMachinePool] = &AzureMachinePoolWebhook{}
var _ admission.Defaulter[*infrav1exp.AzureMachinePool] = &AzureMachinePoolWebhook{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (ampw *AzureMachinePoolWebhook) Default(_ context.Context, amp *infrav1exp.AzureMachinePool) error {
	return setDefaultsAzureMachinePool(amp, ampw.Client)
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (ampw *AzureMachinePoolWebhook) ValidateCreate(_ context.Context, amp *infrav1exp.AzureMachinePool) (admission.Warnings, error) {
	return nil, validateAzureMachinePool(amp, nil, ampw.Client)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (ampw *AzureMachinePoolWebhook) ValidateUpdate(_ context.Context, oldObj, amp *infrav1exp.AzureMachinePool) (admission.Warnings, error) {
	return nil, validateAzureMachinePool(amp, oldObj, ampw.Client)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (ampw *AzureMachinePoolWebhook) ValidateDelete(_ context.Context, _ *infrav1exp.AzureMachinePool) (admission.Warnings, error) {
	return nil, nil
}

func setDefaultsAzureMachinePool(amp *infrav1exp.AzureMachinePool, c client.Client) error {
	var errs []error
	if err := setDefaultAzureMachinePoolSSHPublicKey(amp); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to set default SSH public key"))
	}

	if err := setDefaultAzureMachinePoolIdentity(amp, c); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to set default managed identity defaults"))
	}
	setDefaultAzureMachinePoolDiagnostics(amp)
	amp.SetNetworkInterfacesDefaults()
	setDefaultAzureMachinePoolOSDisk(amp)

	return kerrors.NewAggregate(errs)
}

func setDefaultAzureMachinePoolSSHPublicKey(amp *infrav1exp.AzureMachinePool) error {
	if sshKeyData := amp.Spec.Template.SSHPublicKey; sshKeyData == "" {
		_, publicRsaKey, err := utilSSH.GenerateSSHKey()
		if err != nil {
			return err
		}

		amp.Spec.Template.SSHPublicKey = base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}
	return nil
}

func setDefaultAzureMachinePoolIdentity(amp *infrav1exp.AzureMachinePool, c client.Client) error {
	if amp.Spec.RoleAssignmentName != "" && amp.Spec.SystemAssignedIdentityRole != nil && amp.Spec.SystemAssignedIdentityRole.Name != "" {
		return nil
	}
	if amp.Spec.Identity == infrav1.VMIdentitySystemAssigned {
		machinePool, err := azureutil.FindParentMachinePoolWithRetry(amp.Name, c, 5)
		if err != nil {
			return errors.Wrap(err, "failed to find parent machine pool")
		}

		ownerAzureClusterName, ownerAzureClusterNamespace, err := apiinternal.GetOwnerAzureClusterNameAndNamespace(c, machinePool.Spec.ClusterName, machinePool.Namespace, 5)
		if err != nil {
			return errors.Wrap(err, "failed to get owner cluster")
		}

		subscriptionID, err := apiinternal.GetSubscriptionID(c, ownerAzureClusterName, ownerAzureClusterNamespace, 5)
		if err != nil {
			return errors.Wrap(err, "failed to get subscription ID")
		}

		if amp.Spec.SystemAssignedIdentityRole == nil {
			amp.Spec.SystemAssignedIdentityRole = &infrav1.SystemAssignedIdentityRole{}
		}
		if amp.Spec.RoleAssignmentName != "" {
			amp.Spec.SystemAssignedIdentityRole.Name = amp.Spec.RoleAssignmentName
			amp.Spec.RoleAssignmentName = ""
		} else if amp.Spec.SystemAssignedIdentityRole.Name == "" {
			amp.Spec.SystemAssignedIdentityRole.Name = string(uuid.NewUUID())
		}
		if amp.Spec.SystemAssignedIdentityRole.Scope == "" {
			amp.Spec.SystemAssignedIdentityRole.Scope = fmt.Sprintf("/subscriptions/%s/", subscriptionID)
		}
		if amp.Spec.SystemAssignedIdentityRole.DefinitionID == "" {
			amp.Spec.SystemAssignedIdentityRole.DefinitionID = fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", subscriptionID, apiinternal.ContributorRoleID)
		}
	}
	return nil
}

func setDefaultAzureMachinePoolDiagnostics(amp *infrav1exp.AzureMachinePool) {
	bootDefault := &infrav1.BootDiagnostics{
		StorageAccountType: infrav1.ManagedDiagnosticsStorage,
	}

	if amp.Spec.Template.Diagnostics == nil {
		amp.Spec.Template.Diagnostics = &infrav1.Diagnostics{
			Boot: bootDefault,
		}
	}

	if amp.Spec.Template.Diagnostics.Boot == nil {
		amp.Spec.Template.Diagnostics.Boot = bootDefault
	}
}

func setDefaultAzureMachinePoolOSDisk(amp *infrav1exp.AzureMachinePool) {
	if amp.Spec.Template.OSDisk.OSType == "" {
		amp.Spec.Template.OSDisk.OSType = "Linux"
	}
	if amp.Spec.Template.OSDisk.CachingType == "" {
		amp.Spec.Template.OSDisk.CachingType = "None"
	}
}

func validateAzureMachinePool(amp *infrav1exp.AzureMachinePool, old *infrav1exp.AzureMachinePool, c client.Client) error {
	validators := []func() error{
		validateAzureMachinePoolImage(amp),
		validateAzureMachinePoolTerminateNotificationTimeout(amp),
		validateAzureMachinePoolSSHKey(amp),
		validateAzureMachinePoolUserAssignedIdentity(amp),
		validateAzureMachinePoolDiagnostics(amp),
		validateAzureMachinePoolOrchestrationMode(amp, c),
		validateAzureMachinePoolStrategy(amp),
		validateAzureMachinePoolSystemAssignedIdentity(amp, old),
		validateAzureMachinePoolSystemAssignedIdentityRole(amp),
		validateAzureMachinePoolNetwork(amp),
		validateAzureMachinePoolOSDisk(amp),
	}

	var errs []error
	for _, validator := range validators {
		if err := validator(); err != nil {
			errs = append(errs, err)
		}
	}

	return kerrors.NewAggregate(errs)
}

func validateAzureMachinePoolNetwork(amp *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		if len(amp.Spec.Template.NetworkInterfaces) > 0 && amp.Spec.Template.SubnetName != "" {
			return errors.New("cannot set both NetworkInterfaces and machine SubnetName")
		}
		return nil
	}
}

func validateAzureMachinePoolOSDisk(amp *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		if errs := ValidateOSDisk(amp.Spec.Template.OSDisk, field.NewPath("osDisk")); len(errs) > 0 {
			return errs.ToAggregate()
		}
		return nil
	}
}

func validateAzureMachinePoolImage(amp *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		if amp.Spec.Template.Image != nil {
			if errs := ValidateImage(amp.Spec.Template.Image, field.NewPath("image")); len(errs) > 0 {
				return errs.ToAggregate()
			}
		}
		return nil
	}
}

func validateAzureMachinePoolTerminateNotificationTimeout(amp *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		if amp.Spec.Template.TerminateNotificationTimeout == nil {
			return nil
		}
		if *amp.Spec.Template.TerminateNotificationTimeout < 5 {
			return errors.New("minimum timeout 5 is allowed for TerminateNotificationTimeout")
		}
		if *amp.Spec.Template.TerminateNotificationTimeout > 15 {
			return errors.New("maximum timeout 15 is allowed for TerminateNotificationTimeout")
		}
		return nil
	}
}

func validateAzureMachinePoolSSHKey(amp *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		if amp.Spec.Template.SSHPublicKey != "" {
			if errs := ValidateSSHKey(amp.Spec.Template.SSHPublicKey, field.NewPath("sshKey")); len(errs) > 0 {
				return kerrors.NewAggregate(errs.ToAggregate().Errors())
			}
		}
		return nil
	}
}

func validateAzureMachinePoolUserAssignedIdentity(amp *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		fldPath := field.NewPath("userAssignedIdentities")
		if errs := ValidateUserAssignedIdentity(amp.Spec.Identity, amp.Spec.UserAssignedIdentities, fldPath); len(errs) > 0 {
			return kerrors.NewAggregate(errs.ToAggregate().Errors())
		}
		return nil
	}
}

func validateAzureMachinePoolStrategy(amp *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		if amp.Spec.Strategy.Type == infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType && amp.Spec.Strategy.RollingUpdate != nil {
			rollingUpdateStrategy := amp.Spec.Strategy.RollingUpdate
			maxSurge := rollingUpdateStrategy.MaxSurge
			maxUnavailable := rollingUpdateStrategy.MaxUnavailable
			if maxSurge.Type == intstr.Int && maxSurge.IntVal == 0 &&
				maxUnavailable.Type == intstr.Int && maxUnavailable.IntVal == 0 {
				return errors.New("rolling update strategy MaxUnavailable must not be 0 if MaxSurge is 0")
			}
		}
		return nil
	}
}

func validateAzureMachinePoolSystemAssignedIdentity(amp *infrav1exp.AzureMachinePool, old *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		var oldRole string
		if old != nil {
			if amp.Spec.SystemAssignedIdentityRole != nil {
				oldRole = old.Spec.SystemAssignedIdentityRole.Name
			}
		}

		roleAssignmentName := ""
		if amp.Spec.SystemAssignedIdentityRole != nil {
			roleAssignmentName = amp.Spec.SystemAssignedIdentityRole.Name
		}

		fldPath := field.NewPath("roleAssignmentName")
		if errs := ValidateSystemAssignedIdentity(amp.Spec.Identity, oldRole, roleAssignmentName, fldPath); len(errs) > 0 {
			return kerrors.NewAggregate(errs.ToAggregate().Errors())
		}

		return nil
	}
}

func validateAzureMachinePoolSystemAssignedIdentityRole(amp *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		var allErrs field.ErrorList
		if amp.Spec.RoleAssignmentName != "" && amp.Spec.SystemAssignedIdentityRole != nil && amp.Spec.SystemAssignedIdentityRole.Name != "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("systemAssignedIdentityRole"), amp.Spec.SystemAssignedIdentityRole.Name, "cannot set both roleAssignmentName and systemAssignedIdentityRole.name"))
		}
		if amp.Spec.Identity == infrav1.VMIdentitySystemAssigned {
			if amp.Spec.SystemAssignedIdentityRole.DefinitionID == "" {
				allErrs = append(allErrs, field.Invalid(field.NewPath("systemAssignedIdentityRole", "definitionID"), amp.Spec.SystemAssignedIdentityRole.DefinitionID, "the roleDefinitionID field cannot be empty"))
			}
			if amp.Spec.SystemAssignedIdentityRole.Scope == "" {
				allErrs = append(allErrs, field.Invalid(field.NewPath("systemAssignedIdentityRole", "scope"), amp.Spec.SystemAssignedIdentityRole.Scope, "the scope field cannot be empty"))
			}
		}
		if amp.Spec.Identity != infrav1.VMIdentitySystemAssigned && amp.Spec.SystemAssignedIdentityRole != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("systemAssignedIdentityRole"), amp.Spec.SystemAssignedIdentityRole, "systemAssignedIdentityRole can only be set when identity is set to 'SystemAssigned'"))
		}

		if len(allErrs) > 0 {
			return kerrors.NewAggregate(allErrs.ToAggregate().Errors())
		}

		return nil
	}
}

func validateAzureMachinePoolDiagnostics(amp *infrav1exp.AzureMachinePool) func() error {
	return func() error {
		var allErrs field.ErrorList
		fieldPath := field.NewPath("diagnostics")

		diagnostics := amp.Spec.Template.Diagnostics

		if diagnostics != nil && diagnostics.Boot != nil {
			switch diagnostics.Boot.StorageAccountType {
			case infrav1.UserManagedDiagnosticsStorage:
				if diagnostics.Boot.UserManaged == nil {
					allErrs = append(allErrs, field.Required(fieldPath.Child("UserManaged"),
						fmt.Sprintf("userManaged must be specified when storageAccountType is '%s'", infrav1.UserManagedDiagnosticsStorage)))
				} else if diagnostics.Boot.UserManaged.StorageAccountURI == "" {
					allErrs = append(allErrs, field.Required(fieldPath.Child("StorageAccountURI"),
						fmt.Sprintf("StorageAccountURI cannot be empty when storageAccountType is '%s'", infrav1.UserManagedDiagnosticsStorage)))
				}
			case infrav1.ManagedDiagnosticsStorage:
				if diagnostics.Boot.UserManaged != nil &&
					diagnostics.Boot.UserManaged.StorageAccountURI != "" {
					allErrs = append(allErrs, field.Invalid(fieldPath.Child("StorageAccountURI"), diagnostics.Boot.UserManaged.StorageAccountURI,
						fmt.Sprintf("StorageAccountURI cannot be set when storageAccountType is '%s'",
							infrav1.ManagedDiagnosticsStorage)))
				}
			case infrav1.DisabledDiagnosticsStorage:
				if diagnostics.Boot.UserManaged != nil &&
					diagnostics.Boot.UserManaged.StorageAccountURI != "" {
					allErrs = append(allErrs, field.Invalid(fieldPath.Child("StorageAccountURI"), diagnostics.Boot.UserManaged.StorageAccountURI,
						fmt.Sprintf("StorageAccountURI cannot be set when storageAccountType is '%s'",
							infrav1.ManagedDiagnosticsStorage)))
				}
			}
		}

		if len(allErrs) > 0 {
			return kerrors.NewAggregate(allErrs.ToAggregate().Errors())
		}

		return nil
	}
}

func validateAzureMachinePoolOrchestrationMode(amp *infrav1exp.AzureMachinePool, c client.Client) func() error {
	return func() error {
		if amp.Spec.OrchestrationMode == infrav1.OrchestrationModeType(armcompute.OrchestrationModeFlexible) {
			parent, err := azureutil.FindParentMachinePoolWithRetry(amp.Name, c, 5)
			if err != nil {
				return errors.Wrap(err, "failed to find parent MachinePool")
			}
			if parent.Spec.Template.Spec.Version == "" {
				return errors.New("could not find Kubernetes version in MachinePool")
			}
			k8sVersion, err := semver.ParseTolerant(parent.Spec.Template.Spec.Version)
			if err != nil {
				return errors.Wrap(err, "failed to parse Kubernetes version")
			}
			if k8sVersion.LT(semver.MustParse("1.26.0")) {
				return fmt.Errorf("specified Kubernetes version %s must be >= 1.26.0 for Flexible orchestration mode", k8sVersion)
			}
		}

		return nil
	}
}
