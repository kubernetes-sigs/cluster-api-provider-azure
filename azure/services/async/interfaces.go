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

package async

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

// FutureScope is a scope that can perform store futures and conditions in Status.
type FutureScope interface {
	azure.AsyncStatusUpdater
}

// FutureHandler is a client that can check on the progress of a future.
type FutureHandler interface {
	// IsDone returns true if the operation is complete.
	IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error)
	// Result returns the result of the operation.
	Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error)
}

// Getter is an interface that can get a resource.
type Getter interface {
	Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error)
}

// TagsGetter is an interface that can get a tags resource.
type TagsGetter interface {
	GetAtScope(ctx context.Context, scope string) (result armresources.TagsResource, err error)
}

// Creator is a client that can create or update a resource asynchronously.
type Creator interface {
	FutureHandler
	Getter
	CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error)
}

// Deleter is a client that can delete a resource asynchronously.
type Deleter interface {
	FutureHandler
	DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error)
}

// Reconciler is a generic interface used to perform asynchronous reconciliation of Azure resources.
type Reconciler interface {
	CreateOrUpdateResource(ctx context.Context, spec azure.ResourceSpecGetter, serviceName string) (result interface{}, err error)
	DeleteResource(ctx context.Context, spec azure.ResourceSpecGetter, serviceName string) (err error)
}
