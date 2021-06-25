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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-03-01/containerservice"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/managedclusters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualnetworks"
	infracontroller "sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureManagedControlPlaneReconciler contains the services required by the cluster controller.
type azureManagedControlPlaneReconciler struct {
	kubeclient         client.Client
	managedClustersSvc *managedclusters.Service
	groupsSvc          azure.Reconciler
	vnetSvc            azure.Reconciler
	subnetsSvc         azure.Reconciler
}

// newAzureManagedControlPlaneReconciler populates all the services based on input scope.
func newAzureManagedControlPlaneReconciler(scope *scope.ManagedControlPlaneScope) *azureManagedControlPlaneReconciler {
	return &azureManagedControlPlaneReconciler{
		kubeclient:         scope.Client,
		managedClustersSvc: managedclusters.NewService(scope),
		groupsSvc:          groups.New(scope),
		vnetSvc:            virtualnetworks.New(scope),
		subnetsSvc:         subnets.New(scope),
	}
}

// Reconcile reconciles all the services in a predetermined order.
func (r *azureManagedControlPlaneReconciler) Reconcile(ctx context.Context, scope *scope.ManagedControlPlaneScope) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedControlPlaneReconciler.Reconcile")
	defer span.End()

	decodedSSHPublicKey, err := base64.StdEncoding.DecodeString(scope.ControlPlane.Spec.SSHPublicKey)
	if err != nil {
		return errors.Wrap(err, "failed to decode SSHPublicKey")
	}

	managedClusterSpec := &managedclusters.Spec{
		Name:                  scope.ControlPlane.Name,
		ResourceGroupName:     scope.ControlPlane.Spec.ResourceGroupName,
		NodeResourceGroupName: scope.ControlPlane.Spec.NodeResourceGroupName,
		Location:              scope.ControlPlane.Spec.Location,
		Tags:                  scope.ControlPlane.Spec.AdditionalTags,
		Version:               strings.TrimPrefix(scope.ControlPlane.Spec.Version, "v"),
		SSHPublicKey:          string(decodedSSHPublicKey),
		DNSServiceIP:          scope.ControlPlane.Spec.DNSServiceIP,
		VnetSubnetID: azure.SubnetID(
			scope.ControlPlane.Spec.SubscriptionID,
			scope.ControlPlane.Spec.ResourceGroupName,
			scope.ControlPlane.Spec.VirtualNetwork.Name,
			scope.ControlPlane.Spec.VirtualNetwork.Subnet.Name,
		),
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
		return errors.Wrap(err, "failed to reconcile managed cluster resource group")
	}

	scope.V(2).Info("Reconciling virtual network")
	if err := r.vnetSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile virtual network")
	}

	scope.V(2).Info("Reconciling subnet")
	if err := r.subnetsSvc.Reconcile(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile subnet")
	}

	scope.V(2).Info("Reconciling managed cluster")
	if err := r.reconcileManagedCluster(ctx, scope, managedClusterSpec); err != nil {
		return errors.Wrap(err, "failed to reconcile managed cluster")
	}

	scope.V(2).Info("Reconciling endpoint")
	if err := r.reconcileEndpoint(ctx, scope, managedClusterSpec); err != nil {
		return errors.Wrap(err, "failed to reconcile control plane endpoint")
	}

	scope.V(2).Info("Reconciling kubeconfig")
	if err := r.reconcileKubeconfig(ctx, scope, managedClusterSpec); err != nil {
		return errors.Wrap(err, "failed to reconcile kubeconfig secret")
	}

	return nil
}

// Delete reconciles all the services in a predetermined order.
func (r *azureManagedControlPlaneReconciler) Delete(ctx context.Context, scope *scope.ManagedControlPlaneScope) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedControlPlaneReconciler.Delete")
	defer span.End()

	managedClusterSpec := &managedclusters.Spec{
		Name:              scope.ControlPlane.Name,
		ResourceGroupName: scope.ControlPlane.Spec.ResourceGroupName,
	}

	scope.V(2).Info("Deleting managed cluster")
	if err := r.managedClustersSvc.Delete(ctx, managedClusterSpec); err != nil {
		return errors.Wrapf(err, "failed to delete managed cluster %s", scope.ControlPlane.Name)
	}

	scope.V(2).Info("Deleting virtual network")
	if err := r.vnetSvc.Delete(ctx); err != nil {
		return errors.Wrap(err, "failed to delete virtual network")
	}

	scope.V(2).Info("Deleting managed cluster resource group")
	if err := r.groupsSvc.Delete(ctx); err != nil && !errors.Is(err, azure.ErrNotOwned) {
		return errors.Wrap(err, "failed to delete managed cluster resource group")
	}

	return nil
}

