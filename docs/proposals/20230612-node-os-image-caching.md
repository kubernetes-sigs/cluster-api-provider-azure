---
title: Integrate Node OS Image caching with CAPZ
authors:
  - "@bennycortese"
reviewers:
  - "@nojnhuh"
  - "@CecileRobertMichon"
  - "@jackfrancis"
  - "@willie-yao"
creation-date: 2023-06-12
last-updated: 2023-06-12

status: provisional
see-also:
  - "https://en.wikipedia.org/wiki/Prototype_pattern"
---

# Title 
Integrate Node OS Image caching with CAPZ

## Table of Contents

- [Integrate Node OS Image caching with CAPZ](#title)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal](#proposal)
    - [User Stories](#user-stories)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
    - [Security Model](#security-model)
    - [Risks and Mitigations](#risks-and-mitigations)
  - [Alternatives](#alternatives)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)
    - [Test Plan [optional]](#test-plan-optional)
    - [Graduation Criteria [optional]](#graduation-criteria-optional)
    - [Version Skew Strategy [optional]](#version-skew-strategy-optional)
  - [Implementation History](#implementation-history)

## Glossary

Node Prototype Pattern - Where we take a known good/working OS snapshot and make a “prototype” upon which all future node OS’ images are based.

Warm nodes - The concept of keeping up extraneous and unused nodes to prevent having to wait for patches or security updates, thus allowing you to have those nodes ready when more users try to use your service without the wait times.

Azure Compute Gallery - An Azure Resource for managing and sharing artifacts such as images

Snapshot - A static copy of the image at the point in time that it is taken

Prototype Node - Whichever node is chosen to clone the OS image of and cache it for future nodes to use

## Summary

This proposal introduces modifications to the `AzureMachinePool` controller for an optional feature which will cache the Nodes’ OS image on a regular interval and update the model to use that image for future scale outs.

## Motivation

We want users to have faster horizontal scaling and require fewer warm nodes and overprovisioning to avoid this problem, especially since the new nodes will have the container images of the applications it will run pre-cached so pods will run quicker when scheduled. This feature will also help users have better security compliance as new nodes will already be compliant instead of needing to patch.

### Goals

1. Make the feature configurable
1. Avoid causing and breaking changes to previous users
1. Successfully be able to snapshot and switch to the new image created from that snapshot
1. Show that faster horizontal scaling speeds have been achieved with this feature
1. Prevent security breaking issues on node bootup from security updates being required immediately

We will know we’ve succeeded when we can benchmark speed increases and successful image changes.

### Non-Goals/Future Work

1. Extend the functionality to Windows nodes
1. Optimization for efficiency and scalability
1. A more complicated method of selecting a good candidate node such as incorporating manual forced prototype creation, which would perform better but take research to find the optimal method
1. Optimization of the default time interval into a specific, best general default which would have to be researched
1. Automatic bad snapshot rollbacks since it will be hard to generically determine when a snapshot is bad currently
1. Annotating the timestamp on each node of when it was updated with automatic security updates
1. Allow the feature to be enabled for any type of image (marketplace or custom with ID)
1. `AzureMachineTemplate` support, holding out for now because of immutability of the `AzureMachineTemplates`

## Proposal

### User Stories

#### As an operator, I want to be able to have faster horizontal scaling with pods running quicker when scheduled

As a cluster operator I want to bring up new nodes that already have the container images of the applications it will run pre-cached so that pods start running quicker when scheduled, resulting in faster horizontal scaling.

#### As an operator of clusters managed by CAPZ, I want to reduce monetary and environmental costs

As a cluster operator I want to have lower monetary and environmental costs. I want my Nodes' OS images to be continually cached, and thus I will be able to avoid longer pull times and avoid having to create warm nodes and overprovision. This will be better for the environment and cheaper for me as I will not have to waste excess resources on warm nodes.

#### As an operator, I want to be able to be more inline with security compliance as I spin up nodes

As an operator I would like to be able to have my node’s OS image cache for the ability to avoid security alerts, cache security updates and patches so that I’ll be more security compliant and up to date when I have to spin up a new node. Otherwise, flags will be raised that a node is out of date on boot up before it finishes patching and a temporary security risk will exist.

### Implementation Details/Notes/Constraints

The plan is to modify the existing Controllers with the Node Prototype Pattern as desired. These controller additions can be added to `AzureMachinePool` Controller.

An operator will be able to decide to turn the feature on or off with an environment variable on clusterctl initialization, and then on a cluster by cluster basis can alter the `AzureMachinePools` to specify the use of the feature. They can also update the `AzureMachinePool` to customize how long they want the caching interval to be (see the yaml files below in this section for the caching interval).

Example of the environment variable being turned on:

```
export AZURE_NODE_OS_CACHING=true
```

The controller will maintain a timestamp in each `AzureMachinePool`, and when the current time is the chosen interval ahead or more, the controller will perform the caching. Since the current controller manager requeues all objects every ten minutes by default the objects will be requeued shortly after its due time to be recached. This is because typically we expect to cache every 24 hours and it is very unexpected that this won't be frequent enough considering normal patch rates. 

Example of how the timestamp will be maintained in the `AzureMachinePool`:

```yaml
status:
  lastImagePrototype: "2023-06-12T23:14:55Z"
```

When the process is started it should go through the nodes of the cluster, choose a healthy node, shut it down, take a snapshot of it, restart it, create a Azure Compute Gallery image, delete the snapshot, and then configure the `AzureMachinePool` spec to use that Azure Compute Gallery image. After, it will store the current time as its timestamp. As a note, for the first implementation of this feature we will require the user to also use a Azure Compute Gallery image.

Diagram of the Node OS Caching Process:

![Figure 1](./images/node-os-image-cache.png)

As for why the healthy node has to be shut down while creating a snapshot of it, if it isn’t shut down first then pods can be scheduled as the snapshot is taken which will cause some dangerous states in terms of how it exists after being utilized by the `AzureMachinePools`.

#### Healthy Node Selection

There is already state data present on each `AzureMachinePoolMachine` which we can use and is listed in the examples below. An ideal node would be one which is running and healthy. Whichever node has been running and healthy for the longest amount of time should be chosen as it’s the most overall stable. This means that for `AzureMachinePoolMachines` we will take the node with the earliest creation time from `metadata.creationTimestamp` and has `latestModelApplied : true` present if it is an `AzureMachinePoolMachine` since this means that any user changes to the OS image have been rolled out already on this node. As the prototype is always from a successfully healthy and working node the image is always known to be working before being chosen for replication. In terms of knowing if the node os image has had updates that are not user made (so like automatic kernel patches), for the first development of this feature we won't know but all nodes in a cluster are configured to regularly automatically apply security patches to system packages.

Example `AzureMachinePoolMachine` yaml with the important fields shown (want `status.ready == true`, `latestModelApplied: true` and each `status: "True"`):

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: node-os-image-caching-machine-pool-machine
  namespace: default
  creationTimestamp: "2023-06-20T17:41:54Z"
status:
  conditions:
  - lastTransitionTime: "2023-06-20T17:43:39Z"
    status: "True"
    type: Ready
  - lastTransitionTime: "2023-06-20T17:43:39Z"
    status: "True"
    type: BootstrapSucceeded
  - lastTransitionTime: "2023-06-20T17:43:39Z"
    status: "True"
    type: NodeHealthy
  latestModelApplied: true
  ready: true
```

#### When to take a Snapshot

A day is given as a general example which should be good for typical use but the specification of how often will be customizable as we know that certain operators have different strategies and use cases for how they’re running their services on our clusters.

#### Data model changes

`AzureMachinePool` will be changed and the proposed changes are purely additive and nonbreaking. No removals should be required to the data model. For `AzureMachinePool` we will add a new optional field under `spec.template.image` called `nodePrototyping` which will be enabled if present and it have a required field under it called interval which will map to an interval of 1 day or 24 hours by default.

Example `AzureMachinePool` yaml:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachinePool
metadata:
  name: node-os-image-caching-machine-pool
  namespace: default
spec:
  image:
      nodePrototyping:
        interval: 24h
```

### Security Model

This proposal requires CAPZ to have write permissions for `AzureMachinePools` in order to properly update the nodes’ OS image on the spec. Go has a library called `time` and `time.ParseDuration` will be used to parse the time interval provided by an operator instead of using regular expressions. Denial of service attacks will be protected against by having an update system which doesn’t need to be atomic. If part of the caching is complete there is no risk in the update not finishing since the spec update will happen at once. No sensitive data is being stored in a secret.

### Risks and Mitigations

Example risks:
1. A bad snapshot is taken, and we will mitigate this risk by having trying to prevent it before it happens by checking if things are ready and draining everything before taking the snapshot. Rolling back and determining if a bad snapshot is bad is out of scope for this proposal currently and will be for the operator to watch, so here we will simply try to prevent it as best as we can.
1. A bad security patch or update might have been applied to a user’s node that they don’t want to be applied to future nodes. To mitigate this risk, we will make it easy for users to turn this feature off, and if they fix it on their original node the snapshot will be taken of that node instead.
1. Deleting previous snapshots might not allow for new image instantiations from those snapshots since Azure Compute Gallery Image Definition Version instances may depend directly on those snapshots still being there. Instead deletion can be done after making sure the new image is successful for new deployments.

Link to page with Azure Compute Gallery limits: https://learn.microsoft.com/en-us/azure/virtual-machines/azure-compute-gallery
<!-- markdown-link-check-disable-next-line -->
Link to page with snapshot pricing (Azure Compute Galleries themselves are free): https://azure.microsoft.com/en-us/pricing/details/managed-disks/

The UX will mostly be impactful towards operators and members of the CAPZ community will test these changes and give feedback on them. Security will also likely follow in terms of how it gets reviewed, but no major security problems should be possible from this change. For folks who work outside the SIG or subproject, they should hopefully have faster horizontal scaling without needing to directly do anything outside of setting an environment variable on clusterctl initialization and updating their `AzureMachinePools`.

## Alternatives

No currently known alternatives exist which are public and have been implemented for CAPZ. A savvy operator may also have created a similar system for their nodes but we hope to offload that responsibility to the CAPZ team in this process, and currently no other open source implementations of this for CAPZ are known to exist. 

In terms of how we choose a node as the prototype node, lots of different metrics or heuristics can be used like manually creating a temporary prototype for testing, having the fastest ready time, or anything else which is seen as typically good but a more generalist approach is outlined here since more specific methods may not be as helpful for certain operators.

For architectural details of where else the code could exist, the controller section makes the most sense since this proposal will be constantly modifying the state of our objects, but theoretically it could be largely put into hack with shell scripts and then a controller could simply be ordered to trigger that shell script, but this is less maintainable in the long run and not as preferred.
We can put it in the `AzureMachinePool` controller or make it another controller, both are viable options and putting it in the `AzureMachinePool` controller will be faster to implement versus another controller will allow for a cleaner codebase overall. 

## Upgrade Strategy

Turning off or on the feature for a particular operator is done with them setting an environment variable to enable or disable it with clusterctl initialization. They will also be able to alter their `AzureMachinePool` instances to add the feature or remove it and that is all that is required to keep previous behavior or make use of the enhancement. No backwards compatibility will be broken, all this feature request will do is change previous controllers and add optional fields to `AzureMachinePool` which can be utilized or not as desired.

## Additional Details

### Test Plan [optional]

There will be e2e tests, at least one of which will be as follows:
Have an example node and an example patch, apply the patch to the preexisting node, and then trigger the controller to pretend the interval of time has passed, and then it should attempt to create another node and compare the OS image of the new node and the original node, finding that they are both the same image.

It should be tested primarily in isolation as other components shouldn’t affect what it tries to do, but it may need to be checked with other components to see what happens if certain race conditions or updates at the same time of `AzureMachinePool` are occurring (in which case a lower priority should likely be assigned to this controller for finishing its task after as ideally those changes are in effect before isolating the node).

### Graduation Criteria [optional]

Experimental:

The feature is initially constructed and toggleable with an environment variable. It will be implemented as described in this document and have e2e tests implemented and integrated in the project.

Stable: 

The feature has been used for a while and is widely acceptable as well as reliable and will now be enabled by default.

At this point a more sophisticated method of choosing a healthy node will be used, preferably by annotating the `AzureMachinePoolMachine` instances after every patch. This will allow for more optimization and fewer unnecessary uses of this operation as if no updates in the interval are needed we would be able to now properly know.

### Version Skew Strategy [optional]

The feature itself should not depend significantly on the version of CAPI and will be backwards compatible with old versions of CAPZ since it will be a toggleable feature. If there is a drift in CAPI and CAPZ versions, the functionality should stay the same without breaking anything.

## Implementation History

- [ ] 06/06/2023: Starting Compiling a Google Doc following the CAEP template
- [ ] 06/07/2023: First round of feedback from community
- [ ] 06/12/2023: Open proposal PR [ https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/3624 ]
- [ ] 06/15/2023: Proposed idea and got some feedback on the PR from the community meeting for CAPZ: [ https://docs.google.com/document/d/1ushaVqAKYnZ2VN_aa3GyKlS4kEd6bSug13xaXOakAQI/edit#heading=h.pxsq37pzkbdq ]
- [ ] 07/17/2023: Opened issue for features relating to this PR [ https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3737 ]

<!-- Links -->
[community meeting]: https://docs.google.com/document/d/1ushaVqAKYnZ2VN_aa3GyKlS4kEd6bSug13xaXOakAQI/edit#heading=h.pxsq37pzkbdq

#### Credits

A big thank you to Jack Francis, Michael Sinz, and Amr Hanafi for their work on the Kamino project that used a similar Prototype Pattern with AKSEngine which inspired this feature request: https://github.com/jackfrancis/kamino

