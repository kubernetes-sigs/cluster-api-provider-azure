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

package v1alpha1

import (
apiconversion "k8s.io/apimachinery/pkg/conversion"

infrav1beta2 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

func Convert_v1alpha1_AzureASOManagedClusterStatus_To_v1beta2_AzureASOManagedClusterStatus(in *AzureASOManagedClusterStatus, out *infrav1beta2.AzureASOManagedClusterStatus, s apiconversion.Scope) error {
if err := autoConvert_v1alpha1_AzureASOManagedClusterStatus_To_v1beta2_AzureASOManagedClusterStatus(in, out, s); err != nil {
return err
}
if in.Ready {
provisioned := true
out.Initialization.Provisioned = &provisioned
}
if out.Deprecated == nil {
out.Deprecated = &infrav1beta2.AzureASOManagedClusterDeprecatedStatus{}
}
if out.Deprecated.V1Beta1 == nil {
out.Deprecated.V1Beta1 = &infrav1beta2.AzureASOManagedClusterV1Beta1DeprecatedStatus{}
}
out.Deprecated.V1Beta1.Ready = in.Ready
return nil
}

func Convert_v1beta2_AzureASOManagedClusterStatus_To_v1alpha1_AzureASOManagedClusterStatus(in *infrav1beta2.AzureASOManagedClusterStatus, out *AzureASOManagedClusterStatus, s apiconversion.Scope) error {
if err := autoConvert_v1beta2_AzureASOManagedClusterStatus_To_v1alpha1_AzureASOManagedClusterStatus(in, out, s); err != nil {
return err
}
if in.Initialization.Provisioned != nil {
out.Ready = *in.Initialization.Provisioned
} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
out.Ready = in.Deprecated.V1Beta1.Ready
}
return nil
}

func Convert_v1alpha1_AzureASOManagedControlPlaneStatus_To_v1beta2_AzureASOManagedControlPlaneStatus(in *AzureASOManagedControlPlaneStatus, out *infrav1beta2.AzureASOManagedControlPlaneStatus, s apiconversion.Scope) error {
if err := autoConvert_v1alpha1_AzureASOManagedControlPlaneStatus_To_v1beta2_AzureASOManagedControlPlaneStatus(in, out, s); err != nil {
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
if out.Deprecated == nil {
out.Deprecated = &infrav1beta2.AzureASOManagedControlPlaneDeprecatedStatus{}
}
if out.Deprecated.V1Beta1 == nil {
out.Deprecated.V1Beta1 = &infrav1beta2.AzureASOManagedControlPlaneV1Beta1DeprecatedStatus{}
}
out.Deprecated.V1Beta1.Ready = in.Ready
out.Deprecated.V1Beta1.Initialized = in.Initialized
return nil
}

func Convert_v1beta2_AzureASOManagedControlPlaneStatus_To_v1alpha1_AzureASOManagedControlPlaneStatus(in *infrav1beta2.AzureASOManagedControlPlaneStatus, out *AzureASOManagedControlPlaneStatus, s apiconversion.Scope) error {
if err := autoConvert_v1beta2_AzureASOManagedControlPlaneStatus_To_v1alpha1_AzureASOManagedControlPlaneStatus(in, out, s); err != nil {
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

func Convert_v1alpha1_AzureASOManagedMachinePoolStatus_To_v1beta2_AzureASOManagedMachinePoolStatus(in *AzureASOManagedMachinePoolStatus, out *infrav1beta2.AzureASOManagedMachinePoolStatus, s apiconversion.Scope) error {
if err := autoConvert_v1alpha1_AzureASOManagedMachinePoolStatus_To_v1beta2_AzureASOManagedMachinePoolStatus(in, out, s); err != nil {
return err
}
if in.Ready {
provisioned := true
out.Initialization.Provisioned = &provisioned
}
if out.Deprecated == nil {
out.Deprecated = &infrav1beta2.AzureASOManagedMachinePoolDeprecatedStatus{}
}
if out.Deprecated.V1Beta1 == nil {
out.Deprecated.V1Beta1 = &infrav1beta2.AzureASOManagedMachinePoolV1Beta1DeprecatedStatus{}
}
out.Deprecated.V1Beta1.Ready = in.Ready
return nil
}

func Convert_v1beta2_AzureASOManagedMachinePoolStatus_To_v1alpha1_AzureASOManagedMachinePoolStatus(in *infrav1beta2.AzureASOManagedMachinePoolStatus, out *AzureASOManagedMachinePoolStatus, s apiconversion.Scope) error {
if err := autoConvert_v1beta2_AzureASOManagedMachinePoolStatus_To_v1alpha1_AzureASOManagedMachinePoolStatus(in, out, s); err != nil {
return err
}
if in.Initialization.Provisioned != nil {
out.Ready = *in.Initialization.Provisioned
} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
out.Ready = in.Deprecated.V1Beta1.Ready
}
return nil
}
