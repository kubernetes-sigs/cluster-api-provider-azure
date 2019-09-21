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
	"github.com/pkg/errors"
	"k8s.io/klog"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/certificates"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/internalloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks"
)

// AzureClusterReconciler are list of services required by cluster actuator, easy to create a fake
type AzureClusterReconciler struct {
	scope            *scope.ClusterScope
	certificatesSvc  azure.Service
	groupsSvc        azure.Service
	vnetSvc          azure.Service
	securityGroupSvc azure.Service
	routeTableSvc    azure.Service
	subnetsSvc       azure.Service
	internalLBSvc    azure.Service
	publicIPSvc      azure.Service
	publicLBSvc      azure.Service
}

// NewAzureClusterReconciler populates all the services based on input scope
func NewAzureClusterReconciler(scope *scope.ClusterScope) *AzureClusterReconciler {
	return &AzureClusterReconciler{
		scope:            scope,
		certificatesSvc:  certificates.NewService(scope),
		groupsSvc:        groups.NewService(scope),
		vnetSvc:          virtualnetworks.NewService(scope),
		securityGroupSvc: securitygroups.NewService(scope),
		routeTableSvc:    routetables.NewService(scope),
		subnetsSvc:       subnets.NewService(scope),
		internalLBSvc:    internalloadbalancers.NewService(scope),
		publicIPSvc:      publicips.NewService(scope),
		publicLBSvc:      publicloadbalancers.NewService(scope),
	}
}

// Reconcile reconciles all the services in pre determined order
func (r *AzureClusterReconciler) Reconcile() error {
	klog.V(2).Infof("reconciling cluster %s", r.scope.Cluster.Name)
	actuators.CreateOrUpdateNetworkAPIServerIP(r.scope)

	// Store cert material in spec.
	if err := r.certificatesSvc.Reconcile(r.scope.Context, nil); err != nil {
		return errors.Wrapf(err, "failed to reconcile certificates for cluster %s", r.scope.Cluster.Name)
	}

	if err := r.groupsSvc.Reconcile(r.scope.Context, nil); err != nil {
		return errors.Wrapf(err, "failed to reconcile resource group for cluster %s", r.scope.Cluster.Name)
	}

	vnetSpec := &virtualnetworks.Spec{
		Name: azure.GenerateVnetName(r.scope.Cluster.Name),
		CIDR: azure.DefaultVnetCIDR,
	}
	if err := r.vnetSvc.Reconcile(r.scope.Context, vnetSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile virtual network for cluster %s", r.scope.Cluster.Name)
	}
	sgSpec := &securitygroups.Spec{
		Name:           azure.GenerateControlPlaneSecurityGroupName(r.scope.Cluster.Name),
		IsControlPlane: true,
	}
	if err := r.securityGroupSvc.Reconcile(r.scope.Context, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane network security group for cluster %s", r.scope.Cluster.Name)
	}

	sgSpec = &securitygroups.Spec{
		Name:           azure.GenerateNodeSecurityGroupName(r.scope.Cluster.Name),
		IsControlPlane: false,
	}
	if err := r.securityGroupSvc.Reconcile(r.scope.Context, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node network security group for cluster %s", r.scope.Cluster.Name)
	}

	rtSpec := &routetables.Spec{
		Name: azure.GenerateNodeRouteTableName(r.scope.Cluster.Name),
	}
	if err := r.routeTableSvc.Reconcile(r.scope.Context, rtSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node route table for cluster %s", r.scope.Cluster.Name)
	}

	subnetSpec := &subnets.Spec{
		Name:              azure.GenerateControlPlaneSubnetName(r.scope.Cluster.Name),
		CIDR:              azure.DefaultControlPlaneSubnetCIDR,
		VnetName:          azure.GenerateVnetName(r.scope.Cluster.Name),
		SecurityGroupName: azure.GenerateControlPlaneSecurityGroupName(r.scope.Cluster.Name),
	}
	if err := r.subnetsSvc.Reconcile(r.scope.Context, subnetSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane subnet for cluster %s", r.scope.Cluster.Name)
	}

	subnetSpec = &subnets.Spec{
		Name:              azure.GenerateNodeSubnetName(r.scope.Cluster.Name),
		CIDR:              azure.DefaultNodeSubnetCIDR,
		VnetName:          azure.GenerateVnetName(r.scope.Cluster.Name),
		SecurityGroupName: azure.GenerateNodeSecurityGroupName(r.scope.Cluster.Name),
		RouteTableName:    azure.GenerateNodeRouteTableName(r.scope.Cluster.Name),
	}
	if err := r.subnetsSvc.Reconcile(r.scope.Context, subnetSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node subnet for cluster %s", r.scope.Cluster.Name)
	}

	internalLBSpec := &internalloadbalancers.Spec{
		Name:       azure.GenerateInternalLBName(r.scope.Cluster.Name),
		SubnetName: azure.GenerateControlPlaneSubnetName(r.scope.Cluster.Name),
		VnetName:   azure.GenerateVnetName(r.scope.Cluster.Name),
		IPAddress:  azure.DefaultInternalLBIPAddress,
	}
	if err := r.internalLBSvc.Reconcile(r.scope.Context, internalLBSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane internal load balancer for cluster %s", r.scope.Cluster.Name)
	}

	publicIPSpec := &publicips.Spec{
		Name: r.scope.Network().APIServerIP.Name,
	}
	if err := r.publicIPSvc.Reconcile(r.scope.Context, publicIPSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane public ip for cluster %s", r.scope.Cluster.Name)
	}

	publicLBSpec := &publicloadbalancers.Spec{
		Name:         azure.GeneratePublicLBName(r.scope.Cluster.Name),
		PublicIPName: r.scope.Network().APIServerIP.Name,
	}
	if err := r.publicLBSvc.Reconcile(r.scope.Context, publicLBSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane public load balancer for cluster %s", r.scope.Cluster.Name)
	}

	return nil
}

