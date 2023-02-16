# ClusterClass

- **Feature status:** GA
- **Feature gate:** MachinePool=true ClusterTopology=true

[ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html) is a collection of templates that define a topology (control plane and machine deployments) to be used to continuously reconcile one or more Clusters. It is a new Cluster API feature that is built on top of the existing Cluster API resources and provides a set of tools and operations to streamline cluster lifecycle management while maintaining the same underlying API.

CAPZ currently supports ClusterClass for both managed (AKS) and self-managed clusters. CAPZ implements this with three custom resources:
1. AzureClusterTemplate
2. AzureManagedClusterTemplate
3. AzureManagedControlPlaneTemplate

Each resource is a template for the corresponding CAPZ resource. For example, the AzureClusterTemplate is a template for the CAPZ AzureCluster resource. The template contains a set of parameters that are able to be shared across multiple clusters.

## Deploying a Self-Managed Cluster with ClusterClass

To deploy a self-managed cluster with ClusterClass, you must first create a ClusterClass resource. The ClusterClass resource defines the cluster topology, including the control plane and machine deployment templates. The ClusterClass resource also defines the parameters that can be used to customize the cluster topology. 

Please refer to the Cluster API book for more information on how to write a ClusterClass topology: https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/write-clusterclass.html

For a self-managed cluster, the AzureClusterTemplate is used to define the Azure infrastructure for the cluster. The following example shows a basic AzureClusterTemplate resource:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterTemplate
metadata:
  name: capz-clusterclass-cluster
  namespace: default
spec:
  template:
    spec:
      location: westus2
      networkSpec:
        subnets:
        - name: control-plane-subnet
          role: control-plane
        - name: node-subnet
          natGateway:
            name: node-natgateway
          role: node
      subscriptionID: 00000000-0000-0000-0000-000000000000
```

## Deploying a Managed Cluster (AKS) with ClusterClass

Deploying an AKS cluster with ClusterClass is similar to deploying a self-managed cluster. Instead of using the AzureClusterTemplate, you must use both an AzureManagedClusterTemplate and AzureManagedControlPlaneTemplate. Due to the nature of managed Kubernetes and the control plane implementation, the infrastructure provider (and therefore the AzureManagedClusterTemplate) for AKS cluster is basically a no-op. The AzureManagedControlPlaneTemplate is used to define the AKS cluster configuration, such as the Kubernetes version and the number of nodes. 

The following example shows a basic AzureManagedClusterTemplate and AzureManagedControlPlaneTemplate resource:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedClusterTemplate
metadata:
  name: capz-clusterclass-cluster
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedControlPlaneTemplate
metadata:
  name: capz-clusterclass-control-plane
spec:
  location: westus2
  subscriptionID: 00000000-0000-0000-0000-000000000000
  version: 1.25.2
```

## Excluded Fields

Since a ClusterClass is a template for a Cluster, there are some fields that are not allowed to be shared across multiple clusters. For each of the ClusterClass resources, the following fields are excluded:

### AzureClusterTemplate
- `spec.resourceGroup`
- `spec.controlPlaneEndpoint`
- `spec.bastionSpec.azureBastion.name`
- `spec.bastionSpec.azureBastion.subnetSpec.routeTable`
- `spec.bastionSpec.azureBastion.publicIP`
- `spec.bastionSpec.azureBastion.sku`
- `spec.bastionSpec.azureBastion.enableTunneling`

### AzureManagedControlPlaneTemplate

- `spec.resourceGroupName`
- `spec.nodeResourceGroupName`
- `spec.virtualNetwork.name`
- `spec.virtualNetwork.subnet`
- `spec.virtualNetwork.resourceGroup`
- `spec.controlPlaneEndpoint`
- `spec.sshPublicKey`
- `spec.apiServerAccessProfile.authorizedIPRanges`
