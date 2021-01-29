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

package scope

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
)

func TestMachinePoolScope_Name(t *testing.T) {
	tests := []struct {
		name             string
		machinePoolScope MachinePoolScope
		want             string
		testLength       bool
	}{
		{
			name: "linux can be any length",
			machinePoolScope: MachinePoolScope{
				MachinePool: nil,
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-really-really-long-name",
					},
				},
				ClusterScoper: nil,
			},
			want: "some-really-really-long-name",
		},
		{
			name: "windows longer than 9 should be shortened",
			machinePoolScope: MachinePoolScope{
				MachinePool: nil,
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-90123456",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Windows",
							},
						},
					},
				},
				ClusterScoper: nil,
			},
			want: "win-23456",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.machinePoolScope.Name()
			if got != tt.want {
				t.Errorf("MachinePoolScope.Name() = %v, want %v", got, tt.want)
			}

			if tt.testLength && len(got) > 9 {
				t.Errorf("Length of MachinePoolScope.Name() = %v, want less than %v", len(got), 9)
			}
		})
	}
}
