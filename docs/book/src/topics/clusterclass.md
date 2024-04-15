# ClusterClass

- **Feature status:** Experimental
- **Feature gate:** `ClusterTopology=true`

[ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html) is a collection of templates that define a topology (control plane and machine deployments) to be used to continuously reconcile one or more Clusters. It is built on top of the existing Cluster API resources and provides a set of tools and operations to streamline cluster lifecycle management while maintaining the same underlying API.

CAPZ currently supports ClusterClass for both managed (AKS) and self-managed clusters. CAPZ implements this with four custom resources:
1. AzureClusterTemplate
2. AzureManagedClusterTemplate
3. AzureManagedControlPlaneTemplate
4. AzureManagedMachinePoolTemplate

Each resource is a template for the corresponding CAPZ resource. For example, the AzureClusterTemplate is a template for the CAPZ AzureCluster resource. The template contains a set of parameters that are able to be shared across multiple clusters.

## Deploying a Self-Managed Cluster with ClusterClass

Users must first create a ClusterClass resource to deploy a self-managed cluster with ClusterClass. The ClusterClass resource defines the cluster topology, including the control plane and machine deployment templates. The ClusterClass resource also defines the parameters that can be used to customize the cluster topology. 

Please refer to the Cluster API book for more information on how to write a ClusterClass topology: https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/write-clusterclass.html

For a self-managed cluster, the AzureClusterTemplate defines the Azure infrastructure for the cluster. The following example shows a basic AzureClusterTemplate resource:

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

**Feature gate:** `MachinePool=true`

Deploying an AKS cluster with ClusterClass is similar to deploying a self-managed cluster. However, both an AzureManagedClusterTemplate and AzureManagedControlPlaneTemplate must be used instead of the AzureClusterTemplate. Due to the nature of managed Kubernetes and the control plane implementation, the infrastructure provider (and therefore the AzureManagedClusterTemplate) for AKS cluster is basically a no-op. The AzureManagedControlPlaneTemplate is used to define the AKS cluster configuration, such as the Kubernetes version and the number of nodes. Finally, the AzureManagedMachinePoolTemplate defines the worker nodes (agentpools) for the AKS cluster.

<aside class="note warning">

<h1> Warning </h1>

The field `virtualNetwork.Name` should not be set in the AzureManagedControlPlaneTemplate. Setting this field will result in an error with conflicting vnet names when creating multiple clusters with one template. To prevent a breaking API change, this field is not removed from the API, but it should not be used.

</aside>

The following example shows a basic AzureManagedClusterTemplate, AzureManagedControlPlaneTemplate, and AzureManagedMachinePoolTemplate resource:

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
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureManagedMachinePoolTemplate
metadata:
  name: capz-clusterclass-pool0
  namespace: default
spec:
  template:
    spec:
      mode: System
      name: pool0
      sku: Standard_D2s_v3
```
