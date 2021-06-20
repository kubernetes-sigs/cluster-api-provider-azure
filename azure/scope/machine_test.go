/*
Copyright 2018 The Kubernetes Authors.

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
	"testing"

	"github.com/Azure/go-autorest/autorest/to"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
)

func TestMachineScope_Name(t *testing.T) {
	tests := []struct {
		name         string
		machineScope MachineScope
		want         string
		testLength   bool
	}{
		{
			name: "if provider ID exists, use it",
			machineScope: MachineScope{
				AzureMachine: &infrav1.AzureMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-with-a-long-name",
					},
					Spec: infrav1.AzureMachineSpec{
						ProviderID: to.StringPtr("azure://compute/virtual-machines/machine-name"),
						OSDisk: infrav1.OSDisk{
							OSType: "Windows",
						},
					},
				},
			},
			want: "machine-name",
		},
		{
			name: "linux can be any length",
			machineScope: MachineScope{
				AzureMachine: &infrav1.AzureMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-with-really-really-long-name",
					},
					Spec: infrav1.AzureMachineSpec{
						OSDisk: infrav1.OSDisk{
							OSType: "Linux",
						},
					},
				},
			},
			want: "machine-with-really-really-long-name",
		},
		{
			name: "Windows name with long MachineName and short cluster name",
			machineScope: MachineScope{
				ClusterScoper: &ClusterScope{
					Cluster: &clusterv1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster",
						},
					},
				},
				AzureMachine: &infrav1.AzureMachine{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-90123456",
					},
					Spec: infrav1.AzureMachineSpec{
						OSDisk: infrav1.OSDisk{
							OSType: "Windows",
						},
					},
					Status: infrav1.AzureMachineStatus{},
				},
			},
			want:       "machine-9-23456",
			testLength: true,
		},
		{
			name: "Windows name with long MachineName and long cluster name",
			machineScope: MachineScope{
				ClusterScoper: &ClusterScope{
					Cluster: &clusterv1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster8901234",
						},
					},
				},
				AzureMachine: &infrav1.AzureMachine{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-90123456",
					},
					Spec: infrav1.AzureMachineSpec{
						OSDisk: infrav1.OSDisk{
							OSType: "Windows",
						},
					},
					Status: infrav1.AzureMachineStatus{},
				},
			},
			want:       "machine-9-23456",
			testLength: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.machineScope.Name()
			if got != tt.want {
				t.Errorf("MachineScope.Name() = %v, want %v", got, tt.want)
			}

			if tt.testLength && len(got) > 15 {
				t.Errorf("Length of MachineScope.Name() = %v, want less than %v", len(got), 15)
			}
		})
	}
}

func TestMachineScope_GetVMID(t *testing.T) {
	tests := []struct {
		name         string
		machineScope MachineScope
		want         string
	}{
		{
			name: "returns the vm name from provider ID",
			machineScope: MachineScope{
				AzureMachine: &infrav1.AzureMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "not-this-name",
					},
					Spec: infrav1.AzureMachineSpec{
						ProviderID: to.StringPtr("azure://compute/virtual-machines/machine-name"),
					},
				},
			},
			want: "machine-name",
		},
		{
			name: "returns empty if provider ID is invalid",
			machineScope: MachineScope{
				AzureMachine: &infrav1.AzureMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-name",
					},
					Spec: infrav1.AzureMachineSpec{
						ProviderID: to.StringPtr("foo"),
					},
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.machineScope.GetVMID()
			if got != tt.want {
				t.Errorf("MachineScope.GetVMID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMachineScope_ProviderID(t *testing.T) {
	tests := []struct {
		name         string
		machineScope MachineScope
		want         string
	}{
		{
			name: "returns the entire provider ID",
			machineScope: MachineScope{
				AzureMachine: &infrav1.AzureMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "not-this-name",
					},
					Spec: infrav1.AzureMachineSpec{
						ProviderID: to.StringPtr("azure://compute/virtual-machines/machine-name"),
					},
				},
			},
			want: "azure://compute/virtual-machines/machine-name",
		},
		{
			name: "returns empty if provider ID is invalid",
			machineScope: MachineScope{
				AzureMachine: &infrav1.AzureMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-name",
					},
					Spec: infrav1.AzureMachineSpec{
						ProviderID: to.StringPtr("foo"),
					},
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.machineScope.ProviderID()
			if got != tt.want {
				t.Errorf("MachineScope.ProviderID() = %v, want %v", got, tt.want)
			}
		})
	}
}
