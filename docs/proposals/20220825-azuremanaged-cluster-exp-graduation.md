---
title: AzureManagedCluster graduation from experimental
authors:
  - "@jackfrancis"
reviewers:
  - @CecileRobertMichon
  - @zmalik
  - @NovemberZulu
  - @mtougeron
  - @nojnhuh
creation-date: 2022-08-25
last-updated: 2022-08-25
status: implementable
see-also:
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2204
- https://github.com/kubernetes-sigs/cluster-api/pull/6988
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/2739
---


# AzureManagedCluster graduation from experimental

## Summary

`AzureManagedCluster` and its corresponding set of CRDs (we will refer to these CRDs as simply "`AzureManagedCluster`" in this document) is a CAPZ-native implementation of Azure Managed Kubernetes (AKS). Because there is no standard set of Cluster API resource definitions for a "Managed Kubernetes cluster", it is left up to the provider to reuse the existing Cluster API specification (for example, the `Cluster` and its to-be-implemented-by-provider properties such as `ControlPlaneEndpoint`, `ControlPlaneRef` and `InfrastructureRef`). As a result, CAPZ implemented "`AzureManagedCluster`" with an API contract designation of "experimental", to allow for rapid prototyping and discovery.

With the recent adoption of "`AzureManagedCluster`" by the CAPZ community for practical, real-world use, we want to identify the set of outstanding items that may prevent graduation from experimental, and address each one of them, so that future adoption can be unlocked, and users can confidently build resilient systems on top of a stable API.

## Issue Tracking

Issues that emerge as a result of this effort will be tracked here:

- https://github.com/orgs/kubernetes-sigs/projects/26

## Motivation

### Goals
- Agree upon a durable, post-experimental specification of the set of CRDs that implement Managed Kubernetes on Azure, e.g., `AzureManagedCluster`, `AzureManagedControlPlane`, `AzureManagedMachinePool`
- Prioritize timeliness of a post-experimental definition so that users can confidently plan for any breaking changes that result from a graduated spec as soon as possible
- Prioritize "architectural affinity" with other Cluster API providers implementing Managed Kubernetes

### Non-Goals / Future Work
- Add to, or remove features from, "`AzureManagedCluster`"
- Standardize any opinions about how best to use AKS
- Promote CAPZ-specific opinions about Managed Kubernetes across the Cluster API provider ecosystem
- Define operational support

## Post-graduation Prerequisites

