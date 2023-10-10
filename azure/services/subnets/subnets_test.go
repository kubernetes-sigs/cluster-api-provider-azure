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

package subnets

import (
	"context"
	"testing"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets/mock_subnets"
)

func TestPostCreateOrUpdateResourceHook(t *testing.T) {
	t.Run("error creating or updating", func(t *testing.T) {
		g := NewGomegaWithT(t)
		mockCtrl := gomock.NewController(t)
		scope := mock_subnets.NewMockSubnetScope(mockCtrl)
		err := errors.New("an error")
		g.Expect(postCreateOrUpdateResourceHook(context.Background(), scope, nil, err)).To(MatchError(err))
	})

	t.Run("successfully created or updated", func(t *testing.T) {
		g := NewGomegaWithT(t)
		mockCtrl := gomock.NewController(t)
		scope := mock_subnets.NewMockSubnetScope(mockCtrl)
		scope.EXPECT().UpdateSubnetID("subnet", "id")
		scope.EXPECT().UpdateSubnetCIDRs("subnet", []string{"cidr"})
		subnet := &asonetworkv1.VirtualNetworksSubnet{
			Spec: asonetworkv1.VirtualNetworks_Subnet_Spec{
				AzureName: "subnet",
			},
			Status: asonetworkv1.VirtualNetworks_Subnet_STATUS{
				Id:              ptr.To("id"),
				AddressPrefixes: []string{"cidr"},
			},
		}
		g.Expect(postCreateOrUpdateResourceHook(context.Background(), scope, subnet, nil)).To(Succeed())
	})
}
