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

package controllers

import (
	"context"

	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/bastionhosts"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/natgateways"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/privatedns"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualnetworks"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureClusterService is the reconciler called by the AzureCluster controller.
type azureClusterService struct {
	scope            *scope.ClusterScope
	groupsSvc        azure.Reconciler
	vnetSvc          azure.Reconciler
	securityGroupSvc azure.Reconciler
	routeTableSvc    azure.Reconciler
	subnetsSvc       azure.Reconciler
	publicIPSvc      azure.Reconciler
	loadBalancerSvc  azure.Reconciler
	privateDNSSvc    azure.Reconciler
	bastionSvc       azure.Reconciler
	skuCache         *resourceskus.Cache
	natGatewaySvc    azure.Reconciler
}

// newAzureClusterService populates all the services based on input scope.
func newAzureClusterService(scope *scope.ClusterScope) (*azureClusterService, error) {
	skuCache, err := resourceskus.GetCache(scope, scope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}

	return &azureClusterService{
		scope:            scope,
		groupsSvc:        groups.New(scope),
		vnetSvc:          virtualnetworks.New(scope),
		securityGroupSvc: securitygroups.New(scope),
		routeTableSvc:    routetables.New(scope),
		natGatewaySvc:    natgateways.New(scope),
		subnetsSvc:       subnets.New(scope),
		publicIPSvc:      publicips.New(scope),
		loadBalancerSvc:  loadbalancers.New(scope),
		privateDNSSvc:    privatedns.New(scope),
		bastionSvc:       bastionhosts.New(scope),
		skuCache:         skuCache,
	}, nil
}

var _ azure.Reconciler = (*azureClusterService)(nil)

// Reconcile reconciles all the services in a predetermined order.
func (s *azureClusterService) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureClusterService.Reconcile")
	defer span.End()

	if err := s.setFailureDomainsForLocation(ctx); err != nil {
		return errors.Wrap(err, "failed to get availability zones")
	}

	s.scope.SetDNSName()
	s.scope.SetControlPlaneSecurityRules()

	if err := s.groupsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile resource group")
	}

	if err := s.vnetSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile virtual network")
	}

	if err := s.securityGroupSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile network security group")
	}

	if err := s.routeTableSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile route table")
	}

	if err := s.publicIPSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile public IP")
	}

	if err := s.natGatewaySvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile nat gateway")
	}

	if err := s.subnetsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile subnet")
	}

	if err := s.loadBalancerSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile load balancer")
	}

	if err := s.privateDNSSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile private dns")
	}

	if err := s.bastionSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile bastion")
	}

	return nil
}

// Delete reconciles all the services in a predetermined order.
func (s *azureClusterService) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureClusterService.Delete")
	defer span.End()

	if err := s.groupsSvc.Delete(ctx); err != nil {
		if errors.Is(err, azure.ErrNotOwned) {
			if err := s.privateDNSSvc.Delete(ctx); err != nil {
				return errors.Wrap(err, "failed to delete private dns")
			}

			if err := s.loadBalancerSvc.Delete(ctx); err != nil {
				return errors.Wrap(err, "failed to delete load balancer")
			}

			if err := s.subnetsSvc.Delete(ctx); err != nil {
				return errors.Wrap(err, "failed to delete subnet")
			}

			if err := s.natGatewaySvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete nat gateway")
			}

			if err := s.publicIPSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete public IP")
			}

			if err := s.routeTableSvc.Delete(ctx); err != nil {
				return errors.Wrap(err, "failed to delete route table")
			}

			if err := s.securityGroupSvc.Delete(ctx); err != nil {
				return errors.Wrap(err, "failed to delete network security group")
			}

			if err := s.vnetSvc.Delete(ctx); err != nil {
				return errors.Wrap(err, "failed to delete virtual network")
			}

			if err := s.bastionSvc.Delete(ctx); err != nil {
				return errors.Wrap(err, "failed to delete bastion")
			}
		} else {
			return errors.Wrap(err, "failed to delete resource group")
		}
	}

	return nil
}

// setFailureDomainsForLocation sets the AzureCluster Status failure domains based on which Azure Availability Zones are available in the cluster location.
// Note that this is not done in a webhook as it requires API calls to fetch the availability zones.
func (s *azureClusterService) setFailureDomainsForLocation(ctx context.Context) error {
	zones, err := s.skuCache.GetZones(ctx, s.scope.Location())
	if err != nil {
		return errors.Wrapf(err, "failed to get zones for location %s", s.scope.Location())
	}

	for _, zone := range zones {
		s.scope.SetFailureDomain(zone, clusterv1.FailureDomainSpec{
			ControlPlane: true,
		})
	}

	return nil
}
