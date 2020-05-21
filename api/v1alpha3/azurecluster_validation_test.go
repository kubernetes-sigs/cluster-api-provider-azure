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

package v1alpha3

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestClusterWithVnetResourceGroupValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster with resource group - valid",
		cluster: createValidCluster(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := testCase.cluster.validateCluster()
		g.Expect(err).To(BeNil())
	})
}

func TestClusterValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster - valid",
		cluster: createValidCluster(),
	}

	testCase.cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""

	t.Run(testCase.name, func(t *testing.T) {
		err := testCase.cluster.validateCluster()
		g.Expect(err).To(BeNil())
	})
}

func TestClusterSpecWithVnetResourceGroupValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster spec with resource group - valid",
		cluster: createValidCluster(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		errs := testCase.cluster.validateClusterSpec()
		g.Expect(errs).To(BeNil())
	})
}

func TestClusterSpecValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		cluster *AzureCluster
	}

	testCase := test{
		name:    "azurecluster spec - valid",
		cluster: createValidCluster(),
	}

	testCase.cluster.Spec.NetworkSpec.Vnet.ResourceGroup = ""

	t.Run(testCase.name, func(t *testing.T) {
		errs := testCase.cluster.validateClusterSpec()
		g.Expect(errs).To(BeNil())
	})
}

func TestNetworkSpecWithResourceGroupValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name        string
		networkSpec NetworkSpec
	}

	testCase := test{
		name:        "azurecluster networkspec with resource group - valid",
		networkSpec: createValidNetworkSpec(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateNetworkSpec(testCase.networkSpec, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(BeNil())
	})
}

func TestNetworkSpecInvalidResourceGroup(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name        string
		networkSpec NetworkSpec
	}

	testCase := test{
		name:        "azurecluster networkspec - invalid resource group",
		networkSpec: createValidNetworkSpec(),
	}

	testCase.networkSpec.Vnet.ResourceGroup = "invalid-name###"

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateNetworkSpec(testCase.networkSpec, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.vnet.resourceGroup"))
		g.Expect(errs[0].BadValue).To(BeEquivalentTo(testCase.networkSpec.Vnet.ResourceGroup))
	})
}

func TestNetworkSpecValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name        string
		networkSpec NetworkSpec
	}

	testCase := test{
		name:        "azurecluster networkspec - valid",
		networkSpec: createValidNetworkSpec(),
	}

	testCase.networkSpec.Vnet.ResourceGroup = ""

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateNetworkSpec(testCase.networkSpec, field.NewPath("spec").Child("networkSpec"))
		g.Expect(errs).To(BeNil())
	})
}

func TestResourceGroupValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name          string
		resourceGroup string
	}

	testCase := test{
		name:          "resourcegroup name - valid",
		resourceGroup: "custom-vnet",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := validateResourceGroup(testCase.resourceGroup,
			field.NewPath("spec").Child("networkSpec").Child("vnet").Child("resourceGroup"))
		g.Expect(err).To(BeNil())
	})
}

func TestResourceGroupInvalid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name          string
		resourceGroup string
	}

	testCase := test{
		name:          "resourcegroup name - invalid",
		resourceGroup: "inv@lid-rg",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := validateResourceGroup(testCase.resourceGroup,
			field.NewPath("spec").Child("networkSpec").Child("vnet").Child("resourceGroup"))
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(err.Field).To(Equal("spec.networkSpec.vnet.resourceGroup"))
		g.Expect(err.BadValue).To(BeEquivalentTo(testCase.resourceGroup))
	})
}

func TestSubnetsValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		subnets Subnets
	}

	testCase := test{
		name:    "subnets - valid",
		subnets: createValidSubnets(),
	}

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateSubnets(testCase.subnets,
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(BeNil())
	})
}

func TestSubnetsInvalidSubnetName(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		subnets Subnets
	}

	testCase := test{
		name:    "subnets - invalid subnet name",
		subnets: createValidSubnets(),
	}

	testCase.subnets[0].Name = "invalid-subnet-name-due-to-bracket)"

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateSubnets(testCase.subnets,
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets[0].name"))
		g.Expect(errs[0].BadValue).To(BeEquivalentTo("invalid-subnet-name-due-to-bracket)"))
	})
}

func TestSubnetsInvalidInternalLBIPAddress(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name    string
		subnets Subnets
	}

	testCase := test{
		name:    "subnets - invalid internal load balancer ip address",
		subnets: createValidSubnets(),
	}

	testCase.subnets[0].InternalLBIPAddress = "2550.1.1.1"

	t.Run(testCase.name, func(t *testing.T) {
		errs := validateSubnets(testCase.subnets,
			field.NewPath("spec").Child("networkSpec").Child("subnets"))
		g.Expect(errs).To(HaveLen(1))
		g.Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(errs[0].Field).To(Equal("spec.networkSpec.subnets[0].internalLBIPAddress"))
		g.Expect(errs[0].BadValue).To(BeEquivalentTo("2550.1.1.1"))
	})
}

func TestSubnetNameValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name       string
		subnetName string
	}

	testCase := test{
		name:       "subnet name - valid",
		subnetName: "control-plane-subnet",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := validateSubnetName(testCase.subnetName,
			field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("name"))
		g.Expect(err).To(BeNil())
	})
}

func TestSubnetNameInvalid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name       string
		subnetName string
	}

	testCase := test{
		name:       "subnet name - invalid",
		subnetName: "inv@lid-subnet-name",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := validateSubnetName(testCase.subnetName,
			field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("name"))
		g.Expect(err).NotTo(BeNil())
		g.Expect(err.Type).To(Equal(field.ErrorTypeInvalid))
		g.Expect(err.Field).To(Equal("spec.networkSpec.subnets[0].name"))
		g.Expect(err.BadValue).To(BeEquivalentTo(testCase.subnetName))
	})
}

func TestInternalLBIPAddressValid(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		name                string
		internalLBIPAddress string
	}

	testCase := test{
		name:                "subnet name - invalid",
		internalLBIPAddress: "1.1.1.1",
	}

	t.Run(testCase.name, func(t *testing.T) {
		err := validateInternalLBIPAddress(testCase.internalLBIPAddress,
			field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("internalLBIPAddress"))
		g.Expect(err).To(BeNil())
	})
}

func TestInternalLBIPAddressInvalid(t *testing.T) {
	g := NewWithT(t)

	internalLBIPAddress := "1.1.1"

	err := validateInternalLBIPAddress(internalLBIPAddress,
		field.NewPath("spec").Child("networkSpec").Child("subnets").Index(0).Child("internalLBIPAddress"))
	g.Expect(err).NotTo(BeNil())
	g.Expect(err.Type).To(Equal(field.ErrorTypeInvalid))
	g.Expect(err.Field).To(Equal("spec.networkSpec.subnets[0].internalLBIPAddress"))
	g.Expect(err.BadValue).To(BeEquivalentTo(internalLBIPAddress))
}

func createValidCluster() *AzureCluster {
	return &AzureCluster{
		Spec: AzureClusterSpec{
			NetworkSpec: createValidNetworkSpec(),
		},
	}
}

func createValidNetworkSpec() NetworkSpec {
	return NetworkSpec{
		Vnet: VnetSpec{
			ResourceGroup: "custom-vnet",
			Name:          "my-vnet",
		},
		Subnets: createValidSubnets(),
	}
}

func createValidSubnets() Subnets {
	return Subnets{
		{
			Name: "control-plane-subnet",
			Role: "control-plane",
		},
		{
			Name: "node-subnet",
			Role: "node",
		},
	}
}
