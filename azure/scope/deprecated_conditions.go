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

package scope

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// setV1Beta1ConditionsFromV1Beta2 converts v1beta2 conditions ([]metav1.Condition)
// to v1beta1 format (clusterv1.Conditions) and stores them via the setter.
// This populates Deprecated.V1Beta1.Conditions so that v1beta1 clients see conditions
// when the v1beta2→v1beta1 conversion webhook runs.
//
//nolint:staticcheck // intentional use of deprecated types for v1beta1 backward compat
func setV1Beta1ConditionsFromV1Beta2(setter interface{ SetV1Beta1Conditions(clusterv1.Conditions) }, v1beta2Conditions []metav1.Condition) {
	if len(v1beta2Conditions) == 0 {
		return
	}
	v1beta1Conds := make(clusterv1.Conditions, 0, len(v1beta2Conditions))
	for _, c := range v1beta2Conditions {
		v1beta1Conds = append(v1beta1Conds, clusterv1.Condition{
			Type:               clusterv1.ConditionType(c.Type),
			Status:             corev1.ConditionStatus(c.Status),
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}
	setter.SetV1Beta1Conditions(v1beta1Conds)
}
