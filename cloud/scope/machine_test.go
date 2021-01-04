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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

func TestMachineScope_Name(t *testing.T) {
	type fields struct {
		ClusterScoper azure.ClusterScoper
		AzureMachine  *infrav1.AzureMachine
	}
	tests := []struct {
		name         string
		machineScope MachineScope
		want         string
		testLength   bool
	}{
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
			want:       "cluster-23456",
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
			want:       "cluster89-23456",
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
