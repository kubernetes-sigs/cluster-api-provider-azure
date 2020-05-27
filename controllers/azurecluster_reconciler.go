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

	"github.com/pkg/errors"
	"k8s.io/klog"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/availabilityzones"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/internalloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks"
)

// azureClusterReconciler is the reconciler called by the AzureCluster controller
type azureClusterReconciler struct {
	scope                *scope.ClusterScope
	groupsSvc            azure.Service
	vnetSvc              azure.Service
	securityGroupSvc     azure.Service
	routeTableSvc        azure.Service
	subnetsSvc           azure.Service
	internalLBSvc        azure.Service
	publicIPSvc          azure.Service
	publicLBSvc          azure.Service
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
		internalLBSvc:        internalloadbalancers.NewService(scope),
		publicIPSvc:          publicips.NewService(scope),
		publicLBSvc:          publicloadbalancers.NewService(scope),
		availabilityZonesSvc: availabilityzones.NewService(scope),
	}
}

// Reconcile reconciles all the services in pre determined order
func (r *azureClusterReconciler) Reconcile(ctx context.Context) error {
	klog.V(2).Infof("reconciling cluster %s", r.scope.Name())
	if err := r.createOrUpdateNetworkAPIServerIP(); err != nil {
		return errors.Wrapf(err, "failed to create or update network API server IP for cluster %s in location %s", r.scope.Name(), r.scope.Location())
	}

	if err := r.setFailureDomainsForLocation(ctx); err != nil {
		return errors.Wrapf(err, "failed to get availability zones for cluster %s in location %s", r.scope.Name(), r.scope.Location())
	}

	if err := r.groupsSvc.Reconcile(ctx, nil); err != nil {
		return errors.Wrapf(err, "failed to reconcile resource group for cluster %s", r.scope.Name())
	}

	vnetSpec := &virtualnetworks.Spec{
		ResourceGroup: r.scope.Vnet().ResourceGroup,
		Name:          r.scope.Vnet().Name,
		CIDR:          r.scope.Vnet().CidrBlock,
	}
	if err := r.vnetSvc.Reconcile(ctx, vnetSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile virtual network for cluster %s", r.scope.Name())
	}

	sgSpec := &securitygroups.Spec{
		Name:           r.scope.ControlPlaneSubnet().SecurityGroup.Name,
		IsControlPlane: true,
	}
	if err := r.securityGroupSvc.Reconcile(ctx, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane network security group for cluster %s", r.scope.Name())
	}

	sgSpec = &securitygroups.Spec{
		Name:           r.scope.NodeSubnet().SecurityGroup.Name,
		IsControlPlane: false,
	}
	if err := r.securityGroupSvc.Reconcile(ctx, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node network security group for cluster %s", r.scope.Name())
	}

	rtSpec := &routetables.Spec{
		Name: r.scope.NodeSubnet().RouteTable.Name,
	}
	if err := r.routeTableSvc.Reconcile(ctx, rtSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile route table %s for cluster %s", r.scope.NodeSubnet().RouteTable.Name, r.scope.Name())
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
		return errors.Wrapf(err, "failed to reconcile control plane subnet for cluster %s", r.scope.Name())
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
		return errors.Wrapf(err, "failed to reconcile node subnet for cluster %s", r.scope.Name())
	}

	internalLBSpec := &internalloadbalancers.Spec{
		Name:       azure.GenerateInternalLBName(r.scope.Name()),
		SubnetName: r.scope.ControlPlaneSubnet().Name,
		SubnetCidr: r.scope.ControlPlaneSubnet().CidrBlock,
		VnetName:   r.scope.Vnet().Name,
		IPAddress:  r.scope.ControlPlaneSubnet().InternalLBIPAddress,
	}
	if err := r.internalLBSvc.Reconcile(ctx, internalLBSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane internal load balancer for cluster %s", r.scope.Name())
	}

	publicIPSpec := &publicips.Spec{
		Name:    r.scope.Network().APIServerIP.Name,
		DNSName: r.scope.Network().APIServerIP.DNSName,
	}
	if err := r.publicIPSvc.Reconcile(ctx, publicIPSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane public ip for cluster %s", r.scope.Name())
	}

	publicLBSpec := &publicloadbalancers.Spec{
		Name:         azure.GeneratePublicLBName(r.scope.Name()),
		PublicIPName: r.scope.Network().APIServerIP.Name,
		Role:         infrav1.APIServerRole,
	}
	if err := r.publicLBSvc.Reconcile(ctx, publicLBSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane public load balancer for cluster %s", r.scope.Name())
	}

	nodeOutboundPublicIPSpec := &publicips.Spec{
		Name: azure.GenerateNodeOutboundIPName(r.scope.Name()),
	}
	if err := r.publicIPSvc.Reconcile(ctx, nodeOutboundPublicIPSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node outbound public ip for cluster %s", r.scope.Name())
	}

	nodeOutboundLBSpec := &publicloadbalancers.Spec{
		Name:         r.scope.Name(),
		PublicIPName: azure.GenerateNodeOutboundIPName(r.scope.Name()),
		Role:         infrav1.NodeOutboundRole,
	}
	if err := r.publicLBSvc.Reconcile(ctx, nodeOutboundLBSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node outbound public load balancer for cluster %s", r.scope.Name())
	}

	return nil
}

