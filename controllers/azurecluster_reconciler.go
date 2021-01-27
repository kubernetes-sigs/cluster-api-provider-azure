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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/privatedns"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureClusterService is the reconciler called by the AzureCluster controller
type azureClusterService struct {
	scope            *scope.ClusterScope
	groupsSvc        azure.Service
	vnetSvc          azure.Service
	securityGroupSvc azure.Service
	routeTableSvc    azure.Service
	subnetsSvc       azure.Service
	publicIPSvc      azure.Service
	loadBalancerSvc  azure.Service
	privateDNSSvc    azure.Service
	skuCache         *resourceskus.Cache
}

// newAzureClusterService populates all the services based on input scope
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
		subnetsSvc:       subnets.New(scope),
		publicIPSvc:      publicips.New(scope),
		loadBalancerSvc:  loadbalancers.New(scope),
		privateDNSSvc:    privatedns.New(scope),
		skuCache:         skuCache,
	}, nil
}

var _ azure.Service = (*azureClusterService)(nil)

// Reconcile reconciles all the services in pre determined order
func (s *azureClusterService) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureClusterService.Reconcile")
	defer span.End()

	if err := s.setFailureDomainsForLocation(ctx); err != nil {
		return errors.Wrapf(err, "failed to get availability zones")
	}

	s.scope.SetDNSName()
	s.scope.SetControlPlaneIngressRules()

	if err := s.groupsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile resource group")
	}

	if err := s.vnetSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile virtual network")
	}

	if err := s.securityGroupSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile network security group")
	}

	if err := s.routeTableSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile route table")
	}

	if err := s.subnetsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile subnet")
	}

	if err := s.publicIPSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile public IP")
	}

	if err := s.loadBalancerSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile load balancer")
	}

	if err := s.privateDNSSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile private dns")
	}

	return nil
}

// Delete reconciles all the services in pre determined order
func (s *azureClusterService) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureClusterService.Delete")
	defer span.End()

	if err := s.groupsSvc.Delete(ctx); err != nil {
		if errors.Is(err, azure.ErrNotOwned) {
			if err := s.privateDNSSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete private dns")
			}

			if err := s.loadBalancerSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete load balancer")
			}

			if err := s.publicIPSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete public IP")
			}

			if err := s.subnetsSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete subnet")
			}

			if err := s.routeTableSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete route table")
			}

			if err := s.securityGroupSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete network security group")
			}

			if err := s.vnetSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete virtual network")
			}

		} else {
			return errors.Wrapf(err, "failed to delete resource group")
		}
	}

	return nil
}

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
