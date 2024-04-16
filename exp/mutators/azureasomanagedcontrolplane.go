/*
Copyright 2024 The Kubernetes Authors.

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
	"strings"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	// ErrNoManagedClusterDefined describes an AzureASOManagedControlPlane without a ManagedCluster.
	ErrNoManagedClusterDefined = fmt.Errorf("no %s ManagedCluster defined in AzureASOManagedControlPlane spec.resources", asocontainerservicev1.GroupVersion.Group)
)

// SetManagedClusterDefaults propagates values defined by Cluster API to an ASO ManagedCluster.
func SetManagedClusterDefaults(asoManagedControlPlane *infrav1exp.AzureASOManagedControlPlane) ResourcesMutator {
	return func(ctx context.Context, us []*unstructured.Unstructured) error {
		ctx, _, done := tele.StartSpanWithLogger(ctx, "mutators.SetManagedClusterDefaults")
		defer done()

		var managedCluster *unstructured.Unstructured
		var managedClusterPath string
		for i, u := range us {
			if u.GroupVersionKind().Group == asocontainerservicev1.GroupVersion.Group &&
				u.GroupVersionKind().Kind == "ManagedCluster" {
				managedCluster = u
				managedClusterPath = fmt.Sprintf("spec.resources[%d]", i)
				break
			}
		}
		if managedCluster == nil {
			return reconcile.TerminalError(ErrNoManagedClusterDefined)
		}

		if err := setManagedClusterKubernetesVersion(ctx, asoManagedControlPlane, managedClusterPath, managedCluster); err != nil {
			return err
		}

		return nil
	}
}

func setManagedClusterKubernetesVersion(ctx context.Context, asoManagedControlPlane *infrav1exp.AzureASOManagedControlPlane, managedClusterPath string, managedCluster *unstructured.Unstructured) error {
	_, log, done := tele.StartSpanWithLogger(ctx, "mutators.setManagedClusterKubernetesVersion")
	defer done()

	capzK8sVersion := strings.TrimPrefix(asoManagedControlPlane.Spec.Version, "v")
	if capzK8sVersion == "" {
		// When the CAPI contract field isn't set, any value for version in the embedded ASO resource may be specified.
		return nil
	}

	k8sVersionPath := []string{"spec", "kubernetesVersion"}
	userK8sVersion, k8sVersionFound, err := unstructured.NestedString(managedCluster.UnstructuredContent(), k8sVersionPath...)
	if err != nil {
		return err
	}
	setK8sVersion := mutation{
		location: managedClusterPath + "." + strings.Join(k8sVersionPath, "."),
		val:      capzK8sVersion,
		reason:   "because spec.version is set to " + asoManagedControlPlane.Spec.Version,
	}
	if k8sVersionFound && userK8sVersion != capzK8sVersion {
		return Incompatible{
			mutation: setK8sVersion,
			userVal:  userK8sVersion,
		}
	}
	logMutation(log, setK8sVersion)
	return unstructured.SetNestedField(managedCluster.UnstructuredContent(), capzK8sVersion, k8sVersionPath...)
}
