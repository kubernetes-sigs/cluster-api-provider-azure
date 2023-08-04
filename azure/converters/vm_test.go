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
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestSDKToVM(t *testing.T) {
	tests := []struct {
		name string
		sdk  compute.VirtualMachine
		want *VM
	}{
		{
			name: "Basic conversion with required fields",
			sdk: compute.VirtualMachine{
				ID:   ptr.To("test-vm-id"),
				Name: ptr.To("test-vm-name"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: ptr.To("Succeeded"),
				},
			},
			want: &VM{
				ID:    "test-vm-id",
				Name:  "test-vm-name",
				State: infrav1.ProvisioningState(compute.ProvisioningStateSucceeded),
			},
		},
		{
			name: "Should convert and populate with VMSize",
			sdk: compute.VirtualMachine{
				ID:   ptr.To("test-vm-id"),
				Name: ptr.To("test-vm-name"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: ptr.To("Succeeded"),
					HardwareProfile: &compute.HardwareProfile{
						VMSize: compute.VirtualMachineSizeTypesStandardA1,
					},
				},
			},
			want: &VM{
				ID:     "test-vm-id",
				Name:   "test-vm-name",
				State:  infrav1.ProvisioningState(compute.ProvisioningStateSucceeded),
				VMSize: "Standard_A1",
			},
		},
		{
			name: "Should convert and populate with availability zones",
			sdk: compute.VirtualMachine{
				ID:   ptr.To("test-vm-id"),
				Name: ptr.To("test-vm-name"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: ptr.To("Succeeded"),
				},
				Zones: &[]string{"1", "2"},
			},
			want: &VM{
				ID:               "test-vm-id",
				Name:             "test-vm-name",
				State:            infrav1.ProvisioningState(compute.ProvisioningStateSucceeded),
				AvailabilityZone: "1",
			},
		},
		{
			name: "Should convert and populate with tags",
			sdk: compute.VirtualMachine{
				ID:   ptr.To("test-vm-id"),
				Name: ptr.To("test-vm-name"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: ptr.To("Succeeded"),
				},
				Tags: map[string]*string{"foo": ptr.To("bar")},
			},
			want: &VM{
				ID:    "test-vm-id",
				Name:  "test-vm-name",
				State: infrav1.ProvisioningState(compute.ProvisioningStateSucceeded),
				Tags:  infrav1.Tags{"foo": "bar"},
			},
		},
		{
			name: "Should convert and populate with all fields",
			sdk: compute.VirtualMachine{
				ID:   ptr.To("test-vm-id"),
				Name: ptr.To("test-vm-name"),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: ptr.To("Succeeded"),
					HardwareProfile: &compute.HardwareProfile{
						VMSize: compute.VirtualMachineSizeTypesStandardA1,
					},
				},
				Zones: &[]string{"1"},
				Tags:  map[string]*string{"foo": ptr.To("bar")},
			},
			want: &VM{
				ID:               "test-vm-id",
				Name:             "test-vm-name",
				State:            infrav1.ProvisioningState(compute.ProvisioningStateSucceeded),
				VMSize:           "Standard_A1",
				AvailabilityZone: "1",
				Tags:             infrav1.Tags{"foo": "bar"},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SDKToVM(tt.sdk)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Diff between expected result and actual result:\n%s", cmp.Diff(tt.want, got))
			}
		})
	}
}
