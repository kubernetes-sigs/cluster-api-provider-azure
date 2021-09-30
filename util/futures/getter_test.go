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

package futures

import (
	"testing"

	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestGet(t *testing.T) {
	g := NewWithT(t)

	azurecluster := &infrav1.AzureCluster{}

	vmName := "my-vm"
	vnetName := "my-vnet"
	vm := "virtualmachines"
	vnet := "virtualnetworks"
	vmFuture := fakeFuture(vmName, vm)
	vnetFuture := fakeFuture(vnetName, vnet)

	g.Expect(Get(azurecluster, vmName, vm)).To(BeNil())
	g.Expect(Get(azurecluster, vnetName, vnet)).To(BeNil())

	azurecluster.SetFutures(infrav1.Futures{vmFuture, vnetFuture})

	g.Expect(Get(azurecluster, vmName, vm)).To(Equal(&vmFuture))
	g.Expect(Get(azurecluster, vmName, vnet)).To(BeNil())
	g.Expect(Get(azurecluster, vnetName, vnet)).To(Equal(&vnetFuture))
}

func TestHas(t *testing.T) {
	g := NewWithT(t)

	azurecluster := &infrav1.AzureCluster{}

	vmName := "my-vm"
	vm := "virtualmachines"
	vnet := "virtualnetworks"
	vmFuture := fakeFuture(vmName, vm)

	g.Expect(Has(azurecluster, vmName, vm)).To(BeFalse())
	g.Expect(Has(azurecluster, "foo", vm)).To(BeFalse())

	azurecluster.SetFutures(infrav1.Futures{vmFuture})

	g.Expect(Has(azurecluster, vmName, vm)).To(BeTrue())
	g.Expect(Has(azurecluster, "foo", vm)).To(BeFalse())
	g.Expect(Has(azurecluster, vmName, vnet)).To(BeFalse())
}

func fakeFuture(name string, service string) infrav1.Future {
	return infrav1.Future{
		Type:          "PUT",
		Name:          name,
		ResourceGroup: "test-rg",
		Data:          "",
		ServiceName:   service,
	}
}
