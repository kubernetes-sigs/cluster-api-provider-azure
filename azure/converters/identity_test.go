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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
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
	"/foo":            {},
	"/bar":            {},
	"/without/prefix": {},
}

var expectedVMSSSDKObject = map[string]*armcompute.UserAssignedIdentitiesValue{
	"/foo":            {},
	"/bar":            {},
	"/without/prefix": {},
}

func Test_VMIdentityToVMSDK(t *testing.T) {
	cases := []struct {
		Name         string
		identityType infrav1.VMIdentity
		uami         []infrav1.UserAssignedIdentity
		Expect       func(*GomegaWithT, *compute.VirtualMachineIdentity, error)
	}{
		{
			Name:         "Should return a system assigned identity when identity is system assigned",
			identityType: infrav1.VMIdentitySystemAssigned,
			Expect: func(g *GomegaWithT, m *compute.VirtualMachineIdentity, err error) {
				g.Expect(err).Should(BeNil())
				g.Expect(m).Should(Equal(&compute.VirtualMachineIdentity{
					Type: compute.ResourceIdentityTypeSystemAssigned,
				}))
			},
		},
		{
			Name:         "Should return user assigned identities when identity is user assigned",
			identityType: infrav1.VMIdentityUserAssigned,
			uami:         []infrav1.UserAssignedIdentity{{ProviderID: "my-uami-1"}, {ProviderID: "my-uami-2"}},
			Expect: func(g *GomegaWithT, m *compute.VirtualMachineIdentity, err error) {
				g.Expect(err).Should(BeNil())
				g.Expect(m).Should(Equal(&compute.VirtualMachineIdentity{
					Type: compute.ResourceIdentityTypeUserAssigned,
					UserAssignedIdentities: map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue{
						"my-uami-1": {},
						"my-uami-2": {},
					},
				}))
			},
		},
		{
			Name:         "Should fail when no user assigned identities are specified and identity is user assigned",
			identityType: infrav1.VMIdentityUserAssigned,
			uami:         []infrav1.UserAssignedIdentity{},
			Expect: func(g *GomegaWithT, m *compute.VirtualMachineIdentity, err error) {
				g.Expect(err.Error()).Should(ContainSubstring(ErrUserAssignedIdentitiesNotFound.Error()))
			},
		},
		{
			Name:         "Should return nil if no known identity is specified",
			identityType: "",
			Expect: func(g *GomegaWithT, m *compute.VirtualMachineIdentity, err error) {
				g.Expect(err).Should(BeNil())
				g.Expect(m).Should(BeNil())
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			subject, err := VMIdentityToVMSDK(c.identityType, c.uami)
			c.Expect(g, subject, err)
		})
	}
}

func Test_UserAssignedIdentitiesToVMSDK(t *testing.T) {
	cases := []struct {
		Name           string
		SubjectFactory []infrav1.UserAssignedIdentity
		Expect         func(*GomegaWithT, map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, error)
	}{
		{
			Name:           "ShouldPopulateWithData",
			SubjectFactory: sampleSubjectFactory,
			Expect: func(g *GomegaWithT, m map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, err error) {
				g.Expect(err).Should(BeNil())
				g.Expect(m).Should(Equal(expectedVMSDKObject))
			},
		},

		{
			Name:           "ShouldFailWithError",
			SubjectFactory: []infrav1.UserAssignedIdentity{},
			Expect: func(g *GomegaWithT, m map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, err error) {
				g.Expect(err).Should(Equal(ErrUserAssignedIdentitiesNotFound))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			subject, err := UserAssignedIdentitiesToVMSDK(c.SubjectFactory)
			c.Expect(g, subject, err)
		})
	}
}

func Test_UserAssignedIdentitiesToVMSSSDK(t *testing.T) {
	cases := []struct {
		Name           string
		SubjectFactory []infrav1.UserAssignedIdentity
		Expect         func(*GomegaWithT, map[string]*armcompute.UserAssignedIdentitiesValue, error)
	}{
		{
			Name:           "ShouldPopulateWithData",
			SubjectFactory: sampleSubjectFactory,
			Expect: func(g *GomegaWithT, m map[string]*armcompute.UserAssignedIdentitiesValue, err error) {
				g.Expect(err).Should(BeNil())
				g.Expect(m).Should(Equal(expectedVMSSSDKObject))
			},
		},

		{
			Name:           "ShouldFailWithError",
			SubjectFactory: []infrav1.UserAssignedIdentity{},
			Expect: func(g *GomegaWithT, m map[string]*armcompute.UserAssignedIdentitiesValue, err error) {
				g.Expect(err).Should(Equal(ErrUserAssignedIdentitiesNotFound))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			subject, err := UserAssignedIdentitiesToVMSSSDK(c.SubjectFactory)
			c.Expect(g, subject, err)
		})
	}
}
