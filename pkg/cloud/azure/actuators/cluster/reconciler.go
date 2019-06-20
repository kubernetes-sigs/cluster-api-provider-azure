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

package cluster

import (
	"encoding/base64"
	"fmt"

	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/networkinterfaces"

	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/certificates"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/internalloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/publicloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachines"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualnetworks"
)

// Reconciler are list of services required by cluster actuator, easy to create a fake
type Reconciler struct {
	scope                *actuators.Scope
	certificatesSvc      azure.Service
	groupsSvc            azure.Service
	vnetSvc              azure.Service
	securityGroupSvc     azure.Service
	routeTableSvc        azure.Service
	subnetsSvc           azure.Service
	internalLBSvc        azure.Service
	publicIPSvc          azure.Service
	publicLBSvc          azure.Service
	virtualMachineSvc    azure.Service
	networkInterfacesSvc azure.Service
}

// NewReconciler populates all the services based on input scope
func NewReconciler(scope *actuators.Scope) *Reconciler {
	return &Reconciler{
		scope:                scope,
		certificatesSvc:      certificates.NewService(scope),
		groupsSvc:            groups.NewService(scope),
		vnetSvc:              virtualnetworks.NewService(scope),
		securityGroupSvc:     securitygroups.NewService(scope),
		routeTableSvc:        routetables.NewService(scope),
		subnetsSvc:           subnets.NewService(scope),
		internalLBSvc:        internalloadbalancers.NewService(scope),
		publicIPSvc:          publicips.NewService(scope),
		publicLBSvc:          publicloadbalancers.NewService(scope),
		virtualMachineSvc:    virtualmachines.NewService(scope),
		networkInterfacesSvc: networkinterfaces.NewService(scope),
	}
}

// Reconcile reconciles all the services in pre determined order
func (r *Reconciler) Reconcile() error {
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
		Name: azure.GenerateControlPlaneSecurityGroupName(r.scope.Cluster.Name),
		Role: v1alpha1.ControlPlane,
	}
	if err := r.securityGroupSvc.Reconcile(r.scope.Context, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane network security group for cluster %s", r.scope.Cluster.Name)
	}

	sgSpec = &securitygroups.Spec{
		Name: azure.GenerateNodeSecurityGroupName(r.scope.Cluster.Name),
		Role: v1alpha1.Node,
	}
	if err := r.securityGroupSvc.Reconcile(r.scope.Context, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile node network security group for cluster %s", r.scope.Cluster.Name)
	}

	sgSpec = &securitygroups.Spec{
		Name: azure.GenerateBastionSecurityGroupName(r.scope.Cluster.Name),
		Role: v1alpha1.Bastion,
	}
	if err := r.securityGroupSvc.Reconcile(r.scope.Context, sgSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile bastion network security group for cluster %s", r.scope.Cluster.Name)
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

	subnetSpec = &subnets.Spec{
		Name:              azure.GenerateBastionSubnetName(r.scope.Cluster.Name),
		CIDR:              azure.DefaultBastionSubnetCIDR,
		VnetName:          azure.GenerateVnetName(r.scope.Cluster.Name),
		SecurityGroupName: azure.GenerateBastionSecurityGroupName(r.scope.Cluster.Name),
	}
	if err := r.subnetsSvc.Reconcile(r.scope.Context, subnetSpec); err != nil {
		return errors.Wrapf(err, "failed to createorupdate bastion subnet for cluster %s", r.scope.Cluster.Name)
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

	if err := reconcileBastion(r); err != nil {
		return errors.Wrapf(err, "failed to reconcile bastion host for cluster %s", r.scope.Cluster.Name)
	}

	klog.V(2).Infof("successfully reconciled cluster %s", r.scope.Cluster.Name)
	return nil
}

func reconcileBastion(r *Reconciler) error {
	bastionNicSpec := &networkinterfaces.Spec{
		Name:                   azure.GenerateBastionNicName(r.scope.Cluster.Name),
		SubnetName:             azure.GenerateBastionSubnetName(r.scope.Cluster.Name),
		VnetName:               azure.GenerateVnetName(r.scope.Cluster.Name),
		PublicLoadBalancerName: azure.GeneratePublicLBName(r.scope.Cluster.Name),
		IsBastion:              true,
	}

	if err := r.networkInterfacesSvc.Reconcile(r.scope.Context, bastionNicSpec); err != nil {
		return errors.Wrapf(err, "failed to createofupdate bastion network interface for cluster %s", r.scope.Cluster.Name)
	}

	bastionPublicKey, err := base64.StdEncoding.DecodeString(r.scope.ClusterConfig.SSHPublicKey)
	if err != nil {
		return errors.Wrap(err, "failed to decode ssh public key for bastion host")
	}

	bastionSpec := &virtualmachines.Spec{
		Name:       fmt.Sprintf("%s-bastion", r.scope.Cluster.Name),
		NICName:    azure.GenerateBastionNicName(r.scope.Cluster.Name),
		SSHKeyData: string(bastionPublicKey),
		Size:       "Standard_B1ls",
		Image: v1alpha1.Image{
			Publisher: "Canonical",
			Offer:     "UbuntuServer",
			SKU:       "18.04-LTS",
			Version:   "latest",
		},
		OSDisk: v1alpha1.OSDisk{
			OSType:     "Linux",
			DiskSizeGB: 30,
		},
	}

	if err := r.virtualMachineSvc.Reconcile(r.scope.Context, bastionSpec); err != nil {
		return errors.Wrapf(err, "failed to createorupdate bastion instance for cluster %s", r.scope.Cluster.Name)
	}

	return nil
}

// Delete reconciles all the services in pre determined order
func (r *Reconciler) Delete() error {
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

	if err := r.deleteBastion(); err != nil {
		return errors.Wrap(err, "failed to delete bastion")
	}

	if err := r.groupsSvc.Delete(r.scope.Context, nil); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete resource group for cluster %s", r.scope.Cluster.Name)
		}
	}

	return nil
}

func (r *Reconciler) deleteLB() error {
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

func (r *Reconciler) deleteSubnets() error {
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
		Name:     azure.GenerateBastionSubnetName(r.scope.Cluster.Name),
		VnetName: azure.GenerateVnetName(r.scope.Cluster.Name),
	}
	if err := r.subnetsSvc.Delete(r.scope.Context, subnetSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete %s subnet for cluster %s", azure.GenerateBastionSubnetName(r.scope.Cluster.Name), r.scope.Cluster.Name)
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

func (r *Reconciler) deleteNSG() error {
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

	sgSpec = &securitygroups.Spec{
		Name: azure.GenerateBastionSecurityGroupName(r.scope.Cluster.Name),
	}
	if err := r.securityGroupSvc.Delete(r.scope.Context, sgSpec); err != nil {
		if !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete security group %s for cluster %s", azure.GenerateBastionSecurityGroupName(r.scope.Cluster.Name), r.scope.Cluster.Name)
		}
	}
	return nil
}

func (r *Reconciler) deleteBastion() error {
	vmSpec := &virtualmachines.Spec{
		Name: azure.GenerateBastionVMName(r.scope.Cluster.Name),
	}

	err := r.virtualMachineSvc.Delete(r.scope.Context, vmSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to delete machine")
	}

	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:     azure.GenerateBastionNicName(r.scope.Cluster.Name),
		VnetName: azure.GenerateVnetName(r.scope.Cluster.Name),
	}

	err = r.networkInterfacesSvc.Delete(r.scope.Context, networkInterfaceSpec)
	if err != nil {
		return errors.Wrapf(err, "Unable to delete network interface")
	}
	return nil
}
