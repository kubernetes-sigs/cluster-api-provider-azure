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
	"fmt"
	"hash/fnv"

	"github.com/pkg/errors"
	"k8s.io/klog"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
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

// azureClusterReconciler are list of services required by cluster controller
type azureClusterReconciler struct {
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
func NewAzureClusterReconciler(scope *scope.ClusterScope) *azureClusterReconciler {
	return &azureClusterReconciler{
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
func (r *azureClusterReconciler) Reconcile() error {
	klog.V(2).Infof("reconciling cluster %s", r.scope.Name())
	r.createOrUpdateNetworkAPIServerIP()

	// Store cert material in spec.
	if err := r.certificatesSvc.Reconcile(r.scope.Context, nil); err != nil {
		return errors.Wrapf(err, "failed to reconcile certificates for cluster %s", r.scope.Name())
	}

	if err := r.groupsSvc.Reconcile(r.scope.Context, nil); err != nil {
		return errors.Wrapf(err, "failed to reconcile resource group for cluster %s", r.scope.Name())
	}

	vnetSpec := &virtualnetworks.Spec{
		Name: azure.GenerateVnetName(r.scope.Name()),
		CIDR: azure.DefaultVnetCIDR,
	}
	if err := r.vnetSvc.Reconcile(r.scope.Context, vnetSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile virtual network for cluster %s", r.scope.Name())
	}
	sgSpec := &securitygroups.Spec{
		Name:           azure.GenerateControlPlaneSecurityGroupName(r.scope.Name()),
		IsControlPlane: true,
	}
	if err := r.securityGroupSvc.Reconcile(r.scope.Context, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane network security group for cluster %s", r.scope.Name())
	}

	sgSpec = &securitygroups.Spec{
		Name:           azure.GenerateNodeSecurityGroupName(r.scope.Name()),
		IsControlPlane: false,
	}
	if err := r.securityGroupSvc.Reconcile(r.scope.Context, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node network security group for cluster %s", r.scope.Name())
	}

	rtSpec := &routetables.Spec{
		Name: azure.GenerateNodeRouteTableName(r.scope.Name()),
	}
	if err := r.routeTableSvc.Reconcile(r.scope.Context, rtSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node route table for cluster %s", r.scope.Name())
	}

	subnetSpec := &subnets.Spec{
		Name:              azure.GenerateControlPlaneSubnetName(r.scope.Name()),
		CIDR:              azure.DefaultControlPlaneSubnetCIDR,
		VnetName:          azure.GenerateVnetName(r.scope.Name()),
		SecurityGroupName: azure.GenerateControlPlaneSecurityGroupName(r.scope.Name()),
	}
	if err := r.subnetsSvc.Reconcile(r.scope.Context, subnetSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane subnet for cluster %s", r.scope.Name())
	}

	subnetSpec = &subnets.Spec{
		Name:              azure.GenerateNodeSubnetName(r.scope.Name()),
		CIDR:              azure.DefaultNodeSubnetCIDR,
		VnetName:          azure.GenerateVnetName(r.scope.Name()),
		SecurityGroupName: azure.GenerateNodeSecurityGroupName(r.scope.Name()),
		RouteTableName:    azure.GenerateNodeRouteTableName(r.scope.Name()),
	}
	if err := r.subnetsSvc.Reconcile(r.scope.Context, subnetSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node subnet for cluster %s", r.scope.Name())
	}

	internalLBSpec := &internalloadbalancers.Spec{
		Name:       azure.GenerateInternalLBName(r.scope.Name()),
		SubnetName: azure.GenerateControlPlaneSubnetName(r.scope.Name()),
		VnetName:   azure.GenerateVnetName(r.scope.Name()),
		IPAddress:  azure.DefaultInternalLBIPAddress,
	}
	if err := r.internalLBSvc.Reconcile(r.scope.Context, internalLBSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane internal load balancer for cluster %s", r.scope.Name())
	}

	publicIPSpec := &publicips.Spec{
		Name: r.scope.Network().APIServerIP.Name,
	}
	if err := r.publicIPSvc.Reconcile(r.scope.Context, publicIPSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane public ip for cluster %s", r.scope.Name())
	}

	publicLBSpec := &publicloadbalancers.Spec{
		Name:         azure.GeneratePublicLBName(r.scope.Name()),
		PublicIPName: r.scope.Network().APIServerIP.Name,
	}
	if err := r.publicLBSvc.Reconcile(r.scope.Context, publicLBSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane public load balancer for cluster %s", r.scope.Name())
	}

	return nil
}

// Delete reconciles all the services in pre determined order
func (r *azureClusterReconciler) Delete() error {
	if err := r.deleteLB(); err != nil {
		return errors.Wrap(err, "failed to delete load balancer")
	}

	if err := r.deleteSubnets(); err != nil {
		return errors.Wrap(err, "failed to delete subnets")
	}

	rtSpec := &routetables.Spec{
		Name: azure.GenerateNodeRouteTableName(r.scope.Name()),
	}
	if err := r.routeTableSvc.Delete(r.scope.Context, rtSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete route table %s for cluster %s", azure.GenerateNodeRouteTableName(r.scope.Name()), r.scope.Name())
		}
	}

	if err := r.deleteNSG(); err != nil {
		return errors.Wrap(err, "failed to delete network security group")
	}

	vnetSpec := &virtualnetworks.Spec{
		Name: azure.GenerateVnetName(r.scope.Name()),
	}
	if err := r.vnetSvc.Delete(r.scope.Context, vnetSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete virtual network %s for cluster %s", azure.GenerateVnetName(r.scope.Name()), r.scope.Name())
		}
	}

	if err := r.groupsSvc.Delete(r.scope.Context, nil); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete resource group for cluster %s", r.scope.Name())
		}
	}

	return nil
}