// Delete reconciles all the services in pre determined order
func (r *azureClusterReconciler) Delete(ctx context.Context) error {
	if err := r.deleteLB(ctx); err != nil {
		return errors.Wrap(err, "failed to delete load balancer")
	}

	if err := r.deleteSubnets(ctx); err != nil {
		return errors.Wrap(err, "failed to delete subnets")
	}

	rtSpec := &routetables.Spec{
		Name: r.scope.NodeSubnet().RouteTable.Name,
	}
	if err := r.routeTableSvc.Delete(ctx, rtSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete route table %s for cluster %s", r.scope.NodeSubnet().RouteTable.Name, r.scope.Name())
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
			return errors.Wrapf(err, "failed to delete virtual network %s for cluster %s", r.scope.Vnet().Name, r.scope.Name())
		}
	}

	if err := r.groupsSvc.Delete(ctx, nil); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete resource group for cluster %s", r.scope.Name())
		}
	}

	return nil
}

func (r *azureClusterReconciler) deleteLB(ctx context.Context) error {
	publicLBSpec := &publicloadbalancers.Spec{
		Name: azure.GeneratePublicLBName(r.scope.Name()),
	}
	if err := r.publicLBSvc.Delete(ctx, publicLBSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete lb %s for cluster %s", publicLBSpec.Name, r.scope.Name())
		}
	}
	publicIPSpec := &publicips.Spec{
		Name: r.scope.Network().APIServerIP.Name,
	}
	if err := r.publicIPSvc.Delete(ctx, publicIPSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete public ip %s for cluster %s", publicIPSpec.Name, r.scope.Name())
		}
	}

	nodeOutboundLBSpec := &publicloadbalancers.Spec{
		Name: r.scope.Name(),
	}
	if err := r.publicLBSvc.Delete(ctx, nodeOutboundLBSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete lb %s for cluster %s", nodeOutboundLBSpec.Name, r.scope.Name())
		}
	}
	nodeOutboundPublicIPSpec := &publicips.Spec{
		Name: azure.GenerateNodeOutboundIPName(r.scope.Name()),
	}
	if err := r.publicIPSvc.Delete(ctx, nodeOutboundPublicIPSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete public ip %s for cluster %s", nodeOutboundPublicIPSpec.Name, r.scope.Name())
		}
	}

	internalLBSpec := &internalloadbalancers.Spec{
		Name: azure.GenerateInternalLBName(r.scope.Name()),
	}
	if err := r.internalLBSvc.Delete(ctx, internalLBSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to internal load balancer %s for cluster %s", azure.GenerateInternalLBName(r.scope.Name()), r.scope.Name())
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
				return errors.Wrapf(err, "failed to delete %s subnet for cluster %s", s.Name, r.scope.Name())
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
			return errors.Wrapf(err, "failed to delete security group %s for cluster %s", r.scope.NodeSubnet().SecurityGroup.Name, r.scope.Name())
		}
	}
	sgSpec = &securitygroups.Spec{
		Name: r.scope.ControlPlaneSubnet().SecurityGroup.Name,
	}
	if err := r.securityGroupSvc.Delete(ctx, sgSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete security group %s for cluster %s", r.scope.ControlPlaneSubnet().SecurityGroup.Name, r.scope.Name())
		}
	}

	return nil
}

// CreateOrUpdateNetworkAPIServerIP creates or updates public ip name and dns name
func (r *azureClusterReconciler) createOrUpdateNetworkAPIServerIP() error {
	if r.scope.Network().APIServerIP.Name == "" {
		h := fnv.New32a()
		if _, err := h.Write([]byte(fmt.Sprintf("%s/%s/%s", r.scope.SubscriptionID, r.scope.ResourceGroup(), r.scope.Name()))); err != nil {
			return errors.Wrapf(err, "failed to write hash sum for api server ip")
		}
		r.scope.Network().APIServerIP.Name = azure.GeneratePublicIPName(r.scope.Name(), fmt.Sprintf("%x", h.Sum32()))
	}

	r.scope.Network().APIServerIP.DNSName = r.scope.GenerateFQDN()
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
