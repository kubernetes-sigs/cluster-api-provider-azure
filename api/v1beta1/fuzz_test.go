//go:build !race

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
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/randfill"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubAzureClusterStatus,
		hubAzureClusterClassSpec,
		hubAzureClusterIdentityStatus,
		hubAzureMachineStatus,
		hubAzureManagedClusterStatus,
		hubAzureManagedControlPlaneStatus,
		hubAzureManagedMachinePoolStatus,
		hubAzureASOManagedClusterStatus,
		hubAzureASOManagedControlPlaneStatus,
		hubAzureASOManagedMachinePoolStatus,
		hubFailureDomain,
		spokeCondition,
		spokeAzureMachineSpec,
	}
}

// hubAzureClusterStatus handles lossy fields in hub→spoke→hub for AzureClusterStatus.
// - Conditions ([]metav1.Condition) don't roundtrip (no-op auto-converter).
// - Initialization/Deprecated are coupled through the spoke's Ready field.
// - FailureDomains lose ordering (slice→map→slice).
func hubAzureClusterStatus(in *infrav1.AzureClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// metav1.Conditions don't survive; the spoke uses Deprecated.V1Beta1.Conditions.
	in.Conditions = nil

	// Initialization and Deprecated are derived from the spoke's Ready/Conditions.
	// Clear them so the hub→spoke→hub test covers non-lossy fields; the reverse
	// direction (spoke→hub→spoke) verifies these fields roundtrip correctly.
	in.Initialization = infrav1.AzureClusterInitializationStatus{}
	in.Deprecated = nil

	normalizeHubFailureDomains(in.FailureDomains, c)
}

// hubAzureClusterClassSpec normalizes FailureDomains for deterministic roundtrip.
func hubAzureClusterClassSpec(in *infrav1.AzureClusterClassSpec, c randfill.Continue) {
	c.FillNoCustom(in)
	normalizeHubFailureDomains(in.FailureDomains, c)
}

func hubAzureClusterIdentityStatus(in *infrav1.AzureClusterIdentityStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Conditions = nil
	in.Deprecated = nil
}

func hubAzureMachineStatus(in *infrav1.AzureMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Conditions = nil
	in.Initialization = infrav1.AzureMachineInitializationStatus{}
	in.Deprecated = nil
}

func hubAzureManagedClusterStatus(in *infrav1.AzureManagedClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Initialization = infrav1.AzureManagedClusterInitializationStatus{}
	in.Deprecated = nil
}

func hubAzureManagedControlPlaneStatus(in *infrav1.AzureManagedControlPlaneStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Conditions = nil
	in.Initialization = infrav1.AzureManagedControlPlaneInitializationStatus{}
	in.Deprecated = nil
}

func hubAzureManagedMachinePoolStatus(in *infrav1.AzureManagedMachinePoolStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Conditions = nil
	in.Initialization = infrav1.AzureManagedMachinePoolInitializationStatus{}
	in.Deprecated = nil
}

func hubAzureASOManagedClusterStatus(in *infrav1.AzureASOManagedClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Initialization = infrav1.AzureASOManagedClusterInitializationStatus{}
	in.Deprecated = nil
}

func hubAzureASOManagedControlPlaneStatus(in *infrav1.AzureASOManagedControlPlaneStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Initialization = infrav1.AzureASOManagedControlPlaneInitializationStatus{}
	in.Deprecated = nil
}

func hubAzureASOManagedMachinePoolStatus(in *infrav1.AzureASOManagedMachinePoolStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Initialization = infrav1.AzureASOManagedMachinePoolInitializationStatus{}
	in.Deprecated = nil
}

// hubFailureDomain ensures ControlPlane is never nil (nil→false→&false is lossy).
func hubFailureDomain(in *clusterv1.FailureDomain, c randfill.Continue) {
	c.FillNoCustom(in)
	in.ControlPlane = ptr.To(c.Bool())
}

// spokeCondition ensures v1beta1 conditions have non-empty Type so they
// survive the hasNonEmptyConditions gate in spoke→hub conversion.
func spokeCondition(in *clusterv1beta1.Condition, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.Type == "" {
		in.Type = clusterv1beta1.ConditionType(fmt.Sprintf("FuzzCondition_%s", c.String(10)))
	}
	in.Status = []corev1.ConditionStatus{
		corev1.ConditionTrue,
		corev1.ConditionFalse,
		corev1.ConditionUnknown,
	}[c.Intn(3)]
}

// spokeAzureMachineSpec handles the *string→string→*string lossy conversion
// for ProviderID (v1beta1 *string → v1beta2 string: nil becomes "").
func spokeAzureMachineSpec(in *AzureMachineSpec, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.ProviderID == nil {
		in.ProviderID = ptr.To("")
	}
}

// normalizeHubFailureDomains ensures FailureDomains have unique names and sorted
// order for deterministic hub→spoke→hub comparison (map iteration loses ordering).
func normalizeHubFailureDomains(fds []clusterv1.FailureDomain, c randfill.Continue) {
	seen := make(map[string]bool, len(fds))
	for i := range fds {
		if fds[i].Name == "" || seen[fds[i].Name] {
			fds[i].Name = fmt.Sprintf("fd-%d-%s", i, c.String(10))
		}
		seen[fds[i].Name] = true
		if fds[i].ControlPlane == nil {
			fds[i].ControlPlane = ptr.To(c.Bool())
		}
	}
	sortFailureDomains(fds)
}

func sortFailureDomains(fds []clusterv1.FailureDomain) {
	slices.SortFunc(fds, func(a, b clusterv1.FailureDomain) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
}
