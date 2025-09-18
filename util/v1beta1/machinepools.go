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

package v1beta1

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetMachinePoolByName finds and returns a MachinePool object using the specified params.
func GetMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*clusterv1beta1.MachinePool, error) {
	m := &clusterv1beta1.MachinePool{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetOwnerMachinePool returns the MachinePool objects owning the current resource.
func GetOwnerMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1beta1.MachinePool, error) {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind != "MachinePool" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if gv.Group == clusterv1beta1.GroupVersion.Group {
			return GetMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}
