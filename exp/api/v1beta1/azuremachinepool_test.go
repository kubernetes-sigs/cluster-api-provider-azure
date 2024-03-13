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

package v1beta1_test

import (
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
)

func TestAzureMachinePool_Validate(t *testing.T) {
	cases := []struct {
		Name    string
		Factory func(g *gomega.GomegaWithT) *infrav1exp.AzureMachinePool
		Expect  func(g *gomega.GomegaWithT, actual error)
	}{
		{
			Name: "HasNoImage",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return new(infrav1exp.AzureMachinePool)
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).NotTo(gomega.HaveOccurred())
			},
		},
		{
			Name: "HasValidImage",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							Image: &infrav1.Image{
								SharedGallery: &infrav1.AzureSharedGalleryImage{
									SubscriptionID: "foo",
									ResourceGroup:  "blah",
									Name:           "bin",
									Gallery:        "bazz",
									Version:        "1.2.3",
								},
							},
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).NotTo(gomega.HaveOccurred())
			},
		},
		{
			Name: "HasInvalidImage",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							Image: new(infrav1.Image),
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).To(gomega.HaveOccurred())
				g.Expect(actual.Error()).To(gomega.ContainSubstring("You must supply an ID, Marketplace or ComputeGallery image details"))
			},
		},
		{
			Name: "HasValidTerminateNotificationTimeout",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							TerminateNotificationTimeout: ptr.To(7),
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).NotTo(gomega.HaveOccurred())
			},
		},
		{
			Name: "HasInvalidMaximumTerminateNotificationTimeout",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							TerminateNotificationTimeout: ptr.To(20),
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).To(gomega.HaveOccurred())
				g.Expect(actual.Error()).To(gomega.ContainSubstring("maximum timeout 15 is allowed for TerminateNotificationTimeout"))
			},
		},
		{
			Name: "HasInvalidMinimumTerminateNotificationTimeout",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							TerminateNotificationTimeout: ptr.To(3),
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).To(gomega.HaveOccurred())
				g.Expect(actual.Error()).To(gomega.ContainSubstring("minimum timeout 5 is allowed for TerminateNotificationTimeout"))
			},
		},
		{
			Name: "HasNoDiagnostics",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							Diagnostics: nil,
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).NotTo(gomega.HaveOccurred())
			},
		},
		{
			Name: "HasValidDiagnostics",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							Diagnostics: &infrav1.Diagnostics{
								Boot: &infrav1.BootDiagnostics{
									StorageAccountType: infrav1.ManagedDiagnosticsStorage,
								},
							},
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).NotTo(gomega.HaveOccurred())
			},
		},
		{
			Name: "HasMismatcingManagedDiagnosticsWithStorageAccountURI",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							Diagnostics: &infrav1.Diagnostics{
								Boot: &infrav1.BootDiagnostics{
									StorageAccountType: infrav1.ManagedDiagnosticsStorage,
									UserManaged: &infrav1.UserManagedBootDiagnostics{
										StorageAccountURI: "https://fake",
									},
								},
							},
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).To(gomega.HaveOccurred())
			},
		},
		{
			Name: "HasMismatcingDisabledDiagnosticsWithStorageAccountURI",
			Factory: func(_ *gomega.GomegaWithT) *infrav1exp.AzureMachinePool {
				return &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							Diagnostics: &infrav1.Diagnostics{
								Boot: &infrav1.BootDiagnostics{
									StorageAccountType: infrav1.DisabledDiagnosticsStorage,
									UserManaged: &infrav1.UserManagedBootDiagnostics{
										StorageAccountURI: "https://fake",
									},
								},
							},
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).To(gomega.HaveOccurred())
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			// Don't add t.Parallel() here or the test will fail.
			g := gomega.NewGomegaWithT(t)
			amp := c.Factory(g)
			actualErr := amp.Validate(nil, nil)
			c.Expect(g, actualErr)
		})
	}
}
