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
	"encoding/base64"
	"fmt"
	"net"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-02-01/containerservice"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/managedclusters"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
)

// azureManagedControlPlaneReconciler are list of services required by cluster controller
type azureManagedControlPlaneReconciler struct {
	kubeclient         client.Client
	managedClustersSvc *managedclusters.Service
	groupsSvc          azure.Service
}

// newAzureManagedControlPlaneReconciler populates all the services based on input scope
func newAzureManagedControlPlaneReconciler(scope *scope.ManagedControlPlaneScope) *azureManagedControlPlaneReconciler {
	return &azureManagedControlPlaneReconciler{
		kubeclient:         scope.Client,
		managedClustersSvc: managedclusters.NewService(scope),
		groupsSvc:          groups.NewService(scope),
	}
}

// Reconcile reconciles all the services in pre determined order
func (r *azureManagedControlPlaneReconciler) Reconcile(ctx context.Context, scope *scope.ManagedControlPlaneScope) error {
	decodedSSHPublicKey, err := base64.StdEncoding.DecodeString(scope.ControlPlane.Spec.SSHPublicKey)
	if err != nil {
		return errors.Wrapf(err, "failed to decode SSHPublicKey")
	}

	managedClusterSpec := &managedclusters.Spec{
		Name:          scope.ControlPlane.Name,
		ResourceGroup: scope.ControlPlane.Spec.ResourceGroup,
		Location:      scope.ControlPlane.Spec.Location,
		Tags:          scope.ControlPlane.Spec.AdditionalTags,
		Version:       strings.TrimPrefix(scope.ControlPlane.Spec.Version, "v"),
		SSHPublicKey:  string(decodedSSHPublicKey),
		DNSServiceIP:  scope.ControlPlane.Spec.DNSServiceIP,
	}

	if scope.ControlPlane.Spec.NetworkPlugin != nil {
		managedClusterSpec.NetworkPlugin = *scope.ControlPlane.Spec.NetworkPlugin
	}
	if scope.ControlPlane.Spec.NetworkPolicy != nil {
		managedClusterSpec.NetworkPolicy = *scope.ControlPlane.Spec.NetworkPolicy
	}
	if scope.ControlPlane.Spec.LoadBalancerSKU != nil {
		managedClusterSpec.LoadBalancerSKU = *scope.ControlPlane.Spec.LoadBalancerSKU
	}

	scope.V(2).Info("Reconciling managed cluster resource group")
	if err := r.groupsSvc.Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "failed to reconcile managed cluster resource group")
	}

	scope.V(2).Info("Reconciling managed cluster")
	if err := r.reconcileManagedCluster(ctx, scope, managedClusterSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile managed cluster")
	}

	scope.V(2).Info("Reconciling endpoint")
	if err := r.reconcileEndpoint(ctx, scope, managedClusterSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile control plane endpoint")
	}

	scope.V(2).Info("Reconciling kubeconfig")
	if err := r.reconcileKubeconfig(ctx, scope, managedClusterSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile kubeconfig secret")
	}

	return nil
}

// Delete reconciles all the services in pre determined order
func (r *azureManagedControlPlaneReconciler) Delete(ctx context.Context, scope *scope.ManagedControlPlaneScope) error {
	managedClusterSpec := &managedclusters.Spec{
		Name:          scope.ControlPlane.Name,
		ResourceGroup: scope.ControlPlane.Spec.ResourceGroup,
		Location:      scope.ControlPlane.Spec.Location,
		Tags:          scope.ControlPlane.Spec.AdditionalTags,
		Version:       strings.TrimPrefix(scope.ControlPlane.Spec.Version, "v"),
		SSHPublicKey:  scope.ControlPlane.Spec.SSHPublicKey,
		DNSServiceIP:  scope.ControlPlane.Spec.DNSServiceIP,
	}

	if scope.ControlPlane.Spec.NetworkPlugin != nil {
		managedClusterSpec.NetworkPlugin = *scope.ControlPlane.Spec.NetworkPlugin
	}
	if scope.ControlPlane.Spec.NetworkPolicy != nil {
		managedClusterSpec.NetworkPolicy = *scope.ControlPlane.Spec.NetworkPolicy
	}
	if scope.ControlPlane.Spec.LoadBalancerSKU != nil {
		managedClusterSpec.LoadBalancerSKU = *scope.ControlPlane.Spec.LoadBalancerSKU
	}

	scope.V(2).Info("Deleting managed cluster")
	if err := r.managedClustersSvc.Delete(ctx, managedClusterSpec); err != nil {
		return errors.Wrapf(err, "failed to delete managed cluster %s", scope.ControlPlane.Name)
	}

	scope.V(2).Info("Deleting managed cluster resource group")
	if err := r.groupsSvc.Delete(ctx); err != nil {
		return errors.Wrapf(err, "failed to delete managed cluster resource group")
	}

	return nil
}

