---
title: CAPZ action items to Managed Kubernetes in CAPI Proposal #6988
authors:
    - @jackfrancis
reviewers:
    - TBD
creation-date: 2022-10-19
last-updated: 2022-12-09
status: discussion
see-also:
    - https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/2602
---

# Summary of Accepted Managed Kubernetes in CAPI Proposal

The following is a lightly edited copy/paste of the preferred option for Managed Cluster implementions in Cluster API as defined in [this proposal doc](https://github.com/kubernetes-sigs/cluster-api/pull/6988).

Two new resource kinds will be introduced:

 - **<Provider>ManagedControlPlane**: this presents a provider's actual managed control plane. Its spec would only contain properties that are specific to the provisioning & management of a provider's cluster (excluding worker nodes). It would not contain any properties related to a provider's general operating infrastructure, like the networking or project.
 - **<Provider>ManagedCluster**: this presents the properties needed to provision and manage a provider's general operating infrastructure for the cluster (i.e project, networking, IAM). It would contain similar properties to **<Provider>Cluster** and its reconciliation would be very similar.

 For example:

 ```go
 type MyProviderManagedControlPlaneSpec struct {
     // ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
     // +optional
     ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

     // AddonsConfig defines the addons to enable with the GKE cluster.
     // +optional
     AddonsConfig *AddonsConfig `json:"addonsConfig,omitempty"`

     // Logging contains the logging configuration for the GKE cluster.
     // +optional
     Logging *ControlPlaneLoggingSpec `json:"logging,omitempty"`

     // EnableKubernetesAlpha will indicate the kubernetes alpha features are enabled
     // +optional
     EnableKubernetesAlpha bool

     ...
 }
 ```

 ```go
 type MyProviderManagedClusterSpec struct {
     // Project is the name of the project to deploy the cluster to.
     Project string `json:"project"`

     // The GCP Region the cluster lives in.
     Region string `json:"region"`

     // ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
     // +optional
     ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

     // NetworkSpec encapsulates all things related to the GCP network.
     // +optional
     Network NetworkSpec `json:"network"`

     // FailureDomains is an optional field which is used to assign selected availability zones to a cluster
     // FailureDomains if empty, defaults to all the zones in the selected region and if specified would override
     // the default zones.
     // +optional
     FailureDomains []string `json:"failureDomains,omitempty"`

     // AdditionalLabels is an optional set of tags to add to GCP resources managed by the GCP provider, in addition to the
     // ones added by default.
     // +optional
     AdditionalLabels Labels `json:"additionalLabels,omitempty"`

     ...
 }
```

The option above is the best way to proceed for **new implementations** of Managed Kubernetes in a provider.

 The reasons for this recommendation are as follows:

 - It adheres closely to the original separation of concerns between the infra and control plane providers
 - The infra cluster provisions and manages the general infrastructure required for the cluster but not the control plane.
 - By having a separate infra cluster API definition, it allows differences in the API between managed and unmanaged clusters.

 Providers like CAPZ and CAPA have already implemented Managed Kubernetes support and there should be no requirement on them to move to Option 3. Both Options 2 and 4 are solutions that would work with ClusterClass and so could be used if required.

 Option 1 is the only option that will not work with ClusterClass and would require a change to CAPI. Therefore this option is not recommended.

 *** This means that CAPA will have to make changes to move away from Option 1 if it wants to support ClusterClass.

# CAPZ Ramifications

CAPZ's current Managed Kubernetes solution fulfills part of the above recommendation: it implements distinct `AzureManagedCluster` and `AzureManagedControlPlane` CRDs, in compliance with the Cluster API `ClusterClass` specification.

It differs from the proposal in the following ways:

- `AzureManagedCluster` does not exclusively define "cluster"-specific properties (the examples above are networking and IAM)
- `AzureManagedControlPlane` does not exclusively define "control plane"-specific properties; in fact it defines the entire set of "cluster"-wide configuration supported by CAPZ on top of AKS

In order to better adhere to the recommended implementation model CAPZ would move the bulk of its configuration surface area from `AzureManagedControlPlane` to `AzureManagedCluster`.

## AKS-CAPI API Affinity Observations

As stated above, Cluster API defines a "cluster" as distinct from a "control plane". AKS, however, does not declare such a clean boundary in its own API. Using the `az aks` CLI as an example, we can see that the only current definite abstraction boundary at present (as distinct from the cluster itself is "nodepool". E.g.:

```sh
$ az aks --help

Group
    az aks : Manage Azure Kubernetes Services.

Subgroups:
    command                        : See detail usage in 'az aks command invoke', 'az aks command
                                     result'.
    nodepool                       : Commands to manage node pools in Kubernetes kubernetes cluster.
...
```

The `update` command which exposes the maintenance API for operations against existing cluster further suggests a single abstract primitive (the AKS cluster):

```
$ az aks update --help

Command
    az aks update : Update a managed Kubernetes cluster. When called with no optional arguments this
    attempts to move the cluster to its goal state without changing the current cluster
    configuration. This can be used to move out of a non succeeded state.

Arguments
    --name -n                          [Required] : Name of the managed cluster.
    --resource-group -g                [Required] : Name of resource group. You can configure the
                                                    default group using `az configure --defaults
                                                    group=<name>`.
...
```

There is no mention of a control plane as distinct from a cluster.

The official AKS documentation further supports the idea that the "cluster" and the "control plane" are one and the same concept from the AKS point of view:

> When you create an AKS cluster, a control plane is automatically created and configured. This control plane is provided at no cost as a managed Azure resource abstracted from the user. You only pay for the nodes attached to the AKS cluster. The control plane and its resources reside only on the region where you created the cluster.

Reference: https://learn.microsoft.com/en-us/azure/aks/concepts-clusters-workloads#control-plane

A thorough read of the above document reinforces that the only definite architectural boundary defined by the AKS service is between the cluster and one or more node pools.

In summary, the "AKS control plane" and the "AKS cluster" are largely the same thing (with an exception that is shared with Cluster API: it is also commonplace to refer to the "cluster" as the set of control plane components plus all nodes/node pools).

## Considerations

We can formulate a set of pros/cons of the above recommended approach for Cluster API Managed Kubernetes as it relates to CAPZ:

Pros:

1. We agree with the proposal's assertion that to the extent we can exclusively place cluster-specific configurations in a "AzureManagedCluster" specification and control plane-specific configurations in a "AzureManagedControlPlane" specification, we will produce a Managed Kubernetes CRD surface area that is more Cluster API-idiomatic, which should predict better continuing compatibility with Cluster API platform evolution, and provide a more familiar interface to the existing Cluster API user community when onboarding to CAPZ Managed Kubernetes.
2. As a human, the Cluster API requirement to have distinct control plane and cluster configurations is made easier of those configuration interfaces have some semantic significance. As a maintainer responsible for comprehending how to best maintain my Managed Kubernetes infra, a "control plane" configuration interface designed exclusively for well-known control plane primitives like apiserver, controller-manager, and etcd, and a "cluster" configuration interface designed for control plane + node impact: Azure Networking, CNI, and addons, for example, is preferred.

Cons:

1. An existing Managed Kubernetes implementation already exists with a non-trivial customer footprint. The `AzureManagedCluster` and `AzureManagedControlPlane` CRDs are already published as `infrastructure.cluster.x-k8s.io/v1beta1`. Moving around existing configuration into different CRD resources, or creating an entirely new set of configurations with a CRD distribution that matches the above recommendations, would result in a breaking change for existing customers. In order to allow existing clusters to migrate forward to the new CAPZ implementation of Managed Kubernetes, additional migration tooling would have to be built and rigorously tested.
2. To the extent that we succeed at the objective of distributing AKS configuration across distinct "AzureManagedCluster" and "AzureManagedControlPlane" resources, we further disassociate ourselves from core AKS architectural definitions. We write above that adhering to the recommendation better predicts "continuing compatibility with Cluster API platform evolution"; but we risk ongoing compatibility with AKS platform evolution if we do so.

## Synthesizing the Above

Reviewing the delta between the existing CAPZ Managed Kubernetes implementation of AKS and the recently approved set of Cluster API recommendations, it is clear that a significant engineering investment will be required to refactor `AzureManagedCluster` and `AzureManagedControlPlane` to better agree with the stated proposal. We must weigh the following against each other:

1. The pain to existing customers of forcing breaking changes.
2. The benefit to new customers of a more Cluster API-idiomatic API for CAPZ Managed Kubernetes

Because customer adoption of CAPZ Managed Kubernetes is robust, the negative output from Point 1 grows rapidly. If we do want to do this, we should do it ASAP, as the ROI is arguably less and less as time progresses.

## Feedback From the Community

Based on initial feedback during CAPZ office hours and Kubecon North America 2022 hall roaming, the most conspicuous theme around this topic is "Provider Consistency". In other words, if we consider the "canonical" CAPZ AzureManagedCluster customer to be a Cluster API, multi-cloud, Managed Kubernetes customer first of all, then the biggest gap at present is the lack of consistency across CAPA Managed Kubernetes, CAPZ Managed Kubernetes, etc.

A defensible conclusion from [this Cluster API proposal doc](https://github.com/kubernetes-sigs/cluster-api/pull/6988) is that consistency is not achievable given the current state of Cluster API. As described above, the proposal does describe two acceptable approaches, but that is by definition not consistent. Of all the explanations for the current state of Cluster API Managed Kubernetes inconsistency, the most fundamental one is the absence of a Cluster API Managed Cluster "primitive". So long as there isn't a first class Managed Kubernetes resource in the Cluster API itself, providers will struggle to build Managed Kubernetes solutions that are consistent across the Cluster API ecosystem.

## Conclusions

Achieving durable, consistent Managed Kubernetes interfaces for the entire Cluster API provider community is our highest priority. Doing that work is necessarily a large investment in engineering time + resources, and will involve some short-term inconvenience for existing customers. If we wish to embark on that work with the greatest chance for long-term success we will want to do that work in Cluster API itself, and not across the provider community only.

To that end we have formed a dedicated "Feature Group" in Cluster API to work out how we will solve for Managed Kubernetes consistency across Cluster API providers.

Reference:

- https://github.com/kubernetes-sigs/cluster-api/pull/7546
