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

// Conversion functions intentionally access deprecated fields for v1beta1↔v1beta2 roundtrip.

import (
	"k8s.io/apimachinery/pkg/conversion"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

// hasNonEmptyConditions returns true if the conditions slice contains at least
// one condition with a non-empty Type. During v1beta2→v1beta1→v1beta2 roundtrips,
// CAPI's Convert_v1_Condition_To_v1beta1_Condition is a no-op, producing
// zero-valued conditions ({type:"", status:""}). We must skip these to avoid
// creating empty entries in Deprecated.V1Beta1.Conditions.
func hasNonEmptyConditions(conditions clusterv1beta1.Conditions) bool {
	for i := range conditions {
		if conditions[i].Type != "" {
			return true
		}
	}
	return false
}

// AzureClusterClassSpec.

func Convert_v1beta1_AzureClusterClassSpec_To_v1beta2_AzureClusterClassSpec(in *AzureClusterClassSpec, out *infrav1.AzureClusterClassSpec, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureClusterClassSpec_To_v1beta2_AzureClusterClassSpec(in, out, s); err != nil {
		return err
	}
	if len(in.FailureDomains) > 0 {
		out.FailureDomains = make([]clusterv1.FailureDomain, 0, len(in.FailureDomains))
		for name, fd := range in.FailureDomains {
			cp := fd.ControlPlane
			out.FailureDomains = append(out.FailureDomains, clusterv1.FailureDomain{
				Name:         name,
				ControlPlane: &cp,
				Attributes:   fd.Attributes,
			})
		}
	}
	return nil
}

func Convert_v1beta2_AzureClusterClassSpec_To_v1beta1_AzureClusterClassSpec(in *infrav1.AzureClusterClassSpec, out *AzureClusterClassSpec, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureClusterClassSpec_To_v1beta1_AzureClusterClassSpec(in, out, s); err != nil {
		return err
	}
	if len(in.FailureDomains) > 0 {
		out.FailureDomains = make(clusterv1beta1.FailureDomains)
		for _, fd := range in.FailureDomains {
			cp := false
			if fd.ControlPlane != nil {
				cp = *fd.ControlPlane
			}
			out.FailureDomains[fd.Name] = clusterv1beta1.FailureDomainSpec{
				ControlPlane: cp,
				Attributes:   fd.Attributes,
			}
		}
	}
	return nil
}

// AzureClusterStatus.

func Convert_v1beta1_AzureClusterStatus_To_v1beta2_AzureClusterStatus(in *AzureClusterStatus, out *infrav1.AzureClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureClusterStatus_To_v1beta2_AzureClusterStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta1 conditions into out.Conditions ([]metav1.Condition),
	// but these belong in Deprecated.V1Beta1.Conditions. Clear them here; the v1beta2
	// controller will populate status.conditions with proper metav1.Condition entries.
	out.Conditions = nil
	if len(in.FailureDomains) > 0 {
		out.FailureDomains = make([]clusterv1.FailureDomain, 0, len(in.FailureDomains))
		for name, fd := range in.FailureDomains {
			cp := fd.ControlPlane
			out.FailureDomains = append(out.FailureDomains, clusterv1.FailureDomain{
				Name:         name,
				ControlPlane: &cp,
				Attributes:   fd.Attributes,
			})
		}
	}
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Ready || hasNonEmptyConditions(in.Conditions) {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.AzureClusterDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.AzureClusterV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	return nil
}

func Convert_v1beta2_AzureClusterStatus_To_v1beta1_AzureClusterStatus(in *infrav1.AzureClusterStatus, out *AzureClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureClusterStatus_To_v1beta1_AzureClusterStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta2 conditions ([]metav1.Condition) into out.Conditions
	// using Convert_v1_Condition_To_v1beta1_Condition which is a no-op, producing
	// zero-valued entries. Clear them and restore from Deprecated.V1Beta1 if available.
	out.Conditions = nil
	if len(in.FailureDomains) > 0 {
		out.FailureDomains = make(clusterv1beta1.FailureDomains)
		for _, fd := range in.FailureDomains {
			cp := false
			if fd.ControlPlane != nil {
				cp = *fd.ControlPlane
			}
			out.FailureDomains[fd.Name] = clusterv1beta1.FailureDomainSpec{
				ControlPlane: cp,
				Attributes:   fd.Attributes,
			}
		}
	}
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
	}
	return nil
}

// AzureClusterIdentityStatus.

func Convert_v1beta1_AzureClusterIdentityStatus_To_v1beta2_AzureClusterIdentityStatus(in *AzureClusterIdentityStatus, out *infrav1.AzureClusterIdentityStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureClusterIdentityStatus_To_v1beta2_AzureClusterIdentityStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta1 conditions into out.Conditions ([]metav1.Condition), but these belong in Deprecated.V1Beta1.Conditions only.
	out.Conditions = nil
	if hasNonEmptyConditions(in.Conditions) {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.AzureClusterIdentityDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.AzureClusterIdentityV1Beta1DeprecatedStatus{}
		}
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	return nil
}

func Convert_v1beta2_AzureClusterIdentityStatus_To_v1beta1_AzureClusterIdentityStatus(in *infrav1.AzureClusterIdentityStatus, out *AzureClusterIdentityStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureClusterIdentityStatus_To_v1beta1_AzureClusterIdentityStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta2 conditions ([]metav1.Condition) into out.Conditions
	// using Convert_v1_Condition_To_v1beta1_Condition which is a no-op, producing
	// zero-valued entries. Clear them and restore from Deprecated.V1Beta1 if available.
	out.Conditions = nil
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
	}
	return nil
}