Concrete prereq workstreams are [tracked here](https://github.com/orgs/kubernetes-sigs/projects/26/views/1)

### 1. Land Managed Cluster in Cluster API Proposal (Status: DONE)

See:

- https://github.com/kubernetes-sigs/cluster-api/pull/6988

The above proposal defines a set of recommendations for Cluster API providers implementing Managed Kubernetes solutions. Because this proposal is an opt-in collection of architectural recommendations, it is not required that "`AzureManagedCluster`" strictly agrees with everything therein. However, by contributing to the successful landing of that proposal in Cluster API, we can best ensure that any CAPZ-specific opinions or learnings are reflected, for the benefit of the larger ecosystem, and for the maximum happiness of AKS customers in particular.

Ref:

- https://github.com/kubernetes-sigs/cluster-api/pull/6988

### 2. Consider Cluster API Proposal Recommendations (Status: Finalizing Consensus)

Once the proposal is accepted and merged into the Cluster API project as an endorsed set of provider recommendations, CAPZ can audit the existing "`AzureManagedCluster`" experimental implementation for areas of disagreement with said recommendations. For each discovery of disagreement, there should be a defensible reason to matriculate a divergent CAPZ Managed Kubernetes implementation into a post-experimental definition of "`AzureManagedCluster`", and ideally some form of sign-off from other provider maintainers. Where community consensus cannot be reached, CAPZ should strongly consider evolving the experimental "`AzureManagedCluster`" implementation to meet the Cluster API Managed Kubernetes recommendation as a pre-requisite to graduating from experimental.

Ref:

- https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/2739
- https://github.com/kubernetes-sigs/cluster-api/issues/7494


### 3. Pathway Towards Full AKS Feature Support (Status: DONE)

"`AzureManagedCluster`" should be able to accommodate the entire feature set of AKS. Rather than require the concrete list of features of AKS to be fully implemented as a prerequisite to graduation, we should instead audit the AKS feature matrix against the "`AzureManagedCluster`" architectural surface area and ensure that CAPZ is well prepared to continually integrate existing and new AKS features into "`AzureManagedCluster`" along a non-breaking path forward.

### 4. Affinity between CAPZ / Cluster API resource lifecycle enforcement and AKS lifecycle enforcement. (Status: Adding E2E Tests)

Not unrelated to the above audit of AKS features, we should also overlay the Cluster API lifecycle combinatorics (simply speaking: Create, Read, Update, Delete) against the set of AKS cluster primitives to ensure that sane interfaces exist in CAPZ in order to effectively enforce (for example, add an AKS node pool), or passively delegate authority (for example, defer to the built-in AKS autoscaler when enabled to enforce `MachinePool` replica count), if appropriate.

### 5. Azure API Request Optimizations (Status: DONE)

Prior to graduation from experimental, we should ensure that the sufficient set of configurable interfaces exist to allow "`AzureManagedCluster`" users to effectively tune CAPZ so that no unnecessary Azure API requests are introduced into their AKS environment. Essentially: CAPZ + AKS offers an opportunity for more AKS API requests from at least two dimensions:

- the implementation details of eventual consistency introduce net new "let me check if the current state matches my goal state" GET calls that aren't there if you aren't running an eventual consistency controller on top of your AKS environment
- if we do our jobs correctly CAPZ will optimize for multi-cluster stories, which may yield customer scenarios where more clusters are running on single subscriptions, which is at least _n_x (where n is the number of clusters) calls to the AKS API

We should ship sane, optimized defaults, but allow for flexible overrides to anticipate a wide variety of operational use cases.

Ref:

- https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/2688

### 6. Audit Webhooks (Status: DONE)

Ensure that the CAPZ webhooks that enforce input validation and other configuration requirements are complete for each feature, and match the authoritative AKS API requirements.

Ref:

- https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/2626
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/2717
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/2741
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/2795

### 7. E2E Tests (Status: In Progress)

For every supported AKS feature, and for every supported lifecycle mutation of said feature, CAPZ should have thorough, regular E2E test coverage.

### 8. Documentation

The current `AzureManagedCluster` is ad hoc, and in a cupboard hidden away in the docs structure. We should make this definitive, and in a more easily discoverable place.

Ref:

- https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/2776

## Open Questions (Status: DONE)

### Dependency upon (currently experimental) MachinePool spec (Status: DONE, won't take dependency)

At present we reuse the Cluster API `MachinePool` specification (as `AzureManagedMachinePool`) to implement AKS node pools running on Azure VMSS. A consideration here is that `MachinePool` is considered experimental, and behind a feature flag, by Cluster API. Do we want to add the graduation of `MachinePool` out of experimental as a prerequisite for graduating "`AzureManagedCluster`" out of experimental?

We are tracking a concrete Cluster API implementation of `MachinePoolMachine` [here](https://github.com/kubernetes-sigs/cluster-api/pull/6089).

The CAPZ community has declared an intention to graduate `AzureManagedCluster` independently from `MachinePool`. In practice, momentum suggests `AzureManagedCluster` will graduate prior to `MachinePool`. Based on conversations with folks who have worked on `MachinePool`, we don't expect any API changes; in the unlikely event that API changes to `MachinePool` emerge prior to its graduation, we the `AzureManagedCluster` solution has its own abstraction (`AzureManagedMachinePool` to protect its users from those changes.
