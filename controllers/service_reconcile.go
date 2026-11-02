/*
Copyright 2026 The Kubernetes Authors.

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

package controllers

import (
	"context"

	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/aso"
)

// reconcileServices runs services in order. It remembers a namespace credential
// subscription mismatch and continues, allowing later services to restore
// credential-from on their ASO resources. If no other error occurs, it returns
// the mismatch after the pass. Any other error stops the loop.
func reconcileServices(ctx context.Context, services []azure.ServiceReconciler, ownerKind string) error {
	var mismatch error
	for _, service := range services {
		err := service.Reconcile(ctx)
		if err == nil {
			continue
		}
		if aso.IsSubscriptionMismatch(err) {
			if mismatch == nil {
				mismatch = err
			}
			continue
		}
		return errors.Wrapf(err, "failed to reconcile %s service %s", ownerKind, service.Name())
	}
	if mismatch != nil {
		return errors.Wrapf(mismatch, "failed to reconcile %s services", ownerKind)
	}
	return nil
}
