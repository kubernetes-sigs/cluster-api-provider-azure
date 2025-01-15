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
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/blang/semver"
	"github.com/pkg/errors"
	_ "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	capifeature "sigs.k8s.io/cluster-api/feature"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
)

// SetupAzureMachinePoolWebhookWithManager sets up and registers the webhooks with the manager.
func SetupAzureMachinePoolWebhookWithManager(mgr ctrl.Manager) error {
	// webhook handlers
	mutatingWebhook := &azureMachinePoolMutatingWebhook{
		Client: mgr.GetClient(),
	}
	validatingWebhook := &azureMachinePoolValidatingWebhook{
		Client: mgr.GetClient(),
	}

	// admission decoders
	dec := admission.NewDecoder(mgr.GetScheme())
	mutatingWebhook.decoder = &dec
	validatingWebhook.decoder = &dec

	// register webhooks
	server := mgr.GetWebhookServer()
	server.Register(
		"/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremachinepool",
		&admission.Webhook{Handler: mutatingWebhook},
	)
	server.Register(
		"/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremachinepool",
		&admission.Webhook{Handler: validatingWebhook},
	)

	// Return nil if everything is successful
	return nil
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremachinepool,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepools,verbs=create;update,versions=v1beta1,name=default.azuremachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

type azureMachinePoolMutatingWebhook struct {
	Client  client.Client
	decoder *admission.Decoder
}

// Handle implements admission.Handler so the controller-runtime can call this
// on CREATE or UPDATE for AzureMachinePools.
func (w *azureMachinePoolMutatingWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	// Decode the incoming object into an AzureMachinePool
	amp := &AzureMachinePool{}

	admissionDecoder := *w.decoder
	if err := admissionDecoder.Decode(req, amp); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	fmt.Println("In Custom Mutating Webhook")
	fmt.Println("req.DryRun: ", *req.DryRun)

	// if dry run is enabled, skip the defaulting logic
	if req.DryRun != nil && *req.DryRun {
		return admission.Allowed("dry run")
	}

	switch req.Operation {
	case admissionv1.Create, admissionv1.Update:
		// Apply defaulting logic (similar to your old amp.SetDefaults(ampw.Client))
		if err := amp.SetDefaults(w.Client); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// Return a patch containing the defaulted fields
		marshaled, err := json.Marshal(amp)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)

	default:
		// For safety, if we get e.g. Delete or Connect, we just allow
		return admission.Allowed(fmt.Sprintf("operation %s not handled by mutating webhook", req.Operation))
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremachinepool,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepools,versions=v1beta1,name=validation.azuremachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

type azureMachinePoolValidatingWebhook struct {
	Client  client.Client
	decoder *admission.Decoder
}

// Handle implements admission.Handler so the controller-runtime can call this
// for CREATE, UPDATE, and potentially DELETE (if you configure `verbs=delete` too).
func (w *azureMachinePoolValidatingWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	fmt.Println("In Custom Mutating Webhook")
	fmt.Println("req.DryRun: ", *req.DryRun)
	// if dry run is enabled, skip the defaulting logic
	if req.DryRun != nil && *req.DryRun {
		return admission.Allowed("dry run")
	}

	switch req.Operation {
	case admissionv1.Create:
		return w.handleCreate(ctx, req)
	case admissionv1.Update:
		return w.handleUpdate(ctx, req)
	case admissionv1.Delete:
		return w.handleDelete(ctx, req)
	default:
		return admission.Allowed(fmt.Sprintf("operation %s not explicitly handled", req.Operation))
	}
}

// handleCreate handles Create validation
func (w *azureMachinePoolValidatingWebhook) handleCreate(ctx context.Context, req admission.Request) admission.Response {
	amp := &AzureMachinePool{}
	admissionDecoder := *w.decoder
	if err := admissionDecoder.Decode(req, amp); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// NOTE: AzureMachinePool is behind MachinePool feature gate flag; the webhook
	// must prevent creating new objects in case the feature flag is disabled.
	if !feature.Gates.Enabled(capifeature.MachinePool) {
		return admission.Denied("AzureMachinePool creation is disallowed if the MachinePool feature flag is disabled")
	}

	// Perform validations on the AzureMachinePool object
	if err := amp.Validate(nil, w.Client); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("create is valid")
}

// handleUpdate handles Update validation
func (w *azureMachinePoolValidatingWebhook) handleUpdate(ctx context.Context, req admission.Request) admission.Response {
	newAmp := &AzureMachinePool{}
	admissionDecoder := *w.decoder
	if err := admissionDecoder.Decode(req, newAmp); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Decode the old object from req.OldObject
	oldAmp := &AzureMachinePool{}
	if len(req.OldObject.Raw) > 0 {
		if err := json.Unmarshal(req.OldObject.Raw, oldAmp); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	if err := newAmp.Validate(oldAmp, w.Client); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("update is valid")
}

// handleDelete handles Delete validation
func (w *azureMachinePoolValidatingWebhook) handleDelete(ctx context.Context, req admission.Request) admission.Response {
	// For DELETE, the object is typically in req.OldObject (not req.Object)
	oldAmp := &AzureMachinePool{}
	if len(req.OldObject.Raw) > 0 {
		if err := json.Unmarshal(req.OldObject.Raw, oldAmp); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	// AzureMachinePool deletion is always allowed
	// if err := oldAmp.ValidateDelete(); err != nil {
	// 	return admission.Denied(err.Error())
	// }

	return admission.Allowed("delete is valid")
}

// Validate the Azure Machine Pool and return an aggregate error.
func (amp *AzureMachinePool) Validate(old runtime.Object, client client.Client) error {
	validators := []func() error{
		amp.ValidateImage,
		amp.ValidateTerminateNotificationTimeout,
		amp.ValidateSSHKey,
		amp.ValidateUserAssignedIdentity,
		amp.ValidateDiagnostics,
		amp.ValidateOrchestrationMode(client),
		amp.ValidateStrategy(),
		amp.ValidateSystemAssignedIdentity(old),
		amp.ValidateSystemAssignedIdentityRole,
		amp.ValidateNetwork,
		amp.ValidateOSDisk,
	}

	var errs []error
	for _, validator := range validators {
		if err := validator(); err != nil {
			errs = append(errs, err)
		}
	}

	return kerrors.NewAggregate(errs)
}

// ValidateNetwork of an AzureMachinePool.
func (amp *AzureMachinePool) ValidateNetwork() error {
	if (amp.Spec.Template.NetworkInterfaces != nil) && len(amp.Spec.Template.NetworkInterfaces) > 0 && amp.Spec.Template.SubnetName != "" {
		return errors.New("cannot set both NetworkInterfaces and machine SubnetName")
	}
	return nil
}

// ValidateOSDisk of an AzureMachinePool.
func (amp *AzureMachinePool) ValidateOSDisk() error {
	if errs := infrav1.ValidateOSDisk(amp.Spec.Template.OSDisk, field.NewPath("osDisk")); len(errs) > 0 {
		return errs.ToAggregate()
	}
	return nil
}

// ValidateImage of an AzureMachinePool.
func (amp *AzureMachinePool) ValidateImage() error {
	if amp.Spec.Template.Image != nil {
		image := amp.Spec.Template.Image
		if errs := infrav1.ValidateImage(image, field.NewPath("image")); len(errs) > 0 {
			return errs.ToAggregate()
		}
	}

	return nil
}

// ValidateTerminateNotificationTimeout termination notification timeout to be between 5 and 15.
func (amp *AzureMachinePool) ValidateTerminateNotificationTimeout() error {
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

// ValidateSSHKey validates an SSHKey.
func (amp *AzureMachinePool) ValidateSSHKey() error {
	if amp.Spec.Template.SSHPublicKey != "" {
		sshKey := amp.Spec.Template.SSHPublicKey
		if errs := infrav1.ValidateSSHKey(sshKey, field.NewPath("sshKey")); len(errs) > 0 {
			agg := kerrors.NewAggregate(errs.ToAggregate().Errors())
			return agg
		}
	}

	return nil
}

// ValidateUserAssignedIdentity validates the user-assigned identities list.
func (amp *AzureMachinePool) ValidateUserAssignedIdentity() error {
	fldPath := field.NewPath("userAssignedIdentities")
	if errs := infrav1.ValidateUserAssignedIdentity(amp.Spec.Identity, amp.Spec.UserAssignedIdentities, fldPath); len(errs) > 0 {
		return kerrors.NewAggregate(errs.ToAggregate().Errors())
	}

	return nil
}

// ValidateStrategy validates the strategy.
func (amp *AzureMachinePool) ValidateStrategy() func() error {
	return func() error {
		if amp.Spec.Strategy.Type == RollingUpdateAzureMachinePoolDeploymentStrategyType && amp.Spec.Strategy.RollingUpdate != nil {
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

// ValidateSystemAssignedIdentity validates system-assigned identity role.
func (amp *AzureMachinePool) ValidateSystemAssignedIdentity(old runtime.Object) func() error {
	return func() error {
		var oldRole string
		if old != nil {
			oldMachinePool, ok := old.(*AzureMachinePool)
			if !ok {
				return fmt.Errorf("unexpected type for old azure machine pool object. Expected: %q, Got: %q",
					"AzureMachinePool", reflect.TypeOf(old))
			}
			if amp.Spec.SystemAssignedIdentityRole != nil {
				oldRole = oldMachinePool.Spec.SystemAssignedIdentityRole.Name
			}
		}

		roleAssignmentName := ""
		if amp.Spec.SystemAssignedIdentityRole != nil {
			roleAssignmentName = amp.Spec.SystemAssignedIdentityRole.Name
		}

		fldPath := field.NewPath("roleAssignmentName")
		if errs := infrav1.ValidateSystemAssignedIdentity(amp.Spec.Identity, oldRole, roleAssignmentName, fldPath); len(errs) > 0 {
			return kerrors.NewAggregate(errs.ToAggregate().Errors())
		}

		return nil
	}
}

// ValidateSystemAssignedIdentityRole validates the scope and roleDefinitionID for the system-assigned identity.
func (amp *AzureMachinePool) ValidateSystemAssignedIdentityRole() error {
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

// ValidateDiagnostics validates the Diagnostic spec.
func (amp *AzureMachinePool) ValidateDiagnostics() error {
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

// ValidateOrchestrationMode validates requirements for the VMSS orchestration mode.
func (amp *AzureMachinePool) ValidateOrchestrationMode(c client.Client) func() error {
	return func() error {
		// Only Flexible orchestration mode requires validation.
		if amp.Spec.OrchestrationMode == infrav1.OrchestrationModeType(armcompute.OrchestrationModeFlexible) {
			parent, err := azureutil.FindParentMachinePoolWithRetry(amp.Name, c, 5)
			if err != nil {
				return errors.Wrap(err, "failed to find parent MachinePool")
			}
			// Kubernetes must be >= 1.26.0 for cloud-provider-azure Helm chart support.
			if parent.Spec.Template.Spec.Version == nil {
				return errors.New("could not find Kubernetes version in MachinePool")
			}
			k8sVersion, err := semver.ParseTolerant(*parent.Spec.Template.Spec.Version)
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
