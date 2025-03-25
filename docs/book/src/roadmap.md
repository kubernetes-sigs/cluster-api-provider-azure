# Cluster API Azure Roadmap

The best place to see what's coming within a 1-2 month timeline is in [the public milestones](https://github.com/kubernetes-sigs/cluster-api-provider-azure/milestones).
All open items for the the next numbered milestone (e.g. **1.17**) are visualized in the [Milestone-Open project board view](https://github.com/orgs/kubernetes-sigs/projects/26/views/7) and planned at the very beginning of the 2-month release cycle. This planning and discussion begins at [Cluster API Azure Office Hours](http://bit.ly/k8s-capz-agenda) after a major release. The [CAPZ project board roadmap view](https://github.com/orgs/kubernetes-sigs/projects/26/views/11) tracks the larger "epic issues" and their progress.
Active community PR contributions are prioritized throughout the release, but unplanned work will arise. Hence the items in the milestone are a rough estimate which may change.
The "next" milestone is a very rough collection of issues for the milestone after the current numbered one to help prioritize upcoming work.

## High Level Vision

CAPZ is the official production-ready Cluster API implementation to administer the entire lifecycle of self-managed or managed Kubernetes clusters (AKS) on Azure. Cluster API extends the Kubernetes API to provide tooling consistent across on-premises and cloud providers to build and maintain Kubernetes clusters at scale while working with GitOps and the surrounding tooling ecosystem. See [related blog post](https://cloudblogs.microsoft.com/opensource/2023/04/20/kubernetes-at-scale-with-gitops-and-cluster-api/).

Azure Service Operator (ASO) is automatically installed with CAPZ and can be utilized in addition to CAPZ's Kubernetes cluster Infrastructure as Code (IaC) definition to manage all of your Azure resources on the same management cluster. See [AKS Platform Engineering code sample](https://github.com/Azure-Samples/aks-platform-engineering).

## Long-Term Priorities

CAPZ can provision three major types of clusters, each of which have a different investment priority.

1. Self-Managed clusters - maintain the current functionality via bug fixes and security patches.  New features will be accepted via contributor pull requests.
2. Managed clusters (AzureManaged* current API) - maintain the current functionality via bug fixes and security patches.  New features will be accepted via contributor pull requests. It is recommended that the existing [asoManaged*Patches functionality](./managed/managedcluster.md#warning-warning-this-is-meant-to-be-used-sparingly-to-enable-features-for-development-and-testing-that-are-not-otherwise-represented-in-the-capz-api-misconfiguration-that-conflicts-with-capzs-normal-mode-of-operation-is-possible) be considered as a stop-gap to missing features in the CAPZ definitions for AKS.
See deprecation timeline below.
3. Managed clusters (AzureASOManaged* new API) - was moved out of experimentation in July for the 1.16 release and is where the investment lies moving forward for provisioning AKS clusters.

See the [managed clusters](./managed/managed.md) for further background and comparison of the two managed cluster APIs.

Approximate timeline for deprecation of AzureManaged API:
- Mar 2025 - 1.19
- May 2025 - 1.20: Move AzurASOManaged API to beta
- July 2025 - 1.21: No new features accepted for AzureManaged API. Warning message in code for deprecation.
- Sept 2025 - 1.22: GA AzureASOManaged API
- Dec 2025 - GA 1.23: AzureManaged API is removed from code base, AzureASOManaged API is default.

There may be investment creating a new API definition for AKS from the [Managed Kubernetes CAPI proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220725-managed-kubernetes.md) at some point in the future.  If interested in this functionality, please file an issue on the CAPZ repository and come to the community group meeting to discuss.