func (r *azureManagedControlPlaneReconciler) reconcileManagedCluster(ctx context.Context, scope *scope.ManagedControlPlaneScope, managedClusterSpec *managedclusters.Spec) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedControlPlaneReconciler.reconcileManagedCluster")
	defer span.End()

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
		return errors.Wrap(err, "failed to fetch existing managed cluster")
	}

	// We are creating this cluster for the first time.
	// Configure the system pool, rest will be handled by machinepool controller
	// We do this here because AKS will only let us mutate agent pools via managed
	// clusters API at create time, not update.
	if azure.ResourceNotFound(err) {
		opt1 := client.InNamespace(scope.ControlPlane.Namespace)
		opt2 := client.MatchingLabels(map[string]string{
			infrav1exp.LabelAgentPoolMode: infrav1exp.SystemNodePool,
			clusterv1.ClusterLabelName:    scope.Cluster.Name,
		})

		ammpList := &infrav1exp.AzureManagedMachinePoolList{}

		if err := r.kubeclient.List(ctx, ammpList, opt1, opt2); err != nil {
			return err
		}

		if ammpList == nil || len(ammpList.Items) == 0 {
			return errors.New("failed to fetch azuremanagedMachine pool with mode:System, require atleast 1 system node pool")
		}
		ammps := []managedclusters.PoolSpec{}

		for _, pool := range ammpList.Items {
			// Fetch the owning MachinePool.
			ownerPool, err := infracontroller.GetOwnerMachinePool(ctx, r.kubeclient, pool.ObjectMeta)
			if err != nil {
				scope.Logger.Error(err, "failed to fetch owner ref for system pool: %s", pool.Name)
				continue
			}
			if ownerPool == nil {
				scope.Logger.Info("failed to fetch owner ref for system pool")
				continue
			}

			ammp := managedclusters.PoolSpec{
				Name:         pool.Name,
				SKU:          pool.Spec.SKU,
				Replicas:     1,
				OSDiskSizeGB: 0,
			}

			// Set optional values
			if pool.Spec.OSDiskSizeGB != nil {
				ammp.OSDiskSizeGB = *pool.Spec.OSDiskSizeGB
			}

			if ownerPool.Spec.Replicas != nil {
				ammp.Replicas = *ownerPool.Spec.Replicas
			}

			ammps = append(ammps, ammp)
		}
		if len(ammps) == 0 {
			return errors.New("owner ref for system machine pools not ready")
		}
		// Add to cluster spec
		managedClusterSpec.AgentPools = ammps
	}

	// Send to Azure for create/update.
	if err := r.managedClustersSvc.Reconcile(ctx, managedClusterSpec); err != nil {
		return errors.Wrapf(err, "failed to reconcile managed cluster %s", scope.ControlPlane.Name)
	}
	return nil
}

func (r *azureManagedControlPlaneReconciler) reconcileEndpoint(ctx context.Context, scope *scope.ManagedControlPlaneScope, managedClusterSpec *managedclusters.Spec) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedControlPlaneReconciler.reconcileEndpoint")
	defer span.End()

	// Fetch newly updated cluster
	managedClusterResult, err := r.managedClustersSvc.Get(ctx, managedClusterSpec)
	if err != nil {
		return err
	}

	managedCluster, ok := managedClusterResult.(containerservice.ManagedCluster)
	if !ok {
		return fmt.Errorf("expected containerservice ManagedCluster object")
	}

	old := scope.ControlPlane.DeepCopy()

	scope.ControlPlane.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: *managedCluster.ManagedClusterProperties.Fqdn,
		Port: 443,
	}

	if err := r.kubeclient.Patch(ctx, scope.ControlPlane, client.MergeFrom(old)); err != nil {
		return errors.Wrap(err, "failed to set control plane endpoint")
	}

	return nil
}

func (r *azureManagedControlPlaneReconciler) reconcileKubeconfig(ctx context.Context, scope *scope.ManagedControlPlaneScope, managedClusterSpec *managedclusters.Spec) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.azureManagedControlPlaneReconciler.reconcileKubeconfig")
	defer span.End()

	// Always fetch credentials in case of rotation
	data, err := r.managedClustersSvc.GetCredentials(ctx, managedClusterSpec.ResourceGroupName, managedClusterSpec.Name)
	if err != nil {
		return errors.Wrap(err, "failed to get credentials for managed cluster")
	}

	// Construct and store secret
	kubeconfig := makeKubeconfig(scope.Cluster, scope.ControlPlane)
	if _, err := controllerutil.CreateOrUpdate(ctx, r.kubeclient, kubeconfig, func() error {
		kubeconfig.Data = map[string][]byte{
			secret.KubeconfigDataName: data,
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to kubeconfig secret for cluster")
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