// AzureMachineStatus.

func Convert_v1beta1_AzureMachineStatus_To_v1beta2_AzureMachineStatus(in *AzureMachineStatus, out *infrav1.AzureMachineStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureMachineStatus_To_v1beta2_AzureMachineStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta1 conditions into out.Conditions ([]metav1.Condition),
	// but these belong in Deprecated.V1Beta1.Conditions only.
	out.Conditions = nil
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Ready || hasNonEmptyConditions(in.Conditions) || in.FailureReason != nil || in.FailureMessage != nil {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.AzureMachineDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.AzureMachineV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
		out.Deprecated.V1Beta1.FailureReason = in.FailureReason
		out.Deprecated.V1Beta1.FailureMessage = in.FailureMessage
	}
	return nil
}

func Convert_v1beta2_AzureMachineStatus_To_v1beta1_AzureMachineStatus(in *infrav1.AzureMachineStatus, out *AzureMachineStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureMachineStatus_To_v1beta1_AzureMachineStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta2 conditions ([]metav1.Condition) into out.Conditions
	// using Convert_v1_Condition_To_v1beta1_Condition which is a no-op, producing
	// zero-valued entries. Clear them and restore from Deprecated.V1Beta1 if available.
	out.Conditions = nil
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
		out.FailureReason = in.Deprecated.V1Beta1.FailureReason
		out.FailureMessage = in.Deprecated.V1Beta1.FailureMessage
	}
	return nil
}

// AzureManagedClusterStatus.

func Convert_v1beta1_AzureManagedClusterStatus_To_v1beta2_AzureManagedClusterStatus(in *AzureManagedClusterStatus, out *infrav1.AzureManagedClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureManagedClusterStatus_To_v1beta2_AzureManagedClusterStatus(in, out, s); err != nil {
		return err
	}
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Ready {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.AzureManagedClusterDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.AzureManagedClusterV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
	}
	return nil
}

func Convert_v1beta2_AzureManagedClusterStatus_To_v1beta1_AzureManagedClusterStatus(in *infrav1.AzureManagedClusterStatus, out *AzureManagedClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureManagedClusterStatus_To_v1beta1_AzureManagedClusterStatus(in, out, s); err != nil {
		return err
	}
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	return nil
}

// AzureManagedControlPlaneStatus.

func Convert_v1beta1_AzureManagedControlPlaneStatus_To_v1beta2_AzureManagedControlPlaneStatus(in *AzureManagedControlPlaneStatus, out *infrav1.AzureManagedControlPlaneStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureManagedControlPlaneStatus_To_v1beta2_AzureManagedControlPlaneStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta1 conditions into out.Conditions ([]metav1.Condition), but these belong in Deprecated.V1Beta1.Conditions only.
	out.Conditions = nil
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Initialized {
		initialized := true
		out.Initialization.ControlPlaneInitialized = &initialized
	}
	if in.Ready || in.Initialized || hasNonEmptyConditions(in.Conditions) {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.AzureManagedControlPlaneDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.AzureManagedControlPlaneV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
		out.Deprecated.V1Beta1.Initialized = in.Initialized
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	return nil
}

func Convert_v1beta2_AzureManagedControlPlaneStatus_To_v1beta1_AzureManagedControlPlaneStatus(in *infrav1.AzureManagedControlPlaneStatus, out *AzureManagedControlPlaneStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureManagedControlPlaneStatus_To_v1beta1_AzureManagedControlPlaneStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta2 conditions ([]metav1.Condition) into out.Conditions
	// using Convert_v1_Condition_To_v1beta1_Condition which is a no-op, producing
	// zero-valued entries. Clear them and restore from Deprecated.V1Beta1 if available.
	out.Conditions = nil
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	if in.Initialization.ControlPlaneInitialized != nil {
		out.Initialized = *in.Initialization.ControlPlaneInitialized
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Initialized = in.Deprecated.V1Beta1.Initialized
	}
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
	}
	return nil
}

