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

package v1alpha3_test

import (
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
)

func TestAzureMachinePool_Validate(t *testing.T) {
	cases := []struct {
		Name    string
		Factory func(g *gomega.GomegaWithT) *exp.AzureMachinePool
		Expect  func(g *gomega.GomegaWithT, actual error)
	}{
		{
			Name: "HasNoImage",
			Factory: func(_ *gomega.GomegaWithT) *exp.AzureMachinePool {
				return new(exp.AzureMachinePool)
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).ToNot(gomega.HaveOccurred())
			},
		},
		{
			Name: "HasValidImage",
			Factory: func(_ *gomega.GomegaWithT) *exp.AzureMachinePool {
				return &exp.AzureMachinePool{
					Spec: exp.AzureMachinePoolSpec{
						Template: exp.AzureMachineTemplate{
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
				g.Expect(actual).ToNot(gomega.HaveOccurred())
			},
		},
		{
			Name: "HasInvalidImage",
			Factory: func(_ *gomega.GomegaWithT) *exp.AzureMachinePool {
				return &exp.AzureMachinePool{
					Spec: exp.AzureMachinePoolSpec{
						Template: exp.AzureMachineTemplate{
							Image: new(infrav1.Image),
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).To(gomega.HaveOccurred())
				g.Expect(actual.Error()).To(gomega.ContainSubstring("You must supply a ID, Marketplace or SharedGallery image details"))
			},
		},
		{
			Name: "HasValidTerminateNotificationTimeout",
			Factory: func(_ *gomega.GomegaWithT) *exp.AzureMachinePool {
				return &exp.AzureMachinePool{
					Spec: exp.AzureMachinePoolSpec{
						Template: exp.AzureMachineTemplate{
							TerminateNotificationTimeout: to.IntPtr(7),
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).ToNot(gomega.HaveOccurred())
			},
		},
		{
			Name: "HasInvalidMaximumTerminateNotificationTimeout",
			Factory: func(_ *gomega.GomegaWithT) *exp.AzureMachinePool {
				return &exp.AzureMachinePool{
					Spec: exp.AzureMachinePoolSpec{
						Template: exp.AzureMachineTemplate{
							TerminateNotificationTimeout: to.IntPtr(20),
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).To(gomega.HaveOccurred())
				g.Expect(actual.Error()).To(gomega.ContainSubstring("Maximum timeout 15 is allowed for TerminateNotificationTimeout"))
			},
		},
		{
			Name: "HasInvalidMinimumTerminateNotificationTimeout",
			Factory: func(_ *gomega.GomegaWithT) *exp.AzureMachinePool {
				return &exp.AzureMachinePool{
					Spec: exp.AzureMachinePoolSpec{
						Template: exp.AzureMachineTemplate{
							TerminateNotificationTimeout: to.IntPtr(3),
						},
					},
				}
			},
			Expect: func(g *gomega.GomegaWithT, actual error) {
				g.Expect(actual).To(gomega.HaveOccurred())
				g.Expect(actual.Error()).To(gomega.ContainSubstring("Minimum timeout 5 is allowed for TerminateNotificationTimeout"))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			amp := c.Factory(g)
			actualErr := amp.Validate()
			c.Expect(g, actualErr)
		})
	}
}
