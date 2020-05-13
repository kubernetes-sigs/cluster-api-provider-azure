/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var clusterlog = logf.Log.WithName("azurecluster-resource")

func (c *AzureCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha3-azurecluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azurecluster,versions=v1alpha3,name=validation.azurecluster.infrastructure.cluster.x-k8s.io,sideEffects=None

var _ webhook.Validator = &AzureCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *AzureCluster) ValidateCreate() error {
	clusterlog.Info("validate create", "name", c.Name)

	return c.validateCluster()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *AzureCluster) ValidateUpdate(old runtime.Object) error {
	clusterlog.Info("validate update", "name", c.Name)

	return c.validateCluster()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *AzureCluster) ValidateDelete() error {
	clusterlog.Info("validate delete", "name", c.Name)

	return nil
}
