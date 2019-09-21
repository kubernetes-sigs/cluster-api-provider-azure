/*
Copyright 2018 The Kubernetes Authors.

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

package azure

import (
	"context"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
)

// Service is a generic interface used by components offering a type of service.
// Example: virtualnetworks service would offer Reconcile/Delete methods.
type Service interface {
	Reconcile(ctx context.Context, spec infrav1.ResourceSpec) error
	Delete(ctx context.Context, spec infrav1.ResourceSpec) error
}

// GetterService is a temporary interface used by components which still require Get methods.
// Once all components move to storing provider information within the relevant
// Cluster/Machine specs, this interface should be removed.
type GetterService interface {
	Get(ctx context.Context, spec infrav1.ResourceSpec) (interface{}, error)
	Reconcile(ctx context.Context, spec infrav1.ResourceSpec) error
	Delete(ctx context.Context, spec infrav1.ResourceSpec) error
}
