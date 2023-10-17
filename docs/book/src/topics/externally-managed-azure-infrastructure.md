# Externally managed Azure infrastructure

Normally, Cluster API will create infrastructure on Azure when standing up a new workload cluster. However, it is possible to have Cluster API reuse existing Azure infrastructure instead of creating its own infrastructure.

CAPZ supports [externally managed cluster infrastructure](https://github.com/kubernetes-sigs/cluster-api/blob/10d89ceca938e4d3d94a1d1c2b60515bcdf39829/docs/proposals/20210203-externally-managed-cluster-infrastructure.md).
If the `AzureCluster` resource includes a "cluster.x-k8s.io/managed-by" annotation then the [controller will skip any reconciliation](https://cluster-api.sigs.k8s.io/developer/providers/cluster-infrastructure.html#normal-resource).
This is useful for scenarios where a different persona is managing the cluster infrastructure out-of-band while still wanting to use CAPI for automated machine management.

You should only use this feature if your cluster infrastructure lifecycle management has constraints that the reference implementation does not support. See [user stories](https://github.com/kubernetes-sigs/cluster-api/blob/10d89ceca938e4d3d94a1d1c2b60515bcdf39829/docs/proposals/20210203-externally-managed-cluster-infrastructure.md#user-stories) for more details. 
