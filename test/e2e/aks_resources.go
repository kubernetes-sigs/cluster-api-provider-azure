//go:build e2e
// +build e2e

/*
Copyright 2026 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"fmt"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20250801"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func getAKSControlPlane(ctx context.Context, c client.Client, cluster *clusterv1.Cluster) (client.Object, error) {
	var controlPlane client.Object
	switch cluster.Spec.ControlPlaneRef.Kind {
	case infrav1.AzureManagedControlPlaneKind:
		controlPlane = &infrav1.AzureManagedControlPlane{}
	case infrav1.AzureASOManagedControlPlaneKind:
		controlPlane = &infrav1.AzureASOManagedControlPlane{}
	default:
		return nil, fmt.Errorf("unsupported AKS control plane kind %q", cluster.Spec.ControlPlaneRef.Kind)
	}
	err := c.Get(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.ControlPlaneRef.Name}, controlPlane)
	return controlPlane, err
}

func findAKSResource(resources []runtime.RawExtension, kind string) (*unstructured.Unstructured, int, error) {
	for i, raw := range resources {
		resource := &unstructured.Unstructured{}
		if err := resource.UnmarshalJSON(raw.Raw); err != nil {
			return nil, 0, err
		}
		if resource.GroupVersionKind().Group == asocontainerservicev1.GroupVersion.Group && resource.GetKind() == kind {
			return resource, i, nil
		}
	}
	return nil, 0, fmt.Errorf("no %s %s in spec.resources", asocontainerservicev1.GroupVersion.Group, kind)
}

func getAKSManagedCluster(ctx context.Context, c client.Client, controlPlane client.Object) (*asocontainerservicev1.ManagedCluster, error) {
	var name string
	switch cp := controlPlane.(type) {
	case *infrav1.AzureManagedControlPlane:
		name = azure.GetNormalizedKubernetesName(cp.Name)
	case *infrav1.AzureASOManagedControlPlane:
		resource, _, err := findAKSResource(cp.Spec.Resources, "ManagedCluster")
		if err != nil {
			return nil, err
		}
		name = resource.GetName()
	default:
		return nil, fmt.Errorf("unsupported AKS control plane %T", controlPlane)
	}
	managedCluster := &asocontainerservicev1.ManagedCluster{}
	err := c.Get(ctx, client.ObjectKey{Namespace: controlPlane.GetNamespace(), Name: name}, managedCluster)
	return managedCluster, err
}

func getAKSClusterNames(ctx context.Context, c client.Client, controlPlane client.Object) (string, string, error) {
	managedCluster, err := getAKSManagedCluster(ctx, c, controlPlane)
	if err != nil {
		return "", "", err
	}
	if managedCluster.Spec.Owner == nil || managedCluster.Spec.Owner.Name == "" {
		return "", "", fmt.Errorf("ManagedCluster %s/%s has no ResourceGroup owner", managedCluster.Namespace, managedCluster.Name)
	}
	resourceGroup := &asoresourcesv1.ResourceGroup{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: managedCluster.Namespace, Name: managedCluster.Spec.Owner.Name}, resourceGroup); err != nil {
		return "", "", err
	}
	if resourceGroup.AzureName() == "" || managedCluster.AzureName() == "" {
		return "", "", fmt.Errorf("ManagedCluster %s/%s or its ResourceGroup has no Azure name", managedCluster.Namespace, managedCluster.Name)
	}
	return resourceGroup.AzureName(), managedCluster.AzureName(), nil
}

func getAKSAgentPoolName(ctx context.Context, c client.Client, machinePool *clusterv1.MachinePool) (string, error) {
	key := client.ObjectKey{Namespace: machinePool.Namespace, Name: machinePool.Spec.Template.Spec.InfrastructureRef.Name}
	var name string
	switch machinePool.Spec.Template.Spec.InfrastructureRef.Kind {
	case infrav1.AzureManagedMachinePoolKind:
		pool := &infrav1.AzureManagedMachinePool{}
		if err := c.Get(ctx, key, pool); err != nil {
			return "", err
		}
		name = ptr.Deref(pool.Spec.Name, "")
	case infrav1.AzureASOManagedMachinePoolKind:
		pool := &infrav1.AzureASOManagedMachinePool{}
		if err := c.Get(ctx, key, pool); err != nil {
			return "", err
		}
		resource, _, err := findAKSResource(pool.Spec.Resources, "ManagedClustersAgentPool")
		if err != nil {
			return "", err
		}
		agentPool := &asocontainerservicev1.ManagedClustersAgentPool{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: machinePool.Namespace, Name: resource.GetName()}, agentPool); err != nil {
			return "", err
		}
		name = agentPool.AzureName()
	default:
		return "", fmt.Errorf("unsupported AKS machine pool kind %q", machinePool.Spec.Template.Spec.InfrastructureRef.Kind)
	}
	if name == "" {
		return "", fmt.Errorf("infrastructure resource for MachinePool %s/%s has no Azure agent pool name", machinePool.Namespace, machinePool.Name)
	}
	return name, nil
}
