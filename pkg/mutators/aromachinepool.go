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
	asoredhatopenshiftv1api2025 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20251223preview"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
// This mutator automatically sets owner references for HcpOpenShiftClustersNodePool resources
// and manages the autoscaling annotation on the MachinePool.
func SetHcpOpenShiftNodePoolDefaults(_ client.Client, _ *infrav1exp.AROMachinePool, hcpClusterName string, machinePool *clusterv1.MachinePool) ResourcesMutator {
	return func(ctx context.Context, us []*unstructured.Unstructured) error {
		ctx, log, done := tele.StartSpanWithLogger(ctx, "mutators.SetHcpOpenShiftNodePoolDefaults")
		defer done()

		// Find the HcpOpenShiftClustersNodePool
		var nodePool *unstructured.Unstructured
		var nodePoolPath string
		for i, u := range us {
			if (u.GroupVersionKind().Group == asoredhatopenshiftv1.GroupVersion.Group ||
				u.GroupVersionKind().Group == asoredhatopenshiftv1api2025.GroupVersion.Group) &&
				u.GroupVersionKind().Kind == "HcpOpenShiftClustersNodePool" {
				nodePool = u
				nodePoolPath = fmt.Sprintf("spec.resources[%d]", i)
				break
			}
		}
		if nodePool == nil {
			return reconcile.TerminalError(ErrNoHcpOpenShiftClustersNodePoolDefined)
		}

		// Reconcile autoscaling annotation on the MachinePool based on HcpOpenShiftClustersNodePool's autoscaling setting
		if err := reconcileAROAutoscaling(nodePool, machinePool); err != nil {
			return err
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

// reconcileAROAutoscaling manages the autoscaling annotation on the MachinePool based on
// the HcpOpenShiftClustersNodePool's autoscaling configuration.
func reconcileAROAutoscaling(nodePool *unstructured.Unstructured, machinePool *clusterv1.MachinePool) error {
	// Check if autoscaling is configured in the HcpOpenShiftClustersNodePool
	// Autoscaling is enabled if the autoScaling object exists with min/max values
	autoScalingConfig, found, err := unstructured.NestedMap(nodePool.UnstructuredContent(), "spec", "properties", "autoScaling")
	if err != nil {
		return err
	}

	// Autoscaling is considered enabled if the autoScaling field is present and not empty
	autoscaling := found && len(autoScalingConfig) > 0

	// Update the MachinePool replica manager annotation. This isn't wrapped in a mutation object because
	// it's not modifying an ASO resource and users are not expected to set this manually. This behavior
	// is documented by CAPI as expected of a provider.
	replicaManager, ok := machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]
	if autoscaling {
		if !ok {
			if machinePool.Annotations == nil {
				machinePool.Annotations = make(map[string]string)
			}
			machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation] = infrav1exp.ReplicasManagedByARO
		} else if replicaManager != infrav1exp.ReplicasManagedByARO {
			return fmt.Errorf("failed to enable autoscaling, replicas are already being managed by %s according to MachinePool %s's %s annotation", replicaManager, machinePool.Name, clusterv1.ReplicasManagedByAnnotation)
		}
	} else if !autoscaling && replicaManager == infrav1exp.ReplicasManagedByARO {
		// Removing this annotation informs the MachinePool controller that this MachinePool is no longer
		// being autoscaled.
		delete(machinePool.Annotations, clusterv1.ReplicasManagedByAnnotation)
	}

	return nil
}
