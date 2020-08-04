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

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
)

func TestDefaultingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing amcp defaulting webhook with no baseline")
	amcp := &AzureManagedControlPlane{}
	amcp.Default()
	g.Expect(*amcp.Spec.NetworkPlugin).To(Equal("azure"))
	g.Expect(*amcp.Spec.LoadBalancerSKU).To(Equal("standard"))
	g.Expect(*amcp.Spec.NetworkPolicy).To(Equal("calico"))

	t.Logf("Testing amcp defaulting webhook with baseline")
	netPlug := "kubenet"
	lbSKU := "basic"
	netPol := "azure"
	amcp.Spec.NetworkPlugin = &netPlug
	amcp.Spec.LoadBalancerSKU = &lbSKU
	amcp.Spec.NetworkPolicy = &netPol
	amcp.Default()
	g.Expect(*amcp.Spec.NetworkPlugin).To(Equal(netPlug))
	g.Expect(*amcp.Spec.LoadBalancerSKU).To(Equal(lbSKU))
	g.Expect(*amcp.Spec.NetworkPolicy).To(Equal(netPol))

}

func TestValidatingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing valid DNSServiceIP")
	ip := "192.168.0.0"
	amcp := &AzureManagedControlPlane{Spec: AzureManagedControlPlaneSpec{DNSServiceIP: &ip}}
	err := amcp.validateDNSServiceIP()
	g.Expect(err).ToNot(gomega.HaveOccurred())

	t.Logf("Testing invalid DNSServiceIP")
	ip = "192.168.0.0.3"
	err = amcp.validateDNSServiceIP()
	g.Expect(err.Error()).To(gomega.ContainSubstring("DNSServiceIP must be a valid IP"))

}
