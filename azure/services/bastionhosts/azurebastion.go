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

package bastionhosts

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

func (s *Service) ensureAzureBastion(ctx context.Context, azureBastionSpec azure.AzureBastionSpec) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "bastionhosts.Service.ensureAzureBastion")
	defer done()

	log.V(2).Info("getting azure bastion public IP", "publicIP", azureBastionSpec.PublicIPName)
	publicIP, err := s.publicIPsClient.Get(ctx, s.Scope.ResourceGroup(), azureBastionSpec.PublicIPName)
	if err != nil {
		return errors.Wrap(err, "failed to get public IP for azure bastion")
	}

	log.V(2).Info("getting azure bastion subnet", "subnet", azureBastionSpec.SubnetSpec)
	subnet, err := s.subnetsClient.Get(ctx, s.Scope.ResourceGroup(), azureBastionSpec.VNetName, azureBastionSpec.SubnetSpec.Name)
	if err != nil {
		return errors.Wrap(err, "failed to get subnet for azure bastion")
	}

	log.V(2).Info("creating bastion host", "bastion", azureBastionSpec.Name)
	bastionHostIPConfigName := fmt.Sprintf("%s-%s", azureBastionSpec.Name, "bastionIP")
	err = s.client.CreateOrUpdate(
		ctx,
		s.Scope.ResourceGroup(),
		azureBastionSpec.Name,
		network.BastionHost{
			Name:     to.StringPtr(azureBastionSpec.Name),
			Location: to.StringPtr(s.Scope.Location()),
			Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
				ClusterName: s.Scope.ClusterName(),
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Name:        to.StringPtr(azureBastionSpec.Name),
				Role:        to.StringPtr("Bastion"),
			})),
			BastionHostPropertiesFormat: &network.BastionHostPropertiesFormat{
				DNSName: to.StringPtr(fmt.Sprintf("%s-bastion", strings.ToLower(azureBastionSpec.Name))),
				IPConfigurations: &[]network.BastionHostIPConfiguration{
					{
						Name: to.StringPtr(bastionHostIPConfigName),
						BastionHostIPConfigurationPropertiesFormat: &network.BastionHostIPConfigurationPropertiesFormat{
							Subnet: &network.SubResource{
								ID: subnet.ID,
							},
							PublicIPAddress: &network.SubResource{
								ID: publicIP.ID,
							},
							PrivateIPAllocationMethod: network.IPAllocationMethodDynamic,
						},
					},
				},
			},
		},
	)
	if err != nil {
		return errors.Wrap(err, "cannot create Azure Bastion")
	}

	log.V(2).Info("successfully created bastion host", "bastion", azureBastionSpec.Name)
	return nil
}

func (s *Service) ensureAzureBastionDeleted(ctx context.Context, azureBastionSpec azure.AzureBastionSpec) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "bastionhosts.Service.ensureAzureBastionDeleted")
	defer done()

	log.V(2).Info("deleting bastion host", "bastion", azureBastionSpec.Name)

	err := s.client.Delete(ctx, s.Scope.ResourceGroup(), azureBastionSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// Resource already deleted, all good.
	} else if err != nil {
		return errors.Wrapf(err, "failed to delete Azure Bastion %s in resource group %s", azureBastionSpec.Name, s.Scope.ResourceGroup())
	}

	log.V(2).Info("successfully deleted bastion host", "bastion", azureBastionSpec.Name)

	return nil
}
