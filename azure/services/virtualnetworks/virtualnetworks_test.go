/*
Copyright 2019 The Kubernetes Authors.

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

package virtualnetworks

import (
	"context"
	"errors"
	"testing"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	"github.com/Azure/azure-service-operator/v2/pkg/common/labels"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualnetworks/mock_virtualnetworks"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPostCreateOrUpdateResourceHook(t *testing.T) {
	t.Run("failed to create or update", func(t *testing.T) {
		g := NewGomegaWithT(t)
		mockCtrl := gomock.NewController(t)
		scope := mock_virtualnetworks.NewMockVNetScope(mockCtrl)
		err := errors.New("an error")
		g.Expect(postCreateOrUpdateResourceHook(context.Background(), scope, nil, err)).To(MatchError(err))
	})

	t.Run("successfully created or updated", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)
		scope := mock_virtualnetworks.NewMockVNetScope(mockCtrl)

		existing := &asonetworkv1.VirtualNetwork{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vnet",
			},
			Status: asonetworkv1.VirtualNetwork_STATUS{
				Id:   ptr.To("id"),
				Tags: map[string]string{"actual": "tags"},
				AddressSpace: &asonetworkv1.AddressSpace_STATUS{
					AddressPrefixes: []string{"cidr"},
				},
			},
		}

		vnet := &infrav1.VnetSpec{}
		scope.EXPECT().Vnet().Return(vnet)

		subnets := []client.Object{
			&asonetworkv1.VirtualNetworksSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "subnet",
					Labels: map[string]string{
						labels.OwnerNameLabel: existing.Name,
					},
				},
				Spec: asonetworkv1.VirtualNetworks_Subnet_Spec{
					AzureName: "azure-name",
				},
				Status: asonetworkv1.VirtualNetworks_Subnet_STATUS{
					AddressPrefixes: []string{"address prefixes"},
				},
			},
			&asonetworkv1.VirtualNetworksSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other subnet",
				},
				Spec: asonetworkv1.VirtualNetworks_Subnet_Spec{
					Owner: &genruntime.KnownResourceReference{
						Name: "not this vnet",
					},
				},
			},
		}
		scope.EXPECT().UpdateSubnetCIDRs("azure-name", []string{"address prefixes"})

		s := runtime.NewScheme()
		g.Expect(asonetworkv1.AddToScheme(s)).To(Succeed())
		c := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(subnets...).
			Build()
		scope.EXPECT().GetClient().Return(c)

		g.Expect(postCreateOrUpdateResourceHook(context.Background(), scope, existing, nil)).To(Succeed())

		g.Expect(vnet.ID).To(Equal("id"))
		g.Expect(vnet.Tags).To(Equal(infrav1.Tags{"actual": "tags"}))
		g.Expect(vnet.CIDRBlocks).To(Equal([]string{"cidr"}))
	})
}
