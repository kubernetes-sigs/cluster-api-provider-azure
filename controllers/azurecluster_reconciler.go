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
	"hash/fnv"
	"strconv"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
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
	securityGroupSvc azure.OldService
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
	klog.V(2).Infof("reconciling cluster %s", r.scope.ClusterName())
	if err := r.createOrUpdateNetworkAPIServerIP(); err != nil {
		return errors.Wrapf(err, "failed to create or update network API server IP for cluster %s in location %s", r.scope.ClusterName(), r.scope.Location())
	}

	if err := r.setFailureDomainsForLocation(ctx); err != nil {
		return errors.Wrapf(err, "failed to get availability zones for cluster %s in location %s", r.scope.ClusterName(), r.scope.Location())
	}

	if err := r.groupsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile resource group for cluster %s", r.scope.ClusterName())
	}

	if err := r.vnetSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile virtual network for cluster %s", r.scope.ClusterName())
	}

	cpSubnet := r.scope.ControlPlaneSubnet()
	if cpSubnet.SecurityGroup.IngressRules == nil {
		cpSubnet.SecurityGroup.IngressRules = r.generateControlPlaneIngressRules()
	}

	sgSpec := &securitygroups.Spec{
		Name:           r.scope.ControlPlaneSubnet().SecurityGroup.Name,
		IsControlPlane: true,
	}
	if err := r.securityGroupSvc.Reconcile(ctx, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane network security group for cluster %s", r.scope.ClusterName())
	}

	sgSpec = &securitygroups.Spec{
		Name:           r.scope.NodeSubnet().SecurityGroup.Name,
		IsControlPlane: false,
	}
	if err := r.securityGroupSvc.Reconcile(ctx, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node network security group for cluster %s", r.scope.ClusterName())
	}

	if err := r.routeTableSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile route table %s for cluster %s", r.scope.RouteTable().Name, r.scope.ClusterName())
	}

	if err := r.subnetsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile subnet for cluster %s", r.scope.ClusterName())
	}

	if err := r.publicIPSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile public IPs for cluster %s", r.scope.ClusterName())
	}

	if err := r.loadBalancerSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile load balancers for cluster %s", r.scope.ClusterName())
	}

	return nil
}

// Delete reconciles all the services in pre determined order
func (r *azureClusterReconciler) Delete(ctx context.Context) error {
	if err := r.loadBalancerSvc.Delete(ctx); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete load balancers for cluster %s", r.scope.ClusterName())
		}
	}

	if err := r.deleteSubnets(ctx); err != nil {
		return errors.Wrap(err, "failed to delete subnets")
	}

	if err := r.routeTableSvc.Delete(ctx); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete route table %s for cluster %s", r.scope.RouteTable().Name, r.scope.ClusterName())
		}
	}

	if err := r.deleteNSG(ctx); err != nil {
		return errors.Wrap(err, "failed to delete network security group")
	}

	if err := r.vnetSvc.Delete(ctx); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete virtual network %s for cluster %s", r.scope.Vnet().Name, r.scope.ClusterName())
		}
	}

	if err := r.groupsSvc.Delete(ctx); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete resource group for cluster %s", r.scope.ClusterName())
		}
	}

	return nil
}

func (r *azureClusterReconciler) deleteSubnets(ctx context.Context) error {
	for _, s := range r.scope.Subnets() {
		if err := r.subnetsSvc.Delete(ctx); err != nil {
			if !azure.ResourceNotFound(err) {
				return errors.Wrapf(err, "failed to delete %s subnet for cluster %s", s.Name, r.scope.ClusterName())
			}
		}
	}
	return nil
}

func (r *azureClusterReconciler) deleteNSG(ctx context.Context) error {
	sgSpec := &securitygroups.Spec{
		Name: r.scope.NodeSubnet().SecurityGroup.Name,
	}
	if err := r.securityGroupSvc.Delete(ctx, sgSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete security group %s for cluster %s", r.scope.NodeSubnet().SecurityGroup.Name, r.scope.ClusterName())
		}
	}
	sgSpec = &securitygroups.Spec{
		Name: r.scope.ControlPlaneSubnet().SecurityGroup.Name,
	}
	if err := r.securityGroupSvc.Delete(ctx, sgSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete security group %s for cluster %s", r.scope.ControlPlaneSubnet().SecurityGroup.Name, r.scope.ClusterName())
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

func (r *azureClusterReconciler) generateControlPlaneIngressRules() infrav1.IngressRules {
	apiPort := "6443"
	if r.scope.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		apiPort = strconv.Itoa(
			int(
				to.Int32(r.scope.Cluster.Spec.ClusterNetwork.APIServerPort),
			),
		)
	}

	return infrav1.IngressRules{
		&infrav1.IngressRule{
			Name:             "allow_ssh",
			Description:      "Allow SSH",
			Priority:         100,
			Protocol:         infrav1.SecurityGroupProtocolTCP,
			Source:           to.StringPtr("*"),
			SourcePorts:      to.StringPtr("*"),
			Destination:      to.StringPtr("*"),
			DestinationPorts: to.StringPtr("22"),
		},
		&infrav1.IngressRule{
			Name:             "allow_apiserver",
			Description:      "Allow K8s API Server",
			Priority:         101,
			Protocol:         infrav1.SecurityGroupProtocolTCP,
			Source:           to.StringPtr("*"),
			SourcePorts:      to.StringPtr("*"),
			Destination:      to.StringPtr("*"),
			DestinationPorts: to.StringPtr(apiPort),
		},
	}
}
