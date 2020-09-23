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
	"fmt"
	"github.com/pkg/errors"
	"hash/fnv"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks"
)

// azureClusterReconciler is the reconciler called by the AzureCluster controller
type azureClusterReconciler struct {
	scope            *scope.ClusterScope
	groupsSvc        azure.Service
	vnetSvc          azure.Service
	securityGroupSvc azure.Service
	routeTableSvc    azure.Service
	subnetsSvc       azure.Service
	publicIPSvc      azure.Service
	loadBalancerSvc  azure.Service
	skuCache         *resourceskus.Cache
}

// newAzureClusterReconciler populates all the services based on input scope
func newAzureClusterReconciler(scope *scope.ClusterScope) *azureClusterReconciler {
	return &azureClusterReconciler{
		scope:            scope,
		groupsSvc:        groups.NewService(scope),
		vnetSvc:          virtualnetworks.NewService(scope),
		securityGroupSvc: securitygroups.NewService(scope),
		routeTableSvc:    routetables.NewService(scope),
		subnetsSvc:       subnets.NewService(scope),
		publicIPSvc:      publicips.NewService(scope),
		loadBalancerSvc:  loadbalancers.NewService(scope),
		skuCache:         resourceskus.NewCache(scope, scope.Location()),
	}
}

// Reconcile reconciles all the services in pre determined order
func (r *azureClusterReconciler) Reconcile(ctx context.Context) error {
	if err := r.createOrUpdateNetworkAPIServerIP(); err != nil {
		return errors.Wrapf(err, "failed to create or update network API server IP for cluster %s in location %s", r.scope.ClusterName(), r.scope.Location())
	}

	if err := r.setFailureDomainsForLocation(ctx); err != nil {
		return errors.Wrapf(err, "failed to get availability zones")
	}

	r.scope.SetControlPlaneIngressRules()

	if err := r.groupsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile resource group")
	}

	if err := r.vnetSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile virtual network")
	}

	if err := r.securityGroupSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile network security group")
	}

	if err := r.routeTableSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile route table")
	}

	if err := r.subnetsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile subnet")
	}

	if err := r.publicIPSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile public IP")
	}

	if err := r.loadBalancerSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile load balancer")
	}

	return nil
}

// Delete reconciles all the services in pre determined order
func (r *azureClusterReconciler) Delete(ctx context.Context) error {
	if err := r.groupsSvc.Delete(ctx); err != nil {
		if errors.Is(err, azure.ErrNotOwned) {
			if err := r.loadBalancerSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete load balancer")
			}

			if err := r.publicIPSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete public IP")
			}

			if err := r.subnetsSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete subnet")
			}

			if err := r.routeTableSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete route table")
			}

			if err := r.securityGroupSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete network security group")
			}

			if err := r.vnetSvc.Delete(ctx); err != nil {
				return errors.Wrapf(err, "failed to delete virtual network")
			}
		} else {
			return errors.Wrapf(err, "failed to delete resource group")
		}
	}

	return nil
}

// CreateOrUpdateNetworkAPIServerIP creates or updates public ip name and dns name
func (r *azureClusterReconciler) createOrUpdateNetworkAPIServerIP() error {
	if r.scope.Network().APIServerIP.Name == "" {
		h := fnv.New32a()
		if _, err := h.Write([]byte(fmt.Sprintf("%s/%s/%s", r.scope.SubscriptionID(), r.scope.ResourceGroup(), r.scope.ClusterName()))); err != nil {
			return errors.Wrapf(err, "failed to write hash sum for api server ip")
		}
		r.scope.Network().APIServerIP.Name = azure.GeneratePublicIPName(r.scope.ClusterName(), fmt.Sprintf("%x", h.Sum32()))
	}

	r.scope.Network().APIServerIP.DNSName = r.scope.GenerateFQDN()
	return nil
}

func (r *azureClusterReconciler) setFailureDomainsForLocation(ctx context.Context) error {
	zones, err := r.skuCache.GetZones(ctx, r.scope.Location())
	if err != nil {
		return errors.Wrapf(err, "failed to get zones for location %s", r.scope.Location())
	}

	for _, zone := range zones {
		r.scope.SetFailureDomain(zone, clusterv1.FailureDomainSpec{
			ControlPlane: true,
		})
	}

	return nil
}