func (r *azureClusterReconciler) deleteLB() error {
	publicLBSpec := &publicloadbalancers.Spec{
		Name: azure.GeneratePublicLBName(r.scope.Name()),
	}
	if err := r.publicLBSvc.Delete(r.scope.Context, publicLBSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete lb %s for cluster %s", azure.GeneratePublicLBName(r.scope.Name()), r.scope.Name())
		}
	}
	publicIPSpec := &publicips.Spec{
		Name: r.scope.Network().APIServerIP.Name,
	}
	if err := r.publicIPSvc.Delete(r.scope.Context, publicIPSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete public ip %s for cluster %s", r.scope.Network().APIServerIP.Name, r.scope.Name())
		}
	}

	internalLBSpec := &internalloadbalancers.Spec{
		Name: azure.GenerateInternalLBName(r.scope.Name()),
	}
	if err := r.internalLBSvc.Delete(r.scope.Context, internalLBSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to internal load balancer %s for cluster %s", azure.GenerateInternalLBName(r.scope.Name()), r.scope.Name())
		}
	}

	return nil
}

func (r *azureClusterReconciler) deleteSubnets() error {
	subnetSpec := &subnets.Spec{
		Name:     azure.GenerateNodeSubnetName(r.scope.Name()),
		VnetName: azure.GenerateVnetName(r.scope.Name()),
	}
	if err := r.subnetsSvc.Delete(r.scope.Context, subnetSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete %s subnet for cluster %s", azure.GenerateNodeSubnetName(r.scope.Name()), r.scope.Name())
		}
	}

	subnetSpec = &subnets.Spec{
		Name:     azure.GenerateControlPlaneSubnetName(r.scope.Name()),
		VnetName: azure.GenerateVnetName(r.scope.Name()),
	}
	if err := r.subnetsSvc.Delete(r.scope.Context, subnetSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete %s subnet for cluster %s", azure.GenerateControlPlaneSubnetName(r.scope.Name()), r.scope.Name())
		}
	}

	return nil
}

func (r *azureClusterReconciler) deleteNSG() error {
	sgSpec := &securitygroups.Spec{
		Name: azure.GenerateNodeSecurityGroupName(r.scope.Name()),
	}
	if err := r.securityGroupSvc.Delete(r.scope.Context, sgSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete security group %s for cluster %s", azure.GenerateNodeSecurityGroupName(r.scope.Name()), r.scope.Name())
		}
	}
	sgSpec = &securitygroups.Spec{
		Name: azure.GenerateControlPlaneSecurityGroupName(r.scope.Name()),
	}
	if err := r.securityGroupSvc.Delete(r.scope.Context, sgSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete security group %s for cluster %s", azure.GenerateControlPlaneSecurityGroupName(r.scope.Name()), r.scope.Name())
		}
	}

	return nil
}

// CreateOrUpdateNetworkAPIServerIP creates or updates public ip name and dns name
func (r *azureClusterReconciler) createOrUpdateNetworkAPIServerIP() {
	if r.scope.Network().APIServerIP.Name == "" {
		h := fnv.New32a()
		h.Write([]byte(fmt.Sprintf("%s/%s/%s", r.scope.SubscriptionID, r.scope.AzureCluster.Spec.ResourceGroup, r.scope.Name())))
		r.scope.Network().APIServerIP.Name = azure.GeneratePublicIPName(r.scope.Name(), fmt.Sprintf("%x", h.Sum32()))
	}

	r.scope.Network().APIServerIP.DNSName = azure.GenerateFQDN(r.scope.Network().APIServerIP.Name, r.scope.Location())
}
