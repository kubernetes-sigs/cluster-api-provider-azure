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
apiconversion "k8s.io/apimachinery/pkg/conversion"

infraexpv1beta2 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

func Convert_v1beta1_AzureMachinePoolStatus_To_v1beta2_AzureMachinePoolStatus(in *AzureMachinePoolStatus, out *infraexpv1beta2.AzureMachinePoolStatus, s apiconversion.Scope) error {
if err := autoConvert_v1beta1_AzureMachinePoolStatus_To_v1beta2_AzureMachinePoolStatus(in, out, s); err != nil {
return err
}
if in.Ready {
provisioned := true
out.Initialization.Provisioned = &provisioned
}
if out.Deprecated == nil {
out.Deprecated = &infraexpv1beta2.AzureMachinePoolDeprecatedStatus{}
}
if out.Deprecated.V1Beta1 == nil {
out.Deprecated.V1Beta1 = &infraexpv1beta2.AzureMachinePoolV1Beta1DeprecatedStatus{}
}
out.Deprecated.V1Beta1.Ready = in.Ready
out.Deprecated.V1Beta1.Conditions = in.Conditions
out.Deprecated.V1Beta1.FailureReason = in.FailureReason
out.Deprecated.V1Beta1.FailureMessage = in.FailureMessage
return nil
}

func Convert_v1beta2_AzureMachinePoolStatus_To_v1beta1_AzureMachinePoolStatus(in *infraexpv1beta2.AzureMachinePoolStatus, out *AzureMachinePoolStatus, s apiconversion.Scope) error {
if err := autoConvert_v1beta2_AzureMachinePoolStatus_To_v1beta1_AzureMachinePoolStatus(in, out, s); err != nil {
return err
}
if in.Initialization.Provisioned != nil {
out.Ready = *in.Initialization.Provisioned
} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
out.Ready = in.Deprecated.V1Beta1.Ready
}
if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
out.Conditions = in.Deprecated.V1Beta1.Conditions
out.FailureReason = in.Deprecated.V1Beta1.FailureReason
out.FailureMessage = in.Deprecated.V1Beta1.FailureMessage
}
return nil
}

func Convert_v1beta1_AzureMachinePoolMachineStatus_To_v1beta2_AzureMachinePoolMachineStatus(in *AzureMachinePoolMachineStatus, out *infraexpv1beta2.AzureMachinePoolMachineStatus, s apiconversion.Scope) error {
if err := autoConvert_v1beta1_AzureMachinePoolMachineStatus_To_v1beta2_AzureMachinePoolMachineStatus(in, out, s); err != nil {
return err
}
if in.Ready {
provisioned := true
out.Initialization.Provisioned = &provisioned
}
if out.Deprecated == nil {
out.Deprecated = &infraexpv1beta2.AzureMachinePoolMachineDeprecatedStatus{}
}
if out.Deprecated.V1Beta1 == nil {
out.Deprecated.V1Beta1 = &infraexpv1beta2.AzureMachinePoolMachineV1Beta1DeprecatedStatus{}
}
out.Deprecated.V1Beta1.Ready = in.Ready
out.Deprecated.V1Beta1.Conditions = in.Conditions
out.Deprecated.V1Beta1.FailureReason = in.FailureReason
out.Deprecated.V1Beta1.FailureMessage = in.FailureMessage
return nil
}

func Convert_v1beta2_AzureMachinePoolMachineStatus_To_v1beta1_AzureMachinePoolMachineStatus(in *infraexpv1beta2.AzureMachinePoolMachineStatus, out *AzureMachinePoolMachineStatus, s apiconversion.Scope) error {
if err := autoConvert_v1beta2_AzureMachinePoolMachineStatus_To_v1beta1_AzureMachinePoolMachineStatus(in, out, s); err != nil {
return err
}
if in.Initialization.Provisioned != nil {
out.Ready = *in.Initialization.Provisioned
} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
out.Ready = in.Deprecated.V1Beta1.Ready
}
if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
out.Conditions = in.Deprecated.V1Beta1.Conditions
out.FailureReason = in.Deprecated.V1Beta1.FailureReason
out.FailureMessage = in.Deprecated.V1Beta1.FailureMessage
}
return nil
}
