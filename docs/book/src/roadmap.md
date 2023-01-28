# Cluster API Azure Roadmap

The best place to see what's coming is in [the public milestones](https://github.com/kubernetes-sigs/cluster-api-provider-azure/milestones).  
The next numbered milestone (e.g. **1.8**) is planned at the very beginning of the 2-month release cycle. This planning and discussion begins at [Cluster API Azure Office Hours](http://bit.ly/k8s-capz-agenda) after a major release.
Active community PR contributions are prioritized throughout the release, but unplanned work will arise. Hence the items in the milestone are a rough estimate which may change.
The "next" milestone is a very rough collection of issues for the milestone after the current numbered one to help prioritize upcoming work.

## High Level Vision

CAPZ is the official production-ready Cluster API implementation to administer the entire lifecycle of self-managed or managed Kubernetes clusters (AKS) on Azure. Cluster API extends the Kubernetes API to provide tooling consistent across on-premises and cloud providers to build and maintain Kubernetes clusters at scale while working with GitOps and the surrounding tooling ecosystem.

## Epics

There are a number of large priority "Epics" which may span across milestones which we believe are important to providing CAPZ users an even better experience and improving the vision.  

-  Latest Core Dependencies - The foundation should keep everything under a supported version of dependencies as well as to enable the latest features. 
    - Includes: [track 2 go-sdk](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2670), [Azure Service Operator (ASO)](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/416), 100% [k8s out-of-tree Azure provider](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/715)
-  ManagedClusters - Provisioning new AKS (ManagedClusters) clusters at scale is a common use case we want to enable an excellent experience with stability and all of the same features available in AKS.
    - Includes: [ManagedClusters E2E Tests](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2873), [ClusterClass support](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2684), [Enabling all AKS features](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2625), [Evolution of standardized CAPI ManagedCluster for CAPZ](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220725-managed-kubernetes.md)
-  Latest features for self-managed - These larger or essential features enable a better experience for provisioning self-managed clusters on Azure.
    - Includes: MachinePools support, Azure CNI v2 support
