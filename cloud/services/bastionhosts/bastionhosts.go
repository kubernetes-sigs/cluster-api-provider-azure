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

package bastionhosts

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// BastionScope defines the scope interface for a bastion host service.
type BastionScope interface {
	logr.Logger
	azure.ClusterDescriber
	azure.NetworkDescriber
	BastionSpecs() []azure.BastionSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope BastionScope
	client
	subnetsClient   subnets.Client
	publicIPsClient publicips.Client
}

// New creates a new service.
func New(scope BastionScope) *Service {
	return &Service{
		Scope:           scope,
		client:          newClient(scope),
		subnetsClient:   subnets.NewClient(scope),
		publicIPsClient: publicips.NewClient(scope),
	}
}

// Reconcile gets/creates/updates a bastion host.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "bastionhosts.Service.Reconcile")
	defer span.End()

	for _, bastionSpec := range s.Scope.BastionSpecs() {
		s.Scope.V(2).Info("getting subnet in vnet", "subnet", bastionSpec.SubnetName, "vNet", bastionSpec.VNetName)
		subnet, err := s.subnetsClient.Get(ctx, s.Scope.ResourceGroup(), bastionSpec.VNetName, bastionSpec.SubnetName)
		if err != nil {
			return errors.Wrap(err, "failed to get subnet")
		}
		s.Scope.V(2).Info("successfully got subnet in vnet", "subnet", bastionSpec.SubnetName, "vNet", bastionSpec.VNetName)

		s.Scope.V(2).Info("checking if public ip exist otherwise will try to create", "publicIP", bastionSpec.PublicIPName)
		publicIP := network.PublicIPAddress{}
		publicIP, err = s.publicIPsClient.Get(ctx, s.Scope.ResourceGroup(), bastionSpec.PublicIPName)
		if err != nil && azure.ResourceNotFound(err) {
			iperr := s.createBastionPublicIP(ctx, bastionSpec.PublicIPName)
			if iperr != nil {
				return errors.Wrap(iperr, "failed to create bastion publicIP")
			}
			var errPublicIP error
			publicIP, errPublicIP = s.publicIPsClient.Get(ctx, s.Scope.ResourceGroup(), bastionSpec.PublicIPName)
			if errPublicIP != nil {
				return errors.Wrap(errPublicIP, "failed to get created publicIP")
			}
		} else if err != nil {
			return errors.Wrap(err, "failed to get existing publicIP")
		}
		s.Scope.V(2).Info("successfully got public ip", "publicIP", bastionSpec.PublicIPName)

		s.Scope.V(2).Info("creating bastion host", "bastion", bastionSpec.Name)
		bastionHostIPConfigName := fmt.Sprintf("%s-%s", bastionSpec.Name, "bastionIP")
		err = s.client.CreateOrUpdate(
			ctx,
			s.Scope.ResourceGroup(),
			bastionSpec.Name,
			network.BastionHost{
				Name:     to.StringPtr(bastionSpec.Name),
				Location: to.StringPtr(s.Scope.Location()),
				Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
					ClusterName: s.Scope.ClusterName(),
					Lifecycle:   infrav1.ResourceLifecycleOwned,
					Name:        to.StringPtr(bastionSpec.Name),
					Role:        to.StringPtr("Bastion"),
				})),
				BastionHostPropertiesFormat: &network.BastionHostPropertiesFormat{
					DNSName: to.StringPtr(fmt.Sprintf("%s-bastion", strings.ToLower(bastionSpec.Name))),
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
								PrivateIPAllocationMethod: network.Static,
							},
						},
					},
				},
			},
		)
		if err != nil {
			return errors.Wrap(err, "cannot create bastion host")
		}

		s.Scope.V(2).Info("successfully created bastion host", "bastion", bastionSpec.Name)
	}
	return nil
}

// Delete deletes the bastion host with the provided scope.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "bastionhosts.Service.Delete")
	defer span.End()

	for _, bastionSpec := range s.Scope.BastionSpecs() {

		s.Scope.V(2).Info("deleting bastion host", "bastion", bastionSpec.Name)

		err := s.client.Delete(ctx, s.Scope.ResourceGroup(), bastionSpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete Bastion Host %s in resource group %s", bastionSpec.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("successfully deleted bastion host", "bastion", bastionSpec.Name)
	}
	return nil
}

func (s *Service) createBastionPublicIP(ctx context.Context, ipName string) error {
	ctx, span := tele.Tracer().Start(ctx, "bastionhosts.Service.createBastionPublicIP")
	defer span.End()

	s.Scope.V(2).Info("creating bastion public IP", "public IP", ipName)
	return s.publicIPsClient.CreateOrUpdate(
		ctx,
		s.Scope.ResourceGroup(),
		ipName,
		network.PublicIPAddress{
			Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
			Name:     to.StringPtr(ipName),
			Location: to.StringPtr(s.Scope.Location()),
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				PublicIPAddressVersion:   network.IPv4,
				PublicIPAllocationMethod: network.Static,
				DNSSettings: &network.PublicIPAddressDNSSettings{
					DomainNameLabel: to.StringPtr(strings.ToLower(ipName)),
				},
			},
		},
	)
}
