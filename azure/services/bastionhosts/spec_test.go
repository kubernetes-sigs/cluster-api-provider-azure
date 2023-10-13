/*
Copyright 2023 The Kubernetes Authors.

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

package bastionhosts

import (
	"context"
	"fmt"
	"testing"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	fakeSKU         = asonetworkv1.Sku_Name("fake-SKU")
	fakeBastionHost = asonetworkv1.BastionHost{
		Spec: asonetworkv1.BastionHost_Spec{
			Location:  ptr.To(fakeAzureBastionSpec1.Location),
			AzureName: fakeAzureBastionSpec1.Name,
			DnsName:   ptr.To(fakeAzureBastionSpec1.Name + "-bastion"),
			Sku:       &asonetworkv1.Sku{Name: &fakeSKU},
			Owner: &genruntime.KnownResourceReference{
				Name: fakeAzureBastionSpec1.ResourceGroup,
			},
			Tags:            fakeBastionHostTags,
			EnableTunneling: ptr.To(false),
			IpConfigurations: []asonetworkv1.BastionHostIPConfiguration{
				{
					Name: ptr.To(fmt.Sprintf("%s-%s", fakeAzureBastionSpec1.Name, "bastionIP")),
					Subnet: &asonetworkv1.BastionHostSubResource{
						Reference: &genruntime.ResourceReference{
							ARMID: fakeAzureBastionSpec1.SubnetID,
						},
					},
					PublicIPAddress: &asonetworkv1.BastionHostSubResource{
						Reference: &genruntime.ResourceReference{
							ARMID: fakeAzureBastionSpec1.PublicIPID,
						},
					},
					PrivateIPAllocationMethod: ptr.To(asonetworkv1.IPAllocationMethod_Dynamic),
				},
			},
		},
	}
	fakeAzureBastionSpec1 = AzureBastionSpec{
		Name:            "my-bastion-host",
		Namespace:       "default",
		ClusterName:     "cluster",
		Location:        "westus",
		SubnetID:        "my-subnet-id",
		PublicIPID:      "my-public-ip-id",
		Sku:             infrav1.BastionHostSkuName("fake-SKU"),
		EnableTunneling: false,
	}

	fakeBastionHostStatus = asonetworkv1.BastionHost_STATUS{
		Name:              ptr.To(fakeAzureBastionSpec1.Name),
		ProvisioningState: ptr.To(asonetworkv1.BastionHostProvisioningState_STATUS_Succeeded),
	}

	fakeBastionHostTags = map[string]string{
		"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster": "owned",
		"sigs.k8s.io_cluster-api-provider-azure_role":            "Bastion",
		"Name": fakeAzureBastionSpec1.Name,
	}
)

func getASOBastionHost(changes ...func(*asonetworkv1.BastionHost)) *asonetworkv1.BastionHost {
	BastionHost := fakeBastionHost.DeepCopy()
	for _, change := range changes {
		change(BastionHost)
	}
	return BastionHost
}

func TestAzureBastionSpec_Parameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *AzureBastionSpec
		existing      *asonetworkv1.BastionHost
		expect        func(g *WithT, result asonetworkv1.BastionHost)
		expectedError string
	}{
		{
			name:     "Creating a new BastionHost",
			spec:     &fakeAzureBastionSpec1,
			existing: nil,
			expect: func(g *WithT, result asonetworkv1.BastionHost) {
				g.Expect(result).To(Not(BeNil()))

				// ObjectMeta is populated later in the codeflow
				g.Expect(result.ObjectMeta).To(Equal(metav1.ObjectMeta{}))

				// Spec is populated from the spec passed in
				g.Expect(result.Spec).To(Equal(getASOBastionHost().Spec))
			},
		},
		{
			name: "user updates to bastion hosts DisableCopyPaste should be accepted",
			spec: &fakeAzureBastionSpec1,
			existing: getASOBastionHost(
				// user added DisableCopyPaste
				func(bastion *asonetworkv1.BastionHost) {
					bastion.Spec.DisableCopyPaste = ptr.To(true)
				},
				// user added Status
				func(bastion *asonetworkv1.BastionHost) {
					bastion.Status = fakeBastionHostStatus
				},
			),
			expect: func(g *WithT, result asonetworkv1.BastionHost) {
				g.Expect(result).To(Not(BeNil()))
				resultantASOBastionHost := getASOBastionHost(
					func(bastion *asonetworkv1.BastionHost) {
						bastion.Spec.DisableCopyPaste = ptr.To(true)
					},
				)

				// ObjectMeta should be carried over from existing private endpoint.
				g.Expect(result.ObjectMeta).To(Equal(resultantASOBastionHost.ObjectMeta))

				// EnableTunneling addition is accepted.
				g.Expect(result.Spec).To(Equal(resultantASOBastionHost.Spec))

				// Status should be carried over.
				g.Expect(result.Status).To(Equal(fakeBastionHostStatus))
			},
		},
		{
			name: "user updates to ASO's bastion hosts resource and capz should overwrite it",
			spec: &fakeAzureBastionSpec1,
			existing: getASOBastionHost(
				// user added DisableCopyPaste
				func(bastion *asonetworkv1.BastionHost) {
					bastion.Spec.DisableCopyPaste = ptr.To(true)
				},

				// user also added EnableTunneling which should be overwritten by capz
				func(bastion *asonetworkv1.BastionHost) {
					bastion.Spec.EnableTunneling = ptr.To(true)
				},
				// user added Status
				func(bastion *asonetworkv1.BastionHost) {
					bastion.Status = fakeBastionHostStatus
				},
			),
			expect: func(g *WithT, result asonetworkv1.BastionHost) {
				g.Expect(result).To(Not(BeNil()))
				resultantASOBastionHost := getASOBastionHost(
					func(endpoint *asonetworkv1.BastionHost) {
						endpoint.Spec.DisableCopyPaste = ptr.To(true)
					},
				)

				// user changes except DisableCopyPaste should be overwritten.
				g.Expect(result.ObjectMeta).To(Equal(resultantASOBastionHost.ObjectMeta))

				// DisableCopyPaste addition is accepted.
				g.Expect(result.Spec).To(Equal(resultantASOBastionHost.Spec))

				// Status should be carried over.
				g.Expect(result.Status).To(Equal(fakeBastionHostStatus))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, *result)
		})
	}
}
