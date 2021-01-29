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

package converters

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

var sampleSubjectFactory = []infrav1.UserAssignedIdentity{
	{
		ProviderID: "azure:///foo",
	},
	{
		ProviderID: "azure:///bar",
	},
	{
		ProviderID: "/without/prefix",
	},
}

var expectedVMSDKObject = map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue{
	"foo":             {},
	"bar":             {},
	"/without/prefix": {},
}

var expectedVMSSSDKObject = map[string]*compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue{
	"foo":             {},
	"bar":             {},
	"/without/prefix": {},
}

func Test_UserAssignedIdentitiesToVMSDK(t *testing.T) {
	cases := []struct {
		Name           string
		SubjectFactory []infrav1.UserAssignedIdentity
		Expect         func(*gomega.GomegaWithT, map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, error)
	}{
		{
			Name:           "ShouldPopulateWithData",
			SubjectFactory: sampleSubjectFactory,
			Expect: func(g *gomega.GomegaWithT, m map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, err error) {
				g.Expect(err).Should(gomega.BeNil())
				g.Expect(m).Should(gomega.Equal(expectedVMSDKObject))
			},
		},

		{
			Name:           "ShouldFailWithError",
			SubjectFactory: []infrav1.UserAssignedIdentity{},
			Expect: func(g *gomega.GomegaWithT, m map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, err error) {
				g.Expect(err).Should(gomega.Equal(ErrUserAssignedIdentitiesNotFound))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			subject, err := UserAssignedIdentitiesToVMSDK(c.SubjectFactory)
			c.Expect(g, subject, err)
		})
	}
}

func Test_UserAssignedIdentitiesToVMSSSDK(t *testing.T) {
	cases := []struct {
		Name           string
		SubjectFactory []infrav1.UserAssignedIdentity
		Expect         func(*gomega.GomegaWithT, map[string]*compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue, error)
	}{
		{
			Name:           "ShouldPopulateWithData",
			SubjectFactory: sampleSubjectFactory,
			Expect: func(g *gomega.GomegaWithT, m map[string]*compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue, err error) {
				g.Expect(err).Should(gomega.BeNil())
				g.Expect(m).Should(gomega.Equal(expectedVMSSSDKObject))
			},
		},

		{
			Name:           "ShouldFailWithError",
			SubjectFactory: []infrav1.UserAssignedIdentity{},
			Expect: func(g *gomega.GomegaWithT, m map[string]*compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue, err error) {
				g.Expect(err).Should(gomega.Equal(ErrUserAssignedIdentitiesNotFound))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			subject, err := UserAssignedIdentitiesToVMSSSDK(c.SubjectFactory)
			c.Expect(g, subject, err)
		})
	}
}
