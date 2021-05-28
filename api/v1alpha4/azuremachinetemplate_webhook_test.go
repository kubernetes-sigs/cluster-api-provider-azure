/*
Copyright 2021 The Kubernetes Authors.

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

package v1alpha4

import (
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAzureMachineTemplate_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)
	failureDomain := "domaintest"

	tests := []struct {
		name        string
		oldTemplate *AzureMachineTemplate
		template    *AzureMachineTemplate
		wantErr     bool
	}{
		{
			name: "AzureMachineTemplate with immutable spec",
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:     "type",
								DiskSizeGB: to.Int32Ptr(11),
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "",
						},
					},
				},
			},
			template: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size1",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:     "type",
								DiskSizeGB: to.Int32Ptr(11),
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureMachineTemplate with mutable metadata",
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:     "type",
								DiskSizeGB: to.Int32Ptr(11),
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "",
						},
					},
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "OldTemplate",
				},
			},
			template: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:     "type",
								DiskSizeGB: to.Int32Ptr(11),
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "",
						},
					},
				},
				ObjectMeta: v1.ObjectMeta{
					Name: "NewTemplate",
				},
			},
			wantErr: false,
		},
	}

	for _, amt := range tests {
		t.Run(amt.name, func(t *testing.T) {
			t.Parallel()
			err := amt.template.ValidateUpdate(amt.oldTemplate)
			if amt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}

}
