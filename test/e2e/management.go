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

package e2e

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/framework/management/kind"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Shells out to `docker`, `kind`, `kubectl`

// ManagementCluster wraps a Cluster and has custom logic for GetWorkloadClient and setup.
// This demonstrates how to use the built-in kind management cluster with custom logic.
type ManagementCluster struct {
	*kind.Cluster
}

// NewManagementCluster creates a custom kind cluster with some necessary configuration to run CAPZ.
func NewManagementCluster(ctx context.Context, name string, scheme *runtime.Scheme, images ...string) (*ManagementCluster, error) {
	cluster, err := kind.NewCluster(ctx, name, scheme, images...)
	if err != nil {
		return nil, err
	}
	return &ManagementCluster{cluster}, nil
}

// GetWorkloadClient uses some special logic for darwin architecture due to Docker for Mac limitations.
func (c *ManagementCluster) GetWorkloadClient(ctx context.Context, namespace, name string) (client.Client, error) {
	mgmtClient, err := c.GetClient()
	if err != nil {
		return nil, err
	}
	config := &v1.Secret{}
	key := client.ObjectKey{
		Name:      fmt.Sprintf("%s-kubeconfig", name),
		Namespace: namespace,
	}
	if err := mgmtClient.Get(ctx, key, config); err != nil {
		return nil, err
	}

	f, err := ioutil.TempFile("", "worker-kubeconfig")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if _, err := f.Write(config.Data["value"]); err != nil {
		return nil, errors.WithStack(err)
	}
	c.WorkloadClusterKubeconfigs[namespace+"-"+name] = f.Name()

	master := ""

	restConfig, err := clientcmd.BuildConfigFromFlags(master, f.Name())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return c.ClientFromRestConfig(restConfig)
}
