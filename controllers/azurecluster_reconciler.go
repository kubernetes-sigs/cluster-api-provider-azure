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
	"strconv"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/availabilityzones"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks"
)

// azureClusterReconciler is the reconciler called by the AzureCluster controller
type azureClusterReconciler struct {
	scope                *scope.ClusterScope
	groupsSvc            azure.Service
	vnetSvc              azure.OldService
	securityGroupSvc     azure.OldService
	routeTableSvc        azure.OldService
	subnetsSvc           azure.OldService
	publicIPSvc          azure.Service
	loadBalancerSvc      azure.Service
	availabilityZonesSvc azure.GetterService
}

// newAzureClusterReconciler populates all the services based on input scope
func newAzureClusterReconciler(scope *scope.ClusterScope) *azureClusterReconciler {
	return &azureClusterReconciler{
		scope:                scope,
		groupsSvc:            groups.NewService(scope),
		vnetSvc:              virtualnetworks.NewService(scope),
		securityGroupSvc:     securitygroups.NewService(scope),
		routeTableSvc:        routetables.NewService(scope),
		subnetsSvc:           subnets.NewService(scope),
		publicIPSvc:          publicips.NewService(scope),
		loadBalancerSvc:      loadbalancers.NewService(scope),
		availabilityZonesSvc: availabilityzones.NewService(scope),
	}
}

// Reconcile reconciles all the services in pre determined order
func (r *azureClusterReconciler) Reconcile(ctx context.Context) error {
	klog.V(2).Infof("reconciling cluster %s", r.scope.ClusterName())

	if err := r.scope.SetAPIServerIP(); err != nil {
		return errors.Wrap(err, "failed to set api server IP")
	}

	if err := r.setFailureDomainsForLocation(ctx); err != nil {
		return errors.Wrapf(err, "failed to get availability zones for cluster %s in location %s", r.scope.ClusterName(), r.scope.Location())
	}

	if err := r.groupsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile resource group for cluster %s", r.scope.ClusterName())
	}

	vnetSpec := &virtualnetworks.Spec{
		ResourceGroup: r.scope.Vnet().ResourceGroup,
		Name:          r.scope.Vnet().Name,
		CIDR:          r.scope.Vnet().CidrBlock,
	}
	if err := r.vnetSvc.Reconcile(ctx, vnetSpec); err != nil {
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

	rtSpec := &routetables.Spec{
		Name: r.scope.NodeSubnet().RouteTable.Name,
	}
	if err := r.routeTableSvc.Reconcile(ctx, rtSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile route table %s for cluster %s", r.scope.NodeSubnet().RouteTable.Name, r.scope.ClusterName())
	}

	subnetSpec := &subnets.Spec{
		Name:                r.scope.ControlPlaneSubnet().Name,
		CIDR:                r.scope.ControlPlaneSubnet().CidrBlock,
		VnetName:            r.scope.Vnet().Name,
		SecurityGroupName:   r.scope.ControlPlaneSubnet().SecurityGroup.Name,
		Role:                r.scope.ControlPlaneSubnet().Role,
		RouteTableName:      r.scope.ControlPlaneSubnet().RouteTable.Name,
		InternalLBIPAddress: r.scope.ControlPlaneSubnet().InternalLBIPAddress,
	}
	if err := r.subnetsSvc.Reconcile(ctx, subnetSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane subnet for cluster %s", r.scope.ClusterName())
	}

	subnetSpec = &subnets.Spec{
		Name:              r.scope.NodeSubnet().Name,
		CIDR:              r.scope.NodeSubnet().CidrBlock,
		VnetName:          r.scope.Vnet().Name,
		SecurityGroupName: r.scope.NodeSubnet().SecurityGroup.Name,
		RouteTableName:    r.scope.NodeSubnet().RouteTable.Name,
		Role:              r.scope.NodeSubnet().Role,
	}
	if err := r.subnetsSvc.Reconcile(ctx, subnetSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node subnet for cluster %s", r.scope.ClusterName())
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

	rtSpec := &routetables.Spec{
		Name: r.scope.NodeSubnet().RouteTable.Name,
	}
	if err := r.routeTableSvc.Delete(ctx, rtSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete route table %s for cluster %s", r.scope.NodeSubnet().RouteTable.Name, r.scope.ClusterName())
		}
	}

	if err := r.deleteNSG(ctx); err != nil {
		return errors.Wrap(err, "failed to delete network security group")
	}

	vnetSpec := &virtualnetworks.Spec{
		ResourceGroup: r.scope.Vnet().ResourceGroup,
		Name:          r.scope.Vnet().Name,
	}
	if err := r.vnetSvc.Delete(ctx, vnetSpec); err != nil {
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
		subnetSpec := &subnets.Spec{
			Name:     s.Name,
			VnetName: r.scope.Vnet().Name,
		}
		if err := r.subnetsSvc.Delete(ctx, subnetSpec); err != nil {
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

func (r *azureClusterReconciler) setFailureDomainsForLocation(ctx context.Context) error {
	spec := &availabilityzones.Spec{}
	zonesInterface, err := r.availabilityZonesSvc.Get(ctx, spec)
	if err != nil {
		return err
	}

	zones := zonesInterface.([]string)
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
