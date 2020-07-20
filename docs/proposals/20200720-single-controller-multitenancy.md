---
title: Single Controller Multitenancy
authors:
  - "@devigned"
reviewers:
creation-date: 2020-07-20
last-updated: 2020-07-20
status: implementable
see-also:
- https://github.com/kubernetes-sigs/cluster-api-provider-aws/pull/1674
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/586
replaces: []
superseded-by: []
---

# Single Controller Multitenancy

## Table of Contents

- [Single Controller Multitenancy](#single-controller-multitenancy)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
  - [Proposal](#proposal)
    - [User Stories](#user-stories)
      - [Story 1](#story-1---locked-down-with-service-principal-per-subscription)
      - [Story 2](#story-2---legacy-behavior-preserved)
      - [Story 3](#story-3---software-as-a-service-provider)
  - [Requirements](#requirements)
    - [Functional](#functional)
    - [Non-Functional](#non-functional)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
      - [Proposed Changes](#proposed-changes)
        - [AAD Pod Identity for Attaching Managed Identities](#aad-pod-identity-for-attaching-managed-identities)
        - [Cluster API Provider Azure v1alpha3 types](#cluster-api-provider-azure-v1alpha3-types)
    - [Controller Changes](#controller-changes)
    - [Clusterctl changes](#clusterctl-changes)
      - [Validating webhook changes](#validating-webhook-changes)
      - [Principal Type Credential Provider Behaviour](#principal-type-credential-provider-behaviour)
    - [Security Model](#security-model)
      - [Roles](#roles)
    - [RBAC](#rbac)
        - [Write Permissions](#write-permissions)
    - [Namespace Restrictions](#namespace-restrictions)
    - [CAPZ Controller Requirements](#capz-controller-requirements)
    - [Risks and Mitigations](#risks-and-mitigations)
      - [Caching and handling refresh of credentials](#caching-and-handling-refresh-of-credentials)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)
    - [Test Plan](#test-plan)
    - [Graduation Criteria](#graduation-criteria)
      - [Alpha](#alpha)
      - [Beta](#beta)
      - [Stable](#stable)
  - [Implementation History](#implementation-history)

## Glossary

* Principal Type - One of several ways to provide a form of identity that is ultimately resolved to 
an Azure Active Directory (AAD) Principal.
* Authorizer - An implementation of the [Azure SDK for Golang's Authorizer Interface](https://github.com/Azure/go-autorest/blob/7ac73d3561eaa034f458f97362b2743e8b3c048e/autorest/authorization.go#L42).
* CAPZ - An abbreviation of Cluster API Provider Azure.

## Summary

The CAPZ operator is able to manage Azure cloud infrastructure within the permission scope of the 
AAD principal it is initialized with, usually through environment vars. The CAPZ operator will be 
provided credentials via the deployment, either explicitly via environment variables or implicitly 
via the default SDK credential provider chain, including Azure instance metadata service via System 
Assigned or User Assigned Identities.

In addition, CAPZ uses the environmentally configured identity for the lifetime of the deployment. 
This also means that an AzureCluster could be broken if the instance of CAPZ that created it is 
misconfigured for another set of credentials.

This proposal outlines new capabilities for CAPZ to assumption a different AAD Principal, at 
runtime, on a per-cluster basis. The proposed changes would be fully backwards compatible and maintain
the existing behavior with no changes to user configuration required.

## Motivation

For large organizations, especially highly-regulated organizations, there is a need to be able to 
perform separate duties at various levels of infrastructure - permissions, networks and accounts. 
Azure Role Based Authorization Controls RBAC provides a model which allows admins to provide 
identities with the least privilege to perform activities. Within this model it is appropriate for 
tooling running within the 'management' account to manage infrastructure within the 'workload' 
accounts. With in the AAD model, the controller can assume the identity of the cluster to perform
cluster management activities. For CAPZ to be most useful within these organizations it will need 
to support multi-account models.

Some organizations may also delegate the management of clusters to another third-party. In that 
case, the boundary between organizations needs to be secured. In AAD, this can be accomplished by
providing a third party AAD principal RBAC access to the Azure resources required to manage cluster
infrastructure.

Because a single deployment of the CAPZ operator may reconcile many clusters in its lifetime, it is 
necessary to modify the CAPZ operator to scope its Azure `Authorizer` to within the reconciliation 
process.

It follows that an organization may wish to provision control planes and worker groups in separate 
accounts (including each worker group in a separate account). There is a desire to support this 
configuration also.

Unlike AWS, Azure doesn't provide mechanisms for assuming roles across account boundaries, but rather
allows RBAC rights to be enabled across account boundries. This is the largest break in this proposal
with regard to prior art of [the CAPA multitenancy proposal](https://github.com/kubernetes-sigs/cluster-api-provider-aws/blob/8c0c3db8af44e3c3a1db772145d96154a3d36280/docs/proposal/20200506-single-controller-multitenancy.md)

### Goals

1. To enable AzureCluster resources reconciliation across AAD account boundaries
2. To maintain backwards compatibility and cause no impact for users who don't intend to make use of 
   this capability

## Proposal

### User Stories

#### Story 1 - Locked down with Service Principal Per Subscription

Alex is an engineer in a large organization which has a strict Azure account architecture. This 
architecture dictates that Kubernetes clusters must be hosted in dedicated Subscriptions with AAD
identity with RBAC rights to provision the infrastructure only in the Subscription. The workload 
clusters must run with a System Assigned machine identity. The organization has adopted Cluster API 
in order to manage Kubernetes infrastructure, and expects 'management' clusters running the Cluster 
API controllers to manage 'workload' clusters in dedicated Azure Subscriptions with an AAD account 
which only have access to that Subscription.

The current configuration exists:
* Subscriptions for each cluster
* AAD Service Principals with Subscription Owner rights for each Subscription
* A management Kubernetes cluster running Cluster API Provider Azure controllers

Alex can provision a new workload cluster in the specified Subscription with the corresponding AAD 
Service Principal by creating new Cluster API resources in the management cluster. Each of the
workload cluster machines would run as the System Assigned identity described in the Cluster API
resources. The CAPZ controller in the management cluster uses the Service Principal credentials when
reconciling the AzureCluster so that it can create/use/destroy resources in the workload cluster.


#### Story 2 - Legacy Behavior Preserved

Dascha is an engineer in a smaller, less strict organization with a few Azure accounts intended to 
build all infrastructure. There is a single Azure Subscription named 'dev', and Dascha wants to 
provision a new cluster in this Subscription. An existing Kubernetes cluster is already running the 
Cluster API operators and managing resources in the dev Subscription. Dascha can provision a new 
cluster by creating Cluster API resources in the existing cluster, omitting the ProvisionerIdentity 
field in the AzureCluster spec. The CAPZ operator will use the Azure credentials provided in its 
deployment template.

#### Story 3 - Software as a Service Provider

ACME Industries is offering Kubernetes as a service to other organizations. ACME creates an AAD 
Identity for each organization and each organization grants that Identity access to provision 
infrastructure in one or multiple of their Azure Subscriptions. ACME Industries wants to minimise 
the memory footprint of managing many clusters, and wants to move to having a single instance of 
CAPZ to managed infrastructure across multiple organizations.

## Requirements

### Functional

<a name="FR1">FR1.</a> CAPZ MUST support assuming an identity specified by an `AzureCluster.ProvisioningIdentity`.

<a name="FR2">FR2.</a> CAPZ MUST support static credentials.

<a name="FR3">FR3.</a> CAPZ MUST prevent privilege escalation allowing users to create clusters in Azure accounts they should
  not be able to.

<a name="FR4">FR4.</a> CAPZ SHOULD support credential refreshing modified principal data.

<a name="FR5">FR5.</a> CAPZ SHOULD provide validation for principal data submitted by users.

<a name="FR6">FR6.</a> CAPZ MUST support clusterctl move scenarios.

### Non-Functional

<a name="NFR1">NFR1.</a> Each instance of CAPZ SHOULD be able to support 200 clusters using role assumption.

<a name="NFR2">NFR2.</a> CAPZ MUST call AAD APIs only when necessary to prevent rate limiting.

<a name="NFR3">NFR3.</a> Unit tests MUST exist for all credential provider code.

<a name="NFR4">NFR4.</a> e2e tests SHOULD exist for all credential provider code.

<a name="NFR5">NFR5.</a> Credential provider code COULD be audited by security engineers.

### Implementation Details/Notes/Constraints

The current implementation of CAPZ requests new instances of Azure services per cluster and 
sub-cluster resources. The input for these services is a ClusterScope, which provides the identity
Azure service will operate as.

```go
type ClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	AzureClients
	Cluster      *clusterv1.Cluster
	AzureCluster *infrav1.AzureCluster
}
```
The ClusterScope contains the information needed to make an authenticated request against an
Azure cloud. The `Authorizer` currently contains the AAD identity information loaded from the CAPZ
controller environment. The `Authorizer` is used to fetch and refresh tokes for all Azure clients.

```go
// AzureClients contains all the Azure clients used by the scopes.
type AzureClients struct {
	SubscriptionID             string
	ResourceManagerEndpoint    string
	ResourceManagerVMDNSSuffix string
	Authorizer                 autorest.Authorizer
}

```

The signatures for the functions which create these instances are as follows:

```go
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
  ...
  return &ClusterScope{
    ...
  }, nil
}
```

#### Proposed Changes
The proposed changes below only apply to infrastructure nodes which run the CAPZ controllers.
These changes have no impact on the identities specified on AzureMachines or other workload 
infrastructure.

##### AAD Pod Identity for Attaching Managed Identities
[AAD Pod Identity](https://github.com/Azure/aad-pod-identity) enables Kubernetes applications to 
access cloud resources securely using AAD Managed Identities. To enable CAPZ to authenticate as an 
Azure Managed Identity, either System Assigned or User Assigned Identities, CAPZ needs to have 
access to the Azure IMDS service running on the host. This can be accomplished indirectly by using 
AAD Pod Identity.

With AAD Pod Identity running within the management cluster CAPZ can create [AzureIdentityBindings](https://github.com/Azure/aad-pod-identity#5-deploy-azureidentitybinding)
and other related structures to enable CAPZ to bind to Azure Managed Identities.

To use Azure Managed Identities, the infrastructure nodes hosting the CAPZ controller must be hosted
in Azure. Outside of Azure, Service Principal identities are the only available identity type.

##### Cluster API Provider Azure v1alpha3 types

<strong><em>Changed Resources</strong></em>
* `AzureCluster`

<strong><em>New Resources</strong></em>

<em>Cluster scoped resources</em>

* `AzureServicePrincipal` represents an AAD Service Principal. Should support both 
   certificate and text secrets.
* `AzureSystemAssignedIdentity` represents an Azure System Assigned Identity provided 
   [via Pod Identity](#aad-pod-identity-for-attaching-managed-identities)
* `AzureUserAssignedIdentity` represents an Azure User Assigned Identity provided 
   [via Pod Identity](#aad-pod-identity-for-attaching-managed-identities)

<strong><em>Changes to AzureCluster</em></strong>

A new field is added to the `AzureClusterSpec` to reference a principal. We intend to use 
`corev1.LocalObjectReference` in order to ensure that the only objects that can be references are 
either in the same namespace or are scoped to the entire cluster.

```go
// AzurePrincipalKind defines allowed Azure principal types
// +kubebuilder:validation:Enum=AzureSystemAssigned;AzureUserAssigned;AzureServicePrincipal
type AzurePrincipalKind string

type AzurePrincipalRef struct {
  Kind AzurePrincipalKind `json:"kind"`
  Name string `json:"name"`
}

type  AzureClusterSpec  struct {
  ...
  // +optional
  PrincipalRef *AzurePrincipalRef `json:"principalRef,omitempty"`
```

Example:

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureCluster
metadata:
  name: "test"
  namespace: "test"
spec:
  region: "westus2"
  principalRef:
    kind: AzureServicePrincipal
    name: test-account
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureServicePrincipal
metadata:
  name: "test-account"
spec:
  secretRef:
    name: test-account-creds
    namespace: capz-system
  allowedNamespaces:
  - "test"
---
apiVersion: v1
kind: Secret
metadata:
  name: "test-account-creds"
  namespace: capz-system
stringData:
 tenantID: 6bec3eaa-cac6-11ea-87d0-0242ac130003
 clientID: 74e870e6-cac6-11ea-87d0-0242ac130003
 clientSecret: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

<em>AzureSystemAssignedIdentity</em>

`AzureSystemAssignedIdentity` allows CAPZ to use the SystemAssigned identity provided via Pod 
Identity.

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureCluster
metadata:
  name: "test"
  namespace: "test"
spec:
  region: "westus2"
  principalRef:
    kind: AzureSystemAssignedIdentity
    name: test-account
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureSystemAssignedIdentity
metadata:
  name: "test-account"
spec:
  secretRef:
    name: test-account-creds
    namespace: capz-system
  allowedNamespaces:
  - "test"
---
apiVersion: v1
kind: Secret
metadata:
  name: "test-account-creds"
  namespace: capz-system
stringData:
 tenantID: 6bec3eaa-cac6-11ea-87d0-0242ac130003
```

Example: AzureUserAssignedIdentity

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureCluster
metadata:
  name: "test"
  namespace: "test"
spec:
  region: "westus2"
  principalRef:
    kind: AzureUserAssignedIdentity
    name: test-account
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureUserAssignedIdentity
metadata:
  name: "test-account"
spec:
  secretRef:
    name: test-account-creds
    namespace: capz-system
  allowedNamespaces:
  - "test"
---
apiVersion: v1
kind: Secret
metadata:
  name: "test-account-creds"
  namespace: capz-system
stringData:
 tenantID: 6bec3eaa-cac6-11ea-87d0-0242ac130003
 clientID: 74e870e6-cac6-11ea-87d0-0242ac130003
```

### Controller Changes

* If principalRef is specified, the CRD is fetched and unmarshalled into a Azure `Authorizer` for 
  the principal type.
* The controller will compare the hash of the credential provider against the same secret’s provider
  in a cache ([NFR2](#NFR2)).
* The controller will take the newer of the two and instantiate Azure clients with the selected 
  `Authorizer`.
* The controller will set the Identity resource as one of the OwnerReferences of the AzureCluster.

### Clusterctl changes

Today, `clusterctl move` operates by tracking objectreferences within the same namespace, since we 
are now proposing to use cluster-scoped resources, we will need to add requisite support to 
clusterctl's object graph to track ownerReferences pointing at cluster-scoped resources, and ensure 
they are moved. We will naively not delete cluster-scoped resources during a move, as they maybe 
referenced across namespaces.

#### Validating webhook changes

A validating webhook should be used to validate AAD credentials if possible. For example, it would
be helpful to validate that a Service Principal is able to get an AAD token for Azure Resource
Manager rather than an HTTP 401 error code.

#### Principal Type Credential Provider Behaviour

Implementations for all principal types will implement the AutoREST `Authorizer` interface as well 
as an additional function signature to support caching.

```go
// Authorizer is the interface that provides a PrepareDecorator used to supply request
// authorization. Most often, the Authorizer decorator runs last so it has access to the full
// state of the formed HTTP request.
type Authorizer interface {
    WithAuthorization() PrepareDecorator
}

type AzureIdentityProvider interface {
    Authorizer

    // Hash returns a unique hash of the data forming the credentials
    // for this principal
    Hash() (string, error)
}
```

Azure client sessions are structs implementing the `Authorizer` interface. The Azure SDK for Golang 
will verify the token in the `Authorizer` has not expired. If the token has expired, the 
`Authorizer` will attempt to fetch a new token from AAD.

The controller will maintain a cache of all the `Authorizers` used by the clusters. In practice,
this should be a single `Authorizer` for the cluster. If we were to create clusters which span
Azure AAD Tenants, then there may be a need for multiple `Authorizers` per cluster.

CAPZ should maintain a watch on all Secrets used by owned clusters. Upon receiving an update event, 
the controller will update lookup the key in the cache and replace the relevant `Authorizer`. This 
may be implemented as its own interface. This would require changes to RBAC, and maintaining a 
watch on secrets of a specific type will require further investigation as to feasibility.

### Security Model

The intended RBAC model mirrors that for Service APIs:

#### Roles

For the purposes of this security model, 3 common roles have been identified:

* **Infrastructure provider**: The infrastructure provider (infra) is responsible for the overall 
  environment that the cluster(s) are operating in or the PaaS provider in a company.

* **Management cluster operator**: The cluster operator (ops) is responsible for
  administration of the Cluster API management cluster. They manage policies, network access,
  application permissions.

* **Workload cluster operator**: The workload cluster operator (dev) is responsible for
  management of the cluster relevant to their particular applications .

There are two primary components to the Service APIs security model: RBAC and namespace restrictions.

### RBAC
RBAC (role-based access control) is the standard used for Kubernetes authorization. This allows 
users to configure who can perform actions on resources in specific scopes. RBAC can be used to 
enable each of the roles defined above. In most cases, it will be desirable to have all resources be
readable by most roles, so instead we'll focus on write access for this model.

##### Write Permissions
|                              | AzureServicePrincipal, etc | Azure RBAC API | Cluster |
| ---------------------------- | -------------------- | ----------- | ------- |
| Infrastructure Provider      | Yes                  | Yes         | Yes     |
| Management Cluster Operators | Yes                  | Yes         | Yes     |
| Workload Cluster Operator    | No                   | No          | Yes     |

### Namespace Restrictions
The extra configuration options are not possible to control with RBAC. Instead,
they will be controlled with configuration fields on Identities:

* **allowedNamespaces**: This field is a selector of namespaces that can use the 
  `AzureServicePrincipal`, `AzureUserAssignedIdentity`, and  `AzureSystemAssignedIdentity` from. 
  This is a standard Kubernetes LabelSelector, a label query over a set of resources. The result of 
  matchLabels and matchExpressions are ANDed. CAPZ will not support AzureClusters in namespaces 
  outside this selector. An empty selector (default) indicates that AzureCluster can use this Azure 
  Identity from any namespace. This field is intentionally not a pointer because the nil behavior 
  (no namespaces) is undesirable here.


### CAPZ Controller Requirements
The CAPZ controller will need to:

* Populate condition fields on AzureClusters and indicate if it is
  compatible with the Azure Identity. For example, if SystemAssignedIdentity is specified and is not 
  available, a condition should be set indicating failure.
* Not implement invalid configuration. For example, if the AzureCluster references an Azure Identity
  in an invalid namespace for it, it should indicate it through a condition or ignore.
* Respond to changes in an Azure Identity spec change.

### Risks and Mitigation

#### Caching and handling refresh of credentials

For handling many accounts, the number of calls to AAD must be minimised. To minimize the number of 
calls to AAD, the cache should store `Authorizers` by a key consisting of two parts, the credential
type and the credential name, `key={cred-type|cred-name}, value=Authorizer`.

## Upgrade Strategy

The data changes are additive and optional, so existing AzureCluster specifications will continue 
to reconcile as before. These changes will only come into play when specifying Azure Identity in 
the new field in AzureClusterSpec. Upgrades to versions with this new field will be broken.

## Additional Details

### Test Plan

* Unit tests to validate that the cluster controller can reconcile an AzureCluster when PrincipalRef
  field is nil, or specified for each principal type.
* Unit tests to verify execution of pre-flight checks when PrincipalRef is provided.
* Propose performing an initial Azure API call and fail pre-flight if this fails.
* e2e test for each principal type.
* clusterctl e2e test with a move of a self-hosted cluster using a principalRef.

### Graduation Criteria

#### Alpha

* Support using an Azure Identity using the PrincipalRef
* Ensure `clusterctl move` works with the mechanism.

#### Beta

* Admission controller validation for secrets of type.
* Full e2e coverage.

#### Stable

* Two releases since beta.

## Implementation History

- [ ] 2020/07/20: Initial proposal

<!-- Links -->