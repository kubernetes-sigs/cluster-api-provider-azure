---
title: Automate AKS Features Available in CAPZ
authors:
  - "@nojnhuh"
reviewers:
  - "@CecileRobertMichon"
  - "@matthchr"
  - "@dtzar"
  - "@mtougeron"
creation-date: 2023-11-22
last-updated: 2023-11-28
status: provisional
see-also:
  - "docs/proposals/20230123-azure-service-operator.md"
---

# Automate AKS Features Available in CAPZ

## Table of Contents

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals/Future Work](#non-goalsfuture-work)
- [Proposal](#proposal)
  - [User Stories](#user-stories)
    - [Story 1](#story-1)
    - [Story 2](#story-2)
    - [Story 3](#story-3)
  - [API Design Options](#api-design-options)
    - [Option 1: CAPZ resource references an existing ASO resource](#option-1-capz-resource-references-an-existing-aso-resource)
    - [Option 2: CAPZ resource references a non-functional ASO "template" resource](#option-2-capz-resource-references-a-non-functional-aso-template-resource)
    - [Option 3: CAPZ resource defines an entire unstructured ASO resource inline](#option-3-capz-resource-defines-an-entire-unstructured-aso-resource-inline)
    - [Option 4: CAPZ resource defines an entire typed ASO resource inline](#option-4-capz-resource-defines-an-entire-typed-aso-resource-inline)
    - [Option 5: No change: CAPZ resource evolution proceeds the way it currently does](#option-5-no-change-capz-resource-evolution-proceeds-the-way-it-currently-does)
    - [Option 6: Generate CAPZ code equivalent to what's added manually today](#option-6-generate-capz-code-equivalent-to-whats-added-manually-today)
    - [Option 7: CAPZ resource defines patches to ASO resource](#option-7-capz-resource-defines-patches-to-aso-resource)
    - [Option 8: Users bring-their-own ASO ManagedCluster resource](#option-8-users-bring-their-own-aso-managedcluster-resource)
    - [Decision](#decision)
  - [Security Model](#security-model)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Upgrade Strategy](#upgrade-strategy)
- [Additional Details](#additional-details)
  - [Test Plan](#test-plan)
  - [Graduation Criteria](#graduation-criteria)
  - [Version Skew Strategy](#version-skew-strategy)
- [Implementation History](#implementation-history)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Summary

CAPZ's AzureManagedControlPlane and AzureManagedMachinePool resources expose AKS's managed cluster and agent
pool resources for Cluster API. Currently, new features in AKS require manual changes to teach CAPZ about
those features for users to be able to take advantage of them natively in Cluster API. As a result, there are
several AKS features available that cannot be used from Cluster API. This proposal describes how CAPZ will
automatically make all AKS features available on an ongoing basis with minimal maintenance.

## Motivation

Historically, CAPZ has exposed an opinionated subset of AKS features that are tested and known to work within
the Cluster API ecosystem. Since then, it has become increasingly clear that new AKS features are generally
suitable to implement in CAPZ and users are interested in having all AKS features available to them from
Cluster API.

When gaps exist in the set of features available in AKS and what CAPZ offers, users of other infrastructure
management solutions may not be able to adopt CAPZ. If all AKS features could be used from CAPZ, this would
not be an issue.

The AKS feature set changes rapidly alongside CAPZ's users' desire to utilize those features. Because making
new AKS features available in CAPZ requires a considerable amount of mechanical, manual effort to implement,
review, and test, requests for new AKS features account for a large portion of the cost to maintain CAPZ.

Another long-standing feature request in CAPZ is the ability to adopt existing AKS clusters into management by
Cluster API (https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/1173). Making the entire AKS
API surface area available from CAPZ is required to enable this so existing clusters' full representations can
be reflected in Cluster API.

### Goals

- Narrow the gap between the sets of features offered by AKS and CAPZ.
- Reduce the maintenance cost of making new AKS features available from CAPZ.
- Preserve the behavior of existing CAPZ AKS definitions while allowing users to utilize the new API pattern
  iteratively to use new features not currently implemented in CAPZ.

### Non-Goals/Future Work

- Automate the features available from other Azure services besides AKS.
- Automatically modify AKS cluster definitions to transparently enable new AKS features.

## Proposal

### User Stories

#### Story 1

As a managed cluster user, I want to be able to use all available AKS features natively from CAPZ so that I
can more consistently manage my CAPZ AKS clusters that use advanced or niche features more quickly than having
to wait for each of them to be implemented in CAPZ.

#### Story 2

As an AKS user looking to adopt Cluster API over an existing infrastructure management solution, I want to be
able to use all AKS features natively from CAPZ so that I can adopt Cluster API with the confidence that all
the AKS features I currently utilize are still supported.

#### Story 3

As a CAPZ developer, I want to be able to make new AKS features available from CAPZ more easily in order to
meet user demand.

### API Design Options

There are a few different ways the entire AKS API surface area could be exposed from the CAPZ API. The
following options all rely on ASO's ManagedCluster and ManagedClustersAgentPool resources to define the full
AKS API. The examples below use AzureManagedControlPlane and ManagedCluster to help illustrate, but all of the
same ideas should also apply to AzureManagedMachinePool and ManagedClustersAgentPool.

#### Option 1: CAPZ resource references an existing ASO resource

Here, the AzureManagedControlPlane spec would include only a field that references an ASO ManagedCluster
resource:

```go
type AzureManagedControlPlaneSpec struct {
	// ManagedClusterRef is a reference to the ASO ManagedCluster backing this AzureManagedControlPlane.
	ManagedClusterRef corev1.ObjectReference `json:"managedClusterRef"`
}
```

CAPZ will _not_ create this ManagedCluster and instead rely on it being created by any other means. CAPZ's
`aks` flavor template will be updated to include a ManagedCluster to fulfill this requirement. CAPZ will also
not modify the ManagedCluster except to fulfill CAPI contracts, such as managing replica count. Users modify
other parameters which are not managed by CAPI on the ManagedCluster directly. Users should be fairly familiar
with this pattern since it is already used extensively throughout CAPI, e.g. by modifying a virtual machine
through its InfraMachine resource for parameters not defined on the Machine resource.

This approach has two key benefits. First, it can leverage ASO's conversion webhooks to allow CAPZ to interact
with the ManagedCluster through one version of the API, and users to use a different API version, including
newer or preview (https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2625) API versions.
Second, since ASO can [adopt existing Azure
resources](https://azure.github.io/azure-service-operator/guide/frequently-asked-questions/#how-can-i-import-existing-azure-resources-into-aso),
adopting existing AKS clusters into CAPZ
(https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/1173) additionally would only require
the extra steps to create the AzureManagedControlPlane referring to the adopted ManagedCluster.

One other consideration is that this approach trades the requirement that CAPZ has the necessary RBAC
permissions to create ManagedClusters for users having that same capability. CAPZ would still require
permissions to read, update, and delete ManagedClusters.

Drawbacks with this approach include:
- A requirement to define ASO resources in templates in addition to the existing suite of CAPI/CAPZ resources
  (one ASO ManagedCluster and one ASO ManagedClustersAgentPool for each MahcinePool).
- Inconsistency between how CAPZ manages some resources through references to pre-created ASO objects (the
  managed cluster and agent pools) and some by creating the ASO objects on its own (resource group, virtual
  network, subnets).
- An increased risk of users conflicting with CAPZ if users and CAPZ are both expected to modify mutually
  exclusive sets of fields on the same resources.
- A new inability for CAPZ to automatically make adjustments to AKS-specific parameters that we may determine
  are worthwhile to apply on behalf of users.

Other main roadblocks with this method relate to ClusterClass. If CAPZ's AzureManagedControlPlane controller is
not responsible for creating the ASO ManagedCluster resource, then users would need to manage those
separately, defeating much of the purpose of ClusterClass. Additionally, since each AzureManagedControlPlane
will be referring to a distinct ManagedCluster, the new `ManagedClusterRef` field should not be defined in an
AzureManagedControlPlaneTemplate. Upstream CAPI components could possibly be changed to better enable this use
case by allowing templating of arbitrary Kubernetes resources alongside other cluster resources.

#### Option 2: CAPZ resource references a non-functional ASO "template" resource

This method is similar to [Option 1]. To better enable ClusterClass without changes to CAPI, instead of
defining a full reference to the ManagedCluster resource, a reference to a "template" resource is defined
instead:

```go
type AzureManagedControlPlaneClassSpec struct {
	// ManagedClusterTemplateRef is a reference to the ASO ManagedCluster to be used as a template from which
	// new ManagedClusters will be created.
	ManagedClusterTemplateref corev1.ObjectReference `json:"managedClusterTemplateRef"`
}
```

This template resource will be a ManagedCluster used as a base from which the AzureManagedControlPlane
controller will create a new ManagedCluster. The template ManagedCluster will have ASO's [`skip` reconcile
policy](https://azure.github.io/azure-service-operator/guide/annotations/#serviceoperatorazurecomreconcile-policy)
applied so it does not result in any AKS resource being created in Azure. The ManagedClusters created based on
the template will be reconciled normally to create AKS resources in Azure. The non-template ManagedClusters
will be linked to the AzureManagedControlPlane through the standard `cluster.x-k8s.io/cluster-name` label.

To modify parameters on the AKS cluster, either the template or non-template ManagedCluster may be updated.
CAPZ will propagate changes made to a template to instances of that template. Parameters defined on the
template take precedence over the same parameters on the instances.

The main difference with [Option 1] that enables ClusterClass is that the same ManagedCluster template
resource can be referenced by multiple AzureManagedControlPlanes, so this new `ManagedClusterTemplateRef`
field can be defined on the AzureManagedControlPlaneClassSpec so a set of AKS parameters defined once in the
template can be applied to all Clusters built from that ClusterClass.

This method makes all ManagedCluster fields available to define in a template which could lead to
misconfiguration if certain parameters that must be unique to a cluster are erroneously shared through a
template. Since those fields cannot be automatically identified and may evolve between AKS API versions, CAPZ
will not attempt to categorize ASO ManagedCluster fields that way like it does between the fields present and
omitted from the `AzureClusterClassSpec` type, for example. CAPZ could document a best-effort list of known
fields which could or should not be defined in template types and will otherwise rely on AKS to provide
reasonable error messages for misconfigurations.

Like [Option 1], this method keeps particular versions of CAPZ decoupled from particular API versions of ASO
resources (including allowing preview versions) and opens the door for streamlined adoption of existing AKS
clusters.

#### Option 3: CAPZ resource defines an entire unstructured ASO resource inline

This method is functionally equivalent to [Option 2] except that the template resource is defined inline
within the AzureManagedControlPlane:

```go
type AzureManagedControlPlaneClassSpec struct {
	// ManagedClusterTemplate is the ASO ManagedCluster to be used as a template from which new
	// ManagedClusters will be created.
	ManagedClusterTemplate map[string]interface{} `json:"managedClusterTemplate"`
}
```

One variant of this method could be to using `string` instead of `map[string]interface{}` for the template
type, though that would make defining patches unwieldy (like for ClusterClass).

Compared to [Option 2], this method loses schema and webhook validation that would be performed by ASO when
creating a separate ManagedCluster to serve as a template. That validation would still be performed when CAPZ
creates the ManagedCluster resource, but that would be some time after the AzureManagedControlPlane is created
and error messages may not be quite as visible.

#### Option 4: CAPZ resource defines an entire typed ASO resource inline

This method is functionally equivalent to [Option 3] except that the template field's type is the exact same
as an ASO ManagedCluster:

```go
type AzureManagedControlPlaneClassSpec struct {
	// ManagedClusterSpec defines the spec of the ASO ManagedCluster managed by this AzureManagedControlPlane.
	ManagedClusterSpec v1api20230201.ManagedCluster_Spec `json:"managedClusterSpec"`
}
```

This method allows CAPZ to leverage schema validation defined by ASO's CRDs upon AzureManagedControlPlane
creation, but would still lose any further webhook validation done by ASO unless CAPZ can invoke that itself.

It also has the drawback that one version of CAPZ is tied directly to a single AKS API version. The spec could
potentially contain separate fields for each API version and enforce in webhooks that only one API version is
being used at a time. Alternatively, users may set fields only present in a newer API version directly on the
ManagedCluster after creation (if allowed by AKS) because CAPZ will not override user-provided fields for
which it does not have its own opinion on ASO resources.

Updating the embedded ASO API version in the CAPZ resources may not be possible to do safely without also
bumping the CAPZ API version, however. Because ASO implements conversion webhook logic between several API
versions for each AKS resource type, simply bumping the ASO API version in the CAPZ type without bumping the
CAPZ API version would not allow that same conversion to be applied. This could lead to issues where a new
version of CAPZ suddenly starts constructing invalid ASO resources and user intervention is required to
perform the conversion manually.

While it couples CAPZ to one ASO API version, this approach allows CAPZ to move a more calculated pace with
regards to AKS API versions the way it's done today. This also narrows CAPZ's scope of responsibility which
reduces CAPZ's exposure to potential incompatibilities with certain ASO API versions.

Regarding ClusterClass, this option functions the same as [Option 2] or [Option 3], where all ASO fields can
be defined in a template. This option opens up an additional safeguard though, where webhooks could flag
fields which should not be defined in a template. Similar webhook checks would be less practical in the other
options where CAPZ would need to be aware of the set of disallowed fields for each ASO API version that a user
could use.

Similarly, CAPZ's webhooks are also better able to validate and default the ASO configuration and ensure
fields like ManagedCluster's `spec.owner` that should not be modified by users are set correctly.

#### Option 5: No change: CAPZ resource evolution proceeds the way it currently does

This method describes not making any changes to how the CAPZ API is generally structured for AKS resources.
CAPZ API types will continue to be curated manually without inheriting anything from the ASO API.

Benefits of continuing on our current path include:
- Familiarity with the existing pattern by users and contributors
- Zero up-front cost to implement or transition to a new pattern
- No requirement for users to have ASO knowledge
- Greater freedom to change API implementations which we've recently leveraged to transition between the older
  and newer Azure SDKs and to ASO.

#### Option 6: Generate CAPZ code equivalent to what's added manually today

For this option, a new code generation pipeline would be created to automatically scaffold the code that is
currently manually written to expose additional AKS API fields to CAPZ.

Once implemented, this method would drastically reduce the amount of developer effort to get started adding
new AKS features. There would continue to be some amount of ongoing cost though to identify and handle nuances
for each feature though, which may include issues outside of CAPZ's control like AKS API quirks.

The main drawback of this approach from a developer perspective is the up-front effort to implement and
ongoing cost to maintain the code generation itself. The existing AKS feature set exposed by CAPZ provides a
decent foundation that can help identify regressions by testing the pipeline against features already
implemented and tested. ASO also already implements a full code generation pipeline to transform Azure API
specs into Kubernetes resource definitions, so some of that could possibly be reused by CAPZ.

#### Option 7: CAPZ resource defines patches to ASO resource

This method describes adding a new `spec.asoManagedClusterPatches` field to the existing
AzureManagedControlPlane:

```go
type AzureManagedControlPlaneClassSpec struct {
	...

	// ASOManagedClusterPatches defines patches to be applied to the generated ASO ManagedCluster resource.
	ASOManagedClusterPatches []string `json:"asoManagedClusterPatches,omitempty"`
}
```

After CAPZ calculates the ASO ManagedCluster for a given AzureManagedControlPlane during a reconciliation, it
will apply these patches in order to the ManagedCluster before ultimately submitting that to ASO. This allows
users to specify any AKS feature declaratively within the existing CAPZ spec. The exact format of the patches
is TBD, but likely one or all of the formats described here:
https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/

The main drawback of this approach is the fragility of the patches themselves, which can break underneath
users with a new version of ASO or CAPZ. There is also increased risk that a user could modify the ASO
resource in a way that breaks CAPZ's ability to reconcile it, though CAPZ could perhaps add some extra
validation that particularly sensitive fields do not get modified by a set of patches. Compared to other
options, the syntax for specifying a patch is also more cumbersome than if its equivalent had its own CAPZ API
field. Given the intent of setting patches is to enable niche use-cases for advanced users, these drawbacks
may be acceptable.

#### Option 8: Users bring-their-own ASO ManagedCluster resource

CAPZ can already incorporate existing ASO resources into Cluster API Cluster configurations. BYO ASO resources
are never modified or deleted by CAPZ directly but are still read and play in to the status of CAPZ resources.

The flow for this method is roughly:
1. User creates an ASO ManagedCluster before or at the same time as the other Cluster API and CAPZ resources.
1. Any updates to the AKS cluster are done by the user through the ASO resource.
1. The user may control whether or not the ASO ManagedCluster gets deleted with the rest of the Cluster API
   Cluster by choosing whether or not to pre-create the ASO ResourceGroup.

Benefits of this approach:
- Minimal or zero additional cost for CAPZ to implement outside of a new test. Users can try this today with
  the latest version of CAPZ.
- Freedom for users to craft the ASO ManagedCluster and other resources exactly how they like, even a
  different API version (including preview).
- Protection from CAPZ being responsible for managing AKS configuration that it does not understand.

Drawbacks:
- Requires users to mimic CAPZ and create the ManagedCluster with `spec.agentPoolProfiles` as required by AKS,
  then remove them once created so as not to conflict with the corresponding AzureManagedMachinePool
  definitions, which can't be done in a single install operation.
- Requires users to rework templates to add ASO resources, but only if they want AKS features not exposed by
  CAPZ.
- Doesn't support ClusterClass.
- CAPZ may no longer be able to fulfill CAPI's contract by modifying fields like Kubernetes version or replica
  counts as defined by a MachinePool. Internal changes to CAPZ may be able to overcome this.

#### Decision

We are moving forward with [Option 7] for now as it requires the least amount of change to the CAPZ API and
does not introduce any significant usability issues while still allowing users to declaratively enable
features outside of those explicitly defined by CAPZ.

### Security Model

One possible concern is that with little oversight as to what fields defined in the AKS API ultimately become
exposed in the CAPZ API, bad actors may become able to modify certain sensitive configuration by way of CAPZ's
AzureManagedControlPlane which was not possible before. However, CAPZ has historically not forbidden workload
cluster administrators from modifying any such sensitive configuration in the past.

Overall, none of the approaches outlined above change what data is ultimately represented in the API, only the
higher-level shape of the API. That means there is no further transport or handling of secrets or other
sensitive information beyond what CAPZ already does.

### Risks and Mitigations

Increasing CAPZ's reliance on ASO and exposing ASO to users at the API level further increases the risk
[previously discussed](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/ce3c130266b23a8b67aa5ef9a21f257ff9e6d63e/docs/proposals/20230123-azure-service-operator.md?plain=1#L169)
that since ASO has not yet been proven to be as much of a staple as the other projects that manage
infrastructure on Azure, ASO's lifespan may be more limited than others. If ASO were to sunset while CAPZ
still relies on it, CAPZ would have to rework its APIs. This risk is mitigated by the fact that no
announcements have yet been made regarding ASO's end-of-life and the project continues to be very active. And
since ASO's resource representations are mostly straightforward reflections of the Azure API spec, shifting
away from ASO to another Azure API abstraction should be mostly mechanical.

## Upgrade Strategy

Each of the first four [options above](#api-design-options) would existing in a new v2alpha1 CAPZ API version for
AzureManagedControlPlane and AzureManagedMachinePool. The existing v1beta1 types will continue to be served so
users do not need to take any action for their existing clusters to continue to function as they have been.

An alternative available with [Option 3] and [Option 4] is to introduce a new backwards-compatible CAPZ API
version for AzureManagedControlPlane and AzureManagedMachinePool, such as v1beta2. Then, conversion webhooks
can be implemented to convert between v1beta1 and v1beta2. This option isn't possible with [Option 1] or
[Option 2] because a conversion webhook would not be able to create the new standalone ASO resources.

## Additional Details

### Test Plan

Existing end-to-end tests will verify that CAPZ's current behavior does not regress. New tests will verify
that the new API fields proposed here behave as expected.

### Graduation Criteria

Any new CAPZ API versions or added API fields would be available and enabled by default as soon as they are
functional and stable enough for users to try. Requirements for a v2alpha1 to v2beta1 graduation and
deprecation of the v1 API are TBD.

### Version Skew Strategy

With options 1-3 above, users are free to use any ASO API versions for AKS resources. CAPZ may internally
operate against a different API version and ASO's webhooks will transparently perform any necessary conversion
between versions.

## Implementation History

- [ ] 06/15/2023: Issue opened: https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3629
- [ ] 11/22/2023: Iteration begins on this proposal document
- [ ] 11/28/2023: First complete draft of this document is made available for review

[Option 1]: #option-1-capz-resource-references-an-existing-aso-resource
[Option 2]: #option-2-capz-resource-references-a-non-functional-aso-template-resource
[Option 3]: #option-3-capz-resource-defines-an-entire-unstructured-aso-resource-inline
[Option 4]: #option-4-capz-resource-defines-an-entire-typed-aso-resource-inline
[Option 5]: #option-5-no-change-capz-resource-evolution-proceeds-the-way-it-currently-does
[Option 7]: #option-7-capz-resource-defines-patches-to-aso-resource
