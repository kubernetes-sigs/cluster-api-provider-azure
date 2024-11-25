## Adopting Existing AKS Clusters

### Option 1: Using the new AzureASOManaged API

The [AzureASOManagedControlPlane and related APIs](./asomanagedcluster.md) support
adoption as a first-class use case. Going forward, this method is likely to be easier, more reliable, include
more features, and better supported for adopting AKS clusters than Option 2 below.

To adopt an AKS cluster into a full Cluster API Cluster, create an ASO ManagedCluster and associated
ManagedClustersAgentPool resources annotated with `sigs.k8s.io/cluster-api-provider-azure-adopt=true`. The
annotation may also be added to existing ASO resources to trigger adoption. CAPZ will automatically scaffold
the Cluster API resources like the Cluster, AzureASOManagedCluster, AzureASOManagedControlPlane, MachinePools,
and AzureASOManagedMachinePools. The [`asoctl import
azure-resource`](https://azure.github.io/azure-service-operator/tools/asoctl/#import-azure-resource) command
can help generate the required YAML.

This method can also be used to [migrate](./asomanagedcluster#migrating-existing-clusters-to-azureasomanagedcontrolplane) from AzureManagedControlPlane and its associated APIs.

#### Caveats

- CAPZ currently only records the ASO resources in the CAPZ resources' `spec.resources` that it needs to
  function, which include the ManagedCluster, its ResourceGroup, and associated ManagedClustersAgentPools.
  Other resources owned by the ManagedCluster like Kubernetes extensions or Fleet memberships are not
  currently imported to the CAPZ specs.
- Configuring the automatically generated Cluster API resources is not currently possible. If you need to
  change something like the `metadata.name` of a resource from what CAPZ generates, create the Cluster API
  resources manually referencing the pre-existing resources.
- Adopting existing clusters created with the GA AzureManagedControlPlane API to the experimental API with
  this method is theoretically possible, but untested. Care should be taken to prevent CAPZ from reconciling
  two different representations of the same underlying Azure resources.
- This method cannot be used to import existing clusters as a ClusterClass or a topology, only as a standalone
  Cluster.

### Option 2: Using the current AzureManagedControlPlane API

<aside class="note">

<h1> Warning </h1>

Note: This is a newly-supported feature in CAPZ that is less battle-tested than most other features. Potential
bugs or misuse can result in misconfigured or deleted Azure resources. Use with caution.

</aside>

CAPZ can adopt some AKS clusters created by other means under its management. This works by crafting CAPI and
CAPZ manifests which describe the existing cluster and creating those resources on the CAPI management
cluster. This approach is limited to clusters which can be described by the CAPZ API, which includes the
following constraints:

- the cluster operates within a single Virtual Network and Subnet
- the cluster's Virtual Network exists outside of the AKS-managed `MC_*` resource group
- the cluster's Virtual Network and Subnet are not shared with any other resources outside the context of this cluster

To ensure CAPZ does not introduce any unwarranted changes while adopting an existing cluster, carefully review
the [entire AzureManagedControlPlane spec](../reference/v1beta1-api#infrastructure.cluster.x-k8s.io/v1beta1.AzureManagedControlPlaneSpec)
and specify _every_ field in the CAPZ resource. CAPZ's webhooks apply defaults to many fields which may not
match the existing cluster.

Specific AKS features not represented in the CAPZ API, like those from a newer AKS API version than CAPZ uses,
do not need to be specified in the CAPZ resources to remain configured the way they are. CAPZ will still not
be able to manage that configuration, but it will not modify any settings beyond those for which it has
knowledge.

By default, CAPZ will not make any changes to or delete any pre-existing Resource Group, Virtual Network, or
Subnet resources. To opt-in to CAPZ management for those clusters, tag those resources with the following
before creating the CAPZ resources: `sigs.k8s.io_cluster-api-provider-azure_cluster_<CAPI Cluster name>: owned`.
Managed Cluster and Agent Pool resources do not need this tag in order to be adopted.

After applying the CAPI and CAPZ resources for the cluster, other means of managing the cluster should be
disabled to avoid ongoing conflicts with CAPZ's reconciliation process.

#### Pitfalls

The following describes some specific pieces of configuration that deserve particularly careful attention,
adapted from https://gist.github.com/mtougeron/1e5d7a30df396cd4728a26b2555e0ef0#file-capz-md.

- Make sure `AzureManagedControlPlane.metadata.name` matches the AKS cluster name
- Set the `AzureManagedControlPlane.spec.virtualNetwork` fields to match your existing VNET
- Make sure the `AzureManagedControlPlane.spec.sshPublicKey` matches what was set on the AKS cluster. (including any potential newlines included in the base64 encoding)
  - NOTE: This is a required field in CAPZ, if you don't know what public key was used, you can _change_ or _set_ it via the Azure CLI however before attempting to import the cluster.
- Make sure the `Cluster.spec.clusterNetwork` settings match properly to what you are using in AKS
- Make sure the `AzureManagedControlPlane.spec.dnsServiceIP` matches what is set in AKS
- Set the tag `sigs.k8s.io_cluster-api-provider-azure_cluster_<clusterName>` = `owned` on the AKS cluster
- Set the tag `sigs.k8s.io_cluster-api-provider-azure_role` = `common` on the AKS cluster

NOTE: Several fields, like `networkPlugin`, if not set on the AKS cluster at creation time, will mean that CAPZ will not be able to set that field. AKS doesn't allow such fields to be changed if not set at creation. However, if it was set at creation time, CAPZ will be able to successfully change/manage the field.
