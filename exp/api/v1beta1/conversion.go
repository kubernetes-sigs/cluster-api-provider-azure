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
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

// hasNonEmptyConditions returns true if the conditions slice contains at least
// one condition with a non-empty Type. See api/v1beta1/conversion.go for details.
func hasNonEmptyConditions(conditions clusterv1beta1.Conditions) bool {
	for i := range conditions {
		if conditions[i].Type != "" {
			return true
		}
	}
	return false
}

func Convert_v1beta1_AzureMachinePoolStatus_To_v1beta2_AzureMachinePoolStatus(in *AzureMachinePoolStatus, out *infrav1exp.AzureMachinePoolStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_AzureMachinePoolStatus_To_v1beta2_AzureMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta1 conditions into out.Conditions ([]metav1.Condition), but these belong in Deprecated.V1Beta1.Conditions only.
	out.Conditions = nil
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Ready || hasNonEmptyConditions(in.Conditions) || in.FailureReason != nil || in.FailureMessage != nil {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1exp.AzureMachinePoolDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1exp.AzureMachinePoolV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
		out.Deprecated.V1Beta1.FailureReason = in.FailureReason
		out.Deprecated.V1Beta1.FailureMessage = in.FailureMessage
	}
	return nil
}

func Convert_v1beta2_AzureMachinePoolStatus_To_v1beta1_AzureMachinePoolStatus(in *infrav1exp.AzureMachinePoolStatus, out *AzureMachinePoolStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_AzureMachinePoolStatus_To_v1beta1_AzureMachinePoolStatus(in, out, s); err != nil {
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

func Convert_v1beta1_AzureMachinePoolMachineStatus_To_v1beta2_AzureMachinePoolMachineStatus(in *AzureMachinePoolMachineStatus, out *infrav1exp.AzureMachinePoolMachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_AzureMachinePoolMachineStatus_To_v1beta2_AzureMachinePoolMachineStatus(in, out, s); err != nil {
		return err
	}
	// autoConvert copies v1beta1 conditions into out.Conditions ([]metav1.Condition), but these belong in Deprecated.V1Beta1.Conditions only.
	out.Conditions = nil
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if in.Ready || hasNonEmptyConditions(in.Conditions) || in.FailureReason != nil || in.FailureMessage != nil {
		if out.Deprecated == nil {
			out.Deprecated = &infrav1exp.AzureMachinePoolMachineDeprecatedStatus{}
		}
		if out.Deprecated.V1Beta1 == nil {
			out.Deprecated.V1Beta1 = &infrav1exp.AzureMachinePoolMachineV1Beta1DeprecatedStatus{}
		}
		out.Deprecated.V1Beta1.Ready = in.Ready
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
		out.Deprecated.V1Beta1.FailureReason = in.FailureReason
		out.Deprecated.V1Beta1.FailureMessage = in.FailureMessage
	}
	return nil
}

func Convert_v1beta2_AzureMachinePoolMachineStatus_To_v1beta1_AzureMachinePoolMachineStatus(in *infrav1exp.AzureMachinePoolMachineStatus, out *AzureMachinePoolMachineStatus, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_AzureMachinePoolMachineStatus_To_v1beta1_AzureMachinePoolMachineStatus(in, out, s); err != nil {
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
