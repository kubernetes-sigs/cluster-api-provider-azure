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

	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestDefaultingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing amcp defaulting webhook with no baseline")
	amcp := &AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
		},
		Spec: AzureManagedControlPlaneSpec{
			ResourceGroupName: "fooRg",
			Location:          "fooLocation",
			Version:           "1.17.5",
		},
	}
	amcp.Default()
	g.Expect(*amcp.Spec.NetworkPlugin).To(Equal("azure"))
	g.Expect(*amcp.Spec.LoadBalancerSKU).To(Equal("Standard"))
	g.Expect(*amcp.Spec.NetworkPolicy).To(Equal("calico"))
	g.Expect(amcp.Spec.Version).To(Equal("v1.17.5"))
	g.Expect(amcp.Spec.SSHPublicKey).NotTo(BeEmpty())
	g.Expect(amcp.Spec.NodeResourceGroupName).To(Equal("MC_fooRg_fooName_fooLocation"))
	g.Expect(amcp.Spec.VirtualNetwork.Name).To(Equal("fooName"))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooName"))

	t.Logf("Testing amcp defaulting webhook with baseline")
	netPlug := "kubenet"
	lbSKU := "Basic"
	netPol := "azure"
	amcp.Spec.NetworkPlugin = &netPlug
	amcp.Spec.LoadBalancerSKU = &lbSKU
	amcp.Spec.NetworkPolicy = &netPol
	amcp.Spec.Version = "9.99.99"
	amcp.Spec.SSHPublicKey = ""
	amcp.Spec.NodeResourceGroupName = "fooNodeRg"
	amcp.Spec.VirtualNetwork.Name = "fooVnetName"
	amcp.Spec.VirtualNetwork.Subnet.Name = "fooSubnetName"
	amcp.Default()
	g.Expect(*amcp.Spec.NetworkPlugin).To(Equal(netPlug))
	g.Expect(*amcp.Spec.LoadBalancerSKU).To(Equal(lbSKU))
	g.Expect(*amcp.Spec.NetworkPolicy).To(Equal(netPol))
	g.Expect(amcp.Spec.Version).To(Equal("v9.99.99"))
	g.Expect(amcp.Spec.SSHPublicKey).NotTo(BeEmpty())
	g.Expect(amcp.Spec.NodeResourceGroupName).To(Equal("fooNodeRg"))
	g.Expect(amcp.Spec.VirtualNetwork.Name).To(Equal("fooVnetName"))
	g.Expect(amcp.Spec.VirtualNetwork.Subnet.Name).To(Equal("fooSubnetName"))
}

func TestValidatingWebhook(t *testing.T) {
	tests := []struct {
		name      string
		amcp      AzureManagedControlPlane
		expectErr bool
	}{
		{
			name: "Testing valid DNSServiceIP",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "v1.17.8",
				},
			},
			expectErr: false,
		},
		{
			name: "Testing invalid DNSServiceIP",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0.3"),
					Version:      "v1.17.8",
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid Version",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "honk",
				},
			},
			expectErr: true,
		},
		{
			name: "not following the kuberntes Version pattern",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "1.19.0",
				},
			},
			expectErr: true,
		},
		{
			name: "Version not set",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "",
				},
			},
			expectErr: true,
		},
		{
			name: "Valid Version",
			amcp: AzureManagedControlPlane{
				Spec: AzureManagedControlPlaneSpec{
					DNSServiceIP: pointer.StringPtr("192.168.0.0"),
					Version:      "v1.17.8",
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			if tt.expectErr {
				g.Expect(tt.amcp.ValidateCreate()).NotTo(Succeed())
			} else {
				g.Expect(tt.amcp.ValidateCreate()).To(Succeed())
			}
		})
	}
}

func TestAzureManagedControlPlane_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name     string
		amcp     *AzureManagedControlPlane
		wantErr  bool
		errorLen int
	}{
		{
			name:    "all valid",
			amcp:    createAzureManagedControlPlane(t, "192.168.0.0", "v1.18.0", generateSSHPublicKey(true)),
			wantErr: false,
		},
		{
			name:     "invalid DNSServiceIP",
			amcp:     createAzureManagedControlPlane(t, "192.168.0.0.3", "v1.18.0", generateSSHPublicKey(true)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name:     "invalid sshKey",
			amcp:     createAzureManagedControlPlane(t, "192.168.0.0", "v1.18.0", generateSSHPublicKey(false)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name:     "invalid sshKey with a simple text and invalid DNSServiceIP",
			amcp:     createAzureManagedControlPlane(t, "192.168.0.0.3", "v1.18.0", "invalid_sshkey_honk"),
			wantErr:  true,
			errorLen: 2,
		},
		{
			name:     "invalid version",
			amcp:     createAzureManagedControlPlane(t, "192.168.0.0", "honk.version", generateSSHPublicKey(true)),
			wantErr:  true,
			errorLen: 1,
		},
		{
			name:     "all invalid version",
			amcp:     createAzureManagedControlPlane(t, "192.168.0.0.5", "honk.version", "invalid_sshkey_honk"),
			wantErr:  true,
			errorLen: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.amcp.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(HaveLen(tc.errorLen))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureManagedControlPlane_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		oldAMCP *AzureManagedControlPlane
		amcp    *AzureManagedControlPlane
		wantErr bool
	}{
		{
			name:    "AzureManagedControlPlane with valid SSHPublicKey",
			oldAMCP: createAzureManagedControlPlane(t, "192.168.0.0", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane(t, "192.168.0.0", "v1.18.0", generateSSHPublicKey(true)),
			wantErr: false,
		},
		{
			name:    "AzureManagedControlPlane with invalid SSHPublicKey",
			oldAMCP: createAzureManagedControlPlane(t, "192.168.0.0", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane(t, "192.168.0.0", "v1.18.0", generateSSHPublicKey(false)),
			wantErr: true,
		},
		{
			name:    "AzureManagedControlPlane with invalid serviceIP",
			oldAMCP: createAzureManagedControlPlane(t, "", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane(t, "192.168.0.0.3", "v1.18.0", generateSSHPublicKey(true)),
			wantErr: true,
		},
		{
			name:    "AzureManagedControlPlane with invalid version",
			oldAMCP: createAzureManagedControlPlane(t, "", "v1.18.0", ""),
			amcp:    createAzureManagedControlPlane(t, "192.168.0.0", "1.999.9", generateSSHPublicKey(true)),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.amcp.ValidateUpdate(tc.oldAMCP)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createAzureManagedControlPlane(t *testing.T, serviceIP, version, sshKey string) *AzureManagedControlPlane {
	return &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			SSHPublicKey: sshKey,
			DNSServiceIP: to.StringPtr(serviceIP),
			Version:      version,
		},
	}
}
