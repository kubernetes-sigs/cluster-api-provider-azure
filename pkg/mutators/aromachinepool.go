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

package mutators

import (
	"context"
	"fmt"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

var (
	// ErrNoHcpOpenShiftClustersNodePoolDefined describes an AROMachinePool without a HcpOpenShiftClustersNodePool.
	ErrNoHcpOpenShiftClustersNodePoolDefined = fmt.Errorf("no %s HcpOpenShiftClustersNodePool defined in AROMachinePool spec.resources", asoredhatopenshiftv1.GroupVersion.Group)
)

// SetHcpOpenShiftNodePoolDefaults sets defaults for HcpOpenShiftClustersNodePool resources.
// This mutator automatically sets owner references for HcpOpenShiftClustersNodePool resources.
func SetHcpOpenShiftNodePoolDefaults(_ client.Client, _ *infrav1exp.AROMachinePool, hcpClusterName string) ResourcesMutator {
	return func(ctx context.Context, us []*unstructured.Unstructured) error {
		ctx, log, done := tele.StartSpanWithLogger(ctx, "mutators.SetHcpOpenShiftNodePoolDefaults")
		defer done()

		// Find the HcpOpenShiftClustersNodePool
		var nodePool *unstructured.Unstructured
		var nodePoolPath string
		for i, u := range us {
			if u.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group &&
				u.GroupVersionKind().Kind == "HcpOpenShiftClustersNodePool" {
				nodePool = u
				nodePoolPath = fmt.Sprintf("spec.resources[%d]", i)
				break
			}
		}
		if nodePool == nil {
			return reconcile.TerminalError(ErrNoHcpOpenShiftClustersNodePoolDefined)
		}

		// Set owner reference to the HcpOpenShiftCluster
		return setNodePoolOwner(ctx, nodePool, hcpClusterName, nodePoolPath, log)
	}
}

func setNodePoolOwner(_ context.Context, nodePool *unstructured.Unstructured, hcpClusterName, nodePoolPath string, log logr.Logger) error {
	// Check if owner is already set
	ownerMap, hasOwner, err := unstructured.NestedMap(nodePool.UnstructuredContent(), "spec", "owner")
	if err != nil {
		return err
	}

	// Validate owner name if already set
	if hasOwner {
		ownerName, _, _ := unstructured.NestedString(ownerMap, "name")
		if ownerName != "" && ownerName != hcpClusterName {
			return Incompatible{
				mutation: mutation{
					location: nodePoolPath + ".spec.owner.name",
					val:      hcpClusterName,
					reason:   "because HcpOpenShiftClustersNodePool must reference the HcpOpenShiftCluster",
				},
				userVal: ownerName,
			}
		}
	}

	// Always set/override the owner reference to ensure it only contains 'name'
	// This removes any user-provided 'group' or 'kind' fields which are not valid
	owner := map[string]interface{}{
		"name": hcpClusterName,
	}

	setOwner := mutation{
		location: nodePoolPath + ".spec.owner",
		val:      owner,
		reason:   fmt.Sprintf("because HcpOpenShiftClustersNodePool must reference HcpOpenShiftCluster %s", hcpClusterName),
	}
	logMutation(log, setOwner)
	return unstructured.SetNestedMap(nodePool.UnstructuredContent(), owner, "spec", "owner")
}