// AzureManagedMachinePoolStatus.

func Convert_v1beta1_AzureManagedMachinePoolStatus_To_v1beta2_AzureManagedMachinePoolStatus(in *AzureManagedMachinePoolStatus, out *infrav1.AzureManagedMachinePoolStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureManagedMachinePoolStatus_To_v1beta2_AzureManagedMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta1 conditions into out.Conditions ([]metav1.Condition), but these belong in Deprecated.V1Beta1.Conditions only.
	out.Conditions = nil
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Ready || hasNonEmptyConditions(in.Conditions) || in.ErrorReason != nil || in.ErrorMessage != nil {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.AzureManagedMachinePoolDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.AzureManagedMachinePoolV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
		out.Deprecated.V1Beta1.ErrorReason = in.ErrorReason
		out.Deprecated.V1Beta1.ErrorMessage = in.ErrorMessage
	}
	return nil
}

func Convert_v1beta2_AzureManagedMachinePoolStatus_To_v1beta1_AzureManagedMachinePoolStatus(in *infrav1.AzureManagedMachinePoolStatus, out *AzureManagedMachinePoolStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureManagedMachinePoolStatus_To_v1beta1_AzureManagedMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta2 conditions ([]metav1.Condition) into out.Conditions
	// using Convert_v1_Condition_To_v1beta1_Condition which is a no-op, producing
	// zero-valued entries. Clear them and restore from Deprecated.V1Beta1 if available.
	out.Conditions = nil
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
		out.ErrorReason = in.Deprecated.V1Beta1.ErrorReason
		out.ErrorMessage = in.Deprecated.V1Beta1.ErrorMessage
	}
	return nil
}

// AzureASOManagedClusterStatus.

func Convert_v1beta1_AzureASOManagedClusterStatus_To_v1beta2_AzureASOManagedClusterStatus(in *AzureASOManagedClusterStatus, out *infrav1.AzureASOManagedClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureASOManagedClusterStatus_To_v1beta2_AzureASOManagedClusterStatus(in, out, s); err != nil {
		return err
	}
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Ready {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.AzureASOManagedClusterDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.AzureASOManagedClusterV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
	}
	return nil
}

func Convert_v1beta2_AzureASOManagedClusterStatus_To_v1beta1_AzureASOManagedClusterStatus(in *infrav1.AzureASOManagedClusterStatus, out *AzureASOManagedClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureASOManagedClusterStatus_To_v1beta1_AzureASOManagedClusterStatus(in, out, s); err != nil {
		return err
	}
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	return nil
}

// AzureASOManagedControlPlaneStatus.

func Convert_v1beta1_AzureASOManagedControlPlaneStatus_To_v1beta2_AzureASOManagedControlPlaneStatus(in *AzureASOManagedControlPlaneStatus, out *infrav1.AzureASOManagedControlPlaneStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureASOManagedControlPlaneStatus_To_v1beta2_AzureASOManagedControlPlaneStatus(in, out, s); err != nil {
		return err
	}
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Initialized {
		initialized := true
		out.Initialization.ControlPlaneInitialized = &initialized
	}
	if in.Ready || in.Initialized {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.AzureASOManagedControlPlaneDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.AzureASOManagedControlPlaneV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
		out.Deprecated.V1Beta1.Initialized = in.Initialized
	}
	return nil
}

func Convert_v1beta2_AzureASOManagedControlPlaneStatus_To_v1beta1_AzureASOManagedControlPlaneStatus(in *infrav1.AzureASOManagedControlPlaneStatus, out *AzureASOManagedControlPlaneStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureASOManagedControlPlaneStatus_To_v1beta1_AzureASOManagedControlPlaneStatus(in, out, s); err != nil {
		return err
	}
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	if in.Initialization.ControlPlaneInitialized != nil {
		out.Initialized = *in.Initialization.ControlPlaneInitialized
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Initialized = in.Deprecated.V1Beta1.Initialized
	}
	return nil
}

// AzureASOManagedMachinePoolStatus.

func Convert_v1beta1_AzureASOManagedMachinePoolStatus_To_v1beta2_AzureASOManagedMachinePoolStatus(in *AzureASOManagedMachinePoolStatus, out *infrav1.AzureASOManagedMachinePoolStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureASOManagedMachinePoolStatus_To_v1beta2_AzureASOManagedMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Ready {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1.AzureASOManagedMachinePoolDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1.AzureASOManagedMachinePoolV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
	}
	return nil
}

func Convert_v1beta2_AzureASOManagedMachinePoolStatus_To_v1beta1_AzureASOManagedMachinePoolStatus(in *infrav1.AzureASOManagedMachinePoolStatus, out *AzureASOManagedMachinePoolStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureASOManagedMachinePoolStatus_To_v1beta1_AzureASOManagedMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	return nil
}
