# Managed Clusters

This section contains information specific to configuring managed (AKS) Kubernetes clusters using Cluster API Provider for Azure (CAPZ).
See [self-managed clusters](../self-managed/self-managed.md) for information specific to provisioning these clusters.

Documents under the Managed Clusters area:
- [Adopting Clusters](managed.md)
- [ASO Managed Clusters](asomanagedcluster.md)
- [Managed Clusters](managedcluster.md)
- [Managed Clusters - Join VMSS Nodes](adopting-clusters.md)
- [Troubleshooting](troubleshooting.md)

## CAPZ's Managed Cluster versus CAPZ's ASOManaged Cluster

There are two APIs which can create AKS clusters and it's important to understand the differences.

**ManagedCluster** - is the original API to provision AKS clusters introduced with the [0.4.4 release](https://github.com/kubernetes-sigs/cluster-api-provider-azure/releases/tag/v0.4.4).  This more closely matches the CAPI style for YAML code for other providers. The original code was based on directly using the Azure Go SDK, but in the [1.11.0 release](https://github.com/kubernetes-sigs/cluster-api-provider-azure/releases/tag/v1.11.0) was switched to utilize Azure Service Operator (ASO) as a dependency for provisioning.  This supports the preview API, but does not natively support all AKS features available.  The ManagedCluster API will eventually be deprecated, [see the roadmap for more information](../roadmap.md).

**ASOManagedCluster** - was created in the [1.15 release of CAPZ](https://github.com/kubernetes-sigs/cluster-api-provider-azure/releases/tag/v1.15.0) and creates a CAPI-compliant wrapper around the existing ASO definitions for AKS clusters.  It has 100% API coverage for the preview and current AKS APIs via the ASO AKS CRDs.  This is the long-term plan to support provisioning AKS clusters using CAPZ.  [See the roadmap for more information](../roadmap.md)

### Why is ASOManagedCluster the future?

The biggest challenge with ManagedCluster is attempting to keep up with the velocity of feature changes to the frequently changing AKS API.  This model requires every single feature to be added into code directly.  Even with the simplification of code to utilize ASO as a dependency, there still is quite a bit of time required to keep up with these features.  Ultimately, this is an unsustainable path long-term.  The [asoManaged*Patches feature](managedcluster.md#warning-warning-this-is-meant-to-be-used-sparingly-to-enable-features-for-development-and-testing-that-are-not-otherwise-represented-in-the-capz-api-misconfiguration-that-conflicts-with-capzs-normal-mode-of-operation-is-possible) enables patching/adding various AKS fields via ASO to help try to fill some of these gaps, but this comes at the cost of unknown full testing.

ASOManagedCluster enables 100% API coverage natively and is easy to keep up with since it is primarily a dependency update to CAPZ.  Furthermore it has the advantage of making it easier to import existing deployed clusters which have no existing CAPZ or ASO YAML code defined.

For the complete history of this transition, see the [Managing Azure Resources with Azure Service Operator](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/docs/proposals/20230123-azure-service-operator.md) and [Automate AKS Features available in CAPZ](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/docs/proposals/20231122-automate-aks-features.md) design proposal documents.

## ASO's AKS versus CAPZ's ASOManaged AKS

It is possible to not utilize CAPZ at all and simply utilize ASO to provision an [AKS cluster definition directly](https://azure.github.io/azure-service-operator/reference/containerservice/v1api20231001/#containerservice.azure.com/v1api20240901.ManagedCluster).  The advantages that CAPZ brings over this approach are the following:
  - Robust Testing - CAPZ is utilized to test Kubernetes and AKS using this code with numerous end-to-end tests.  ASO has no AKS-specific testing.
  - Simplification of Infrastructure as Code (IaC) definitions - With ASO you have to figure out how to put together every field and there are some small examples.  CAPZ provides `kustomize` template samples connected to `clusterctl generate template` as well as a [helm chart](https://github.com/mboersma/cluster-api-charts/).
  - Management scale - CAPZ enables use of [ClusterClass](../topics/clusterclass.md) so you can have a smaller chunk of code to manage numerous clusters with the same configuration.
  - Heterogeneous Kubernetes management - it is possible with CAPZ to manage self-managed (not possible at all with ASO) and managed clusters with a single management control plane and similar IaC definitions. 
  - Multi-cloud IaC consistency - even though it's still wrapping ASO, there still is some consistency in the code contract to provision Kubernetes clusters with the [~30 other CAPI infrastructure providers](https://cluster-api.sigs.k8s.io/reference/providers).

## General AKS Best Practices

A set of best practices for managing AKS clusters is documented here: [https://learn.microsoft.com/azure/aks/best-practices](https://learn.microsoft.com/azure/aks/best-practices)
