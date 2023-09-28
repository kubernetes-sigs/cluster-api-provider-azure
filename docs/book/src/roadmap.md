# Cluster API Azure Roadmap

The best place to see what's coming within a 1-2 month timeline is in [the public milestones](https://github.com/kubernetes-sigs/cluster-api-provider-azure/milestones).
All open items for the the next numbered milestone (e.g. **1.12**) are visualized in the [Milestone-Open project board view](https://github.com/orgs/kubernetes-sigs/projects/26/views/7) and planned at the very beginning of the 2-month release cycle. This planning and discussion begins at [Cluster API Azure Office Hours](http://bit.ly/k8s-capz-agenda) after a major release.
Active community PR contributions are prioritized throughout the release, but unplanned work will arise. Hence the items in the milestone are a rough estimate which may change.
The "next" milestone is a very rough collection of issues for the milestone after the current numbered one to help prioritize upcoming work.

## High Level Vision

CAPZ is the official production-ready Cluster API implementation to administer the entire lifecycle of self-managed or managed Kubernetes clusters (AKS) on Azure. Cluster API extends the Kubernetes API to provide tooling consistent across on-premises and cloud providers to build and maintain Kubernetes clusters at scale while working with GitOps and the surrounding tooling ecosystem. [See related blog post.](https://cloudblogs.microsoft.com/opensource/2023/04/20/kubernetes-at-scale-with-gitops-and-cluster-api/)

## Epics

There are a number of large priority "Epics" which may span across milestones which we believe are important to providing CAPZ users an even better experience and improving the vision.  The [CAPZ project board roadmap view](https://github.com/orgs/kubernetes-sigs/projects/26/views/11) tracks the larger "epic issues" and their progress.

- ManagedClusters - Provisioning new AKS (ManagedClusters) clusters at scale is a common use case we want to enable an excellent experience with stability and all of the same features available in AKS. Includes:
  - [ClusterClass support](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2684)
  - [Enabling all AKS features](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3446)
  - [Enable AKS preview features](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2625)
  - [Evolution of standardized CAPI ManagedCluster for CAPZ](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220725-managed-kubernetes.md)

- Latest features for self-managed - These larger or essential features enable a better experience for provisioning self-managed clusters on Azure. Includes:
  - [MachinePool autoscale support](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2328)
  - Azure CNI v2 support (waiting on functionality to be available from the Azure Networking team)

- Supported adoption of existing AKS clusters - makes it easier to onboard to the CAPZ/ASO control plane. Includes:
  - [BYO nodes on AKS](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/826)
  - [Adopt existing AKS clusters](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/1173)
