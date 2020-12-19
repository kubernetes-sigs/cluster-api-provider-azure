---
title: Single Controller Multitenancy
authors:
  - "@devigned"
reviewers:
  - "@nader-ziada"
  - "@CecileRobertMichon"
creation-date: 2020-07-20
last-updated: 2020-07-20
status: implementable
see-also:
- https://github.com/kubernetes-sigs/cluster-api-provider-aws/pull/1674
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/586
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/977
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
      - [Story 2](#story-2---locked-down-by-namespace-and-subscription)
      - [Story 3](#story-3---using-an-azure-user-assigned-identity)
      - [Story 4](#story-4---legacy-behavior-preserved)
      - [Story 5](#story-5---software-as-a-service-provider)
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
via the default SDK credential provider chain, including Azure instance metadata service via User Assigned Identities.

In addition, CAPZ uses the environmentally configured identity for the lifetime of the deployment. 
This also means that an AzureCluster could be broken if the instance of CAPZ that created it is 
misconfigured for another set of credentials.

This proposal outlines new capabilities for CAPZ to assume a different AAD Principal, at 
runtime, on a per-cluster basis. The proposed changes would be fully backwards compatible and maintain
the existing behavior with no changes to user configuration required.

## Motivation

For large organizations, especially highly-regulated organizations, there is a need to be able to 
perform separate duties at various levels of infrastructure - permissions, networks and accounts. 
Azure Role Based Authorization (RBAC) Controls provides a model which allows admins to provide 
identities with the least privilege to perform activities. Within this model it is appropriate for 
tooling running within the 'management' account to manage infrastructure within the 'workload' 
accounts. Within the AAD model, the controller can assume the identity of the cluster to perform
cluster management activities. For CAPZ to be most useful within these organizations it will need 
to support multi-account models.

Some organizations may also delegate the management of clusters to another third-party. In that 
case, the boundary between organizations needs to be secured. In AAD, this can be accomplished by
providing a third party AAD principal RBAC access to the Azure resources required to manage cluster
infrastructure.

Because a single deployment of the CAPZ operator may reconcile many clusters in its lifetime, it is 
necessary to modify the CAPZ operator to scope its Azure `Authorizer` to within the reconciliation 
process.

Unlike AWS, Azure doesn't provide mechanisms for assuming roles across account boundaries, but rather
allows RBAC rights to be enabled across account boundaries. This is the largest break in this proposal
with regard to prior art of [the CAPA multitenancy proposal](https://github.com/kubernetes-sigs/cluster-api-provider-aws/blob/8c0c3db8af44e3c3a1db772145d96154a3d36280/docs/proposal/20200506-single-controller-multitenancy.md)

### Goals

1. To enable AzureCluster resources reconciliation using a cluster specified AAD Identity
2. To maintain backwards compatibility and cause no impact for users who don't intend to make use of 
   this capability

## Proposal

### User Stories

#### Story 1 - Locked down with Service Principal Per Subscription

Alex is an engineer in a large organization which has a strict Azure account architecture. This 
architecture dictates that Kubernetes clusters must be hosted in dedicated Subscriptions with AAD
identity having RBAC rights to provision the infrastructure only in the Subscription. The workload 
clusters must run with a System Assigned machine identity. The organization has adopted Cluster API 
in order to manage Kubernetes infrastructure, and expects 'management' clusters running the Cluster 
API controllers to manage 'workload' clusters in dedicated Azure Subscriptions with an AAD account 
which only has access to that Subscription.

The current configuration exists:
* Subscription for each cluster
* AAD Service Principals with Subscription Owner rights for each Subscription
* A management Kubernetes cluster running Cluster API Provider Azure controllers

Alex can provision a new workload cluster in the specified Subscription with the corresponding AAD 
Service Principal by creating new Cluster API resources in the management cluster. Each of the
workload cluster machines would run as the System Assigned identity described in the Cluster API
resources. The CAPZ controller in the management cluster uses the Service Principal credentials when
reconciling the AzureCluster so that it can create/use/destroy resources in the workload cluster.

#### Story 2 - Locked down by Namespace and Subscription

Alex is an engineer in a large organization which has a strict Azure account architecture. This
architecture dictates that Kubernetes clusters must be hosted in dedicated Subscriptions with AAD
identity having RBAC rights to provision the infrastructure only in the Subscription. The workload
clusters must run with a System Assigned machine identity.

Erin is a security engineer in the same company as Alex. Erin is responsible for provisioning
identities. Erin will create a Service Principal for use by Alex to provision the infrastructure in
Alex's cluster. The identity Erin creates should only be able to be used in a predetermined
Kubernetes namespace where Alex will define the workload cluster. The identity should be able
to be used by CAPZ to provision workload clusters in other namespaces.

The organization has adopted Cluster API
in order to manage Kubernetes infrastructure, and expects 'management' clusters running the Cluster
API controllers to manage 'workload' clusters in dedicated Azure Subscriptions with an AAD account
which only has access to that Subscription.

The current configuration exists:
* Subscription for each cluster
* AAD Service Principals with Subscription Owner rights for each Subscription
* A management Kubernetes cluster running Cluster API Provider Azure controllers

Alex can provision a new workload cluster in the specified Subscription with the corresponding AAD
Service Principal by creating new Cluster API resources in the management cluster in the 
predetermined namespace. Each of the workload cluster machines would run as the System Assigned 
identity described in the Cluster API resources. The CAPZ controller in the management cluster 
uses the Service Principal credentials when reconciling the AzureCluster so that it can 
create/use/destroy resources in the workload cluster.

Erin can provision an identity in a namespace of limited access and define the allowed namespaces,
which will include the predetermined namespace for the workload cluster.

#### Story 3 - Using an Azure User Assigned Identity

Erin is an engineer working in a large organization. Erin does not want to be responsible for
ensuring Service Principal secrets are rotated on a regular basis. Erin would like to use an
Azure User Assigned Identity to provision workload cluster infrastructure. The User Assigned
Identity will have the RBAC rights needed to provision the infrastructure in Erin's subscription.

The current configuration exists:
* Subscription for the workload cluster
* A User Assigned Identity with RBAC with Subscription Owner rights for the Subscription
* A management Kubernetes cluster running Cluster API Provider Azure controllers

Erin can provision a new workload cluster in the specified Subscription with the Azure User
Assigned Identity by creating new Cluster API resources in the management cluster. The CAPZ 
controller in the management cluster uses the User Assigned Identity credentials when reconciling 
the AzureCluster so that it can create/use/destroy resources in the workload cluster.

#### Story 4 - Legacy Behavior Preserved

Dascha is an engineer in a smaller, less strict organization with a few Azure accounts intended to 
build all infrastructure. There is a single Azure Subscription named 'dev', and Dascha wants to 
provision a new cluster in this Subscription. An existing Kubernetes cluster is already running the 
Cluster API operators and managing resources in the dev Subscription. Dascha can provision a new 
cluster by creating Cluster API resources in the existing cluster, omitting the ProvisionerIdentity 
field in the AzureCluster spec. The CAPZ operator will use the Azure credentials provided in its 
deployment template.

#### Story 5 - Software as a Service Provider

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

##### AAD Pod Identity for Attaching Cluster Provisioning Identities
[AAD Pod Identity](https://github.com/Azure/aad-pod-identity) enables Kubernetes applications to 
access cloud resources securely using AAD Identities, Service Principal and User Assigned Identities. 
To enable CAPZ to authenticate as a User Assigned Identity, CAPZ needs to have access to the Azure 
IMDS service running on the host. This can be accomplished indirectly by using AAD Pod Identity.

With AAD Pod Identity running within the management cluster CAPZ can create [AzureIdentityBindings](https://github.com/Azure/aad-pod-identity#5-deploy-azureidentitybinding)
and other related structures to enable CAPZ to bind to Azure Identities.

To use Azure Managed Identities, the infrastructure nodes hosting the CAPZ controller must be hosted
in Azure. Outside of Azure, Service Principal identities are the only available identity type.

##### Cluster API Provider Azure v1alpha3 types

<strong><em>Changed Resources</strong></em>
* `AzureCluster`

<strong><em>New Resources</strong></em>

<em>Cluster scoped resources</em>

* ` AzureClusterIdentity` represents the information needed to create an AzureIdentity and 
  AzureIdentityBinding [via Pod Identity](#aad-pod-identity-for-attaching-managed-identities). The type should
  also contain a string list representing the namespace which it is allowed to be used.

<strong><em>Changes to AzureCluster</em></strong>

A new field is added to the `AzureClusterSpec` to reference an `AzureClusterIdentity`. We intend to use 
`corev1.LocalObjectReference` in order to ensure that the only objects that can be references are 
either in the same namespace or are scoped to the entire cluster.

```go
// AzureClusterIdentity is the Schema for the azureclustersidentities API
type AzureClusterIdentity struct {
  metav1.TypeMeta   `json:",inline"`
  metav1.ObjectMeta `json:"metadata,omitempty"`
  
  Spec   AzureClusterIdentitySpec   `json:"spec,omitempty"`
  Status AzureClusterIdentityStatus `json:"status,omitempty"`
}


type AzureClusterIdentitySpec struct {
  // UserAssignedMSI or Service Principal
  Type IdentityType `json:"type"`
  // User assigned MSI resource id.
  // +optional
  ResourceID string `json:"resourceID,omitempty"`
  // Both User Assigned MSI and SP can use this field.
  ClientID string `json:"clientID"`
  // ClientSecret is a secret reference which should contain either a Service Principal password or certificate secret.
  // +optional
  ClientSecret corev1.SecretReference `json:"clientSecret,omitempty"`
  // Service principal primary tenant id.
  TenantID string `json:"tenantID"`
  // AllowedNamespaces is an array of namespaces that AzureClusters can
  // use this Identity from.
  //
  // An empty list (default) indicates that AzureClusters can use this
  // Identity from any namespace. This field is intentionally not a
  // pointer because the nil behavior (no namespaces) is undesirable here.
  // +optional
  AllowedNamespaces []string `json:"allowedNamespaces"`
}


type  AzureClusterSpec  struct {
  ...
  // +optional
  IdentityRef *corev1.ObjectReference `json:"identityRef,omitempty"`
```

Example:

<em>Service Principal</em>
```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureCluster
metadata:
  name: cluster1
  namespace: default
spec:
  identityRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: AzureClusterIdentity
    name: sp-identity
    namespace: default
  location: westus2
  networkSpec:
    vnet:
      name: cluster1-vnet
  resourceGroup: cluster1
  subscriptionID: 8000873c-41a6-11eb-8907-4db4e45a2e79
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureClusterIdentity
metadata:
  name: sp-identity
  namespace: default
spec:
  clientID: 74e870e6-cac6-11ea-87d0-0242ac130003
  clientSecret:
    name: secretName
    namespace: secretNamespace
  tenantID: 6bec3eaa-cac6-11ea-87d0-0242ac130003
  type: ServicePrincipal
```

<em>User Assigned Identity</em>
```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureCluster
metadata:
  name: cluster1
  namespace: default
spec:
  identityRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: AzureClusterIdentity
    name: sp-identity
    namespace: default
  location: westus2
  networkSpec:
    vnet:
      name: cluster1-vnet
  resourceGroup: cluster1
  subscriptionID: 8000873c-41a6-11eb-8907-4db4e45a2e79
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: AzureClusterIdentity
metadata:
  name: sp-identity
  namespace: default
spec:
  clientID: 74e870e6-cac6-11ea-87d0-0242ac130003
  tenantID: 6bec3eaa-cac6-11ea-87d0-0242ac130003
  type: UserAssignedMSI
```

### Controller Changes

* If IdentityRef is specified, the CRD is fetched and used to create an `azure.Authorizer`
* The controller will compare the hash of the credential provider against the same secret’s provider
  in a cache ([NFR2](#NFR2)).
* The controller will take the newer of the two and instantiate Azure clients with the selected 
  `Authorizer`.
* The controller will set the `AzureClusterIdentity` resource as one of the OwnerReferences of the 
  `AzureCluster` if the resource is in the same namespace as the `AzureCluster`.
* The controller should reconcile `AzureClusterIdentity` and create corresponding `AzureIdentity`
  and `AzureIdentityBinding` resources within the AAD Pod Identity namespace to enable the management
  cluster to use the identities specified.
* The controller should have a label which will be used by the `AzureIdentityBinding` to inform AAD
  Pod Identity.

### Clusterctl changes

Today, `clusterctl move` operates by tracking objectreferences within the same namespace, since we 
are now proposing to use cluster-scoped resources, we will need to add requisite support to 
clusterctl's object graph to track ownerReferences pointing at cluster-scoped resources, and ensure 
they are moved. We will naively not delete cluster-scoped resources during a move, as they maybe 
referenced across namespaces.

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
they will be controlled with configuration fields on `AzureClusterIdentity`:

* **allowedNamespaces**: This field is a list of namespaces that can use the 
  `AzureClusterIdentity`from. CAPZ will not support AzureClusters in namespaces outside this selector. 
  An empty selector (default) indicates that AzureCluster can use this Azure Identity from any 
  namespace. This field is intentionally not a pointer because the nil behavior (no namespaces) is 
  undesirable here.


### CAPZ Controller Requirements
The CAPZ controller will need to:

* Populate condition fields on AzureClusters and indicate if it is
  compatible with the Azure Identity. For example, if UserAssignedMSI is specified and is not 
  available, a condition should be set indicating failure.
* Not implement invalid configuration. For example, if the AzureCluster references an `AzureClusterIdentity`
  in an invalid namespace for it, it should indicate it through a condition or ignore.
* Respond to changes in an `AzureClusterIdentity` spec.

### Risks and Mitigation

#### Caching and handling refresh of credentials

For handling many accounts, the number of calls to AAD must be minimised. To minimise the number of 
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
* Propose performing an initial Azure API call and fail pre-flight if this fails.
* e2e test for each principal type.
* clusterctl e2e test with a move of a self-hosted cluster using a principalRef.

### Graduation Criteria

#### Alpha

* Support using an Azure Service Principal using the IdentityRef
* Ensure `clusterctl move` works with the mechanism.

#### Beta

* Support using Azure User Assigned Identities
* Documentation describing all identities used in Management and Workload Clusters and their roles
* Full e2e coverage for both Service Principals and User Assigned Identities
* Identity caching to minimize authentication requests

#### Stable

* Two releases since beta.
* Describe cluster identities as the preferred way to provision infrastructure as opposed to env vars

## Implementation History

- 2020/07/20: Initial proposal
- 2020/12/17: Initial PR merge https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/977
- 2020/12/18: Proposal update to reflect the adaptations from #977

<!-- Links -->