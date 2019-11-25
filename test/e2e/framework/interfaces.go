/*
Copyright 2019 The Kubernetes Authors.

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

package framework

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ComponentGenerator is used to install components, generally any YAML bundle.
type ComponentGenerator interface {
	// GetName returns the name of the component.
	GetName() string
	// Manifests return the YAML bundle.
	Manifests(context.Context) ([]byte, error)
}

// Applier is an interface around applying YAML to a cluster
type Applier interface {
	// Apply allows us to apply YAML to the cluster, `kubectl apply`
	Apply(context.Context, []byte) error
}

// Waiter is an interface around waiting for something on a kubernetes cluster.
type Waiter interface {
	// Wait allows us to wait for something in the cluster, `kubectl wait`
	Wait(context.Context, ...string) error
}

// ManagementCluster are all the features we need out of a kubernetes cluster to qualify as a management cluster.
type ManagementCluster interface {
	Applier
	Waiter
	// Teardown will completely clean up the ManagementCluster.
	// This should be implemented as a synchronous function.
	// Generally to be used in the AfterSuite function if a management cluster is shared between tests.
	Teardown(context.Context) error
	// GetName returns the name of the cluster.
	GetName() string
	// GetClient returns a client to the Management cluster.
	GetClient() (client.Client, error)
	// GetWorkdloadClient returns a client to the specified workload cluster.
	GetWorkloadClient(ctx context.Context, namespace, name string) (client.Client, error)
}
