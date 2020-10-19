/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

type (
	// VMSSVM defines a VM in a virtual machine scale set.
	VMSSVM struct {
		ID                 string          `json:"id,omitempty"`
		InstanceID         string          `json:"instanceID,omitempty"`
		Name               string          `json:"name,omitempty"`
		AvailabilityZone   string          `json:"availabilityZone,omitempty"`
		State              infrav1.VMState `json:"vmState,omitempty"`
		LatestModelApplied bool            `json:"latestModelApplied,omitempty"`
	}

	// VMSS defines a virtual machine scale set.
	VMSS struct {
		ID        string             `json:"id,omitempty"`
		Name      string             `json:"name,omitempty"`
		Sku       string             `json:"sku,omitempty"`
		Capacity  int64              `json:"capacity,omitempty"`
		Zones     []string           `json:"zones,omitempty"`
		Image     infrav1.Image      `json:"image,omitempty"`
		State     infrav1.VMState    `json:"vmState,omitempty"`
		Identity  infrav1.VMIdentity `json:"identity,omitempty"`
		Tags      infrav1.Tags       `json:"tags,omitempty"`
		Instances []VMSSVM           `json:"instances,omitempty"`
	}
)
