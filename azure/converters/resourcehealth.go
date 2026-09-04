/*
Copyright 2022 The Kubernetes Authors.

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

package converters

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcehealth/armresourcehealth"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

// SDKAvailabilityStatusToCondition converts an Azure Resource Health availability status to a status condition.
func SDKAvailabilityStatusToCondition(availStatus armresourcehealth.AvailabilityStatus) *metav1.Condition {
	if availStatus.Properties == nil {
		return &metav1.Condition{Type: string(infrav1.AzureResourceAvailableCondition), Status: metav1.ConditionFalse, Reason: "Unknown"}
	}

	state := ptr.Deref(availStatus.Properties.AvailabilityState, armresourcehealth.AvailabilityStateValuesUnknown)

	if state == armresourcehealth.AvailabilityStateValuesAvailable {
		return &metav1.Condition{Type: string(infrav1.AzureResourceAvailableCondition), Status: metav1.ConditionTrue, Reason: string(infrav1.AzureResourceAvailableCondition)}
	}
	if state == "" {
		state = armresourcehealth.AvailabilityStateValuesUnknown
	}

	var message string
	if availStatus.Properties.Summary != nil {
		message = *availStatus.Properties.Summary
	}

	return &metav1.Condition{Type: string(infrav1.AzureResourceAvailableCondition), Status: metav1.ConditionFalse, Reason: string(state), Message: message}
}