// Delete reconciles all the services in pre determined order
func (r *AzureClusterReconciler) Delete() error {
	if err := r.deleteLB(); err != nil {
		return errors.Wrap(err, "failed to delete load balancer")
	}

	if err := r.deleteSubnets(); err != nil {
		return errors.Wrap(err, "failed to delete subnets")
	}

	rtSpec := &routetables.Spec{
		Name: azure.GenerateNodeRouteTableName(r.scope.Cluster.Name),
	}
	if err := r.routeTableSvc.Delete(r.scope.Context, rtSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete route table %s for cluster %s", azure.GenerateNodeRouteTableName(r.scope.Cluster.Name), r.scope.Cluster.Name)
		}
	}

	if err := r.deleteNSG(); err != nil {
		return errors.Wrap(err, "failed to delete network security group")
	}

	vnetSpec := &virtualnetworks.Spec{
		Name: azure.GenerateVnetName(r.scope.Cluster.Name),
	}
	if err := r.vnetSvc.Delete(r.scope.Context, vnetSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete virtual network %s for cluster %s", azure.GenerateVnetName(r.scope.Cluster.Name), r.scope.Cluster.Name)
		}
	}

	if err := r.groupsSvc.Delete(r.scope.Context, nil); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete resource group for cluster %s", r.scope.Cluster.Name)
		}
	}

	return nil
}

func (r *AzureClusterReconciler) deleteLB() error {
	publicLBSpec := &publicloadbalancers.Spec{
		Name: azure.GeneratePublicLBName(r.scope.Cluster.Name),
	}
	if err := r.publicLBSvc.Delete(r.scope.Context, publicLBSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete lb %s for cluster %s", azure.GeneratePublicLBName(r.scope.Cluster.Name), r.scope.Cluster.Name)
		}
	}
	publicIPSpec := &publicips.Spec{
		Name: r.scope.Network().APIServerIP.Name,
	}
	if err := r.publicIPSvc.Delete(r.scope.Context, publicIPSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete public ip %s for cluster %s", r.scope.Network().APIServerIP.Name, r.scope.Cluster.Name)
		}
	}

	internalLBSpec := &internalloadbalancers.Spec{
		Name: azure.GenerateInternalLBName(r.scope.Cluster.Name),
	}
	if err := r.internalLBSvc.Delete(r.scope.Context, internalLBSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to internal load balancer %s for cluster %s", azure.GenerateInternalLBName(r.scope.Cluster.Name), r.scope.Cluster.Name)
		}
	}

	return nil
}

func (r *AzureClusterReconciler) deleteSubnets() error {
	subnetSpec := &subnets.Spec{
		Name:     azure.GenerateNodeSubnetName(r.scope.Cluster.Name),
		VnetName: azure.GenerateVnetName(r.scope.Cluster.Name),
	}
	if err := r.subnetsSvc.Delete(r.scope.Context, subnetSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete %s subnet for cluster %s", azure.GenerateNodeSubnetName(r.scope.Cluster.Name), r.scope.Cluster.Name)
		}
	}

	subnetSpec = &subnets.Spec{
		Name:     azure.GenerateControlPlaneSubnetName(r.scope.Cluster.Name),
		VnetName: azure.GenerateVnetName(r.scope.Cluster.Name),
	}
	if err := r.subnetsSvc.Delete(r.scope.Context, subnetSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete %s subnet for cluster %s", azure.GenerateControlPlaneSubnetName(r.scope.Cluster.Name), r.scope.Cluster.Name)
		}
	}

	return nil
}

func (r *AzureClusterReconciler) deleteNSG() error {
	sgSpec := &securitygroups.Spec{
		Name: azure.GenerateNodeSecurityGroupName(r.scope.Cluster.Name),
	}
	if err := r.securityGroupSvc.Delete(r.scope.Context, sgSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete security group %s for cluster %s", azure.GenerateNodeSecurityGroupName(r.scope.Cluster.Name), r.scope.Cluster.Name)
		}
	}
	sgSpec = &securitygroups.Spec{
		Name: azure.GenerateControlPlaneSecurityGroupName(r.scope.Cluster.Name),
	}
	if err := r.securityGroupSvc.Delete(r.scope.Context, sgSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete security group %s for cluster %s", azure.GenerateControlPlaneSecurityGroupName(r.scope.Cluster.Name), r.scope.Cluster.Name)
		}
	}

	return nil
}
