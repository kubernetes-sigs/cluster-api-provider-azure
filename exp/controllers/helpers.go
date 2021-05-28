/*
Copyright 2020 The Kubernetes Authors.

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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/controllers"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AzureClusterToAzureMachinePoolsMapper creates a mapping handler to transform AzureClusters into AzureMachinePools. The transform
// requires AzureCluster to map to the owning Cluster, then from the Cluster, collect the MachinePools belonging to the cluster,
// then finally projecting the infrastructure reference to the AzureMachinePool.
func AzureClusterToAzureMachinePoolsMapper(ctx context.Context, c client.Client, scheme *runtime.Scheme, log logr.Logger) (handler.MapFunc, error) {
	gvk, err := apiutil.GVKForObject(new(infrav1exp.AzureMachinePool), scheme)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find GVK for AzureMachinePool")
	}

	return func(o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		azCluster, ok := o.(*infrav1.AzureCluster)
		if !ok {
			log.Error(errors.Errorf("expected an AzureCluster, got %T instead", o.GetObjectKind()), "failed to map AzureCluster")
			return nil
		}

		log = log.WithValues("AzureCluster", azCluster.Name, "Namespace", azCluster.Namespace)

		// Don't handle deleted AzureClusters
		if !azCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			log.V(4).Info("AzureCluster has a deletion timestamp, skipping mapping.")
			return nil
		}

		clusterName, ok := controllers.GetOwnerClusterName(azCluster.ObjectMeta)
		if !ok {
			log.Info("unable to get the owner cluster")
			return nil
		}

		machineList := &clusterv1exp.MachinePoolList{}
		machineList.SetGroupVersionKind(gvk)
		// list all of the requested objects within the cluster namespace with the cluster name label
		if err := c.List(ctx, machineList, client.InNamespace(azCluster.Namespace), client.MatchingLabels{clusterv1.ClusterLabelName: clusterName}); err != nil {
			return nil
		}

		mapFunc := MachinePoolToInfrastructureMapFunc(gvk, log)
		var results []ctrl.Request
		for _, machine := range machineList.Items {
			m := machine
			azureMachines := mapFunc(&m)
			results = append(results, azureMachines...)
		}

		return results
	}, nil
}

// AzureManagedClusterToAzureManagedMachinePoolsMapper creates a mapping handler to transform AzureManagedClusters into
// AzureManagedMachinePools. The transform requires AzureManagedCluster to map to the owning Cluster, then from the
// Cluster, collect the MachinePools belonging to the cluster, then finally projecting the infrastructure reference
// to the AzureManagedMachinePools.
func AzureManagedClusterToAzureManagedMachinePoolsMapper(ctx context.Context, c client.Client, scheme *runtime.Scheme, log logr.Logger) (handler.MapFunc, error) {
	gvk, err := apiutil.GVKForObject(new(infrav1exp.AzureManagedMachinePool), scheme)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find GVK for AzureManagedMachinePool")
	}

	return func(o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		azCluster, ok := o.(*infrav1exp.AzureManagedCluster)
		if !ok {
			log.Error(errors.Errorf("expected an AzureManagedCluster, got %T instead", o.GetObjectKind()), "failed to map AzureManagedCluster")
			return nil
		}

		log = log.WithValues("AzureManagedCluster", azCluster.Name, "Namespace", azCluster.Namespace)

		// Don't handle deleted AzureManagedClusters
		if !azCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			log.V(4).Info("AzureManagedCluster has a deletion timestamp, skipping mapping.")
			return nil
		}

		clusterName, ok := controllers.GetOwnerClusterName(azCluster.ObjectMeta)
		if !ok {
			log.Info("unable to get the owner cluster")
			return nil
		}

		machineList := &clusterv1exp.MachinePoolList{}
		machineList.SetGroupVersionKind(gvk)
		// list all of the requested objects within the cluster namespace with the cluster name label
		if err := c.List(ctx, machineList, client.InNamespace(azCluster.Namespace), client.MatchingLabels{clusterv1.ClusterLabelName: clusterName}); err != nil {
			return nil
		}

		mapFunc := MachinePoolToInfrastructureMapFunc(gvk, log)
		var results []ctrl.Request
		for _, machine := range machineList.Items {
			m := machine
			azureMachines := mapFunc(&m)
			results = append(results, azureMachines...)
		}

		return results
	}, nil
}

// AzureManagedControlPlaneToAzureManagedMachinePoolsMapper creates a mapping handler to transform AzureManagedControlPlanes into
// AzureManagedMachinePools. The transform requires AzureManagedControlPlane to map to the owning Cluster, then from the
// Cluster, collect the MachinePools belonging to the cluster, then finally projecting the infrastructure reference
// to the AzureManagedMachinePools.
func AzureManagedControlPlaneToAzureManagedMachinePoolsMapper(ctx context.Context, c client.Client, scheme *runtime.Scheme, log logr.Logger) (handler.MapFunc, error) {
	gvk, err := apiutil.GVKForObject(new(infrav1exp.AzureManagedMachinePool), scheme)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find GVK for AzureManagedMachinePool")
	}

	return func(o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		azControlPlane, ok := o.(*infrav1exp.AzureManagedControlPlane)
		if !ok {
			log.Error(errors.Errorf("expected an AzureManagedControlPlane, got %T instead", o.GetObjectKind()), "failed to map AzureManagedControlPlane")
			return nil
		}

		log = log.WithValues("AzureManagedControlPlane", azControlPlane.Name, "Namespace", azControlPlane.Namespace)

		// Don't handle deleted AzureManagedControlPlanes
		if !azControlPlane.ObjectMeta.DeletionTimestamp.IsZero() {
			log.V(4).Info("AzureManagedControlPlane has a deletion timestamp, skipping mapping.")
			return nil
		}

		clusterName, ok := controllers.GetOwnerClusterName(azControlPlane.ObjectMeta)
		if !ok {
			log.Info("unable to get the owner cluster")
			return nil
		}

		machineList := &clusterv1exp.MachinePoolList{}
		machineList.SetGroupVersionKind(gvk)
		// list all of the requested objects within the cluster namespace with the cluster name label
		if err := c.List(ctx, machineList, client.InNamespace(azControlPlane.Namespace), client.MatchingLabels{clusterv1.ClusterLabelName: clusterName}); err != nil {
			return nil
		}

		mapFunc := MachinePoolToInfrastructureMapFunc(gvk, log)
		var results []ctrl.Request
		for _, machine := range machineList.Items {
			m := machine
			azureMachines := mapFunc(&m)
			results = append(results, azureMachines...)
		}

		return results
	}, nil
}

// AzureManagedClusterToAzureManagedControlPlaneMapper creates a mapping handler to transform AzureManagedClusters into
// AzureManagedControlPlane. The transform requires AzureManagedCluster to map to the owning Cluster, then from the
// Cluster, collect the control plane infrastructure reference.
func AzureManagedClusterToAzureManagedControlPlaneMapper(ctx context.Context, c client.Client, log logr.Logger) (handler.MapFunc, error) {
	return func(o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		azCluster, ok := o.(*infrav1exp.AzureManagedCluster)
		if !ok {
			log.Error(errors.Errorf("expected an AzureManagedCluster, got %T instead", o), "failed to map AzureManagedCluster")
			return nil
		}

		log = log.WithValues("AzureManagedCluster", azCluster.Name, "Namespace", azCluster.Namespace)

		// Don't handle deleted AzureManagedClusters
		if !azCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			log.V(4).Info("AzureManagedCluster has a deletion timestamp, skipping mapping.")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, c, azCluster.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to get the owning cluster")
			return nil
		}

		if cluster == nil {
			log.Error(err, "cluster has not set owner ref yet")
			return nil
		}

		ref := cluster.Spec.ControlPlaneRef
		if ref == nil || ref.Name == "" {
			return nil
		}

		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: ref.Namespace,
					Name:      ref.Name,
				},
			},
		}
	}, nil
}

// AzureManagedControlPlaneToAzureManagedClusterMapper creates a mapping handler to transform AzureManagedClusters into
// AzureManagedControlPlane. The transform requires AzureManagedCluster to map to the owning Cluster, then from the
// Cluster, collect the control plane infrastructure reference.
func AzureManagedControlPlaneToAzureManagedClusterMapper(ctx context.Context, c client.Client, log logr.Logger) (handler.MapFunc, error) {
	return func(o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		azManagedControlPlane, ok := o.(*infrav1exp.AzureManagedControlPlane)
		if !ok {
			log.Error(errors.Errorf("expected an AzureManagedControlPlane, got %T instead", o), "failed to map AzureManagedControlPlane")
			return nil
		}

		log = log.WithValues("AzureManagedControlPlane", azManagedControlPlane.Name, "Namespace", azManagedControlPlane.Namespace)

		// Don't handle deleted AzureManagedControlPlanes
		if !azManagedControlPlane.ObjectMeta.DeletionTimestamp.IsZero() {
			log.V(4).Info("AzureManagedControlPlane has a deletion timestamp, skipping mapping.")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, c, azManagedControlPlane.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to get the owning cluster")
			return nil
		}

		if cluster == nil {
			log.Error(err, "cluster has not set owner ref yet")
			return nil
		}

		ref := cluster.Spec.InfrastructureRef
		if ref == nil || ref.Name == "" {
			return nil
		}

		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: ref.Namespace,
					Name:      ref.Name,
				},
			},
		}
	}, nil
}

// MachinePoolToAzureManagedControlPlaneMapFunc returns a handler.MapFunc that watches for
// MachinePool events and returns reconciliation requests for a control plane object.
func MachinePoolToAzureManagedControlPlaneMapFunc(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, log logr.Logger) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		machinePool, ok := o.(*clusterv1exp.MachinePool)
		if !ok {
			log.Info("expected a MachinePool, got wrong type", "type", fmt.Sprintf("%T", o))
			return nil
		}

		cluster, err := util.GetClusterByName(ctx, c, machinePool.ObjectMeta.Namespace, machinePool.Spec.ClusterName)
		if err != nil {
			log.Error(err, "failed to get the owning cluster")
			return nil
		}

		gk := gvk.GroupKind()
		ref := cluster.Spec.ControlPlaneRef
		// Return early if the GroupKind doesn't match what we expect.
		controlPlaneGK := ref.GroupVersionKind().GroupKind()
		if gk != controlPlaneGK {
			log.Info("gk does not match", "gk", gk, "controlPlaneGK", controlPlaneGK)
			return nil
		}

		controlPlaneKey := client.ObjectKey{
			Name:      ref.Name,
			Namespace: ref.Namespace,
		}
		controlPlane := &infrav1exp.AzureManagedControlPlane{}
		if err := c.Get(ctx, controlPlaneKey, controlPlane); err != nil {
			log.Error(err, "failed to fetch default pool reference")
			// If we get here, we might want to reconcile but aren't sure.
			// Do it anyway to be safe. Worst case we reconcile a few extra times with no-ops.
			return []reconcile.Request{
				{
					NamespacedName: client.ObjectKey{
						Namespace: ref.Namespace,
						Name:      ref.Name,
					},
				},
			}
		}

		infraMachinePoolRef := machinePool.Spec.Template.Spec.InfrastructureRef

		gv, err := schema.ParseGroupVersion(infraMachinePoolRef.APIVersion)
		if err != nil {
			log.Error(err, "failed to parse group version")
			// If we get here, we might want to reconcile but aren't sure.
			// Do it anyway to be safe. Worst case we reconcile a few extra times with no-ops.
			return []reconcile.Request{
				{
					NamespacedName: client.ObjectKey{
						Namespace: ref.Namespace,
						Name:      ref.Name,
					},
				},
			}
		}

		nameMatches := controlPlane.Spec.DefaultPoolRef.Name == infraMachinePoolRef.Name
		kindMatches := infraMachinePoolRef.Kind == "AzureManagedMachinePool"
		groupMatches := controlPlaneGK.Group == gv.Group

		if groupMatches && kindMatches && nameMatches {
			return []reconcile.Request{
				{
					NamespacedName: client.ObjectKey{
						Namespace: ref.Namespace,
						Name:      ref.Name,
					},
				},
			}
		}

		// By default, return nothing for a machine pool which is not the default pool for a control plane.
		return nil
	}
}

// MachinePoolToInfrastructureMapFunc returns a handler.MapFunc that watches for
// MachinePool events and returns reconciliation requests for an infrastructure provider object.
func MachinePoolToInfrastructureMapFunc(gvk schema.GroupVersionKind, log logr.Logger) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		m, ok := o.(*clusterv1exp.MachinePool)
		if !ok {
			log.Info("attempt to map incorrect type", "type", fmt.Sprintf("%T", o))
			return nil
		}

		gk := gvk.GroupKind()
		ref := m.Spec.Template.Spec.InfrastructureRef
		// Return early if the GroupKind doesn't match what we expect.
		infraGK := ref.GroupVersionKind().GroupKind()
		if gk != infraGK {
			log.Info("gk does not match", "gk", gk, "infraGK", infraGK)
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      ref.Name,
				},
			},
		}
	}
}

// AzureClusterToAzureMachinePoolsFunc is a handler.MapFunc to be used to enqueue
// requests for reconciliation of AzureMachinePools.
func AzureClusterToAzureMachinePoolsFunc(ctx context.Context, kClient client.Client, log logr.Logger) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		c, ok := o.(*infrav1.AzureCluster)
		if !ok {
			log.Error(errors.Errorf("expected a AzureCluster but got a %T", o), "failed to get AzureCluster")
			return nil
		}
		logWithValues := log.WithValues("AzureCluster", c.Name, "Namespace", c.Namespace)

		cluster, err := util.GetOwnerCluster(ctx, kClient, c.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			logWithValues.Info("owning cluster not found")
			return nil
		case err != nil:
			logWithValues.Error(err, "failed to get owning cluster")
			return nil
		}

		labels := map[string]string{clusterv1.ClusterLabelName: cluster.Name}
		ampl := &infrav1exp.AzureMachinePoolList{}
		if err := kClient.List(ctx, ampl, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
			logWithValues.Error(err, "failed to list AzureMachinePools")
			return nil
		}

		var result []reconcile.Request
		for _, m := range ampl.Items {
			result = append(result, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      m.Name,
				},
			})
		}

		return result
	}
}
