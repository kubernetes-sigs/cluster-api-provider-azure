package v1alpha3

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var azuremachinepoolmachinelog = logf.Log.WithName("azuremachinepoolmachine-resource")

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (ampm *AzureMachinePoolMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(ampm).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-exp-infrastructure-cluster-x-k8s-io-v1alpha3-azuremachinepoolmachine,mutating=false,failurePolicy=fail,groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremachinepoolmachines,versions=v1alpha3,name=azuremachinepoolmachine.kb.io,sideEffects=None

var _ webhook.Validator = &AzureMachinePoolMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (ampm *AzureMachinePoolMachine) ValidateCreate() error {
	azuremachinepoolmachinelog.Info("validate create", "name", ampm.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (ampm *AzureMachinePoolMachine) ValidateUpdate(old runtime.Object) error {
	azuremachinepoolmachinelog.Info("validate update", "name", ampm.Name)
	oldMachine, ok := old.(*AzureMachinePoolMachine)
	if !ok {
		return errors.New("expected and AzureMachinePoolMachine")
	}

	if oldMachine.Spec.ProviderID != "" && ampm.Spec.ProviderID != oldMachine.Spec.ProviderID {
		return errors.New("providerID is immutable")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (ampm *AzureMachinePoolMachine) ValidateDelete() error {
	azuremachinepoolmachinelog.Info("validate delete", "name", ampm.Name)
	return nil
}


