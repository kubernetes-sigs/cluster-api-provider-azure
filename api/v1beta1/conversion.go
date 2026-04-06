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
	"k8s.io/apimachinery/pkg/conversion"
	infrav1beta2 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// AzureClusterClassSpec

func Convert_v1beta1_AzureClusterClassSpec_To_v1beta2_AzureClusterClassSpec(in *AzureClusterClassSpec, out *infrav1beta2.AzureClusterClassSpec, s conversion.Scope) error {
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

func Convert_v1beta2_AzureClusterClassSpec_To_v1beta1_AzureClusterClassSpec(in *infrav1beta2.AzureClusterClassSpec, out *AzureClusterClassSpec, s conversion.Scope) error {
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

// AzureClusterStatus

func Convert_v1beta1_AzureClusterStatus_To_v1beta2_AzureClusterStatus(in *AzureClusterStatus, out *infrav1beta2.AzureClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureClusterStatus_To_v1beta2_AzureClusterStatus(in, out, s); err != nil {
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
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if out.Deprecated == nil {
		out.Deprecated = &infrav1beta2.AzureClusterDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1beta2.AzureClusterV1Beta1DeprecatedStatus{}
	}
	out.Deprecated.V1Beta1.Ready = in.Ready
	out.Deprecated.V1Beta1.Conditions = in.Conditions
	return nil
}

func Convert_v1beta2_AzureClusterStatus_To_v1beta1_AzureClusterStatus(in *infrav1beta2.AzureClusterStatus, out *AzureClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureClusterStatus_To_v1beta1_AzureClusterStatus(in, out, s); err != nil {
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
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Conditions = in.Deprecated.V1Beta1.Conditions
	}
	return nil
}

// AzureClusterIdentityStatus (only v1beta2→v1beta1 needs manual implementation)

func Convert_v1beta2_AzureClusterIdentityStatus_To_v1beta1_AzureClusterIdentityStatus(in *infrav1beta2.AzureClusterIdentityStatus, out *AzureClusterIdentityStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureClusterIdentityStatus_To_v1beta1_AzureClusterIdentityStatus(in, out, s); err != nil {
		return err
	}
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Conditions = in.Deprecated.V1Beta1.Conditions
	}
	return nil
}

// AzureMachineStatus

func Convert_v1beta1_AzureMachineStatus_To_v1beta2_AzureMachineStatus(in *AzureMachineStatus, out *infrav1beta2.AzureMachineStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureMachineStatus_To_v1beta2_AzureMachineStatus(in, out, s); err != nil {
		return err
	}
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if out.Deprecated == nil {
		out.Deprecated = &infrav1beta2.AzureMachineDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1beta2.AzureMachineV1Beta1DeprecatedStatus{}
	}
	out.Deprecated.V1Beta1.Ready = in.Ready
	out.Deprecated.V1Beta1.Conditions = in.Conditions
	out.Deprecated.V1Beta1.FailureReason = in.FailureReason
	out.Deprecated.V1Beta1.FailureMessage = in.FailureMessage
	return nil
}

func Convert_v1beta2_AzureMachineStatus_To_v1beta1_AzureMachineStatus(in *infrav1beta2.AzureMachineStatus, out *AzureMachineStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureMachineStatus_To_v1beta1_AzureMachineStatus(in, out, s); err != nil {
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

// AzureManagedClusterStatus

func Convert_v1beta1_AzureManagedClusterStatus_To_v1beta2_AzureManagedClusterStatus(in *AzureManagedClusterStatus, out *infrav1beta2.AzureManagedClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureManagedClusterStatus_To_v1beta2_AzureManagedClusterStatus(in, out, s); err != nil {
		return err
	}
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if out.Deprecated == nil {
		out.Deprecated = &infrav1beta2.AzureManagedClusterDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1beta2.AzureManagedClusterV1Beta1DeprecatedStatus{}
	}
	out.Deprecated.V1Beta1.Ready = in.Ready
	return nil
}

func Convert_v1beta2_AzureManagedClusterStatus_To_v1beta1_AzureManagedClusterStatus(in *infrav1beta2.AzureManagedClusterStatus, out *AzureManagedClusterStatus, s conversion.Scope) error {
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

// AzureManagedControlPlaneStatus

func Convert_v1beta1_AzureManagedControlPlaneStatus_To_v1beta2_AzureManagedControlPlaneStatus(in *AzureManagedControlPlaneStatus, out *infrav1beta2.AzureManagedControlPlaneStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureManagedControlPlaneStatus_To_v1beta2_AzureManagedControlPlaneStatus(in, out, s); err != nil {
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
		out.Deprecated = &infrav1beta2.AzureManagedControlPlaneDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1beta2.AzureManagedControlPlaneV1Beta1DeprecatedStatus{}
	}
	out.Deprecated.V1Beta1.Ready = in.Ready
	out.Deprecated.V1Beta1.Initialized = in.Initialized
	out.Deprecated.V1Beta1.Conditions = in.Conditions
	return nil
}

func Convert_v1beta2_AzureManagedControlPlaneStatus_To_v1beta1_AzureManagedControlPlaneStatus(in *infrav1beta2.AzureManagedControlPlaneStatus, out *AzureManagedControlPlaneStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureManagedControlPlaneStatus_To_v1beta1_AzureManagedControlPlaneStatus(in, out, s); err != nil {
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
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Conditions = in.Deprecated.V1Beta1.Conditions
	}
	return nil
}

// AzureManagedMachinePoolStatus

func Convert_v1beta1_AzureManagedMachinePoolStatus_To_v1beta2_AzureManagedMachinePoolStatus(in *AzureManagedMachinePoolStatus, out *infrav1beta2.AzureManagedMachinePoolStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureManagedMachinePoolStatus_To_v1beta2_AzureManagedMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	if in.Ready {
		provisioned := true
		out.Initialization.Provisioned = &provisioned
	}
	if out.Deprecated == nil {
		out.Deprecated = &infrav1beta2.AzureManagedMachinePoolDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1beta2.AzureManagedMachinePoolV1Beta1DeprecatedStatus{}
	}
	out.Deprecated.V1Beta1.Ready = in.Ready
	out.Deprecated.V1Beta1.Conditions = in.Conditions
	out.Deprecated.V1Beta1.ErrorReason = in.ErrorReason
	out.Deprecated.V1Beta1.ErrorMessage = in.ErrorMessage
	return nil
}

func Convert_v1beta2_AzureManagedMachinePoolStatus_To_v1beta1_AzureManagedMachinePoolStatus(in *infrav1beta2.AzureManagedMachinePoolStatus, out *AzureManagedMachinePoolStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta2_AzureManagedMachinePoolStatus_To_v1beta1_AzureManagedMachinePoolStatus(in, out, s); err != nil {
		return err
	}
	if in.Initialization.Provisioned != nil {
		out.Ready = *in.Initialization.Provisioned
	} else if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Ready = in.Deprecated.V1Beta1.Ready
	}
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.Conditions = in.Deprecated.V1Beta1.Conditions
		out.ErrorReason = in.Deprecated.V1Beta1.ErrorReason
		out.ErrorMessage = in.Deprecated.V1Beta1.ErrorMessage
	}
	return nil
}

// AzureASOManagedClusterStatus

func Convert_v1beta1_AzureASOManagedClusterStatus_To_v1beta2_AzureASOManagedClusterStatus(in *AzureASOManagedClusterStatus, out *infrav1beta2.AzureASOManagedClusterStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureASOManagedClusterStatus_To_v1beta2_AzureASOManagedClusterStatus(in, out, s); err != nil {
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

func Convert_v1beta2_AzureASOManagedClusterStatus_To_v1beta1_AzureASOManagedClusterStatus(in *infrav1beta2.AzureASOManagedClusterStatus, out *AzureASOManagedClusterStatus, s conversion.Scope) error {
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

// AzureASOManagedControlPlaneStatus

func Convert_v1beta1_AzureASOManagedControlPlaneStatus_To_v1beta2_AzureASOManagedControlPlaneStatus(in *AzureASOManagedControlPlaneStatus, out *infrav1beta2.AzureASOManagedControlPlaneStatus, s conversion.Scope) error {
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

func Convert_v1beta2_AzureASOManagedControlPlaneStatus_To_v1beta1_AzureASOManagedControlPlaneStatus(in *infrav1beta2.AzureASOManagedControlPlaneStatus, out *AzureASOManagedControlPlaneStatus, s conversion.Scope) error {
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

// AzureASOManagedMachinePoolStatus

func Convert_v1beta1_AzureASOManagedMachinePoolStatus_To_v1beta2_AzureASOManagedMachinePoolStatus(in *AzureASOManagedMachinePoolStatus, out *infrav1beta2.AzureASOManagedMachinePoolStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_AzureASOManagedMachinePoolStatus_To_v1beta2_AzureASOManagedMachinePoolStatus(in, out, s); err != nil {
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

func Convert_v1beta2_AzureASOManagedMachinePoolStatus_To_v1beta1_AzureASOManagedMachinePoolStatus(in *infrav1beta2.AzureASOManagedMachinePoolStatus, out *AzureASOManagedMachinePoolStatus, s conversion.Scope) error {
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
