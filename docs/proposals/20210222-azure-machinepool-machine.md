---
title: Azure Machine Pool Machines
authors:
    - @devigned
reviewers:
    - @CecileRobertMichon
    - @nader-ziada
creation-date: 2021-02-22
last-updated: 2021-02-22
status: implementable
see-also:
    - https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/819
    - https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/1067
---


# Azure Machine Pool Machines

## Table of Contents
- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals / Future Work](#non-goals--future-work)
        - [Notes About VMSS Terminate Notifications](#notes-about-vmss-terminate-notifications)
- [Proposal](#proposal)
    - [User Stories](#user-stories)
        - [Story 1 - Upgrading the Kubernetes Version of a MachinePool](#story-1---upgrading-the-kubernetes-version-of-a-machinepool)
        - [Story 2 - Reducing the Number of Replicas in a MachinePool](#story-2---reducing-the-number-of-replicas-in-a-machinepool)
        - [Story 3 - Deleting an individual Azure Machine Pool Machine](#story-3---deleting-an-individual-azure-machine-pool-machine)
    - [Requirements](#requirements)
        - [Functional](#functional)
        - [Non-Functional](#non-functional)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
        - [Existing APIs for Clarity](#existing-apis-for-clarity)
        - [Proposed API Changes](#proposed-api-changes)
        - [Proposed Controllers Changes](#proposed-controller-changes)
        - [Proposed Changes of Responsibily](#proposed-changes-of-responsibility)
- [Available Options](#available-options-for-cluster-api-provider-azure)
    - [Add Annotations to AzureMachinePool for Instance Delete Selection](#option-1-add-annotations-to-azuremachinepool-for-instance-delete-selection)
        - [Pros](#option-1-pros)
        - [Cons](#option-1-cons)
    - [Separate AzureMachinePool and AzureMachinePoolMachines](#option-2-separate-azuremachinepool-and-azuremachinepoolmachines)
        - [Pros](#option-2-pros)
        - [Cons](#option-2-cons)
- [Conclusions](#conclusions)
- [Additional Details](#additional-details)
    - [Test Plan](#test-plan)
- [Implementation History](#implementation-history)

## Summary

Azure MachinePool currently embeds the state of each of the instances in the MachinePool within the status of the Azure
MachinePool. MachinePool instances should be their own resources to enable individual lifecycles.

## Motivation

By giving each AzureMachinePoolMachine an individual lifecycle, a user would be able to inform CAPZ of the specific
instance to delete and then have the AzureMachinePoolMachine controller cordon and drain the node prior to deleting
the underlying infrastructure.

### Goals
- Be able to delete specific AzureMachinePool instances
- Rolling updates with max unavailable and max surge
  - MaxUnavailable is the max number of machines that are allowed to be unavailable at any time
  - MaxSurge is the number of machines to surge, add to the current replica count, during an upgrade of the VMSS model
- Safely update by cordoning and draining nodes prior to deleting the underlying infrastructure
- Be able to take advantage of [Azure's Virtual Machine Scale Set Update Instance API](https://learn.microsoft.com/rest/api/compute/virtualmachinescalesets/updateinstances)
  to in-place update a VMSS instance rather than delete and recreate the infrastructure, which would result in a much
  quicker upgrade.

### Non-Goals / Future Work
- Create a CAPI Machine owner for each AzureMachinePoolMachine
- Implementing different roll out and scale down strategies
- Adopting individual Machine instances to be managed by the MachinePool
- Create or use an on instance agent to cordon and drain in response to Azure Virtual Machine Scale Sets provide [terminate notifications](https://learn.microsoft.com/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-terminate-notification)

#### Notes About VMSS Terminate Notifications
Azure Virtual Machine Scale Sets provide [terminate notifications](https://learn.microsoft.com/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-terminate-notification).
These terminate notifications would be helpful to inform Kubernetes when a node is going to be deleted. Unfortunately,
terminate notifications do not provide notifications when an instance is Updated, in this case "Updated" means the
instance is reimaged to match the updated VMSS model by using the [Update Instance API](https://learn.microsoft.com/rest/api/compute/virtualmachinescalesets/updateinstances).
If a VMSS instance were to be reimaged, rather than deleted and recreated the instance will not receive a notification.
Due to the design of terminate notifications the CAPZ controller needs to alert Kubernetes when an instance is being
Updated. Without some way to inform Kubernetes of the specific instance that is to be updated, the underlying
infrastructure may be removed before workloads can be safely migrated from the machine / node. By managing the lifecycle
from CAPZ, we are able to safely delete / upgrade machines / nodes.

In the future, it would be useful to integrate [awesomenix/drainsafe](https://github.com/awesomenix/drainsafe) or
something similar to handle scenarios when Azure will delete or migrate a VMSS instance. Two scenarios come to mind.

1. VMSS is configured to use [Spot instances](https://learn.microsoft.com/azure/virtual-machines/spot-vms) and
   Azure must evict an instance.
2. Azure must [perform maintenance on an instance](https://learn.microsoft.com/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-maintenance-notifications).

## Proposal

### User Stories

#### Story 1 - Upgrading the Kubernetes Version of a MachinePool
Alex is an engineer in a large organization which has a MachinePool running 1.18.x and would like to upgrade the
MachinePool 1.19.x. It is important to Alex that the MachinePool doesn't experience downtime during the upgrade. Alex
has set the MaxUnavailable and MaxSurge values on the AzureMachinePool to limit the number of machines that will be
unavailable during the upgrade, and the number of extra machines VMSS will add during upgrade. The MachinePool
upgrades each machine in the pool by first cordoning and draining, then replacing the machine in the pool.

#### Story 2 - Reducing the Number of Replicas in a MachinePool
Alex is an engineer in a large organization which has a MachinePool running. Alex has too many nodes running on the
cluster and would like to reduce the replicas. It is important to Alex that the MachinePool doesn't experience downtime.
Alex decreases the replica count of the MachinePool by 2. The MachinePool deletes 2 machines from the pool by first
cordoning and draining, then deleting the underlying infrastructure.

#### Story 3 - Deleting an individual Azure Machine Pool Machine
Alex is an engineer in a large organization which has a MachinePool running with 5 replicas. Alex would like to delete a
specific MachinePool machine. It is important to Alex that the MachinePool doesn't experience downtime while deleting
the individual machine. Alex uses `kubectl` to delete the specific MachinePool machine resource. The MachinePool machine
is cordoned and drained, then the underlying infrastructure is deleted. The MachinePool still has a replica count of 5,
but only has 4 running replicas. The MachinePool creates a new machine to take the place of the deleted instance.


### Requirements

#### Functional

<a name="FR1">FR1.</a> CAPZ MUST support deleting an individual Virtual Machine Scale Set instance.

<a name="FR2">FR2.</a> CAPZ SHOULD support cordon and draining workload from a Virtual Machine Scale Set instance.

<a name="FR3">FR3.</a> CAPZ SHOULD support updating an instance in-place using Virtual Machine Scale Set Update API

#### Non-Functional

<a name="NFR1">NFR1.</a> CAPZ SHOULD provide resource status updates as the Azure resources are provisioned

<a name="NFR2">NFR2.</a> CAPZ SHOULD not overwhelm Azure API request limits and should rate limit reconciliation cycles

<a name="NFR3">NFR3.</a> Unit tests MUST exist for upgrade and delete instance selection

<a name="NFR4">NFR4.</a> e2e tests MUST exist for MachinePool upgrade, scale up / down, and instance delete scenarios

### Implementation Details/Notes/Constraints

The current implementation of CAPZ AzureMachinePool embeds the state of each of the instances in the Scale Set within
the status of the AzureMachinePool.

```go
// AzureMachinePoolStatus defines the observed state of AzureMachinePool
AzureMachinePoolStatus struct {

    /*
        Other fields omitted for brevity
    */

    // Instances is the VM instance status for each VM in the VMSS
    // +optional
    Instances []*AzureMachinePoolInstanceStatus `json:"instances,omitempty"`
}

// AzureMachinePoolInstanceStatus provides status information for each instance in the VMSS
AzureMachinePoolInstanceStatus struct {
    // Version defines the Kubernetes version for the VM Instance
    // +optional
    Version string `json:"version"`

    // ProvisioningState is the provisioning state of the Azure virtual machine instance.
    // +optional
    ProvisioningState *infrav1.VMState `json:"provisioningState"`

    // ProviderID is the provider identification of the VMSS Instance
    // +optional
    ProviderID string `json:"providerID"`

    // InstanceID is the identification of the Machine Instance within the VMSS
    // +optional
    InstanceID string `json:"instanceID"`

    // InstanceName is the name of the Machine Instance within the VMSS
    // +optional
    InstanceName string `json:"instanceName"`

    // LatestModelApplied indicates the instance is running the most up-to-date VMSS model. A VMSS model describes
    // the image version the VM is running. If the instance is not running the latest model, it means the instance
    // may not be running the version of Kubernetes the Machine Pool has specified and needs to be updated.
    LatestModelApplied bool `json:"latestModelApplied"`
}
```

#### Existing APIs for Clarity
These are included here to provide a description of the structures as they exist in CAPI and will be leveraged to
extend AzureMachinePool. There are no changes to these structures. They are simply for reference.

```go
// MachineDeploymentStrategy describes how to replace existing machines with new ones.
type MachineDeploymentStrategy struct {
    // Type of deployment. Currently the only supported strategy is
    // "RollingUpdate".
    // Default is RollingUpdate.
    // +optional
    Type MachineDeploymentStrategyType `json:"type,omitempty"`

    // Rolling update config params. Present only if
    // MachineDeploymentStrategyType = RollingUpdate.
    // +optional
    RollingUpdate *MachineRollingUpdateDeployment `json:"rollingUpdate,omitempty"`
}

// MachineRollingUpdateDeployment is used to control the desired behavior of rolling update.
type MachineRollingUpdateDeployment struct {
    // The maximum number of machines that can be unavailable during the update.
    // Value can be an absolute number (ex: 5) or a percentage of desired
    // machines (ex: 10%).
    // Absolute number is calculated from percentage by rounding down.
    // This can not be 0 if MaxSurge is 0.
    // Defaults to 0.
    // Example: when this is set to 30%, the old MachineSet can be scaled
    // down to 70% of desired machines immediately when the rolling update
    // starts. Once new machines are ready, old MachineSet can be scaled
    // down further, followed by scaling up the new MachineSet, ensuring
    // that the total number of machines available at all times
    // during the update is at least 70% of desired machines.
    // +optional
    MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

    // The maximum number of machines that can be scheduled above the
    // desired number of machines.
    // Value can be an absolute number (ex: 5) or a percentage of
    // desired machines (ex: 10%).
    // This can not be 0 if MaxUnavailable is 0.
    // Absolute number is calculated from percentage by rounding up.
    // Defaults to 1.
    // Example: when this is set to 30%, the new MachineSet can be scaled
    // up immediately when the rolling update starts, such that the total
    // number of old and new machines do not exceed 130% of desired
    // machines. Once old machines have been killed, new MachineSet can
    // be scaled up further, ensuring that total number of machines running
    // at any time during the update is at most 130% of desired machines.
    // +optional
    MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`

    // DeletePolicy defines the policy used by the MachineDeployment to identify nodes to delete when downscaling.
    // Valid values are "Random, "Newest", "Oldest"
    // When no value is supplied, the default DeletePolicy of MachineSet is used
    // +kubebuilder:validation:Enum=Random;Newest;Oldest
    // +optional
    DeletePolicy *string `json:"deletePolicy,omitempty"`
}
```

#### Proposed API Changes
The proposed changes below show the CAPZ AzureMachinePool and AzureMachinePoolMachine.

```go
const azureMachinePoolUpdateInstanceAnnotation = "azuremachinepool.infrastructure.cluster.x-k8s.io/updateInstance"

type AzureMachinePoolSpec struct {
    // The deployment strategy to use to replace existing machines with
    // new ones.
    // +optional
    Strategy MachineDeploymentStrategy `json:"strategy,omitempty"`

    // NodeDrainTimeout is the total amount of time that the controller will spend on draining a node.
    // The default value is 0, meaning that the node can be drained without any time limitations.
    // NOTE: NodeDrainTimeout is different from `kubectl drain --timeout`
    // +optional
    NodeDrainTimeout *metav1.Duration `json:"nodeDrainTimeout,omitempty"`
}

// AzureMachinePoolMachineSpec defines the desired state of AzureMachinePoolMachine
type AzureMachinePoolMachineSpec struct {
    // ProviderID is the identification ID of the Virtual Machine Scale Set
    ProviderID string `json:"providerID"`
}

// AzureMachinePoolMachineStatus defines the observed state of AzureMachinePoolMachine
type AzureMachinePoolMachineStatus struct {
    // NodeRef will point to the corresponding Node if it exists.
    // +optional
    NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

    // Version defines the Kubernetes version for the VM Instance
    // +optional
    Version string `json:"version"`

    // ProvisioningState is the provisioning state of the Azure virtual machine instance.
    // +optional
    ProvisioningState *infrav1.VMState `json:"provisioningState"`

    // InstanceID is the identification of the Machine Instance within the VMSS
    InstanceID string `json:"instanceID"`

    // InstanceName is the name of the Machine Instance within the VMSS
    // +optional
    InstanceName string `json:"instanceName"`

    // FailureReason will be set in the event that there is a terminal problem
    // reconciling the MachinePool machine and will contain a succinct value suitable
    // for machine interpretation.
    //
    // Any transient errors that occur during the reconciliation of MachinePools
    // can be added as events to the MachinePool object and/or logged in the
    // controller's output.
    // +optional
    FailureReason *string `json:"failureReason,omitempty"`

    // FailureMessage will be set in the event that there is a terminal problem
    // reconciling the MachinePool and will contain a more verbose string suitable
    // for logging and human consumption.
    //
    // Any transient errors that occur during the reconciliation of MachinePools
    // can be added as events to the MachinePool object and/or logged in the
    // controller's output.
    // +optional
    FailureMessage *string `json:"failureMessage,omitempty"`

    // Conditions defines current service state of the AzureMachinePool.
    // +optional
    Conditions clusterv1.Conditions `json:"conditions,omitempty"`

    // LongRunningOperationState saves the state for an Azure long running operations so it can be continued on the
    // next reconciliation loop.
    // +optional
    LongRunningOperationState *infrav1.Future `json:"longRunningOperationState,omitempty"`

    // LatestModelApplied indicates the instance is running the most up-to-date VMSS model. A VMSS model describes
    // the image version the VM is running. If the instance is not running the latest model, it means the instance
    // may not be running the version of Kubernetes the Machine Pool has specified and needs to be updated.
    LatestModelApplied bool `json:"latestModelApplied"`

    // Ready is true when the provider resource is ready.
    // +optional
    Ready bool `json:"ready"`
}
```

#### Proposed Controller Changes

* Create a new AzureMachinePoolMachine controller.
* Remove VMSS instance status tracking logic from AzureMachinePool controller and moving it to AzureMachinePoolMachine
  controller.
* Introduce rate limiting behavior to AzureMachinePool* controllers to ensure Azure API limits are not
  exceeded.

#### Proposed Changes of Responsibility
Currently in CAPZ, the AzureMachinePool controller is responsible for both the Virtual Machine Scale Set (VMSS) and the
instances created by the VMSS. The proposed change would separate the responsibility of managing the state of the VMSS
and the instances created by the VMSS. This would introduce a new AzureMachinePoolMachine controller and a new
MachinePoolMachineScope. The responsibilities would be as follows.

**AzureMachinePool Responsibilities:**
- Create AzureMachinePoolMachine instances when a new VMSS instance is observed. The AzureMachinePoolMachine spec should
  have the `ProviderID` field set with the observed resource ID. The AzureMachinePool should also be added to the
  AzureMachinePoolMachine's OwnerReferences.
- Selection of AzureMachinePoolMachine instances for deletion or upgrade. When a change to the AzureMachinePool model
  occurs, the `MachinePoolScope` will be responsible for coordinating the rollout of the updated model by selecting
  AzureMachinePoolMachines to delete or upgrade with respect to MaxUnavailable and the DeletePolicy.
- Scale up: AzureMachinePool should increase the number of VMSS replicas if the replica count increases on MachinePool
- Scale down: AzureMachinePool should select and delete AzureMachinePoolMachines that are overprovisioned with respect
  to MaxUnavailable and DeletePolicy from the proposed MachinePool Strategy.
- Upgrade: AzureMachinePool should select the AzureMachinePoolMachines to upgrade, set the
  `azureMachinePoolUpdateInstanceAnnotation` on the AzureMachinePoolMachine and wait for the annotation to be removed
  before proceeding with the rolling upgrade.
- Clean up. When a AzureMachinePoolMachine is no longer in the list of instances in Azure, but a matching
  AzureMachinePoolMachine resource exists, delete the AzureMachinePoolMachine.

**AzureMachinePoolMachine Responsibilities:**
- Update Azure Provisioning State: when creating a new VMSS instance, the AzureMachinePoolMachine controller will poll
  the Azure API until the instance reaches a terminal state.
- Cordon and Drain: when deleting or upgrading the AzureMachinePoolMachine resource, the AzureMachinePoolMachine
  controller is responsible for ensuring workload is moved from the node prior to removing the underlying Azure
  infrastructure.
- NodeRef: as a VMSS instance joins the cluster, the AzureMachinePoolMachine controller is responsible for ensuring
  the node is found and ready before marking the AzureMachinePoolMachine resource as ready.
- Upgrade: The AzureMachinePoolMachine is responsible for removing the `azureMachinePoolUpdateInstanceAnnotation` upon
  successful instance upgrade.

## Available Options

### Option 1: Add Annotations to AzureMachinePool for Instance Delete Selection
Create annotations on AzureMachinePool resources to indicate which machine should be upgraded next or deleted.

#### Option 1 Pros:
- No custom resource schema changes would be needed
- Would enable a user to provide input to the help the controller to decide the next machine to delete / upgrade

#### Option 1 Cons:
- Annotations don't have strong schema
- Controller would be dependent on the application of annotations to inform machine selection, which could be error
  prone and brittle.
- Each machine lifecycle will need to be embedded in the status of the AzureMachinePool to enable cordon and drain

### Option 2: Separate AzureMachinePool and AzureMachinePoolMachines
Introduce a new custom resource, AzureMachinePoolMachine, to represent AzureMachinePool instances rather than persisting
each instance status in the `AzureMachinePool.Status.Instances`

#### Option 2 Pros:
- Allows for easier tracking of state of individual AzureMachinePool instances via their own resource
- Each AzureMachinePoolMachine can be responsible for their own lifecycle, decomposing the logic in the controllers
- Would enable a user to interact with an AzureMachinePoolMachine the same way they would any other machine

#### Option 2 Cons:
- Breaking change to the status of the AzureMachinePool by removing the instances array

## Conclusions
Separate AzureMachinePool and AzureMachinePoolMachine resources provide a reasonable way to break down concerns and
offer the functionality to enable safe rolling upgrades and individual instance deletion.

## Additional Details

### Test Plan

* Unit tests to validate the proper selection of VMSS nodes to delete / upgrade
* Unit tests for the new MachinePoolMachineScope
* e2e tests for upgrade, scale down / up, and instance delete

## Implementation History

- 2021/02/22: Initial proposal
- 2021/01/06: Initial PR opened https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/1105