func (r *azureManagedControlPlaneReconciler) reconcileManagedCluster(ctx context.Context, scope *scope.ManagedControlPlaneScope, managedClusterSpec *managedclusters.Spec) error {
	if net := scope.Cluster.Spec.ClusterNetwork; net != nil {
		if net.Services != nil {
			// A user may provide zero or one CIDR blocks. If they provide an empty array,
			// we ignore it and use the default. AKS doesn't support > 1 Service/Pod CIDR.
			if len(net.Services.CIDRBlocks) > 1 {
				return errors.New("managed control planes only allow one service cidr")
			}
			if len(net.Services.CIDRBlocks) == 1 {
				managedClusterSpec.ServiceCIDR = net.Services.CIDRBlocks[0]
			}
		}
		if net.Pods != nil {
			// A user may provide zero or one CIDR blocks. If they provide an empty array,
			// we ignore it and use the default. AKS doesn't support > 1 Service/Pod CIDR.
			if len(net.Pods.CIDRBlocks) > 1 {
				return errors.New("managed control planes only allow one service cidr")
			}
			if len(net.Pods.CIDRBlocks) == 1 {
				managedClusterSpec.PodCIDR = net.Pods.CIDRBlocks[0]
			}
		}
	}

	// if DNSServiceIP is specified, ensure it is within the ServiceCIDR address range
	if scope.ControlPlane.Spec.DNSServiceIP != nil {
		if managedClusterSpec.ServiceCIDR == "" {
			return fmt.Errorf(scope.Cluster.Name + " cluster serviceCIDR must be specified if specifying DNSServiceIP")
		}
		_, cidr, err := net.ParseCIDR(managedClusterSpec.ServiceCIDR)
		if err != nil {
			return fmt.Errorf("failed to parse cluster service cidr: %w", err)
		}
		ip := net.ParseIP(*scope.ControlPlane.Spec.DNSServiceIP)
		if !cidr.Contains(ip) {
			return fmt.Errorf(scope.ControlPlane.Name + " DNSServiceIP must reside within the associated cluster serviceCIDR")
		}
	}

	_, err := r.managedClustersSvc.Get(ctx, managedClusterSpec)
	// Transient or other failure not due to 404
	if err != nil && !azure.ResourceNotFound(err) {
		return errors.Wrapf(err, "failed to fetch existing managed cluster")
	}

	// We are creating this cluster for the first time.
	// Configure the default pool, rest will be handled by machinepool controller
	// We do this here because AKS will only let us mutate agent pools via managed
	// clusters API at create time, not update.
	if azure.ResourceNotFound(err) {
		defaultPoolSpec := managedclusters.PoolSpec{
			Name:         scope.InfraMachinePool.Name,
			SKU:          scope.InfraMachinePool.Spec.SKU,
			Replicas:     1,
			OSDiskSizeGB: 0,
		}

		// Set optional values
		if scope.InfraMachinePool.Spec.OSDiskSizeGB != nil {
			defaultPoolSpec.OSDiskSizeGB = *scope.InfraMachinePool.Spec.OSDiskSizeGB
		}
		if scope.MachinePool.Spec.Replicas != nil {
			defaultPoolSpec.Replicas = *scope.MachinePool.Spec.Replicas
		}

		// Add to cluster spec
		managedClusterSpec.AgentPools = []managedclusters.PoolSpec{defaultPoolSpec}
	}

	// Send to Azure for create/update.
	if err := r.managedClustersSvc.Reconcile(ctx, managedClusterSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile managed cluster %s", scope.ControlPlane.Name)
	}
	return nil
}

func (r *azureManagedControlPlaneReconciler) reconcileEndpoint(ctx context.Context, scope *scope.ManagedControlPlaneScope, managedClusterSpec *managedclusters.Spec) error {
	// Fetch newly updated cluster
	managedClusterResult, err := r.managedClustersSvc.Get(ctx, managedClusterSpec)
	if err != nil {
		return err
	}

	managedCluster, ok := managedClusterResult.(containerservice.ManagedCluster)
	if !ok {
		return fmt.Errorf("expected containerservice ManagedCluster object")
	}

	old := scope.ControlPlane.DeepCopyObject()

	scope.ControlPlane.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: *managedCluster.ManagedClusterProperties.Fqdn,
		Port: 443,
	}

	if err := r.kubeclient.Patch(ctx, scope.ControlPlane, client.MergeFrom(old)); err != nil {
		return errors.Wrapf(err, "failed to set control plane endpoint")
	}

	return nil
}

func (r *azureManagedControlPlaneReconciler) reconcileKubeconfig(ctx context.Context, scope *scope.ManagedControlPlaneScope, managedClusterSpec *managedclusters.Spec) error {
	// Always fetch credentials in case of rotation
	data, err := r.managedClustersSvc.GetCredentials(ctx, managedClusterSpec.ResourceGroup, managedClusterSpec.Name)
	if err != nil {
		return errors.Wrapf(err, "failed to get credentials for managed cluster")
	}

	// Construct and store secret
	kubeconfig := makeKubeconfig(scope.Cluster, scope.ControlPlane)
	if _, err := controllerutil.CreateOrUpdate(ctx, r.kubeclient, kubeconfig, func() error {
		kubeconfig.Data = map[string][]byte{
			secret.KubeconfigDataName: data,
		}
		return nil
	}); err != nil {
		return errors.Wrapf(err, "failed to kubeconfig secret for cluster")
	}
	return nil
}

func makeKubeconfig(cluster *clusterv1.Cluster, controlPlane *infrav1exp.AzureManagedControlPlane) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name(cluster.Name, secret.Kubeconfig),
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(controlPlane, infrav1exp.GroupVersion.WithKind("AzureManagedControlPlane")),
			},
		},
	}
}
