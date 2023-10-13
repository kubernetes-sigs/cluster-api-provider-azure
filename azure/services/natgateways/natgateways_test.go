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

package natgateways

import (
	"testing"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/natgateways/mock_natgateways"
)

func TestPostCreateOrUpdateResourceHook(t *testing.T) {
	t.Run("error creating or updating", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		scope := mock_natgateways.NewMockNatGatewayScope(mockCtrl)

		postCreateOrUpdateResourceHook(scope, nil, errors.New("an error"))
	})

	t.Run("successful create or update", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		scope := mock_natgateways.NewMockNatGatewayScope(mockCtrl)

		scope.EXPECT().SetNatGatewayIDInSubnets("dummy-natgateway-name", "dummy-natgateway-id")

		natGateway := &asonetworkv1.NatGateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dummy-natgateway-name",
				Namespace: "dummy",
			},
			Status: asonetworkv1.NatGateway_STATUS{
				Id: ptr.To("dummy-natgateway-id"),
			},
		}

		postCreateOrUpdateResourceHook(scope, natGateway, nil)
	})
}
