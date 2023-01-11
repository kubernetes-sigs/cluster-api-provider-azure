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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestGetDiagnosticsProfile(t *testing.T) {
	tests := []struct {
		name        string
		diagnostics *infrav1.Diagnostics
		want        *compute.DiagnosticsProfile
	}{
		{
			name: "managed diagnostics",
			diagnostics: &infrav1.Diagnostics{
				Boot: &infrav1.BootDiagnostics{
					StorageAccountType: infrav1.ManagedDiagnosticsStorage,
				},
			},
			want: &compute.DiagnosticsProfile{
				BootDiagnostics: &compute.BootDiagnostics{
					Enabled: pointer.Bool(true),
				},
			},
		},
		{
			name: "user managed diagnostics",
			diagnostics: &infrav1.Diagnostics{
				Boot: &infrav1.BootDiagnostics{
					StorageAccountType: infrav1.UserManagedDiagnosticsStorage,
					UserManaged: &infrav1.UserManagedBootDiagnostics{
						StorageAccountURI: "https://fake",
					},
				},
			},
			want: &compute.DiagnosticsProfile{
				BootDiagnostics: &compute.BootDiagnostics{
					Enabled:    pointer.Bool(true),
					StorageURI: pointer.String("https://fake"),
				},
			},
		},
		{
			name: "disabled diagnostics",
			diagnostics: &infrav1.Diagnostics{
				Boot: &infrav1.BootDiagnostics{
					StorageAccountType: infrav1.DisabledDiagnosticsStorage,
				},
			},
			want: &compute.DiagnosticsProfile{
				BootDiagnostics: &compute.BootDiagnostics{
					Enabled: pointer.Bool(false),
				},
			},
		},
		{
			name: "nil diagnostics boot",
			diagnostics: &infrav1.Diagnostics{
				Boot: nil,
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GetDiagnosticsProfile(tt.diagnostics)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetDiagnosticsProfile(%s) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
